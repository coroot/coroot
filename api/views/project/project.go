package project

import (
	"fmt"
	"github.com/coroot/coroot/cache"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/utils"
)

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

	res.Prometheus.Status = model.OK
	res.Prometheus.Message = "ok"
	switch {
	case p.Prometheus.Url == "":
		res.Prometheus.Status = model.WARNING
		res.Prometheus.Message = "Prometheus is not configured"
		res.Prometheus.Action = "configure"
	case cacheStatus.Error != "":
		res.Prometheus.Status = model.WARNING
		res.Prometheus.Message = "An error has been occurred while querying Prometheus"
		res.Prometheus.Error = cacheStatus.Error
		res.Prometheus.Action = "configure"
	case cacheStatus.LagMax > 5*p.Prometheus.RefreshInterval:
		msg := fmt.Sprintf("Prometheus cache is %s behind.", utils.FormatDuration(cacheStatus.LagAvg, 1))
		if w == nil {
			res.Prometheus.Status = model.WARNING
			msg += " Please wait until synchronization is complete."
		} else {
			res.Prometheus.Status = model.INFO
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
