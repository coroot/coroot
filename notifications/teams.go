package notifications

import (
	"context"
	"fmt"
	"strings"

	goteamsnotify "github.com/atc0005/go-teams-notify/v2"
	"github.com/atc0005/go-teams-notify/v2/adaptivecard"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
)

type Teams struct {
	client     *goteamsnotify.TeamsClient
	webhookUrl string
}

func NewTeams(webhookUrl string) *Teams {
	var client = goteamsnotify.NewTeamsClient()
	client.SkipWebhookURLValidationOnSend(true)
	return &Teams{
		webhookUrl: webhookUrl,
		client:     client,
	}
}

func (t *Teams) SendIncident(ctx context.Context, baseUrl string, n *db.IncidentNotification) error {
	var title string
	if n.Status == model.OK {
		title = fmt.Sprintf("**%s** incident resolved", n.ApplicationId.Name)
	} else {
		title = fmt.Sprintf("[%s] **%s** is not meeting its SLOs", strings.ToUpper(n.Status.String()), n.ApplicationId.Name)
	}
	text := ""
	if n.Details != nil {
		for _, r := range n.Details.Reports {
			text += fmt.Sprintf("* **%s** / %s: %s\n", r.Name, r.Check, r.Message)
		}
	}
	if text == "" {
		text = " "
	}
	card, err := adaptivecard.NewTextBlockCard(text, title, true)
	if err != nil {
		return err
	}
	action, err := adaptivecard.NewActionOpenURL(incidentUrl(baseUrl, n), "View incident")
	if err != nil {
		return err
	}
	err = card.AddAction(true, action)
	if err != nil {
		return err
	}
	msg, err := adaptivecard.NewMessageFromCard(card)
	if err != nil {
		return err
	}
	if err = t.client.SendWithContext(ctx, t.webhookUrl, msg); err != nil {
		return err
	}
	return nil
}

func (t *Teams) SendAlert(ctx context.Context, baseUrl string, n *db.AlertNotification) error {
	displayName := alertDisplayName(n)
	var title string
	if n.Status == model.OK {
		resolvedText := "resolved"
		if n.Details != nil && n.Details.ResolvedBy != "" {
			resolvedText = fmt.Sprintf("manually resolved by **%s**", n.Details.ResolvedBy)
		}
		if n.Details != nil && n.Details.Duration != "" {
			title = fmt.Sprintf("**%s** alert %s (duration: %s)", displayName, resolvedText, n.Details.Duration)
		} else {
			title = fmt.Sprintf("**%s** alert %s", displayName, resolvedText)
		}
	} else {
		title = fmt.Sprintf("[%s] **%s**: %s", strings.ToUpper(n.Status.String()), displayName, n.Details.Summary)
	}
	text := ""
	if n.Details != nil {
		if n.Details.RuleName != "" {
			text += fmt.Sprintf("**Alerting rule**: %s\n\n", n.Details.RuleName)
		}
		for _, d := range n.Details.Details {
			if d.Code {
				text += fmt.Sprintf("**%s**:\n```\n%s\n```\n\n", d.Name, d.Value)
			} else {
				text += fmt.Sprintf("**%s**: %s\n\n", d.Name, d.Value)
			}
		}
	}
	if text == "" {
		text = " "
	}
	card, err := adaptivecard.NewTextBlockCard(text, title, true)
	if err != nil {
		return err
	}
	action, err := adaptivecard.NewActionOpenURL(alertUrl(baseUrl, n), "View alert")
	if err != nil {
		return err
	}
	err = card.AddAction(true, action)
	if err != nil {
		return err
	}
	msg, err := adaptivecard.NewMessageFromCard(card)
	if err != nil {
		return err
	}
	if err = t.client.SendWithContext(ctx, t.webhookUrl, msg); err != nil {
		return err
	}
	return nil
}

func (t *Teams) SendDeployment(ctx context.Context, project *db.Project, ds model.ApplicationDeploymentStatus) error {
	d := ds.Deployment

	status := "Deployed"
	switch ds.State {
	case model.ApplicationDeploymentStateInProgress:
		return nil
	case model.ApplicationDeploymentStateStuck:
		status = "Stuck"
	case model.ApplicationDeploymentStateCancelled:
		status = "Cancelled"
	}

	title := fmt.Sprintf("Deployment of **%s** to **%s**", d.ApplicationId.Name, project.Name)

	text := fmt.Sprintf("**Status**: %s\n\n", status)
	text += fmt.Sprintf("**Version**: %s\n\n", d.Version())
	if ds.State == model.ApplicationDeploymentStateSummary {
		summary := ""
		if len(ds.Summary) > 0 {
			for _, s := range ds.Summary {
				summary += fmt.Sprintf("* %s %s\n", s.Emoji(), s.Message)
			}
		} else {
			summary = "No notable changes"
		}
		text += "**Summary:**\n\n"
		text += summary
	}

	card, err := adaptivecard.NewTextBlockCard(text, title, true)
	if err != nil {
		return err
	}
	action, err := adaptivecard.NewActionOpenURL(deploymentUrl(project.Settings.Integrations.BaseUrl, project.Id, d), "View deployment")
	if err != nil {
		return err
	}
	err = card.AddAction(true, action)
	if err != nil {
		return err
	}
	msg, err := adaptivecard.NewMessageFromCard(card)
	if err != nil {
		return err
	}
	if err = t.client.SendWithContext(ctx, t.webhookUrl, msg); err != nil {
		return err
	}

	return nil
}
