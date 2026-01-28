package alert

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

type AlertNotification struct {
	Type    string `json:"type"`
	Channel string `json:"channel,omitempty"`
}

type Alert struct {
	Id                 string                `json:"id"`
	Fingerprint        string                `json:"fingerprint"`
	RuleId             string                `json:"rule_id"`
	RuleName           string                `json:"rule_name"`
	ProjectId          string                `json:"project_id"`
	ApplicationId      model.ApplicationId   `json:"application_id"`
	Severity           model.Status          `json:"severity"`
	Summary            string                `json:"summary"`
	Details            []model.AlertDetail   `json:"details,omitempty"`
	OpenedAt           timeseries.Time       `json:"opened_at"`
	ResolvedAt         timeseries.Time       `json:"resolved_at"`
	ManuallyResolvedAt timeseries.Time       `json:"manually_resolved_at"`
	UpdatedAt          timeseries.Time       `json:"updated_at"`
	Suppressed         bool                  `json:"suppressed"`
	ResolvedBy         string                `json:"resolved_by,omitempty"`
	Report             model.AuditReportName `json:"report,omitempty"`
	Duration           timeseries.Duration   `json:"duration"`
	Notifications      []AlertNotification   `json:"notifications,omitempty"`
	Widgets            []*model.Widget       `json:"widgets,omitempty"`
	LogPatternHash     string                `json:"log_pattern_hash,omitempty"`
}

func Render(w *model.World, a *model.Alert, app *model.Application, rules []*model.AlertingRule, notifications []db.AlertNotification) Alert {
	rulesMap := make(map[string]string)
	var matchedRule *model.AlertingRule
	for _, r := range rules {
		rulesMap[string(r.Id)] = r.Name
		if string(r.Id) == a.RuleId {
			matchedRule = r
		}
	}
	res := renderAlert(w, a, rulesMap, notifications)
	if app != nil && matchedRule != nil {
		switch matchedRule.Source.Type {
		case model.AlertSourceTypeCheck:
			if matchedRule.Source.Check != nil {
				if ch, _ := app.GetCheckWithReport(matchedRule.Source.Check.CheckId); ch != nil {
					res.Widgets = ch.Widgets
				}
			}
		case model.AlertSourceTypeLogPatterns:
			if widgets, hash := logPatternWidgets(app, w, a, string(matchedRule.Id)); widgets != nil {
				res.Widgets = widgets
				res.LogPatternHash = hash
			}
		}
	}
	return res
}

func logPatternWidgets(app *model.Application, w *model.World, a *model.Alert, ruleId string) ([]*model.Widget, string) {
	appId := app.Id.String()
	for severity, msgs := range app.LogMessages {
		if msgs == nil {
			continue
		}
		for _, lp := range msgs.Patterns {
			if lp == nil || lp.SimilarPatternHashes == nil {
				continue
			}
			for _, hash := range lp.SimilarPatternHashes.Items() {
				if alertFingerprint(ruleId, appId, hash) != a.Fingerprint {
					continue
				}
				ch := model.NewChart(w.Ctx, "Messages").
					AddSeries(severity.String(), lp.Messages, severity.Color()).
					Column()
				return []*model.Widget{{Chart: ch, Width: "100%"}}, hash
			}
		}
	}
	return nil, ""
}

func alertFingerprint(ruleId, appId, patternHash string) string {
	h := sha256.New()
	h.Write([]byte(ruleId))
	h.Write([]byte(appId))
	h.Write([]byte("ph"))
	h.Write([]byte(patternHash))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

type AlertsListView struct {
	Alerts   []Alert `json:"alerts"`
	Total    int     `json:"total"`
	Firing   int     `json:"firing"`
	Resolved int     `json:"resolved"`
}

func RenderList(w *model.World, result *db.AlertsResult, rules []*model.AlertingRule, notifications map[string][]db.AlertNotification) *AlertsListView {
	rulesMap := make(map[string]string)
	for _, r := range rules {
		rulesMap[string(r.Id)] = r.Name
	}

	alerts := make([]Alert, 0, len(result.Alerts))
	for _, a := range result.Alerts {
		alerts = append(alerts, renderAlert(w, a, rulesMap, notifications[a.Id]))
	}
	return &AlertsListView{
		Alerts:   alerts,
		Total:    result.Total,
		Firing:   result.Firing,
		Resolved: result.Resolved,
	}
}

func renderAlert(w *model.World, a *model.Alert, rulesMap map[string]string, notifications []db.AlertNotification) Alert {
	to := timeseries.Now()
	if a.ResolvedAt > 0 {
		to = a.ResolvedAt
	}
	duration := to.Sub(a.OpenedAt)

	ruleName := rulesMap[a.RuleId]

	seen := map[string]bool{}
	var alertNotifications []AlertNotification
	for _, n := range notifications {
		key := string(n.Destination.IntegrationType) + ":" + n.Destination.SlackChannel
		if seen[key] {
			continue
		}
		seen[key] = true
		alertNotifications = append(alertNotifications, AlertNotification{
			Type:    string(n.Destination.IntegrationType),
			Channel: n.Destination.SlackChannel,
		})
	}

	return Alert{
		Id:                 a.Id,
		Fingerprint:        a.Fingerprint,
		RuleId:             a.RuleId,
		RuleName:           ruleName,
		ProjectId:          a.ProjectId,
		ApplicationId:      a.ApplicationId,
		Severity:           a.Severity,
		Summary:            a.Summary,
		Details:            a.Details,
		OpenedAt:           a.OpenedAt,
		ResolvedAt:         a.ResolvedAt,
		ManuallyResolvedAt: a.ManuallyResolvedAt,
		UpdatedAt:          a.UpdatedAt,
		Suppressed:         a.Suppressed,
		ResolvedBy:         a.ResolvedBy,
		Report:             a.Report,
		Duration:           duration,
		Notifications:      alertNotifications,
	}
}
