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
	integrations := project.Settings.Integrations
	for _, i := range integrations.GetInfo() {
		if i.Configured && i.Incidents {
			n.enqueue(project, app, incident, i.Type, now)
		}
	}
	n.sendIncidents()
}

type destinationKey struct {
	integration db.IntegrationType
	projectId   db.ProjectId
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
	failedDestinations := map[destinationKey]bool{}
	notifications, err := n.db.GetNotSentIncidentNotifications(timeseries.Now().Add(-retryWindow))
	if err != nil {
		klog.Errorln(err)
		return
	}
	for _, notification := range notifications {
		dKey := destinationKey{integration: notification.Destination, projectId: notification.ProjectId}
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
			if notification.Destination == db.IntegrationTypeSlack {
				if prevNotifications, err := n.db.GetPreviousIncidentNotifications(notification); err != nil {
					klog.Errorln(err)
				} else {
					for _, n := range prevNotifications {
						if n.ExternalKey != "" {
							notification.ExternalKey = n.ExternalKey
						}
					}
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
			sendErr = client.SendIncident(ctx, integrations.BaseUrl, &notification)
			cancel()
		}
		if sendErr != nil {
			klog.Errorf("send error %s: %s", notification.Destination, sendErr)
			failedDestinations[dKey] = true
		} else {
			notification.SentAt = timeseries.Now()
			if err := n.db.UpdateIncidentNotification(notification); err != nil {
				klog.Errorln(err)
			}
		}
	}
}

func (n *IncidentNotifier) enqueue(project *db.Project, app *model.Application, incident *model.ApplicationIncident, destination db.IntegrationType, now timeseries.Time) {
	notification := db.IncidentNotification{
		ProjectId:     project.Id,
		ApplicationId: app.Id,
		IncidentKey:   incident.Key,
		Destination:   destination,
		Timestamp:     now,
		Status:        incident.Severity,
	}
	switch destination {
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
	for _, n := range prevNotifications {
		switch n.Status {
		case model.CRITICAL:
			openCriticalKey = n.ExternalKey
		case model.WARNING:
			openWarningKey = n.ExternalKey
		case model.OK:
			if openWarningKey == n.ExternalKey {
				openWarningKey = ""
			} else if openCriticalKey == n.ExternalKey {
				openCriticalKey = ""
			}
		}
	}
	return openCriticalKey, openWarningKey, nil
}
