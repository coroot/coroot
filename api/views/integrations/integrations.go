package integrations

import (
	"context"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/notifications"
	"k8s.io/klog"
)

type View struct {
	BaseUrl string `json:"base_url"`
	Slack   *Slack `json:"slack,omitempty"`
}

type Slack struct {
	Channel   string `json:"channel"`
	Available bool   `json:"available"`
	Enabled   bool   `json:"enabled"`
}

func Render(ctx context.Context, p *db.Project) *View {
	integrations := p.Settings.Integrations
	v := &View{
		BaseUrl: integrations.BaseUrl,
	}
	if cfg := integrations.Slack; cfg != nil {
		v.Slack = &Slack{
			Channel: cfg.DefaultChannel,
			Enabled: cfg.Enabled,
		}
		ok, err := notifications.IsSlackChannelAvailable(ctx, cfg.Token, cfg.DefaultChannel)
		if err != nil {
			klog.Warningln(err)
		}
		v.Slack.Available = ok
	}
	return v
}
