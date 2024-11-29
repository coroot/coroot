package notifications

import (
	"context"
	"fmt"
	"time"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

const (
	sendTimeout   = 30 * time.Second
	retryInterval = time.Minute
	retryWindow   = timeseries.Hour
)

type NotificationClient interface {
	SendIncident(ctx context.Context, baseUrl string, n *db.IncidentNotification) error
}

func getClient(destination db.IntegrationType, integrations db.Integrations) NotificationClient {
	switch destination {
	case db.IntegrationTypeSlack:
		if cfg := integrations.Slack; cfg != nil && cfg.Incidents {
			return NewSlack(cfg.Token, cfg.DefaultChannel)
		}
	case db.IntegrationTypeTeams:
		if cfg := integrations.Teams; cfg != nil && cfg.Incidents {
			return NewTeams(cfg.WebhookUrl)
		}
	case db.IntegrationTypePagerduty:
		if cfg := integrations.Pagerduty; cfg != nil && cfg.Incidents {
			return NewPagerduty(cfg.IntegrationKey)
		}
	case db.IntegrationTypeOpsgenie:
		if cfg := integrations.Opsgenie; cfg != nil && cfg.Incidents {
			return NewOpsgenie(cfg.ApiKey, cfg.EUInstance)
		}
	case db.IntegrationTypeWebhook:
		if cfg := integrations.Webhook; cfg != nil && cfg.Incidents {
			return NewWebhook(cfg)
		}
	}
	return nil
}

func incidentDetails(app *model.Application, incident *model.ApplicationIncident) *db.IncidentNotificationDetails {
	var reports []db.IncidentNotificationDetailsReport
	if !incident.Resolved() {
		for _, r := range app.Reports {
			for _, ch := range r.Checks {
				if ch.Status < model.WARNING {
					continue
				}
				reports = append(reports, db.IncidentNotificationDetailsReport{Name: r.Name, Check: ch.Title, Message: ch.Message})
			}
		}
	} else {
		for _, r := range app.Reports {
			if r.Name != model.AuditReportSLO {
				continue
			}
			for _, ch := range r.Checks {
				reports = append(reports, db.IncidentNotificationDetailsReport{Name: r.Name, Check: ch.Title, Message: ch.Message})
			}
		}
	}
	if len(reports) == 0 {
		return nil
	}
	return &db.IncidentNotificationDetails{Reports: reports}
}

func incidentUrl(baseUrl string, n *db.IncidentNotification) string {
	return fmt.Sprintf("%s/p/%s/incidents?incident=%s", baseUrl, n.ProjectId, n.IncidentKey)
}

func deploymentUrl(baseUrl string, projectId db.ProjectId, d *model.ApplicationDeployment) string {
	return fmt.Sprintf("%s/p/%s/app/%s/Deployments#%s", baseUrl, projectId, d.ApplicationId.String(), d.Id())
}
