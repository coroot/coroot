package model

import (
	"github.com/coroot/coroot/timeseries"
	"golang.org/x/exp/maps"
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

	CustomApplications map[string]CustomApplication
	Categories         []ApplicationCategory
	CheckConfigs       CheckConfigs

	Nodes           []*Node
	Applications    map[ApplicationId]*Application
	appsByNsAndName map[nsAndName]*Application

	AWS AWS

	IntegrationStatus IntegrationStatus
}

type nsAndName struct {
	ns   string
	name string
}

func NewWorld(from, to timeseries.Time, step, rawStep timeseries.Duration) *World {
	return &World{
		Ctx:                timeseries.Context{From: from, To: to, Step: step, RawStep: rawStep},
		Applications:       map[ApplicationId]*Application{},
		AWS:                AWS{DiscoveryErrors: map[string]bool{}},
		CustomApplications: map[string]CustomApplication{},
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

func (w *World) GetOrCreateApplication(id ApplicationId, custom bool) *Application {
	app := w.GetApplication(id)
	if app == nil {
		app = NewApplication(id)
		app.Custom = custom
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

func (w *World) GetCorootComponents() []*Application {
	components := map[ApplicationId]*Application{}
	for _, app := range w.Applications {
		if !app.IsCorootComponent() {
			continue
		}
		components[app.Id] = app
		types := app.ApplicationTypes()
		if types[ApplicationTypeCorootCE] || types[ApplicationTypeCorootEE] {
			for _, i := range app.Instances {
				for _, u := range i.Upstreams { // prometheus and clickhouse
					if u.RemoteInstance != nil && u.RemoteInstance.Owner.Id.Kind != ApplicationKindExternalService {
						components[u.RemoteInstance.Owner.Id] = u.RemoteInstance.Owner
					}
				}
			}
		}
	}
	return maps.Values(components)
}
