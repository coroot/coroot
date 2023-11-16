package overview

import (
	"github.com/coroot/coroot/auditor"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
)

type Overview struct {
	Health      []*ApplicationStatus `json:"health"`
	Map         []*Application       `json:"map"`
	Nodes       *model.Table         `json:"nodes"`
	Deployments []*Deployment        `json:"deployments"`
	Costs       *Costs               `json:"costs"`
}

func Render(w *model.World, p *db.Project, view string) *Overview {
	v := &Overview{}

	for _, n := range w.Nodes {
		if n.Price != nil {
			v.Costs = &Costs{}
			break
		}
	}

	switch view {
	case "health":
		auditor.Audit(w, p, nil)
		v.Health = renderHealth(w)
	case "map":
		auditor.Audit(w, p, nil)
		v.Map = renderServiceMap(w)
	case "nodes":
		v.Nodes = renderNodes(w)
	case "deployments":
		v.Deployments = renderDeployments(w)
	case "costs":
		v.Costs = renderCosts(w)
	}
	return v
}
