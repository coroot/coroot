package notifications

import (
	"context"
	"fmt"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/slack-go/slack"
	"k8s.io/klog"
	"net/http"
	"time"
)

type Slack struct {
	project *db.Project
	channel string
	baseUrl string
	client  *slack.Client
}

func NewSlack(project *db.Project) *Slack {
	integrations := project.Settings.Integrations
	cfg := integrations.Slack
	if cfg == nil || !cfg.Enabled || cfg.Token == "" {
		return nil
	}
	return &Slack{
		project: project,
		channel: cfg.DefaultChannel,
		baseUrl: integrations.BaseUrl,
		client:  slack.New(cfg.Token, slack.OptionHTTPClient(&http.Client{Timeout: time.Minute})),
	}
}

func (s *Slack) SendIncident(app *model.Application, incident *db.Incident) bool {
	if s == nil {
		return false
	}
	appLink := fmt.Sprintf("<%s/p/%s/app/%s?incident=%s|*%s*>", s.baseUrl, s.project.Id, app.Id.String(), incident.Key, app.Id.Name)
	header, snippet, color, details := "", "", "", ""
	if incident.ResolvedAt.IsZero() {
		header = fmt.Sprintf("%s is not meeting its SLOs", appLink)
		snippet = fmt.Sprintf("%s is not meeting its SLOs", app.Id.Name)
		color = incident.Severity.Color()
		for _, r := range app.Reports {
			checks := ""
			for _, ch := range r.Checks {
				if ch.Status < model.WARNING {
					continue
				}
				checks += fmt.Sprintf("• %s: %s\n", ch.Title, ch.Message)
			}
			if checks != "" {
				details += fmt.Sprintf("*%s*:\n%s", r.Name, checks)
			}
		}
	} else {
		header = fmt.Sprintf("%s incident resolved", appLink)
		snippet = fmt.Sprintf("%s incident resolved", app.Id.Name)
		color = model.OK.Color()
		for _, r := range app.Reports {
			if r.Name != model.AuditReportSLO {
				continue
			}
			checks := ""
			for _, ch := range r.Checks {
				checks += fmt.Sprintf("• %s: %s\n", ch.Title, ch.Message)
			}
			if checks != "" {
				details += fmt.Sprintf("*%s*:\n%s", r.Name, checks)
			}
		}
	}
	text := slack.MsgOptionText(snippet, false)
	blocks := slack.MsgOptionBlocks(
		slack.NewSectionBlock(slack.NewTextBlockObject(slack.MarkdownType, header, false, false), nil, nil),
	)
	attachments := slack.MsgOptionAttachments(slack.Attachment{Color: color, Blocks: slack.Blocks{BlockSet: []slack.Block{
		slack.NewSectionBlock(slack.NewTextBlockObject(slack.MarkdownType, details, false, false), nil, nil),
	}}})
	_, _, err := s.client.PostMessage(s.channel, text, blocks, attachments, slack.MsgOptionDisableLinkUnfurl())
	if err != nil {
		klog.Warningln(err)
		return false
	}
	return true
}

func (s *Slack) SendDeployment(ds model.ApplicationDeploymentStatus) error {
	if s == nil {
		return nil
	}

	d := ds.Deployment
	ns := d.Notifications

	link := func(title string) string {
		return fmt.Sprintf("<%s/p/%s/app/%s/Deployments#%s|*%s*>", s.baseUrl, s.project.Id, d.ApplicationId.String(), d.Id(), title)
	}

	stateStr := "Deployed"
	switch ds.State {
	case model.ApplicationDeploymentStateInProgress:
		stateStr = "In-progress"
	case model.ApplicationDeploymentStateStuck:
		stateStr = "Stuck"
	case model.ApplicationDeploymentStateCancelled:
		stateStr = "Cancelled"
	}

	var summary *slack.SectionBlock
	if ds.State == model.ApplicationDeploymentStateSummary {
		items := "No notable changes"
		if len(ds.Summary) > 0 {
			items = ""
			for _, s := range ds.Summary {
				emoji := ":tada:"
				if !s.Ok {
					emoji = ":broken_heart:"
				}
				items += fmt.Sprintf("%s %s\n", emoji, s.Message)
			}
		}
		summary = section(text("*Summary*\n%s", items))
	}
	blocks := []slack.Block{
		section(text("Deployment of %s to *%s*", link(d.ApplicationId.Name), s.project.Name)),
		section(nil,
			text("*Status*\n%s", stateStr),
			text("*Version*\n%s", link(ds.Deployment.Version())),
		),
	}
	if summary != nil {
		blocks = append(blocks, summary)
	}
	blocks = append(blocks, slack.NewContextBlock("", text("<!date^%d^{date_short_pretty} at {time}|%s>", d.StartedAt, d.StartedAt.ToStandard().Format(time.RFC1123))))

	fallback := fmt.Sprintf("Deployment of %s to %s", d.ApplicationId.Name, s.project.Name)
	channel := s.channel
	options := []slack.MsgOption{body(ds.Status, fallback, blocks...)}
	if ns.Slack.Channel != "" && ns.Slack.ThreadTs != "" {
		channel = ns.Slack.Channel
		options = append(options, slack.MsgOptionUpdate(ns.Slack.ThreadTs))
	}
	ch, ts, _, err := s.client.SendMessage(channel, options...)
	if err != nil {
		return err
	}
	ns.Slack.Channel = ch
	ns.Slack.ThreadTs = ts

	if ds.State == model.ApplicationDeploymentStateSummary {
		stateStr = "Summary"
	}
	fallback += ": " + stateStr
	reply := body(ds.Status, fallback, section(text(ds.Message)))
	if summary != nil {
		reply = slack.MsgOptionBlocks(summary)
	}
	_, _, _, err = s.client.SendMessage(ch, slack.MsgOptionPost(), slack.MsgOptionTS(ts), reply)
	if err != nil {
		return err
	}

	return nil
}

func IsSlackChannelAvailable(ctx context.Context, token string, channel string) (bool, error) {
	client := slack.New(token, slack.OptionHTTPClient(&http.Client{Timeout: time.Minute}))
	params := &slack.GetConversationsParameters{
		ExcludeArchived: true,
		Limit:           200,
		Types:           []string{"public_channel"},
	}
	for {
		channels, nextCursor, err := client.GetConversationsContext(ctx, params)
		if err != nil {
			return false, err
		}
		for _, ch := range channels {
			if ch.Name == channel {
				return true, nil
			}
		}
		if nextCursor == "" {
			break
		}
		params.Cursor = nextCursor
	}
	return false, nil
}

func body(status model.Status, fallback string, blocks ...slack.Block) slack.MsgOption {
	return slack.MsgOptionAttachments(slack.Attachment{
		Color:    status.Color(),
		Blocks:   slack.Blocks{BlockSet: blocks},
		Fallback: fallback,
	})
}

func section(text *slack.TextBlockObject, fields ...*slack.TextBlockObject) *slack.SectionBlock {
	return slack.NewSectionBlock(text, fields, nil)
}

func text(format string, a ...any) *slack.TextBlockObject {
	return slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf(format, a...), false, false)
}
