package applications

import (
	"sort"
	"strings"

	"github.com/coroot/coroot/db"
)

type CustomApplicationsView struct {
	CustomApplications []CustomApplication `json:"custom_applications"`
}

type CustomApplication struct {
	Name             string `json:"name"`
	InstancePatterns string `json:"instance_patterns"`
}

func RenderCustomApplications(p *db.Project) *CustomApplicationsView {
	v := &CustomApplicationsView{}
	for name, app := range p.Settings.CustomApplications {
		v.CustomApplications = append(v.CustomApplications, CustomApplication{
			Name:             name,
			InstancePatterns: strings.Join(app.InstancePattens, " "),
		})
	}
	sort.Slice(v.CustomApplications, func(i, j int) bool {
		return v.CustomApplications[i].Name < v.CustomApplications[j].Name
	})
	return v
}
