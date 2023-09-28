package model

import (
	"github.com/coroot/coroot/timeseries"
)

type IntegrationStatus struct {
	NodeAgent struct {
		Installed bool
	}
	KubeStateMetrics struct {
		Required  bool
		Installed bool
	}
}

type World struct {
	Ctx timeseries.Context

	CheckConfigs CheckConfigs

	Nodes        []*Node
	Applications []*Application

	IntegrationStatus IntegrationStatus
}

func NewWorld(from, to timeseries.Time, step timeseries.Duration) *World {
	return &World{
		Ctx: timeseries.Context{From: from, To: to, Step: step},
	}
}

func (w *World) GetApplication(id ApplicationId) *Application {
	for _, a := range w.Applications {
		if a.Id == id {
			return a
		}
	}
	return nil
}

func (w *World) GetOrCreateApplication(id ApplicationId) *Application {
	app := w.GetApplication(id)
	if app == nil {
		app = NewApplication(id)
		w.Applications = append(w.Applications, app)
	}
	return app
}

func (w *World) GetNode(name string) *Node {
	for _, n := range w.Nodes {
		if n.Name.Value() == name || n.K8sName.Value() == name {
			return n
		}
	}
	return nil
}
