package integrations

import (
	"github.com/coroot/coroot/db"
)

type View struct {
	BaseUrl      string        `json:"base_url"`
	Integrations []Integration `json:"integrations"`
}

type Integration struct {
	Type        db.IntegrationType `json:"type"`
	Title       string             `json:"title"`
	Configured  bool               `json:"configured"`
	Incidents   bool               `json:"incidents"`
	Deployments bool               `json:"deployments"`
	Details     string             `json:"details"`
}

func Render(p *db.Project) *View {
	integrations := p.Settings.Integrations
	v := &View{
		BaseUrl: integrations.BaseUrl,
	}

	for _, i := range integrations.GetInfo() {
		v.Integrations = append(v.Integrations, Integration{
			Type:        i.Type,
			Title:       i.Title,
			Configured:  i.Configured,
			Incidents:   i.Incidents,
			Deployments: i.Deployments,
			Details:     i.Details,
		})
	}

	return v
}
