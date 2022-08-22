package model

type World struct {
	Nodes        []*Node
	Applications []*Application
	Services     []*Service
}

func (w *World) GetApplication(id ApplicationId) *Application {
	for _, a := range w.Applications {
		if a.ApplicationId == id {
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
