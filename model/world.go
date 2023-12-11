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

	Nodes           []*Node
	Applications    map[ApplicationId]*Application
	appsByNsAndName map[nsAndName]*Application

	IntegrationStatus IntegrationStatus
}

type nsAndName struct {
	ns   string
	name string
}

func NewWorld(from, to timeseries.Time, step timeseries.Duration) *World {
	return &World{
		Ctx:          timeseries.Context{From: from, To: to, Step: step},
		Applications: map[ApplicationId]*Application{},
	}
}

func (w *World) GetApplication(id ApplicationId) *Application {
	return w.Applications[id]
}

func (w *World) GetApplicationByNsAndName(ns, name string) *Application {
	if w.appsByNsAndName == nil {
		w.appsByNsAndName = map[nsAndName]*Application{}
		for id, app := range w.Applications {
			w.appsByNsAndName[nsAndName{ns: id.Namespace, name: id.Name}] = app
		}
	}
	return w.appsByNsAndName[nsAndName{ns: ns, name: name}]
}

func (w *World) GetOrCreateApplication(id ApplicationId) *Application {
	app := w.GetApplication(id)
	if app == nil {
		app = NewApplication(id)
		w.Applications[id] = app
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
