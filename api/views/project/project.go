package project

import (
	"github.com/coroot/coroot/cache"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

type Prometheus struct {
	Status model.Status `json:"status"`
	Error  string       `json:"error"`
	Cache  struct {
		LagMax timeseries.Duration `json:"lag_max"`
		LagAvg timeseries.Duration `json:"lag_avg"`
	} `json:"cache"`
}

type NodeAgent struct {
	Status model.Status `json:"status"`
	Nodes  int          `json:"nodes"`
}

type KubeStateMetrics struct {
	Status       model.Status `json:"status"`
	Applications int          `json:"applications"`
}

type Status struct {
	Prometheus       Prometheus        `json:"prometheus"`
	NodeAgent        NodeAgent         `json:"node_agent"`
	KubeStateMetrics *KubeStateMetrics `json:"kube_state_metrics"`
}

func RenderStatus(p *db.Project, cacheStatus *cache.Status, w *model.World) *Status {
	res := &Status{}

	if cacheStatus.Error != "" {
		res.Prometheus.Error = cacheStatus.Error
		res.Prometheus.Status = model.WARNING
	} else {
		res.Prometheus.Cache.LagMax = cacheStatus.LagMax
		res.Prometheus.Cache.LagAvg = cacheStatus.LagAvg
		switch {
		case w == nil:
			res.Prometheus.Status = model.WARNING
		case cacheStatus.LagMax > 5*p.Prometheus.RefreshInterval:
			res.Prometheus.Status = model.INFO
		default:
			res.Prometheus.Status = model.OK
		}
	}

	if w == nil {
		return res
	}

	is := w.IntegrationStatus
	if !is.NodeAgent.Installed {
		res.NodeAgent.Status = model.WARNING
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
		}
	}

	return res
}
