package notifications

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/slack-go/slack"
)

type Slack struct {
	channel string
	client  *slack.Client
}

func NewSlack(token, channel string) *Slack {
	return &Slack{
		channel: channel,
		client:  slack.New(token),
	}
}

func (s *Slack) SendIncident(ctx context.Context, baseUrl string, n *db.IncidentNotification) error {
	var ch, ts string
	parts := strings.Split(n.ExternalKey, ":")
	if len(parts) == 2 {
		ch, ts = parts[0], parts[1]
	}
	if ch == "" {
		ch = s.channel
	}
	var header, snippet string
	if n.Status == model.OK {
		header = fmt.Sprintf("<%s|*%s* incident resolved>", incidentUrl(baseUrl, n), n.ApplicationId.Name)
		snippet = fmt.Sprintf("%s incident resolved", n.ApplicationId.Name)
	} else {
		header = fmt.Sprintf("[%s] <%s|*%s* is not meeting its SLOs>", strings.ToUpper(n.Status.String()), incidentUrl(baseUrl, n), n.ApplicationId.Name)
		snippet = fmt.Sprintf("%s is not meeting its SLOs", n.ApplicationId.Name)
	}
	var details []string
	if n.Details != nil {
		for _, r := range n.Details.Reports {
			details = append(details, fmt.Sprintf("• *%s* / %s: %s", r.Name, r.Check, r.Message))
		}
	}
	body := s.body(n.Status.Color(), snippet, s.section(s.text(header)), s.section(s.text(strings.Join(details, "\n"))))
	opts := []slack.MsgOption{body, slack.MsgOptionDisableLinkUnfurl()}
	if ts != "" {
		opts = append(opts, slack.MsgOptionTS(ts), slack.MsgOptionBroadcast())
	}
	var err error
	ch, ts, err = s.client.PostMessageContext(ctx, ch, opts...)
	if err != nil {
		return fmt.Errorf("slack error: %w", err)
	}
	n.ExternalKey = fmt.Sprintf("%s:%s", ch, ts)
	return nil
}

func (s *Slack) SendDeployment(ctx context.Context, project *db.Project, ds model.ApplicationDeploymentStatus) error {
	d := ds.Deployment

	status := "Deployed"
	switch ds.State {
	case model.ApplicationDeploymentStateInProgress:
		status = "In-progress"
	case model.ApplicationDeploymentStateStuck:
		status = "Stuck"
	case model.ApplicationDeploymentStateCancelled:
		status = "Cancelled"
	}

	var summary *slack.SectionBlock
	if ds.State == model.ApplicationDeploymentStateSummary {
		items := "No notable changes"
		if len(ds.Summary) > 0 {
			items = ""
			for _, s := range ds.Summary {
				items += fmt.Sprintf("%s %s\n", s.Emoji(), s.Message)
			}
		}
		summary = s.section(s.text("*Summary*\n%s", items))
	}
	url := deploymentUrl(project.Settings.Integrations.BaseUrl, project.Id, d)
	blocks := []slack.Block{
		s.section(s.text("Deployment of <%s|*%s*> to *%s*", url, d.ApplicationId.Name, project.Name)),
		s.section(nil,
			s.text("*Status*\n%s", status),
			s.text("*Version*\n<%s|*%s*>", url, d.Version()),
		),
	}
	if summary != nil {
		blocks = append(blocks, summary)
	}
	blocks = append(blocks, slack.NewContextBlock("", s.text("<!date^%d^{date_short_pretty} at {time}|%s>", d.StartedAt, d.StartedAt.ToStandard().Format(time.RFC1123))))

	snippet := fmt.Sprintf("Deployment of %s to %s", d.ApplicationId.Name, project.Name)
	channel := s.channel
	options := []slack.MsgOption{s.body(ds.Status.Color(), snippet, blocks...)}
	if d.Notifications.Slack.Channel != "" && d.Notifications.Slack.ThreadTs != "" {
		channel = d.Notifications.Slack.Channel
		options = append(options, slack.MsgOptionUpdate(d.Notifications.Slack.ThreadTs))
	}
	ch, ts, _, err := s.client.SendMessageContext(ctx, channel, options...)
	if err != nil {
		return fmt.Errorf("slack error: %w", err)
	}
	if err != nil {
		return err
	}
	d.Notifications.Slack.Channel = ch
	d.Notifications.Slack.ThreadTs = ts

	if ds.State == model.ApplicationDeploymentStateSummary {
		status = "Summary"
	}
	snippet += ": " + status
	reply := s.body(ds.Status.Color(), snippet, s.section(s.text(ds.Message)))
	if summary != nil {
		reply = slack.MsgOptionBlocks(summary)
	}
	_, _, _, err = s.client.SendMessageContext(ctx, ch, slack.MsgOptionPost(), slack.MsgOptionTS(ts), reply)
	if err != nil {
		return fmt.Errorf("slack error: %w", err)
	}

	return nil
}

func (s *Slack) body(color string, fallback string, blocks ...slack.Block) slack.MsgOption {
	return slack.MsgOptionAttachments(slack.Attachment{
		Color:    color,
		Blocks:   slack.Blocks{BlockSet: blocks},
		Fallback: fallback,
	})
}

func (s *Slack) section(text *slack.TextBlockObject, fields ...*slack.TextBlockObject) *slack.SectionBlock {
	return slack.NewSectionBlock(text, fields, nil)
}

func (s *Slack) text(format string, a ...any) *slack.TextBlockObject {
	return slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf(format, a...), false, false)
}
