package watchers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/notifications"
	"github.com/coroot/coroot/prom"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/coroot/logparser"
	"k8s.io/klog"
)

type LogPatternEvaluation struct {
	ShouldAlert bool
	Explanation string
}

type LogPatternEvaluator interface {
	Evaluate(project *db.Project, app *model.Application, severity model.Severity, lp *model.LogPattern) (*LogPatternEvaluation, error)
	Enabled() bool
}

type Alerts struct {
	db                  *db.DB
	notifier            *notifications.AlertNotifier
	pendingAlerts       map[string]timeseries.Time
	initializedProjects map[string]bool
	logPatternEvaluator LogPatternEvaluator
	globalPrometheus    *db.IntegrationPrometheus
	globalClickHouse    *db.IntegrationClickhouse
}

func NewAlerts(database *db.DB, globalPrometheus *db.IntegrationPrometheus, globalClickHouse *db.IntegrationClickhouse, logPatternEvaluator LogPatternEvaluator) *Alerts {
	return &Alerts{
		db:                  database,
		notifier:            notifications.NewAlertNotifier(database),
		pendingAlerts:       make(map[string]timeseries.Time),
		initializedProjects: make(map[string]bool),
		logPatternEvaluator: logPatternEvaluator,
		globalPrometheus:    globalPrometheus,
		globalClickHouse:    globalClickHouse,
	}
}

func (w *Alerts) Check(project *db.Project, world *model.World, from, to timeseries.Time, step timeseries.Duration) {
	start := time.Now()

	rules, err := w.db.GetAlertingRules(project.Id)
	if err != nil {
		klog.Errorln("failed to get alerting rules:", err)
		return
	}
	now := timeseries.Now()
	var alertsEvaluated int

	projectKey := string(project.Id)
	if !w.initializedProjects[projectKey] {
		hasAlerts, err := w.db.HasAlerts(project.Id)
		if err != nil {
			klog.Errorln("failed to check alerts for project:", err)
			return
		}
		w.initializedProjects[projectKey] = hasAlerts
	}

	for _, rule := range rules {
		if rule.Enabled {
			switch rule.Source.Type {
			case model.AlertSourceTypeCheck:
				if rule.Source.Check == nil {
					continue
				}
				for _, app := range world.Applications {
					if !rule.Matches(app) {
						continue
					}
					check, report := app.GetCheckWithReport(rule.Source.Check.CheckId)
					if check == nil {
						continue
					}
					alertsEvaluated++
					w.evaluateCheckAlert(project, rule, app, check, report, now)
				}
			case model.AlertSourceTypeLogPatterns:
				if rule.Source.LogPattern == nil {
					continue
				}
				w.evaluateLogPatternAlerts(project, rule, world, now)
			case model.AlertSourceTypePromQL:
				if rule.Source.PromQL == nil || rule.Source.PromQL.Expression == "" {
					continue
				}
				w.evaluatePromQLAlerts(project, rule, from, to, step, now)
			}
		}
		w.resolveNonMatchingAlerts(project, rule, world, now)
	}

	w.cleanupPendingAlerts(now)

	klog.Infof("%s: evaluated %d alerts in %s", project.Id, alertsEvaluated, time.Since(start).Truncate(time.Millisecond))
}

func (w *Alerts) evaluateCheckAlert(project *db.Project, rule *model.AlertingRule, app *model.Application, check *model.Check, report model.AuditReportName, now timeseries.Time) {
	fingerprint := calcFingerprint(string(rule.Id), app.Id.String(), nil)

	isFiring := check.Status > model.OK

	existingAlert, err := w.db.GetActiveOrSuppressedAlertByFingerprint(project.Id, fingerprint)
	if err != nil {
		klog.Errorln("failed to get alert:", err)
		return
	}

	if existingAlert != nil && existingAlert.Suppressed {
		if isFiring {
			return
		} else {
			if err := w.db.ClearSuppression(project.Id, fingerprint); err != nil {
				klog.Errorln("failed to clear suppression:", err)
			}
			return
		}
	}

	if isFiring {
		severity := rule.Severity
		if severity == model.UNKNOWN {
			severity = model.WARNING
		}
		templateData := buildTemplateData(app, check)
		summary := check.Message
		description := renderTemplate(rule.Templates.Description, templateData)
		var details []model.AlertDetail
		if description != "" {
			details = []model.AlertDetail{{Name: "Description", Value: description}}
		}

		if existingAlert == nil {
			if rule.For > 0 {
				firstSeen, ok := w.pendingAlerts[fingerprint]
				if !ok {
					w.pendingAlerts[fingerprint] = now
					return
				}
				if now.Sub(firstSeen) < rule.For {
					return
				}
			}
			delete(w.pendingAlerts, fingerprint)
			category := model.ApplicationCategoryApplication
			if app != nil {
				category = app.Category
			}
			alert := &model.Alert{
				Id:                  utils.NanoId(12),
				Fingerprint:         fingerprint,
				RuleId:              string(rule.Id),
				ProjectId:           string(project.Id),
				ApplicationId:       app.Id,
				ApplicationCategory: category,
				Severity:            severity,
				Summary:             summary,
				Details:             details,
				Report:              report,
			}
			if err := w.db.CreateAlert(project.Id, alert); err != nil {
				klog.Errorln("failed to create alert:", err)
			} else {
				w.notifier.Enqueue(project, app, alert, rule, now)
			}
		} else {
			existingAlert.Severity = severity
			existingAlert.Summary = summary
			existingAlert.Details = details
			if err := w.db.UpdateAlert(project.Id, existingAlert); err != nil {
				klog.Errorln("failed to update alert:", err)
			}
		}
	} else {
		delete(w.pendingAlerts, fingerprint)
		if existingAlert != nil {
			if rule.KeepFiringFor > 0 {
				if now.Sub(existingAlert.UpdatedAt) < rule.KeepFiringFor {
					return
				}
			}
			existingAlert.ResolvedAt = now
			if err := w.db.ResolveAlert(project.Id, existingAlert.Id, now); err != nil {
				klog.Errorln("failed to resolve alert:", err)
			} else {
				w.notifier.Enqueue(project, app, existingAlert, rule, now)
			}
		}
	}
}

func (w *Alerts) evaluateLogPatternAlerts(project *db.Project, rule *model.AlertingRule, world *model.World, now timeseries.Time) {
	src := rule.Source.LogPattern

	severities := make(map[model.Severity]bool)
	for _, s := range src.Severities {
		severities[model.SeverityFromString(s)] = true
	}
	minCount := src.MinCount
	if minCount <= 0 {
		minCount = 10
	}
	maxAlertsPerApp := src.MaxAlertsPerApp
	if maxAlertsPerApp <= 0 {
		maxAlertsPerApp = 20
	}

	var matchingApps []*model.Application
	for _, app := range world.Applications {
		if rule.Matches(app) {
			matchingApps = append(matchingApps, app)
		}
	}

	if !w.initializedProjects[string(project.Id)] {
		if w.logPatternEvaluator == nil || !w.logPatternEvaluator.Enabled() {
			w.initLogPatternAlerts(project, rule, matchingApps, severities, minCount, now)
			return
		}
		w.initializedProjects[string(project.Id)] = true
	}

	latestAlerts, err := w.db.GetLatestAlertsByRule(project.Id, string(rule.Id))
	if err != nil {
		klog.Errorln("failed to get latest alerts:", err)
		return
	}

	alertByFingerprint := map[string]*model.Alert{}
	alertsByApp := map[string][]*model.Alert{}
	firingCountByApp := map[string]int{}
	for _, a := range latestAlerts {
		if alertByFingerprint[a.Fingerprint] != nil {
			continue
		}
		alertByFingerprint[a.Fingerprint] = a
		appId := a.ApplicationId.String()
		alertsByApp[appId] = append(alertsByApp[appId], a)
		if a.ManuallyResolvedAt == 0 && !a.Suppressed {
			firingCountByApp[appId]++
		}
	}

	severity := rule.Severity
	if severity == model.UNKNOWN {
		severity = model.WARNING
	}

	activeFingerprints := map[string]bool{}

	for _, app := range matchingApps {
		appId := app.Id.String()

		for sev, logMsgs := range app.LogMessages {
			if !severities[sev] || logMsgs == nil {
				continue
			}
			for hash, lp := range logMsgs.Patterns {
				if lp == nil || lp.Messages == nil || lp.Pattern == nil {
					continue
				}
				messageCount := lp.Messages.Reduce(timeseries.NanSum)
				if timeseries.IsNaN(messageCount) || messageCount < float32(minCount) {
					continue
				}

				fingerprint := calcFingerprint(string(rule.Id), appId, map[string]string{"ph": hash})
				activeFingerprints[fingerprint] = true

				if blockingAlert := w.findBlockingAlert(string(rule.Id), fingerprint, appId, lp, alertByFingerprint, alertsByApp); blockingAlert != nil {
					activeFingerprints[blockingAlert.Fingerprint] = true
					continue
				}

				if firingCountByApp[appId] >= maxAlertsPerApp {
					continue
				}

				var aiExplanation string
				var aiSuppressed bool
				if w.logPatternEvaluator != nil && src.EvaluateWithAI {
					eval, err := w.logPatternEvaluator.Evaluate(project, app, sev, lp)
					if err != nil {
						klog.Errorf("AI evaluation failed for %s/%s: %v", appId, hash, err)
						aiExplanation = fmt.Sprintf("AI evaluation failed: %s", err)
					} else {
						aiExplanation = eval.Explanation
						aiSuppressed = !eval.ShouldAlert
					}
				}

				summary := fmt.Sprintf("new %s in the logs (%d messages in the last %s)", sev.String(), int(math.Round(float64(messageCount))), world.Ctx.To.Sub(world.Ctx.From))

				var logDetails []model.AlertDetail
				if rule.Templates.Description != "" {
					logDetails = append(logDetails, model.AlertDetail{Name: "Description", Value: rule.Templates.Description})
				}
				if lp.Sample != "" {
					logDetails = append(logDetails, model.AlertDetail{Name: "Sample", Value: lp.Sample, Code: true})
				}
				if aiExplanation != "" {
					logDetails = append(logDetails, model.AlertDetail{Name: "AI analysis", Value: aiExplanation})
				}

				var resolvedBy string
				if aiSuppressed {
					resolvedBy = "AI"
				}
				alert := &model.Alert{
					Id:                  utils.NanoId(12),
					Fingerprint:         fingerprint,
					RuleId:              string(rule.Id),
					ProjectId:           string(project.Id),
					ApplicationId:       app.Id,
					ApplicationCategory: app.Category,
					Severity:            severity,
					Summary:             summary,
					Details:             logDetails,
					Report:              model.AuditReportLogs,
					PatternWords:        lp.Pattern.String(),
					Suppressed:          aiSuppressed,
					ResolvedBy:          resolvedBy,
				}
				if err := w.db.CreateAlert(project.Id, alert); err != nil {
					klog.Errorln("failed to create log pattern alert:", err)
				} else {
					if !aiSuppressed {
						w.notifier.Enqueue(project, app, alert, rule, now)
					}
					firingCountByApp[appId]++
				}
			}
		}
	}

	for _, a := range alertByFingerprint {
		if activeFingerprints[a.Fingerprint] {
			continue
		}
		if a.Suppressed {
			continue
		}
		if rule.KeepFiringFor > 0 && now.Sub(a.UpdatedAt) < rule.KeepFiringFor {
			continue
		}
		a.ResolvedAt = now
		if err := w.db.ResolveAlert(project.Id, a.Id, now); err != nil {
			klog.Errorln("failed to resolve log pattern alert:", err)
		} else if a.ManuallyResolvedAt == 0 {
			app := world.GetApplication(a.ApplicationId)
			w.notifier.Enqueue(project, app, a, rule, now)
		}
	}
}

func (w *Alerts) evaluatePromQLAlerts(project *db.Project, rule *model.AlertingRule, from, to timeseries.Time, step timeseries.Duration, now timeseries.Time) {
	client, err := prom.NewClient(project.PrometheusConfig(w.globalPrometheus), project.ClickHouseConfig(w.globalClickHouse))
	if err != nil {
		klog.Errorf("failed to create prom client for PromQL rule %s: %v", rule.Id, err)
		return
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := client.QueryRange(ctx, rule.Source.PromQL.Expression, prom.FilterLabelsKeepAll, from, to, step)
	if err != nil {
		klog.Errorf("failed to evaluate PromQL rule %s: %v", rule.Id, err)
		return
	}

	severity := rule.Severity
	if severity == model.UNKNOWN {
		severity = model.WARNING
	}

	activeFingerprints := map[string]bool{}

	for _, mv := range results {
		value := mv.Values.Last()
		if timeseries.IsNaN(value) {
			continue
		}

		labels := make(map[string]string, len(mv.Labels))
		for k, v := range mv.Labels {
			labels[k] = v
		}
		fingerprint := calcFingerprint(string(rule.Id), "", labels)
		activeFingerprints[fingerprint] = true

		existingAlert, err := w.db.GetActiveOrSuppressedAlertByFingerprint(project.Id, fingerprint)
		if err != nil {
			klog.Errorln("failed to get alert:", err)
			continue
		}

		if existingAlert != nil && existingAlert.Suppressed {
			continue
		}

		templateData := buildPromQLTemplateData(mv.Labels, value)
		summary := renderTemplate(rule.Templates.Summary, templateData)
		if summary == "" {
			summary = fmt.Sprintf("PromQL alert is firing (value: %g)", value)
		}
		description := renderTemplate(rule.Templates.Description, templateData)
		var details []model.AlertDetail
		if description != "" {
			details = append(details, model.AlertDetail{Name: "Description", Value: description})
		}
		scopedQuery := rule.Source.PromQL.Expression
		if len(mv.Labels) > 0 {
			parts := make([]string, 0, len(mv.Labels))
			for k, v := range mv.Labels {
				parts = append(parts, fmt.Sprintf(`%s="%s"`, k, v))
			}
			sort.Strings(parts)
			details = append(details, model.AlertDetail{Name: "Labels", Value: strings.Join(parts, "\n"), Code: true})
			if q, err := prom.AddExtraSelector(rule.Source.PromQL.Expression, "{"+strings.Join(parts, ",")+"}"); err == nil {
				scopedQuery = q
			}
		}
		details = append(details, model.AlertDetail{Name: "PromQL", Value: rule.Source.PromQL.Expression, Code: true})
		details = append(details, model.AlertDetail{Name: "PromQLChart", Value: scopedQuery})

		if existingAlert == nil {
			if rule.For > 0 {
				firstSeen, ok := w.pendingAlerts[fingerprint]
				if !ok {
					w.pendingAlerts[fingerprint] = now
					continue
				}
				if now.Sub(firstSeen) < rule.For {
					continue
				}
			}
			delete(w.pendingAlerts, fingerprint)
			alert := &model.Alert{
				Id:            utils.NanoId(12),
				Fingerprint:   fingerprint,
				RuleId:        string(rule.Id),
				ProjectId:     string(project.Id),
				ApplicationId: model.ApplicationIdZero,
				Severity:      severity,
				Summary:       summary,
				Details:       details,
			}
			if err := w.db.CreateAlert(project.Id, alert); err != nil {
				klog.Errorln("failed to create PromQL alert:", err)
			} else {
				w.notifier.Enqueue(project, nil, alert, rule, now)
			}
		} else {
			existingAlert.Severity = severity
			existingAlert.Summary = summary
			existingAlert.Details = details
			if err := w.db.UpdateAlert(project.Id, existingAlert); err != nil {
				klog.Errorln("failed to update PromQL alert:", err)
			}
		}
	}

	alerts, err := w.db.GetLatestAlertsByRule(project.Id, string(rule.Id))
	if err != nil {
		klog.Errorln("failed to get alerts for PromQL rule:", err)
		return
	}
	for _, a := range alerts {
		if activeFingerprints[a.Fingerprint] {
			continue
		}
		if a.Suppressed {
			continue
		}
		delete(w.pendingAlerts, a.Fingerprint)
		if rule.KeepFiringFor > 0 && now.Sub(a.UpdatedAt) < rule.KeepFiringFor {
			continue
		}
		a.ResolvedAt = now
		if err := w.db.ResolveAlert(project.Id, a.Id, now); err != nil {
			klog.Errorln("failed to resolve PromQL alert:", err)
		} else if a.ManuallyResolvedAt == 0 {
			w.notifier.Enqueue(project, nil, a, rule, now)
		}
	}
}

func buildPromQLTemplateData(labels model.Labels, value float32) map[string]any {
	data := map[string]any{
		"value":  value,
		"labels": map[string]string(labels),
	}
	for k, v := range labels {
		data[k] = v
	}
	return data
}

func (w *Alerts) initLogPatternAlerts(project *db.Project, rule *model.AlertingRule, apps []*model.Application, severities map[model.Severity]bool, minCount int, now timeseries.Time) {
	severity := rule.Severity
	if severity == model.UNKNOWN {
		severity = model.WARNING
	}
	for _, app := range apps {
		appId := app.Id.String()
		for sev, logMsgs := range app.LogMessages {
			if !severities[sev] || logMsgs == nil {
				continue
			}
			for hash, lp := range logMsgs.Patterns {
				if lp == nil || lp.Messages == nil || lp.Pattern == nil {
					continue
				}
				messageCount := lp.Messages.Reduce(timeseries.NanSum)
				if timeseries.IsNaN(messageCount) || messageCount < float32(minCount) {
					continue
				}
				fingerprint := calcFingerprint(string(rule.Id), appId, map[string]string{"ph": hash})
				var initDetails []model.AlertDetail
				if rule.Templates.Description != "" {
					initDetails = append(initDetails, model.AlertDetail{Name: "Description", Value: rule.Templates.Description})
				}
				if lp.Sample != "" {
					initDetails = append(initDetails, model.AlertDetail{Name: "Sample", Value: lp.Sample, Code: true})
				}
				alert := &model.Alert{
					Id:                  utils.NanoId(12),
					Fingerprint:         fingerprint,
					RuleId:              string(rule.Id),
					ProjectId:           string(project.Id),
					ApplicationId:       app.Id,
					ApplicationCategory: app.Category,
					Severity:            severity,
					Summary:             fmt.Sprintf("new %s in the logs (%d messages)", sev.String(), int(math.Round(float64(messageCount)))),
					Details:             initDetails,
					Report:              model.AuditReportLogs,
					ManuallyResolvedAt:  now,
					ResolvedBy:          "init",
					PatternWords:        lp.Pattern.String(),
				}
				if err := w.db.CreateAlert(project.Id, alert); err != nil {
					klog.Errorln("failed to create init log pattern alert:", err)
				}
			}
		}
	}
}

func (w *Alerts) findBlockingAlert(ruleId, fingerprint, appId string, lp *model.LogPattern, alertByFingerprint map[string]*model.Alert, alertsByApp map[string][]*model.Alert) *model.Alert {
	if a := alertByFingerprint[fingerprint]; a != nil {
		return a
	}

	if lp.SimilarPatternHashes != nil {
		for _, sh := range lp.SimilarPatternHashes.Items() {
			fp := calcFingerprint(ruleId, appId, map[string]string{"ph": sh})
			if a := alertByFingerprint[fp]; a != nil {
				return a
			}
		}
	}

	if lp.Pattern != nil {
		for _, a := range alertsByApp[appId] {
			if a.PatternWords == "" {
				continue
			}
			if lp.Pattern.WeakEqual(logparser.NewPatternFromWords(a.PatternWords)) {
				return a
			}
		}
	}

	return nil
}

func (w *Alerts) resolveNonMatchingAlerts(project *db.Project, rule *model.AlertingRule, world *model.World, now timeseries.Time) {
	alerts, err := w.db.GetLatestAlertsByRule(project.Id, string(rule.Id))
	if err != nil {
		klog.Errorln("failed to get alerts for rule:", err)
		return
	}
	for _, a := range alerts {
		if rule.Enabled && rule.MatchesAlert(a) {
			continue
		}
		a.ResolvedAt = now
		if err := w.db.ResolveAlert(project.Id, a.Id, now); err != nil {
			klog.Errorln("failed to resolve non-matching alert:", err)
		} else if a.ManuallyResolvedAt == 0 {
			app := world.GetApplication(a.ApplicationId)
			w.notifier.Enqueue(project, app, a, rule, now)
		}
	}
}

func (w *Alerts) cleanupPendingAlerts(now timeseries.Time) {
	threshold := timeseries.DurationFromStandard(30 * time.Minute)
	for fingerprint, firstSeen := range w.pendingAlerts {
		if now.Sub(firstSeen) > threshold {
			delete(w.pendingAlerts, fingerprint)
		}
	}
}

func calcFingerprint(ruleId, appId string, extraLabels map[string]string) string {
	h := sha256.New()
	h.Write([]byte(ruleId))
	h.Write([]byte(appId))
	if len(extraLabels) > 0 {
		keys := make([]string, 0, len(extraLabels))
		for k := range extraLabels {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			h.Write([]byte(k))
			h.Write([]byte(extraLabels[k]))
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func buildTemplateData(app *model.Application, check *model.Check) map[string]any {
	return map[string]any{
		"app":           app.Id.Name,
		"namespace":     app.Id.Namespace,
		"check_title":   check.Title,
		"check_message": check.Message,
		"check_value":   check.Value(),
	}
}

func renderTemplate(tmpl string, labels map[string]any) string {
	if tmpl == "" {
		return ""
	}
	t, err := template.New("").Parse(tmpl)
	if err != nil {
		return tmpl
	}
	buf := &bytes.Buffer{}
	if err := t.Execute(buf, labels); err != nil {
		return tmpl
	}
	return buf.String()
}
