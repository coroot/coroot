package application

import (
	"sort"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"golang.org/x/exp/maps"
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

	CustomApplications []string                    `json:"custom_applications"`
	Categories         []model.ApplicationCategory `json:"categories"`
}

type Application struct {
	Id         model.ApplicationId       `json:"id"`
	Category   model.ApplicationCategory `json:"category"`
	Custom     bool                      `json:"custom"`
	Status     model.Status              `json:"status"`
	Indicators []model.Indicator         `json:"indicators"`
	Labels     model.Labels              `json:"labels"`
}

type Instance struct {
	Id     string       `json:"id"`
	Labels model.Labels `json:"labels"`

	Clients       []*ApplicationLink `json:"clients"`
	Dependencies  []*ApplicationLink `json:"dependencies"`
	InternalLinks []*InstanceLink    `json:"internal_links"`
}

type ApplicationLink struct {
	Id           model.ApplicationId `json:"id"`
	Status       model.Status        `json:"status"`
	StatusReason string              `json:"status_reason"`
	Direction    string              `json:"direction"`
	Stats        []string            `json:"stats"`
	Weight       float32             `json:"weight"`

	connections []*model.Connection
}

type InstanceLink struct {
	Id        string       `json:"id"`
	Status    model.Status `json:"status"`
	Direction string       `json:"direction"`
}

func Render(world *model.World, app *model.Application) *View {
	appMap := &AppMap{
		Application: &Application{
			Id:         app.Id,
			Category:   app.Category,
			Custom:     app.Custom,
			Status:     app.Status,
			Indicators: model.CalcIndicators(app),
			Labels:     app.Labels(),
		},
		CustomApplications: maps.Keys(world.CustomApplications),
		Categories:         world.Categories,
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
		} else if instance.ApplicationTypes()[model.ApplicationTypePgbouncer] {
			i.Labels["proxy"] = "pgbouncer"
		}
		if instance.Mongodb != nil {
			if v := instance.Mongodb.ReplicaSet.Value(); v != "" {
				i.Labels["rs"] = v
			}
		}
		if instance.ApplicationTypes()[model.ApplicationTypeMongos] {
			i.Labels["proxy"] = "mongos"
		}
		for _, connection := range instance.Upstreams {
			if connection.RemoteApplication == nil {
				continue
			}
			if connection.RemoteApplication.Id != app.Id {
				deps[connection.RemoteApplication.Id] = true
				status, reason := connection.Status()
				i.addDependency(connection.RemoteApplication.Id, status, reason, "to", connection)
			} else if connection.RemoteInstance != nil && connection.RemoteInstance.Name != instance.Name {
				status, _ := connection.Status()
				i.addInternalLink(connection.RemoteInstance.Name, status)
			}
		}
		for _, connection := range app.Downstreams {
			if connection.Instance.Owner != app {
				switch {
				case connection.RemoteInstance != nil && connection.RemoteInstance.Name == instance.Name:
				case connection.RemoteInstance == nil:
				default:
					continue
				}
				status, reason := connection.Status()
				i.addClient(connection.Instance.Owner.Id, status, reason, "to", connection)
			}
		}
		appMap.Instances = append(appMap.Instances, i)
	}
	for _, i := range appMap.Instances {
		clients := make([]*ApplicationLink, 0, len(i.Clients))
		for _, c := range i.Clients {
			if deps[c.Id] {
				i.addDependency(c.Id, c.Status, c.StatusReason, "from", c.connections...)
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

	if len(app.Incidents) > 0 {
		annotations := model.IncidentsToAnnotations(app.Incidents, world.Ctx)
		for _, r := range app.Reports {
			for _, w := range r.Widgets {
				w.AddAnnotation(annotations...)
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
		Custom:     app.Custom,
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
		Custom:     app.Custom,
		Status:     app.Status,
		Indicators: model.CalcIndicators(app),
		Labels:     app.Labels(),
	})
}

func (i *Instance) addClient(id model.ApplicationId, status model.Status, statusReason string, direction string, connections ...*model.Connection) {
	for _, a := range i.Clients {
		if a.Id == id {
			if a.Status < status {
				a.Status = status
				a.StatusReason = statusReason
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
	i.Clients = append(i.Clients, &ApplicationLink{Id: id, Status: status, StatusReason: statusReason, Direction: direction, connections: connections})
}

func (i *Instance) addDependency(id model.ApplicationId, status model.Status, statusReason string, direction string, connections ...*model.Connection) {
	for _, a := range i.Dependencies {
		if a.Id == id {
			if a.Status < status {
				a.Status = status
				a.StatusReason = statusReason
			}
			a.connections = append(a.connections, connections...)
			return
		}
	}
	i.Dependencies = append(i.Dependencies, &ApplicationLink{Id: id, Status: status, StatusReason: statusReason, Direction: direction, connections: connections})
}

func (i *Instance) addInternalLink(id string, status model.Status) {
	for _, l := range i.InternalLinks {
		if l.Id == id {
			if l.Status < status {
				l.Status = status
			}
			return
		}
	}
	i.InternalLinks = append(i.InternalLinks, &InstanceLink{Id: id, Status: status, Direction: "to"})
}

func (l *ApplicationLink) calcStats() {
	requests := model.GetConnectionsRequestsSum(l.connections, nil).Last()
	latency := model.GetConnectionsRequestsLatency(l.connections, nil).Last()
	if !timeseries.IsNaN(requests) {
		l.Weight = requests
	}

	var bytesSent, bytesReceived float32
	for _, c := range l.connections {
		if v := c.BytesSent.Last(); !timeseries.IsNaN(v) {
			bytesSent += v
		}
		if v := c.BytesReceived.Last(); !timeseries.IsNaN(v) {
			bytesReceived += v
		}
	}
	l.Stats = utils.FormatLinkStats(requests, latency, bytesSent, bytesReceived, l.StatusReason)
}
