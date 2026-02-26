package notifications

import (
	"context"
	"fmt"
	"time"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"k8s.io/klog"
)

type AlertNotifier struct {
	db *db.DB
}

func NewAlertNotifier(database *db.DB) *AlertNotifier {
	n := &AlertNotifier{db: database}
	go func() {
		for range time.Tick(retryInterval) {
			n.sendAlerts()
		}
	}()
	return n
}

func (n *AlertNotifier) Enqueue(project *db.Project, app *model.Application, alert *model.Alert, rule *model.AlertingRule, now timeseries.Time) {
	category := model.ApplicationCategoryApplication
	if app != nil {
		category = app.Category
	} else if rule.NotificationCategory != "" {
		category = rule.NotificationCategory
	}
	categorySettings := project.GetApplicationCategories()[category]
	if categorySettings == nil {
		return
	}
	notificationSettings := categorySettings.NotificationSettings.Alerts
	if !notificationSettings.Enabled {
		return
	}
	if slack := notificationSettings.Slack; slack != nil && slack.Enabled {
		n.enqueue(now, project, alert, rule, db.IncidentNotificationDestination{IntegrationType: db.IntegrationTypeSlack, SlackChannel: slack.Channel})
	}
	if teams := notificationSettings.Teams; teams != nil && teams.Enabled {
		n.enqueue(now, project, alert, rule, db.IncidentNotificationDestination{IntegrationType: db.IntegrationTypeTeams})
	}
	if pagerduty := notificationSettings.Pagerduty; pagerduty != nil && pagerduty.Enabled {
		n.enqueue(now, project, alert, rule, db.IncidentNotificationDestination{IntegrationType: db.IntegrationTypePagerduty})
	}
	if opsgenie := notificationSettings.Opsgenie; opsgenie != nil && opsgenie.Enabled {
		n.enqueue(now, project, alert, rule, db.IncidentNotificationDestination{IntegrationType: db.IntegrationTypeOpsgenie})
	}
	if webhook := notificationSettings.Webhook; webhook != nil && webhook.Enabled {
		n.enqueue(now, project, alert, rule, db.IncidentNotificationDestination{IntegrationType: db.IntegrationTypeWebhook})
	}
	n.sendAlerts()
}

func (n *AlertNotifier) sendAlerts() {
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
	notifications, err := n.db.GetNotSentAlertNotifications(timeseries.Now().Add(-retryWindow))
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
		client := getClient(notification.Destination, integrations, NotificationTypeAlert)
		if client != nil {
			if notification.Destination.IntegrationType == db.IntegrationTypeSlack {
				if prevNotifications, err := n.db.GetPreviousAlertNotifications(notification); err != nil {
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
			sendErr = client.SendAlert(ctx, integrations.BaseUrl, &notification)
			cancel()
		}
		if sendErr != nil {
			klog.Errorf("failed to send alert to %s: %s", notification.Destination.IntegrationType, sendErr)
			failedDestinations[dKey] = true
		} else {
			notification.SentAt = timeseries.Now()
			if err = n.db.UpdateAlertNotification(notification); err != nil {
				klog.Errorln(err)
			}
		}
	}
}

func (n *AlertNotifier) enqueue(now timeseries.Time, project *db.Project, alert *model.Alert, rule *model.AlertingRule, destination db.IncidentNotificationDestination) {
	notification := db.AlertNotification{
		ProjectId:     project.Id,
		AlertId:       alert.Id,
		RuleId:        alert.RuleId,
		ApplicationId: alert.ApplicationId,
		Destination:   destination,
		Timestamp:     now,
		Status:        alert.Severity,
	}
	details := &db.AlertNotificationDetails{
		ProjectName: project.Name,
		RuleName:    rule.Name,
		Severity:    alert.Severity.String(),
		Summary:     alert.Summary,
		Details:     filterAlertDetails(alert.Details),
	}
	if alert.ResolvedAt > 0 {
		details.Duration = utils.FormatDurationShort(alert.ResolvedAt.Sub(alert.OpenedAt), 2)
	}
	switch destination.IntegrationType {
	case db.IntegrationTypeSlack, db.IntegrationTypeTeams, db.IntegrationTypeWebhook:
		if alert.ResolvedAt > 0 {
			n.onResolve("", notification, details)
		} else {
			n.onOpen("", notification, details)
		}
	case db.IntegrationTypePagerduty, db.IntegrationTypeOpsgenie:
		openKey, err := n.getOpenAlert(notification)
		if err != nil {
			klog.Errorln(err)
			return
		}
		externalKey := fmt.Sprintf("%s:%s:%s", project.Id, alert.Id, alert.Severity.String())
		if alert.ResolvedAt > 0 {
			if openKey != "" {
				n.onResolve(openKey, notification, details)
			}
		} else {
			n.onOpen(externalKey, notification, details)
		}
	default:
		klog.Errorln("unknown destination:", destination)
	}
}

func (n *AlertNotifier) onOpen(externalKey string, notification db.AlertNotification, details *db.AlertNotificationDetails) {
	notification.ExternalKey = externalKey
	notification.Details = details
	n.db.PutAlertNotification(notification)
}

func (n *AlertNotifier) onResolve(externalKey string, notification db.AlertNotification, details *db.AlertNotificationDetails) {
	notification.Status = model.OK
	notification.ExternalKey = externalKey
	notification.Details = details
	n.db.PutAlertNotification(notification)
}

func (n *AlertNotifier) getOpenAlert(notification db.AlertNotification) (string, error) {
	prevNotifications, err := n.db.GetPreviousAlertNotifications(notification)
	if err != nil {
		return "", err
	}
	var openKey string
	for _, prev := range prevNotifications {
		if prev.Status == model.OK {
			if openKey == prev.ExternalKey {
				openKey = ""
			}
		} else {
			openKey = prev.ExternalKey
		}
	}
	return openKey, nil
}

func filterAlertDetails(details []model.AlertDetail) []model.AlertDetail {
	var filtered []model.AlertDetail
	for _, d := range details {
		if d.Name == "PromQL" || d.Name == "PromQLChart" {
			continue
		}
		filtered = append(filtered, d)
	}
	return filtered
}

func alertUrl(baseUrl string, n *db.AlertNotification) string {
	return fmt.Sprintf("%s/p/%s/alerts?alert=%s", baseUrl, n.ProjectId, n.AlertId)
}

func EnqueueResolvedAlerts(database *db.DB, project *db.Project, alerts []*model.Alert, rule *model.AlertingRule) {
	now := timeseries.Now()
	for _, alert := range alerts {
		alert.ResolvedAt = now
		category := alert.ApplicationCategory
		if category == "" {
			if rule.NotificationCategory != "" {
				category = rule.NotificationCategory
			} else {
				category = model.ApplicationCategoryApplication
			}
		}
		categorySettings := project.GetApplicationCategories()[category]
		if categorySettings == nil {
			continue
		}
		notificationSettings := categorySettings.NotificationSettings.Alerts
		if !notificationSettings.Enabled {
			continue
		}
		enqueueResolvedAlert(database, now, project, alert, rule, notificationSettings)
	}
}

func enqueueResolvedAlert(database *db.DB, now timeseries.Time, project *db.Project, alert *model.Alert, rule *model.AlertingRule, settings db.ApplicationCategoryAlertNotificationSettings) {
	details := &db.AlertNotificationDetails{
		ProjectName: project.Name,
		RuleName:    rule.Name,
		Severity:    alert.Severity.String(),
		Summary:     alert.Summary,
		Details:     filterAlertDetails(alert.Details),
		Duration:    utils.FormatDurationShort(now.Sub(alert.OpenedAt), 2),
		ResolvedBy:  alert.ResolvedBy,
	}

	if slack := settings.Slack; slack != nil && slack.Enabled {
		notification := db.AlertNotification{
			ProjectId:     project.Id,
			AlertId:       alert.Id,
			RuleId:        alert.RuleId,
			ApplicationId: alert.ApplicationId,
			Destination:   db.IncidentNotificationDestination{IntegrationType: db.IntegrationTypeSlack, SlackChannel: slack.Channel},
			Timestamp:     now,
			Status:        model.OK,
			Details:       details,
		}
		database.PutAlertNotification(notification)
	}
	if teams := settings.Teams; teams != nil && teams.Enabled {
		notification := db.AlertNotification{
			ProjectId:     project.Id,
			AlertId:       alert.Id,
			RuleId:        alert.RuleId,
			ApplicationId: alert.ApplicationId,
			Destination:   db.IncidentNotificationDestination{IntegrationType: db.IntegrationTypeTeams},
			Timestamp:     now,
			Status:        model.OK,
			Details:       details,
		}
		database.PutAlertNotification(notification)
	}
	if pagerduty := settings.Pagerduty; pagerduty != nil && pagerduty.Enabled {
		externalKey := fmt.Sprintf("%s:%s:%s", project.Id, alert.Id, alert.Severity.String())
		notification := db.AlertNotification{
			ProjectId:     project.Id,
			AlertId:       alert.Id,
			RuleId:        alert.RuleId,
			ApplicationId: alert.ApplicationId,
			Destination:   db.IncidentNotificationDestination{IntegrationType: db.IntegrationTypePagerduty},
			Timestamp:     now,
			Status:        model.OK,
			ExternalKey:   externalKey,
			Details:       details,
		}
		database.PutAlertNotification(notification)
	}
	if opsgenie := settings.Opsgenie; opsgenie != nil && opsgenie.Enabled {
		externalKey := fmt.Sprintf("%s:%s:%s", project.Id, alert.Id, alert.Severity.String())
		notification := db.AlertNotification{
			ProjectId:     project.Id,
			AlertId:       alert.Id,
			RuleId:        alert.RuleId,
			ApplicationId: alert.ApplicationId,
			Destination:   db.IncidentNotificationDestination{IntegrationType: db.IntegrationTypeOpsgenie},
			Timestamp:     now,
			Status:        model.OK,
			ExternalKey:   externalKey,
			Details:       details,
		}
		database.PutAlertNotification(notification)
	}
	if webhook := settings.Webhook; webhook != nil && webhook.Enabled {
		notification := db.AlertNotification{
			ProjectId:     project.Id,
			AlertId:       alert.Id,
			RuleId:        alert.RuleId,
			ApplicationId: alert.ApplicationId,
			Destination:   db.IncidentNotificationDestination{IntegrationType: db.IntegrationTypeWebhook},
			Timestamp:     now,
			Status:        model.OK,
			Details:       details,
		}
		database.PutAlertNotification(notification)
	}
}
