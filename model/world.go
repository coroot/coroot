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
	Services     []*Service

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

func (w *World) GetServiceForConnection(c *Connection) *Service {
	for _, s := range w.Services {
		if s.ClusterIP == c.ServiceRemoteIP {
			return s
		}
		for _, sc := range s.Connections {
			if sc.ActualRemoteIP == c.ActualRemoteIP {
				return s
			}
		}
	}
	return nil
}

func (w *World) FindInstanceByListen(ip, port string) *Instance {
	for _, l := range []Listen{{IP: ip, Port: port, Proxied: true}, {IP: ip, Port: port, Proxied: false}} {
		for _, app := range w.Applications {
			for _, i := range app.Instances {
				if _, ok := i.TcpListens[l]; ok {
					return i
				}
			}
		}
	}
	return nil
}

func (w *World) FindInstanceByPod(ns, pod string) *Instance {
	for _, app := range w.Applications {
		if app.Id.Namespace != ns {
			continue
		}
		for _, i := range app.Instances {
			if i.Pod != nil && i.Name == pod {
				return i
			}
		}
	}
	return nil
}

func (w *World) GetNode(name string) *Node {
	for _, n := range w.Nodes {
		if n.Name.Value() == name {
			return n
		}
	}
	return nil
}
