package overview

import (
	"context"

	"github.com/coroot/coroot/api/views/incident"
	"github.com/coroot/coroot/clickhouse"
	"github.com/coroot/coroot/model"
)

type Overview struct {
	Applications []*ApplicationStatus        `json:"applications"`
	Incidents    []incident.Summary          `json:"incidents"`
	Map          []*Application              `json:"map"`
	Nodes        *model.Table                `json:"nodes"`
	Deployments  []*Deployment               `json:"deployments"`
	Traces       *Traces                     `json:"traces"`
	Costs        *Costs                      `json:"costs"`
	Risks        []*Risk                     `json:"risks"`
	Categories   []model.ApplicationCategory `json:"categories"`
}

func Render(ctx context.Context, ch *clickhouse.Client, w *model.World, view, query string) *Overview {
	v := &Overview{
		Categories: w.Categories,
	}

	switch view {
	case "applications":
		v.Applications = renderApplications(w)
	case "incidents":
		for _, app := range w.Applications {
			switch {
			case app.IsK8s():
			case app.Id.Kind == model.ApplicationKindNomadJobGroup:
			case !app.IsStandalone():
			default:
				continue
			}
			for _, i := range app.Incidents {
				v.Incidents = append(v.Incidents, incident.CalcSummary(w, app, i))
			}
		}
	case "map":
		v.Map = renderServiceMap(w)
	case "nodes":
		v.Nodes = renderNodes(w)
	case "deployments":
		v.Deployments = renderDeployments(w)
	case "traces":
		v.Traces = renderTraces(ctx, ch, w, query)
	case "costs":
		v.Costs = renderCosts(w)
	case "risks":
		v.Risks = renderRisks(w)
	}
	return v
}
