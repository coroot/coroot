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
	Rum          *RumOverview                `json:"rum"`
	Logs         *Logs                       `json:"logs"`
	Costs        *Costs                      `json:"costs"`
	Risks        []*Risk                     `json:"risks"`
	FluxCD       []*FluxCDResource           `json:"fluxcd"`
	ArgoCD       []*ArgoCDResource           `json:"argocd"`
	Categories   []model.ApplicationCategory `json:"categories"`
}

type RumOverview struct {
	Status   model.Status `json:"status"`
	Message  string       `json:"message"`
	Services []RumService `json:"services"`
}

type RumService struct {
	Name          string              `json:"name"`
	AppId         model.ApplicationId `json:"app_id"`
	LastSeen      int64               `json:"last_seen,omitempty"`
	PageViews     uint64              `json:"page_views,omitempty"`
	ErrorCount    uint64              `json:"error_count,omitempty"`
	FetchErrorPct float32             `json:"fetch_error_pct,omitempty"`
	P75LcpMs      float64             `json:"p75_lcp_ms,omitempty"`
	P75LoadMs     float64             `json:"p75_load_ms,omitempty"`
}

func Render(ctx context.Context, chs clickhouse.Clients, project *db.Project, w *model.World, view, query string) *Overview {
	v := &Overview{}
	for name := range project.Settings.ApplicationCategorySettings {
		if !name.Default() {
			v.Categories = append(v.Categories, name)
		}
	}
	slices.Sort(v.Categories)

	switch view {
	case "applications":
		clickhouse.EnrichWorldWithRum(ctx, chs, w, project)
		v.Applications = renderApplications(w)
	case "map":
		clickhouse.EnrichWorldWithRum(ctx, chs, w, project)
		v.Map = renderServiceMap(w)
	case "nodes":
		v.Nodes = RenderNodes(w, project)
	case "deployments":
		v.Deployments = renderDeployments(w)
	case "traces":
		v.Traces = RenderTraces(ctx, chs, w, query)
	case "rum":
		clickhouse.EnrichWorldWithRum(ctx, chs, w, project)
		v.Rum = renderRumOverview(ctx, chs, w)
		v.Map = renderServiceMap(w)
	case "logs":
		v.Logs = renderLogs(ctx, chs, w, query)
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
