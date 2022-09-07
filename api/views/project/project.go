package project

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

type Prometheus struct {
	Status model.Status        `json:"status"`
	Error  string              `json:"error"`
	Lag    timeseries.Duration `json:"lag"`
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

func RenderStatus(now timeseries.Time, cacheUpdateTime timeseries.Time, cacheError string, world *model.World) *Status {
	res := &Status{}

	if cacheError != "" {
		res.Prometheus.Error = cacheError
		res.Prometheus.Status = model.WARNING
	} else {
		res.Prometheus.Status = model.OK
		if !cacheUpdateTime.IsZero() {
			res.Prometheus.Lag = now.Sub(cacheUpdateTime)
			if world == nil {
				res.Prometheus.Status = model.WARNING
			}
		}
	}

	if world == nil {
		return res
	}

	is := world.IntegrationStatus
	if !is.NodeAgent.Installed {
		res.NodeAgent.Status = model.WARNING
	} else {
		res.NodeAgent.Status = model.OK
		res.NodeAgent.Nodes = len(world.Nodes)
	}
	if is.KubeStateMetrics.Required {
		res.KubeStateMetrics = &KubeStateMetrics{}
		if is.KubeStateMetrics.Installed {
			res.KubeStateMetrics.Status = model.OK
			res.KubeStateMetrics.Applications = len(world.Applications) // TODO: count k8s apps only,
		} else {
			res.KubeStateMetrics.Status = model.WARNING
		}
	}

	return res
}
