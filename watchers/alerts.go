package watchers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/coroot/coroot/clickhouse"
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

type KubernetesEventEvaluation struct {
	ShouldAlert bool
	Explanation string
}

type KubernetesEventEvaluator interface {
	Evaluate(project *db.Project, app *model.Application, event *model.LogEntry) (*KubernetesEventEvaluation, error)
	Enabled() bool
}

type Alerts struct {
	db                       *db.DB
	notifier                 *notifications.AlertNotifier
	pendingAlerts            map[string]timeseries.Time
	initializedProjects      map[string]bool
	logPatternEvaluator      LogPatternEvaluator
	kubernetesEventEvaluator KubernetesEventEvaluator
	globalPrometheus         *db.IntegrationPrometheus
	globalClickHouse         *db.IntegrationClickhouse
}

func NewAlerts(database *db.DB, globalPrometheus *db.IntegrationPrometheus, globalClickHouse *db.IntegrationClickhouse, logPatternEvaluator LogPatternEvaluator, kubernetesEventEvaluator KubernetesEventEvaluator) *Alerts {
	return &Alerts{
		db:                       database,
		notifier:                 notifications.NewAlertNotifier(database),
		pendingAlerts:            make(map[string]timeseries.Time),
		initializedProjects:      make(map[string]bool),
		logPatternEvaluator:      logPatternEvaluator,
		kubernetesEventEvaluator: kubernetesEventEvaluator,
		globalPrometheus:         globalPrometheus,
		globalClickHouse:         globalClickHouse,
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
			case model.AlertSourceTypeKubernetesEvents:
				if rule.Source.KubernetesEvents == nil {
					continue
				}
				w.evaluateKubernetesEventsAlerts(project, rule, world, from, to, now)
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
					if !blockingAlert.Suppressed && blockingAlert.ManuallyResolvedAt == 0 {
						blockingAlert.Summary = fmt.Sprintf("new %s in the logs (%d messages in the last %s)", sev.String(), int(math.Round(float64(messageCount))), world.Ctx.To.Sub(world.Ctx.From))
						var updatedDetails []model.AlertDetail
						if rule.Templates.Description != "" {
							updatedDetails = append(updatedDetails, model.AlertDetail{Name: "Description", Value: rule.Templates.Description})
						}
						if lp.Sample != "" {
							updatedDetails = append(updatedDetails, model.AlertDetail{Name: "Sample", Value: lp.Sample, Code: true})
						}
						// preserve existing AI analysis
						for _, d := range blockingAlert.Details {
							if d.Name == "AI analysis" {
								updatedDetails = append(updatedDetails, d)
								break
							}
						}
						blockingAlert.Details = updatedDetails
						if err := w.db.UpdateAlert(project.Id, blockingAlert); err != nil {
							klog.Errorln("failed to update log pattern alert:", err)
						}
					}
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

func (w *Alerts) evaluateKubernetesEventsAlerts(project *db.Project, rule *model.AlertingRule, world *model.World, from, to timeseries.Time, now timeseries.Time) {
	src := rule.Source.KubernetesEvents

	minCount := src.MinCount
	if minCount <= 0 {
		minCount = 1
	}
	maxAlertsPerApp := src.MaxAlertsPerApp
	if maxAlertsPerApp <= 0 {
		maxAlertsPerApp = 20
	}

	chClients := clickhouse.GetClients(w.db, project, w.globalClickHouse)
	if chClients.Error != nil {
		klog.Errorf("failed to get clickhouse clients for k8s events rule %s: %v", rule.Id, chClients.Error)
		return
	}
	defer chClients.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var events []*model.LogEntry
	for _, chClient := range chClients.Clients {
		evts, err := chClient.GetKubernetesEvents(ctx, from, to, 10000, clickhouse.LogFilter{Name: "Severity", Op: "!=", Value: model.SeverityInfo.String()})
		if err != nil {
			klog.Errorf("failed to get k8s events for rule %s: %v", rule.Id, err)
			continue
		}
		events = append(events, evts...)
	}

	type eventGroup struct {
		appId           model.ApplicationId
		app             *model.Application
		reason          string
		objKind         string
		objNs           string
		objName         string
		clusterId       string
		clusterName     string
		sourceComponent string
		nodeLevel       bool
		events          []*model.LogEntry
	}

	instanceByName := map[string]*model.Application{}
	appByComponentId := map[model.ApplicationId]*model.Application{}
	for _, a := range world.Applications {
		for _, inst := range a.Instances {
			if inst.Pod == nil {
				continue
			}
			instanceByName[inst.Name] = a
			if inst.ClusterComponent != nil {
				appByComponentId[inst.ClusterComponent.Id] = a
			}
		}
	}

	groups := map[string]*eventGroup{}
	for _, event := range events {
		ns := event.LogAttributes["object.namespace"]
		name := event.LogAttributes["object.name"]
		kind := event.LogAttributes["object.kind"]
		reason := event.LogAttributes["event.reason"]
		sourceComponent := event.LogAttributes["source.component"]

		// Node-level events (e.g., NodeNotReady from node-controller) are grouped
		// by cluster+reason instead of per-app to avoid alert storms when a node fails.
		if sourceComponent == "node-controller" {
			key := event.ClusterId + "|" + reason
			g := groups[key]
			if g == nil {
				g = &eventGroup{reason: reason, clusterId: event.ClusterId, clusterName: event.ClusterName, sourceComponent: sourceComponent, nodeLevel: true}
				groups[key] = g
			}
			g.events = append(g.events, event)
			continue
		}

		appId := model.NewApplicationId(event.ClusterId, ns, model.ApplicationKind(kind), name)
		app := world.GetApplication(appId)
		if app == nil {
			app = instanceByName[name]
		}
		if app == nil {
			app = appByComponentId[appId]
		}

		var groupAppId model.ApplicationId
		if app != nil {
			groupAppId = app.Id
		} else {
			groupAppId = appId
		}

		key := groupAppId.String() + "|" + reason
		g := groups[key]
		if g == nil {
			g = &eventGroup{appId: groupAppId, app: app, reason: reason, objKind: kind, objNs: ns, objName: name, clusterId: event.ClusterId, clusterName: event.ClusterName, sourceComponent: sourceComponent}
			groups[key] = g
		}
		g.events = append(g.events, event)
	}

	latestAlerts, err := w.db.GetLatestAlertsByRule(project.Id, string(rule.Id))
	if err != nil {
		klog.Errorln("failed to get latest alerts:", err)
		return
	}

	alertByFingerprint := map[string]*model.Alert{}
	firingCountByApp := map[string]int{}
	for _, a := range latestAlerts {
		if alertByFingerprint[a.Fingerprint] != nil {
			continue
		}
		alertByFingerprint[a.Fingerprint] = a
		appId := a.ApplicationId.String()
		if a.ManuallyResolvedAt == 0 && !a.Suppressed {
			firingCountByApp[appId]++
		}
	}

	severity := rule.Severity
	if severity == model.UNKNOWN {
		severity = model.WARNING
	}

	activeFingerprints := map[string]bool{}

	for _, g := range groups {
		if len(g.events) < minCount {
			continue
		}

		var fingerprintAppId string
		if !g.nodeLevel {
			fingerprintAppId = g.appId.String()
		}
		fingerprint := calcFingerprint(string(rule.Id), fingerprintAppId, map[string]string{"reason": g.reason})
		activeFingerprints[fingerprint] = true

		summary := fmt.Sprintf("%s (%d events in the last %s)", g.reason, len(g.events), to.Sub(from))

		var details []model.AlertDetail
		if rule.Templates.Description != "" {
			details = append(details, model.AlertDetail{Name: "Description", Value: rule.Templates.Description})
		}
		var labelParts []string
		if g.clusterName != "" {
			labelParts = append(labelParts, fmt.Sprintf(`cluster="%s"`, g.clusterName))
		} else if g.clusterId != "" {
			labelParts = append(labelParts, fmt.Sprintf(`cluster="%s"`, g.clusterId))
		}
		labelParts = append(labelParts, fmt.Sprintf(`reason="%s"`, g.reason))
		if g.sourceComponent != "" {
			labelParts = append(labelParts, fmt.Sprintf(`source="%s"`, g.sourceComponent))
		}
		var queryFilters []map[string]string
		if g.clusterName != "" {
			queryFilters = append(queryFilters, map[string]string{"name": "Cluster", "op": "=", "value": g.clusterName})
		}
		queryFilters = append(queryFilters, map[string]string{"name": "event.reason", "op": "=", "value": g.reason})

		if g.nodeLevel {
			queryFilters = append(queryFilters, map[string]string{"name": "source.component", "op": "=", "value": "node-controller"})
			objNames := map[string]bool{}
			for _, e := range g.events {
				if n := e.LogAttributes["object.name"]; n != "" {
					objNames[n] = true
				}
			}
			if len(objNames) > 0 {
				names := make([]string, 0, len(objNames))
				for n := range objNames {
					names = append(names, n)
				}
				sort.Strings(names)
				labelParts = append(labelParts, fmt.Sprintf(`affected="%s"`, strings.Join(names, ", ")))
			}
		} else if g.app == nil {
			if g.objKind != "" {
				labelParts = append(labelParts, fmt.Sprintf(`kind="%s"`, g.objKind))
				queryFilters = append(queryFilters, map[string]string{"name": "object.kind", "op": "=", "value": g.objKind})
			}
			if g.objNs != "" {
				labelParts = append(labelParts, fmt.Sprintf(`namespace="%s"`, g.objNs))
				queryFilters = append(queryFilters, map[string]string{"name": "object.namespace", "op": "=", "value": g.objNs})
			}
			labelParts = append(labelParts, fmt.Sprintf(`name="%s"`, g.objName))
			queryFilters = append(queryFilters, map[string]string{"name": "object.name", "op": "=", "value": g.objName})
		} else {
			objNames := map[string]bool{}
			for _, e := range g.events {
				if n := e.LogAttributes["object.name"]; n != "" {
					objNames[n] = true
				}
			}
			if len(objNames) == 1 {
				for n := range objNames {
					queryFilters = append(queryFilters, map[string]string{"name": "object.name", "op": "=", "value": n})
				}
			} else if len(objNames) > 1 {
				names := make([]string, 0, len(objNames))
				for n := range objNames {
					names = append(names, regexp.QuoteMeta(n))
				}
				sort.Strings(names)
				queryFilters = append(queryFilters, map[string]string{"name": "object.name", "op": "~", "value": "^(" + strings.Join(names, "|") + ")$"})
			}
		}
		sort.Strings(labelParts)
		if g.events[0].Body != "" {
			details = append(details, model.AlertDetail{Name: "Event message", Value: g.events[0].Body, Code: true})
		}
		details = append(details, model.AlertDetail{Name: "Labels", Value: strings.Join(labelParts, "\n"), Code: true})
		if queryJSON, err := json.Marshal(queryFilters); err == nil {
			details = append(details, model.AlertDetail{Name: "KubernetesEventsQuery", Value: string(queryJSON)})
		}

		existingAlert := alertByFingerprint[fingerprint]

		if existingAlert != nil {
			if existingAlert.Suppressed {
				continue
			}
			// preserve existing AI analysis
			for _, d := range existingAlert.Details {
				if d.Name == "AI analysis" {
					details = append(details, d)
					break
				}
			}
			existingAlert.Severity = severity
			existingAlert.Summary = summary
			existingAlert.Details = details
			if err := w.db.UpdateAlert(project.Id, existingAlert); err != nil {
				klog.Errorln("failed to update k8s event alert:", err)
			}
			continue
		}

		if !g.nodeLevel {
			appIdStr := g.appId.String()
			if firingCountByApp[appIdStr] >= maxAlertsPerApp {
				continue
			}
		}

		var aiExplanation string
		var aiSuppressed bool
		if w.kubernetesEventEvaluator != nil && src.EvaluateWithAI && g.app != nil {
			eval, err := w.kubernetesEventEvaluator.Evaluate(project, g.app, g.events[0])
			if err != nil {
				klog.Errorf("AI evaluation failed for k8s event %s/%s: %v", g.appId.String(), g.reason, err)
				aiExplanation = fmt.Sprintf("AI evaluation failed: %s", err)
			} else {
				aiExplanation = eval.Explanation
				aiSuppressed = !eval.ShouldAlert
			}
		}
		if aiExplanation != "" {
			details = append(details, model.AlertDetail{Name: "AI analysis", Value: aiExplanation})
		}

		var alertAppId model.ApplicationId
		var alertCategory model.ApplicationCategory
		if g.app != nil {
			alertAppId = g.app.Id
			alertCategory = g.app.Category
		} else {
			alertAppId = model.ApplicationIdZero
			alertCategory = model.ApplicationCategoryApplication
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
			ApplicationId:       alertAppId,
			ApplicationCategory: alertCategory,
			Severity:            severity,
			Summary:             summary,
			Details:             details,
			Suppressed:          aiSuppressed,
			ResolvedBy:          resolvedBy,
		}
		if err := w.db.CreateAlert(project.Id, alert); err != nil {
			klog.Errorln("failed to create k8s event alert:", err)
		} else {
			if !aiSuppressed {
				w.notifier.Enqueue(project, g.app, alert, rule, now)
			}
			if !g.nodeLevel {
				firingCountByApp[g.appId.String()]++
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
			klog.Errorln("failed to resolve k8s event alert:", err)
		} else if a.ManuallyResolvedAt == 0 {
			app := world.GetApplication(a.ApplicationId)
			w.notifier.Enqueue(project, app, a, rule, now)
		}
	}
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
