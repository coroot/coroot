package overview

import (
	"context"
	"slices"

	"github.com/coroot/coroot/clickhouse"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
)

type Overview struct {
	Applications []*ApplicationStatus        `json:"applications"`
	Map          []*Application              `json:"map"`
	Nodes        *model.Table                `json:"nodes"`
	Deployments  []*Deployment               `json:"deployments"`
	Traces       *Traces                     `json:"traces"`
	Logs         *Logs                       `json:"logs"`
	Costs        *Costs                      `json:"costs"`
	Risks        []*Risk                     `json:"risks"`
	Categories   []model.ApplicationCategory `json:"categories"`
}

func Render(ctx context.Context, ch *clickhouse.Client, project *db.Project, w *model.World, view, query string) *Overview {
	v := &Overview{}
	for name := range project.Settings.ApplicationCategorySettings {
		if !name.Default() {
			v.Categories = append(v.Categories, name)
		}
	}
	slices.Sort(v.Categories)

	switch view {
	case "applications":
		v.Applications = renderApplications(w)
	case "map":
		v.Map = renderServiceMap(w)
	case "nodes":
		v.Nodes = renderNodes(w)
	case "deployments":
		v.Deployments = renderDeployments(w)
	case "traces":
		v.Traces = renderTraces(ctx, ch, w, query)
	case "logs":
		v.Logs = renderLogs(ctx, ch, w, query)
	case "costs":
		v.Costs = renderCosts(w)
	case "risks":
		v.Risks = renderRisks(w)
	}
	return v
}
