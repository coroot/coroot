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

	CheckConfigs CheckConfigs

	Nodes        []*Node
	Applications map[ApplicationId]*Application

	Flux *Flux

	AWS AWS

	IntegrationStatus IntegrationStatus

	ProjectNamesById map[string]string
}

func NewWorld(from, to timeseries.Time, step, rawStep timeseries.Duration) *World {
	return &World{
		Ctx:              timeseries.Context{From: from, To: to, Step: step, RawStep: rawStep},
		Applications:     map[ApplicationId]*Application{},
		AWS:              AWS{DiscoveryErrors: map[string]bool{}},
		ProjectNamesById: map[string]string{},
	}
}

func (w *World) GetApplication(id ApplicationId) *Application {
	return w.Applications[id]
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
			for _, u := range app.Upstreams { // prometheus and clickhouse
				if a := u.RemoteApplication; a != nil && a.Id.Kind != ApplicationKindExternalService {
					components[a.Id] = a
				}
			}
		}
	}
	return maps.Values(components)
}

func (w *World) ClusterName(clusterId string) string {
	name := w.ProjectNamesById[clusterId]
	if name == "" {
		name = clusterId
	}
	return name
}
