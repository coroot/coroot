package notifications

import (
	"context"
	"fmt"
	"time"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
)

type IncidentNotifier struct {
	db *db.DB
}

func NewIncidentNotifier(db *db.DB) *IncidentNotifier {
	n := IncidentNotifier{db: db}
	go func() {
		for range time.Tick(retryInterval) {
			n.sendIncidents()
		}
	}()
	return &n
}

func (n *IncidentNotifier) Enqueue(project *db.Project, app *model.Application, incident *model.ApplicationIncident, now timeseries.Time) {
	categorySettings := project.GetApplicationCategories()[app.Category]
	if categorySettings == nil {
		return
	}
	notificationSettings := categorySettings.NotificationSettings.Incidents
	if !notificationSettings.Enabled {
		return
	}
	if slack := notificationSettings.Slack; slack != nil && slack.Enabled {
		n.enqueue(now, project, app, incident, db.IncidentNotificationDestination{IntegrationType: db.IntegrationTypeSlack, SlackChannel: slack.Channel})
	}
	if teams := notificationSettings.Teams; teams != nil && teams.Enabled {
		n.enqueue(now, project, app, incident, db.IncidentNotificationDestination{IntegrationType: db.IntegrationTypeTeams})
	}
	if pagerduty := notificationSettings.Pagerduty; pagerduty != nil && pagerduty.Enabled {
		n.enqueue(now, project, app, incident, db.IncidentNotificationDestination{IntegrationType: db.IntegrationTypePagerduty})
	}
	if opsgenie := notificationSettings.Opsgenie; opsgenie != nil && opsgenie.Enabled {
		n.enqueue(now, project, app, incident, db.IncidentNotificationDestination{IntegrationType: db.IntegrationTypeOpsgenie})
	}
	if webhook := notificationSettings.Webhook; webhook != nil && webhook.Enabled {
		n.enqueue(now, project, app, incident, db.IncidentNotificationDestination{IntegrationType: db.IntegrationTypeWebhook})
	}
	n.sendIncidents()
}

func (n *IncidentNotifier) sendIncidents() {
	ps, err := n.db.GetProjects()
	if err != nil {
		klog.Errorln(err)
		return
	}
	if len(ps) == 0 {
		return
	}
	projects := map[db.ProjectId]*db.Project{}
	for _, p := range ps {
		projects[p.Id] = p
	}

	type destinationKey struct {
		projectId   db.ProjectId
		destination db.IncidentNotificationDestination
	}
	failedDestinations := map[destinationKey]bool{}
	notifications, err := n.db.GetNotSentIncidentNotifications(timeseries.Now().Add(-retryWindow))
	if err != nil {
		klog.Errorln(err)
		return
	}
	for _, notification := range notifications {
		dKey := destinationKey{projectId: notification.ProjectId, destination: notification.Destination}
		if failedDestinations[dKey] {
			continue
		}
		project := projects[notification.ProjectId]
		if project == nil {
			continue
		}
		integrations := project.Settings.Integrations
		var sendErr error
		client := getClient(notification.Destination, integrations)
		if client != nil {
			if notification.Destination.IntegrationType == db.IntegrationTypeSlack {
				if prevNotifications, err := n.db.GetPreviousIncidentNotifications(notification); err != nil {
					klog.Errorln(err)
				} else {
					for _, pn := range prevNotifications {
						if pn.ExternalKey != "" {
							notification.ExternalKey = pn.ExternalKey
						}
					}
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
			sendErr = client.SendIncident(ctx, integrations.BaseUrl, &notification)
			cancel()
		}
		if sendErr != nil {
			klog.Errorf("failed to send to %s: %s", notification.Destination.IntegrationType, sendErr)
			failedDestinations[dKey] = true
		} else {
			notification.SentAt = timeseries.Now()
			if err = n.db.UpdateIncidentNotification(notification); err != nil {
				klog.Errorln(err)
			}
		}
	}
}

func (n *IncidentNotifier) enqueue(now timeseries.Time, project *db.Project, app *model.Application, incident *model.ApplicationIncident, destination db.IncidentNotificationDestination) {
	notification := db.IncidentNotification{
		ProjectId:     project.Id,
		ApplicationId: app.Id,
		IncidentKey:   incident.Key,
		Destination:   destination,
		Timestamp:     now,
		Status:        incident.Severity,
	}
	switch destination.IntegrationType {
	case db.IntegrationTypeSlack, db.IntegrationTypeTeams, db.IntegrationTypeWebhook:
		if incident.Resolved() {
			n.onResolve("", notification, incidentDetails(app, incident))
		} else {
			n.onOpen("", notification, incidentDetails(app, incident))
		}
	case db.IntegrationTypePagerduty, db.IntegrationTypeOpsgenie:
		openCriticalKey, openWarningKey, err := n.getOpenIncidents(notification)
		if err != nil {
			klog.Errorln(err)
			return
		}
		externalKey := fmt.Sprintf("%s:%s:%s", project.Id, incident.Key, incident.Severity.String())
		switch {
		case incident.Resolved():
			if openCriticalKey != "" {
				n.onResolve(openCriticalKey, notification, nil)
			}
			if openWarningKey != "" {
				n.onResolve(openWarningKey, notification, nil)
			}
		case incident.Severity == model.WARNING:
			if openCriticalKey != "" {
				n.onResolve(openCriticalKey, notification, nil)
			}
			n.onOpen(externalKey, notification, incidentDetails(app, incident))
		case incident.Severity == model.CRITICAL:
			n.onOpen(externalKey, notification, incidentDetails(app, incident))
		}
	default:
		klog.Errorln("unknown destination:", destination)
	}
}

func (n *IncidentNotifier) onOpen(externalKey string, notification db.IncidentNotification, details *db.IncidentNotificationDetails) {
	notification.ExternalKey = externalKey
	notification.Details = details
	n.db.PutIncidentNotification(notification)
}

func (n *IncidentNotifier) onResolve(externalKey string, notification db.IncidentNotification, details *db.IncidentNotificationDetails) {
	notification.Status = model.OK
	notification.ExternalKey = externalKey
	notification.Details = details
	n.db.PutIncidentNotification(notification)
}

func (n *IncidentNotifier) getOpenIncidents(notification db.IncidentNotification) (string, string, error) {
	prevNotifications, err := n.db.GetPreviousIncidentNotifications(notification)
	if err != nil {
		return "", "", err
	}
	var openCriticalKey, openWarningKey string
	for _, prev := range prevNotifications {
		switch prev.Status {
		case model.CRITICAL:
			openCriticalKey = prev.ExternalKey
		case model.WARNING:
			openWarningKey = prev.ExternalKey
		case model.OK:
			if openWarningKey == prev.ExternalKey {
				openWarningKey = ""
			} else if openCriticalKey == prev.ExternalKey {
				openCriticalKey = ""
			}
		default:
		}
	}
	return openCriticalKey, openWarningKey, nil
}
