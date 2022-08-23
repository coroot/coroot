package view

import (
	"github.com/coroot/coroot-focus/model"
	"github.com/coroot/coroot-focus/timeseries"
	"sort"
)

type Application struct {
	Id     model.ApplicationId `json:"id"`
	Labels model.Labels        `json:"labels"`
}

type ApplicationLink struct {
	Id        model.ApplicationId `json:"id"`
	Status    model.Status        `json:"status"`
	Direction string              `json:"direction"`
}

type AppView struct {
	Application *Application `json:"application"`
	Instances   []*Instance  `json:"instances"`

	Clients      []*Application `json:"clients"`
	Dependencies []*Application `json:"dependencies"`

	Dashboards []*Dashboard `json:"dashboards"`
}

func (v *AppView) addDashboard(ctx timeseries.Context, d *Dashboard) {
	if len(d.Widgets) == 0 {
		return
	}
	for _, w := range d.Widgets {
		if w.Chart != nil {
			w.Chart.Ctx = ctx
		}
		if w.ChartGroup != nil {
			for _, ch := range w.ChartGroup.Charts {
				ch.Ctx = ctx
			}
			autoFeatureChartInGroup(w.ChartGroup)
		}
		if w.LogPatterns != nil {
			for _, p := range w.LogPatterns.Patterns {
				p.Instances.Ctx = ctx
			}
		}
	}
	v.Dashboards = append(v.Dashboards, d)
}

func (v *AppView) addDependency(w *model.World, id model.ApplicationId) {
	for _, a := range v.Dependencies {
		if a.Id == id {
			return
		}
	}
	app := w.GetApplication(id)
	if app == nil {
		return
	}
	v.Dependencies = append(v.Dependencies, &Application{Id: id, Labels: app.Labels()})
}

func (v *AppView) addClient(w *model.World, id model.ApplicationId) {
	for _, a := range v.Clients {
		if a.Id == id {
			return
		}
	}
	app := w.GetApplication(id)
	if app == nil {
		return
	}
	v.Clients = append(v.Clients, &Application{Id: id, Labels: app.Labels()})
}

func RenderApp(world *model.World, app *model.Application) *AppView {
	view := &AppView{Application: &Application{
		Id:     app.Id,
		Labels: app.Labels(),
	}}

	deps := map[model.ApplicationId]bool{}
	for _, instance := range app.Instances {
		if instance.Pod != nil && instance.Pod.IsObsolete() {
			continue
		}
		i := &Instance{Id: instance.Name}
		for _, connection := range instance.Upstreams {
			if connection.RemoteInstance == nil {
				continue
			}
			if connection.RemoteInstance.OwnerId != app.Id {
				deps[connection.RemoteInstance.OwnerId] = true
				i.addDependency(connection.RemoteInstance.OwnerId, connection.Status(), "to")
			} else if connection.RemoteInstance.Name != instance.Name {
				i.addInternalLink(connection.RemoteInstance.Name, connection.Status())
			}
		}
		for _, connection := range instance.Downstreams {
			if connection.Instance.OwnerId != app.Id {
				i.addClient(connection.Instance.OwnerId, connection.Status(), "to")
			}
		}
		view.Instances = append(view.Instances, i)
	}
	for _, i := range view.Instances {
		clients := make([]*ApplicationLink, 0, len(i.Clients))
		for _, c := range i.Clients {
			if deps[c.Id] {
				i.addDependency(c.Id, c.Status, "from")
			} else {
				clients = append(clients, c)
			}
		}
		i.Clients = clients
	}

	for _, i := range view.Instances {
		for _, a := range i.Dependencies {
			view.addDependency(world, a.Id)
		}
	}
	for _, i := range view.Instances {
		for _, a := range i.Clients {
			view.addClient(world, a.Id)
		}
	}
	sort.Slice(view.Instances, func(i1, i2 int) bool {
		return view.Instances[i1].Id < view.Instances[i2].Id
	})
	sort.Slice(view.Clients, func(i, j int) bool {
		return view.Clients[i].Id.Name < view.Clients[j].Id.Name
	})
	sort.Slice(view.Dependencies, func(i, j int) bool {
		return view.Dependencies[i].Id.Name < view.Dependencies[j].Id.Name
	})

	view.addDashboard(world.Ctx, cpu(app))
	view.addDashboard(world.Ctx, memory(app))
	view.addDashboard(world.Ctx, storage(app))
	view.addDashboard(world.Ctx, network(app, world))
	view.addDashboard(world.Ctx, logs(app))

	return view
}

func autoFeatureChartInGroup(cg *ChartGroup) {
	if len(cg.Charts) < 2 {
		return
	}
	type weightedChart struct {
		ch *Chart
		w  float64
	}
	for _, ch := range cg.Charts {
		if ch.Featured {
			return
		}
	}
	charts := make([]weightedChart, 0, len(cg.Charts))
	for _, ch := range cg.Charts {
		var w float64
		for _, s := range ch.Series {
			w += timeseries.Reduce(timeseries.NanSum, s.Data)
		}
		charts = append(charts, weightedChart{ch: ch, w: w})
	}
	sort.Slice(charts, func(i, j int) bool {
		return charts[i].w > charts[j].w
	})
	if charts[0].w/charts[1].w > 1.2 {
		charts[0].ch.Featured = true
	}
}
