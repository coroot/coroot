package notifications

import (
	"context"
	"fmt"
	"strings"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
)

type Pagerduty struct {
	integrationKey string
}

func NewPagerduty(integrationKey string) *Pagerduty {
	return &Pagerduty{integrationKey: integrationKey}
}

func (pd *Pagerduty) SendIncident(ctx context.Context, baseUrl string, n *db.IncidentNotification) error {
	e := pagerduty.V2Event{
		RoutingKey: pd.integrationKey,
		DedupKey:   n.ExternalKey,
	}
	if n.Status == model.OK {
		e.Action = "resolve"
	} else {
		e.Action = "trigger"
		e.Client = "Coroot"
		e.ClientURL = incidentUrl(baseUrl, n)
		e.Payload = &pagerduty.V2Payload{
			Summary:   fmt.Sprintf("[%s] %s is not meeting its SLOs", strings.ToUpper(n.Status.String()), n.ApplicationId.Name),
			Source:    "Coroot",
			Severity:  n.Status.String(),
			Timestamp: n.Timestamp.ToStandard().String(),
		}
		if n.Details != nil && len(n.Details.Reports) > 0 {
			details := map[string]string{}
			for _, r := range n.Details.Reports {
				details[fmt.Sprintf("%s / %s", r.Name, r.Check)] = r.Message
			}
			e.Payload.Details = details
		}
	}
	_, err := pagerduty.ManageEventWithContext(ctx, e)
	return err
}

func (pd *Pagerduty) SendAlert(ctx context.Context, baseUrl string, n *db.AlertNotification) error {
	e := pagerduty.V2Event{
		RoutingKey: pd.integrationKey,
		DedupKey:   n.ExternalKey,
	}
	if n.Status == model.OK {
		e.Action = "resolve"
	} else {
		e.Action = "trigger"
		e.Client = "Coroot"
		e.ClientURL = alertUrl(baseUrl, n)
		displayName := alertDisplayName(n)
		e.Payload = &pagerduty.V2Payload{
			Summary:   fmt.Sprintf("[%s] %s: %s", strings.ToUpper(n.Status.String()), displayName, n.Details.Summary),
			Source:    "Coroot",
			Severity:  n.Status.String(),
			Timestamp: n.Timestamp.ToStandard().String(),
		}
		if n.Details != nil {
			details := map[string]string{}
			if n.Details.ProjectName != "" {
				details["Project"] = n.Details.ProjectName
			}
			if n.Details.RuleName != "" {
				details["Alerting rule"] = n.Details.RuleName
			}
			for _, d := range n.Details.Details {
				details[d.Name] = d.Value
			}
			if len(details) > 0 {
				e.Payload.Details = details
			}
		}
	}
	_, err := pagerduty.ManageEventWithContext(ctx, e)
	return err
}

func (pd *Pagerduty) SendDeployment(ctx context.Context, project *db.Project, ds model.ApplicationDeploymentStatus) error {
	return fmt.Errorf("not supported")
}
