package application

import (
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"math"
	"sort"
)

type View struct {
	AppMap  *AppMap              `json:"app_map"`
	Reports []*model.AuditReport `json:"reports"`
}

type AppMap struct {
	Application *Application `json:"application"`
	Instances   []*Instance  `json:"instances"`

	Clients      []*Application `json:"clients"`
	Dependencies []*Application `json:"dependencies"`
}

type Application struct {
	Id         model.ApplicationId `json:"id"`
	Status     model.Status        `json:"status"`
	Indicators []model.Indicator   `json:"indicators"`
	Labels     model.Labels        `json:"labels"`
}

type Instance struct {
	Id     string       `json:"id"`
	Labels model.Labels `json:"labels"`

	Clients       []*ApplicationLink `json:"clients"`
	Dependencies  []*ApplicationLink `json:"dependencies"`
	InternalLinks []*InstanceLink    `json:"internal_links"`
}

type ApplicationLink struct {
	Id        model.ApplicationId `json:"id"`
	Status    model.Status        `json:"status"`
	Direction string              `json:"direction"`
	Stats     []string            `json:"stats"`
	Weight    float32             `json:"weight"`

	connections []*model.Connection
}

type InstanceLink struct {
	Id        string       `json:"id"`
	Status    model.Status `json:"status"`
	Direction string       `json:"direction"`
}

func Render(world *model.World, app *model.Application, incidents []db.Incident) *View {
	appMap := &AppMap{
		Application: &Application{
			Id:         app.Id,
			Status:     app.Status,
			Indicators: model.CalcIndicators(app),
			Labels:     app.Labels(),
		},
	}

	deps := map[model.ApplicationId]bool{}
	for _, instance := range app.Instances {
		if instance.IsObsolete() || instance.IsFailed() {
			continue
		}
		i := &Instance{Id: instance.Name, Labels: model.Labels{}}
		if instance.Postgres != nil && instance.Postgres.Version.Value() != "" {
			i.Labels["version"] = instance.Postgres.Version.Value()
		}
		if role := instance.ClusterRoleLast(); role != model.ClusterRoleNone {
			i.Labels["role"] = role.String()
		}
		if instance.ApplicationTypes()[model.ApplicationTypePgbouncer] {
			i.Labels["pooler"] = "pgbouncer"
		}

		for _, connection := range instance.Upstreams {
			if connection.RemoteInstance == nil {
				continue
			}
			if connection.RemoteInstance.OwnerId != app.Id {
				deps[connection.RemoteInstance.OwnerId] = true
				i.addDependency(connection.RemoteInstance.OwnerId, connection.Status(), "to", connection)
			} else if connection.RemoteInstance.Name != instance.Name {
				i.addInternalLink(connection.RemoteInstance.Name, connection.Status())
			}
		}
		for _, connection := range app.Downstreams {
			if connection.Instance.OwnerId != app.Id && connection.RemoteInstance != nil && connection.RemoteInstance.Name == instance.Name {
				i.addClient(connection.Instance.OwnerId, connection.Status(), "to", connection)
			}
		}
		appMap.Instances = append(appMap.Instances, i)
	}
	for _, i := range appMap.Instances {
		clients := make([]*ApplicationLink, 0, len(i.Clients))
		for _, c := range i.Clients {
			if deps[c.Id] {
				i.addDependency(c.Id, c.Status, "from", c.connections...)
			} else {
				clients = append(clients, c)
			}
		}
		i.Clients = clients
	}

	for _, i := range appMap.Instances {
		for _, a := range i.Dependencies {
			appMap.addDependency(world, a.Id)
			a.calcStats()
		}
		for _, a := range i.Clients {
			appMap.addClient(world, a.Id)
			a.calcStats()
		}
	}
	sort.Slice(appMap.Instances, func(i1, i2 int) bool {
		return appMap.Instances[i1].Id < appMap.Instances[i2].Id
	})
	sort.Slice(appMap.Clients, func(i, j int) bool {
		return appMap.Clients[i].Id.Name < appMap.Clients[j].Id.Name
	})
	sort.Slice(appMap.Dependencies, func(i, j int) bool {
		return appMap.Dependencies[i].Id.Name < appMap.Dependencies[j].Id.Name
	})

	if len(incidents) > 0 {
		now := timeseries.Now()
		for i := range incidents {
			if !incidents[i].Resolved() {
				incidents[i].ResolvedAt = now
			}
		}
		for _, r := range app.Reports {
			for _, w := range r.Widgets {
				var charts []*model.Chart
				switch {
				case w.ChartGroup != nil:
					charts = w.ChartGroup.Charts
				case w.Chart != nil:
					charts = []*model.Chart{w.Chart}
				default:
					continue
				}
				for _, ch := range charts {
					for _, i := range incidents {
						ch.AddAnnotation("incident", i.OpenedAt, i.ResolvedAt, "")
					}
				}
			}
		}
	}

	v := &View{
		AppMap:  appMap,
		Reports: app.Reports,
	}
	return v
}

func (m *AppMap) addDependency(w *model.World, id model.ApplicationId) {
	for _, a := range m.Dependencies {
		if a.Id == id {
			return
		}
	}
	app := w.GetApplication(id)
	if app == nil {
		return
	}
	m.Dependencies = append(m.Dependencies, &Application{
		Id:         id,
		Status:     app.Status,
		Indicators: model.CalcIndicators(app),
		Labels:     app.Labels(),
	})
}

func (m *AppMap) addClient(w *model.World, id model.ApplicationId) {
	for _, a := range m.Clients {
		if a.Id == id {
			return
		}
	}
	app := w.GetApplication(id)
	if app == nil {
		return
	}
	m.Clients = append(m.Clients, &Application{
		Id:         id,
		Status:     app.Status,
		Indicators: model.CalcIndicators(app),
		Labels:     app.Labels(),
	})
}

func (i *Instance) addClient(id model.ApplicationId, status model.Status, direction string, connections ...*model.Connection) {
	for _, a := range i.Clients {
		if a.Id == id {
			if a.Status < status {
				a.Status = status
			}
			a.connections = append(a.connections, connections...)
			return
		}
	}
	for _, a := range i.Dependencies {
		if a.Id == id {
			a.Direction = "both"
			return
		}
	}
	i.Clients = append(i.Clients, &ApplicationLink{Id: id, Status: status, Direction: direction, connections: connections})
}

func (i *Instance) addDependency(id model.ApplicationId, status model.Status, direction string, connections ...*model.Connection) {
	for _, a := range i.Dependencies {
		if a.Id == id {
			if a.Status < status {
				a.Status = status
			}
			a.connections = append(a.connections, connections...)
			return
		}
	}
	i.Dependencies = append(i.Dependencies, &ApplicationLink{Id: id, Status: status, Direction: direction, connections: connections})
}

func (i *Instance) addInternalLink(id string, status model.Status) {
	for _, l := range i.InternalLinks {
		if l.Id == id {
			return
		}
	}
	i.InternalLinks = append(i.InternalLinks, &InstanceLink{Id: id, Status: status, Direction: "to"})
}

func (l *ApplicationLink) calcStats() {
	requests := model.GetConnectionsRequestsSum(l.connections).Last()
	latency := model.GetConnectionsRequestsLatency(l.connections).Last()
	if !math.IsNaN(requests) {
		l.Weight = float32(requests)
		l.Stats = append(l.Stats, utils.FormatFloat(requests)+" rps")
	}
	if !math.IsNaN(latency) {
		l.Stats = append(l.Stats, utils.FormatLatency(latency))
	}
}
