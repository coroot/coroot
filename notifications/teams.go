package notifications

import (
	"context"
	"fmt"
	"strings"

	goteamsnotify "github.com/atc0005/go-teams-notify/v2"
	"github.com/atc0005/go-teams-notify/v2/messagecard"
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

	msg := messagecard.NewMessageCard()
	msg.Summary = title
	msg.ThemeColor = n.Status.Color()
	msg.Text = "# " + title + "\n"
	if n.Details != nil {
		s := &messagecard.Section{}
		for _, r := range n.Details.Reports {
			s.Text += fmt.Sprintf("• **%s** / %s: %s<br>", r.Name, r.Check, r.Message)
		}
		_ = msg.AddSection(s)
	}
	action, _ := messagecard.NewPotentialAction(messagecard.PotentialActionOpenURIType, "View incident")
	action.Targets = []messagecard.PotentialActionOpenURITarget{{OS: "default", URI: incidentUrl(baseUrl, n)}}
	_ = msg.AddPotentialAction(action)
	if err := t.client.SendWithContext(ctx, t.webhookUrl, msg); err != nil {
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

	msg := messagecard.NewMessageCard()
	msg.Summary = title
	msg.ThemeColor = ds.Status.Color()
	_ = msg.AddSection(&messagecard.Section{
		Text: "# " + title,
		Facts: []messagecard.SectionFact{
			{Name: "Status", Value: status},
			{Name: "Version", Value: d.Version()},
		},
	})

	if ds.State == model.ApplicationDeploymentStateSummary {
		summary := "No notable changes"
		if len(ds.Summary) > 0 {
			summary = ""
			for _, s := range ds.Summary {
				summary += fmt.Sprintf("%s %s<br>", s.Emoji(), s.Message)
			}
		}
		_ = msg.AddSection(&messagecard.Section{Text: "**Summary**<br>" + summary})
	}

	action, _ := messagecard.NewPotentialAction(messagecard.PotentialActionOpenURIType, "View deployment")
	action.Targets = []messagecard.PotentialActionOpenURITarget{{OS: "default", URI: deploymentUrl(project.Settings.Integrations.BaseUrl, project.Id, d)}}
	_ = msg.AddPotentialAction(action)

	if err := t.client.SendWithContext(ctx, t.webhookUrl, msg); err != nil {
		return err
	}

	return nil
}
