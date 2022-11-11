package alerts

import (
	"context"
	"fmt"
	"github.com/coroot/coroot/model"
	"github.com/slack-go/slack"
	"net/http"
	"time"
)

type Slack struct {
	client *slack.Client
}

func NewSlack(token string) *Slack {
	return &Slack{client: slack.New(token, slack.OptionHTTPClient(&http.Client{Timeout: time.Minute}))}
}

func (s *Slack) IsChannelAvailable(ctx context.Context, channel string) (bool, error) {
	params := &slack.GetConversationsParameters{
		ExcludeArchived: true,
		Limit:           200,
		Types:           []string{"public_channel"},
	}
	for {
		channels, nextCursor, err := s.client.GetConversationsContext(ctx, params)
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

func (s *Slack) SendAlert(baseUrl, channel string, a Alert) error {
	appLink := fmt.Sprintf("<%s/p/%s/app/%s?incident=%s|*%s*>", baseUrl, a.ProjectId, a.ApplicationId.String(), a.Incident.Key, a.ApplicationId.Name)
	header, snippet, color, details := "", "", "", ""
	if a.Incident.ResolvedAt.IsZero() {
		header = fmt.Sprintf("%s is not meeting its SLOs", appLink)
		snippet = fmt.Sprintf("%s is not meeting its SLOs", a.ApplicationId.Name)
		if a.Incident.Severity == model.CRITICAL {
			color = "#f44034"
		} else {
			color = "#ffdd57"
		}
		for _, r := range a.Reports {
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
		snippet = fmt.Sprintf("%s incident resolved", a.ApplicationId.Name)
		color = "#23d160"
		for _, r := range a.Reports {
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
	_, _, err := s.client.PostMessage(channel, text, blocks, attachments, slack.MsgOptionDisableLinkUnfurl())
	return err
}
