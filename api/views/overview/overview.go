package overview

import (
	"context"

	"github.com/coroot/coroot/clickhouse"
	"github.com/coroot/coroot/model"
)

type Overview struct {
	Health      []*ApplicationStatus        `json:"health"`
	Map         []*Application              `json:"map"`
	Nodes       *model.Table                `json:"nodes"`
	Deployments []*Deployment               `json:"deployments"`
	Traces      *Traces                     `json:"traces"`
	Costs       *Costs                      `json:"costs"`
	Categories  []model.ApplicationCategory `json:"categories"`
}

func Render(ctx context.Context, ch *clickhouse.Client, w *model.World, view, query string) *Overview {
	v := &Overview{
		Categories: w.Categories,
	}

	for _, n := range w.Nodes {
		if n.Price != nil {
			v.Costs = &Costs{}
			break
		}
	}

	switch view {
	case "health":
		v.Health = renderHealth(w)
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
	}
	return v
}
