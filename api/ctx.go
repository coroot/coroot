package api

import (
	"fmt"

	"github.com/coroot/coroot/cache"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/utils"
)

type DataWithContext struct {
	Context Context `json:"context"`
	Data    any     `json:"data"`
}

type Context struct {
	Status Status `json:"status"`
	Search Search `json:"search"`
}

type Status struct {
	Status           model.Status      `json:"status"`
	Error            string            `json:"error"`
	Prometheus       Prometheus        `json:"prometheus"`
	NodeAgent        NodeAgent         `json:"node_agent"`
	KubeStateMetrics *KubeStateMetrics `json:"kube_state_metrics"`
}

type Prometheus struct {
	Status  model.Status `json:"status"`
	Message string       `json:"message"`
	Error   string       `json:"error"`
	Action  string       `json:"action"`
}

type NodeAgent struct {
	Status model.Status `json:"status"`
	Nodes  int          `json:"nodes"`
}

type KubeStateMetrics struct {
	Status       model.Status `json:"status"`
	Applications int          `json:"applications"`
}

type Search struct {
	Applications []Application `json:"applications"`
	Nodes        []Node        `json:"nodes"`
}

type Application struct {
	Id     model.ApplicationId `json:"id"`
	Status model.Status        `json:"status"`
}

type Node struct {
	Name   string       `json:"name"`
	Status model.Status `json:"status"`
}

func (api *Api) WithContext(p *db.Project, cacheStatus *cache.Status, w *model.World, data any) DataWithContext {
	return DataWithContext{
		Context: Context{
			Status: renderStatus(p, cacheStatus, w, api.globalPrometheus),
			Search: renderSearch(w),
		},
		Data: data,
	}
}

func renderStatus(p *db.Project, cacheStatus *cache.Status, w *model.World, globalPrometheus *db.IntegrationPrometheus) Status {
	res := Status{
		Status: model.OK,
	}

	if p == nil {
		res.Status = model.WARNING
		res.Error = "Project not found"
		return res
	}

	res.Prometheus.Status = model.OK
	res.Prometheus.Message = "ok"
	promCfg := p.PrometheusConfig(globalPrometheus)
	refreshInterval := promCfg.RefreshInterval
	if refreshInterval < cache.MinRefreshInterval {
		refreshInterval = cache.MinRefreshInterval
	}
	switch {
	case promCfg.Url == "":
		res.Prometheus.Status = model.WARNING
		res.Prometheus.Message = "Prometheus is not configured"
		res.Prometheus.Action = "configure"
	case cacheStatus != nil && cacheStatus.Error != "":
		res.Prometheus.Status = model.WARNING
		res.Prometheus.Message = "An error has been occurred while querying Prometheus"
		res.Prometheus.Error = cacheStatus.Error
		res.Prometheus.Action = "configure"
	case cacheStatus != nil && cacheStatus.LagMax > 5*refreshInterval:
		msg := fmt.Sprintf("Prometheus cache is %s behind (this could be expected after a restart/upgrade)", utils.FormatDuration(cacheStatus.LagAvg, 1))
		res.Prometheus.Status = model.WARNING
		if w == nil {
			msg += " Please wait until synchronization is complete."
		}
		res.Prometheus.Message = msg
		res.Prometheus.Action = "wait"
	}

	if res.Prometheus.Status >= model.WARNING {
		res.Status = model.WARNING
	}

	if w == nil {
		return res
	}

	is := w.IntegrationStatus
	if !is.NodeAgent.Installed {
		res.NodeAgent.Status = model.WARNING
		res.Status = model.WARNING
	} else {
		res.NodeAgent.Status = model.OK
		res.NodeAgent.Nodes = len(w.Nodes)
	}

	if is.KubeStateMetrics.Required {
		res.KubeStateMetrics = &KubeStateMetrics{}
		if is.KubeStateMetrics.Installed {
			res.KubeStateMetrics.Status = model.OK
			res.KubeStateMetrics.Applications = len(w.Applications) // TODO: count k8s apps only,
		} else {
			res.KubeStateMetrics.Status = model.WARNING
			res.Status = model.WARNING
		}
	}

	return res
}

func renderSearch(w *model.World) Search {
	search := Search{}
	if w == nil {
		return search
	}
	for _, app := range w.Applications {
		search.Applications = append(search.Applications, Application{
			Id:     app.Id,
			Status: app.Status,
		})
	}
	for _, node := range w.Nodes {
		search.Nodes = append(search.Nodes, Node{
			Name:   node.GetName(),
			Status: node.Status(),
		})
	}
	return search
}
