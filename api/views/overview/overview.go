package overview

import (
	"github.com/coroot/coroot/model"
	"sort"
)

type Overview struct {
	Views       []View               `json:"views"`
	ServiceMap  []*Application       `json:"service_map"`
	Health      []*ApplicationStatus `json:"health"`
	Nodes       *model.Table         `json:"nodes"`
	Deployments []*Deployment        `json:"deployments"`
	Costs       *Costs               `json:"costs"`
}

type View struct {
	Name   string      `json:"name"`
	Badges []ViewBadge `json:"badges"`
}

type ViewBadge struct {
	Status model.Status `json:"status"`
	Value  int          `json:"value"`
}

func Render(w *model.World, view string) *Overview {
	health := renderHealth(w)

	v := &Overview{Views: []View{
		{Name: "service map"},
		{Name: "health", Badges: healthBadges(health)},
		{Name: "nodes"},
		{Name: "deployments"},
	}}
	for _, n := range w.Nodes {
		if n.Price != nil {
			v.Views = append(v.Views, View{Name: "costs"})
			break
		}
	}

	switch view {
	case "service map":
		v.ServiceMap = renderServiceMap(w)
	case "health":
		v.Health = health
	case "nodes":
		v.Nodes = renderNodes(w)
	case "deployments":
		v.Deployments = renderDeployments(w)
	case "costs":
		v.Costs = renderCosts(w)
	}
	return v
}

func healthBadges(health []*ApplicationStatus) []ViewBadge {
	byStatus := map[model.Status]int{}
	for _, a := range health {
		byStatus[a.Status]++
	}
	var badges []ViewBadge
	for status, count := range byStatus {
		badges = append(badges, ViewBadge{Status: status, Value: count})
	}
	sort.Slice(badges, func(i, j int) bool {
		return badges[i].Status > badges[j].Status
	})
	if len(badges) > 2 {
		badges = badges[:2]
	}
	return badges
}
