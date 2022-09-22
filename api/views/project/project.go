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

type ApplicationExporter struct {
	Status       model.Status                 `json:"status"`
	Muted        bool                         `json:"muted"`
	Applications map[model.ApplicationId]bool `json:"applications"`
}

type Status struct {
	Status           model.Status      `json:"status"`
	Error            string            `json:"error"`
	Prometheus       Prometheus        `json:"prometheus"`
	NodeAgent        NodeAgent         `json:"node_agent"`
	KubeStateMetrics *KubeStateMetrics `json:"kube_state_metrics"`

	ApplicationExporters map[model.ApplicationType]ApplicationExporter `json:"application_exporters"`
}

func RenderStatus(p *db.Project, cacheStatus *cache.Status, w *model.World) *Status {
	res := &Status{
		Status: model.OK,

		ApplicationExporters: map[model.ApplicationType]ApplicationExporter{},
	}

	if p == nil {
		res.Error = "Project not found"
		return res
	}

	if cacheStatus.Error != "" {
		res.Prometheus.Error = cacheStatus.Error
		res.Prometheus.Status = model.WARNING
		res.Status = model.WARNING
	} else {
		res.Prometheus.Cache.LagMax = cacheStatus.LagMax
		res.Prometheus.Cache.LagAvg = cacheStatus.LagAvg
		switch {
		case w == nil:
			res.Prometheus.Status = model.WARNING
			res.Status = model.WARNING
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
	for _, app := range w.Applications {
		for appType, ok := range app.InstrumentationStatus() {
			ex := res.ApplicationExporters[appType]
			if ex.Applications == nil {
				ex.Muted = p.Settings.ConfigurationHintsMuted[appType]
				ex.Status = model.OK
				ex.Applications = map[model.ApplicationId]bool{}
			}
			switch {
			case ex.Muted:
				ex.Status = model.UNKNOWN
			case !ok:
				ex.Status = model.WARNING
				if res.Status < model.INFO {
					res.Status = model.INFO
				}
			}
			ex.Applications[app.Id] = ok
			res.ApplicationExporters[appType] = ex
		}
	}

	return res
}
