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
	Nodes        []Node                      `json:"nodes"`
	Deployments  []*Deployment               `json:"deployments"`
	Traces       *Traces                     `json:"traces"`
	Logs         *Logs                       `json:"logs"`
	Costs        *Costs                      `json:"costs"`
	Risks        []*Risk                     `json:"risks"`
	FluxCD       []*FluxCDResource           `json:"fluxcd"`
	ArgoCD       []*ArgoCDResource           `json:"argocd"`
	Categories   []model.ApplicationCategory `json:"categories"`
}

// RenderOpts controls RBAC scoping for project-wide telemetry views.
type RenderOpts struct {
	// RestrictTelemetry scopes logs/traces ClickHouse queries to services of
	// apps present in w (already filtered to viewable apps).
	RestrictTelemetry bool
	// FullWorld is used for GuessService uniqueness when w is a restricted copy.
	FullWorld *model.World
}

func Render(ctx context.Context, chs clickhouse.Clients, project *db.Project, w *model.World, view, query string, opts RenderOpts) *Overview {
	v := &Overview{}
	for name := range project.Settings.ApplicationCategorySettings {
		if !name.Default() {
			v.Categories = append(v.Categories, name)
		}
	}
	slices.Sort(v.Categories)

	fullWorld := opts.FullWorld
	if fullWorld == nil {
		fullWorld = w
	}

	switch view {
	case "applications":
		v.Applications = renderApplications(w)
	case "map":
		v.Map = renderServiceMap(w)
	case "nodes":
		v.Nodes = RenderNodes(w, project)
	case "deployments":
		v.Deployments = renderDeployments(w)
	case "traces":
		v.Traces = RenderTraces(ctx, chs, w, query, opts.RestrictTelemetry, fullWorld)
	case "logs":
		v.Logs = renderLogs(ctx, chs, w, query, opts.RestrictTelemetry, fullWorld)
	case "costs":
		v.Costs = renderCosts(w)
	case "risks":
		v.Risks = renderRisks(w)
	case "fluxcd":
		v.FluxCD = renderFluxCD(w)
	case "argocd":
		v.ArgoCD = renderArgoCD(w)
	}
	return v
}
