package application

import (
	"slices"
	"sort"

	"github.com/coroot/coroot/db"
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
	Icon       string                    `json:"icon"`
	Indicators []model.Indicator         `json:"indicators"`
	Labels     model.Labels              `json:"labels"`

	LinkStatus       model.Status `json:"link_status"`
	LinkStatusReason string       `json:"link_status_reason"`
	LinkDirection    string       `json:"link_direction"`
	LinkStats        []string     `json:"link_stats"`
	LinkWeight       float32      `json:"link_weight"`
}

type Instance struct {
	Id     string       `json:"id"`
	Labels model.Labels `json:"labels"`
}

func Render(project *db.Project, world *model.World, app *model.Application) *View {
	appMap := &AppMap{
		Application: &Application{
			Id:         app.Id,
			Category:   app.Category,
			Custom:     app.Custom,
			Status:     app.Status,
			Icon:       app.ApplicationType().Icon(),
			Indicators: model.CalcIndicators(app),
			Labels:     app.Labels(),
		},
		CustomApplications: maps.Keys(project.Settings.CustomApplications),
	}
	for name := range project.Settings.ApplicationCategorySettings {
		if !name.Default() {
			appMap.Categories = append(appMap.Categories, name)
		}
	}
	slices.Sort(appMap.Categories)

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
		appMap.Instances = append(appMap.Instances, i)
	}

	for _, connection := range app.Upstreams {
		if connection.RemoteApplication.Id != app.Id {
			appMap.addDependency(connection)
		}
	}
	for _, connection := range app.Downstreams {
		if connection.Application.Id != app.Id {
			appMap.addClient(connection)
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

func (m *AppMap) addDependency(c *model.AppToAppConnection) {
	status, reason := c.Status()
	a := &Application{
		Id:         c.RemoteApplication.Id,
		Custom:     c.RemoteApplication.Custom,
		Status:     c.RemoteApplication.Status,
		Icon:       c.RemoteApplication.ApplicationType().Icon(),
		Indicators: model.CalcIndicators(c.RemoteApplication),
		Labels:     c.RemoteApplication.Labels(),

		LinkDirection:    "to",
		LinkStatus:       status,
		LinkStatusReason: reason,
	}
	m.Dependencies = append(m.Dependencies, a)

	requests := c.GetConnectionsRequestsSum(nil).Last()
	latency := c.GetConnectionsRequestsLatency(nil).Last()
	if !timeseries.IsNaN(requests) {
		a.LinkWeight = requests
	}
	bytesSent := c.BytesSent.Last()
	bytesReceived := c.BytesReceived.Last()
	a.LinkStats = utils.FormatLinkStats(requests, latency, bytesSent, bytesReceived, reason)
}

func (m *AppMap) addClient(c *model.AppToAppConnection) {
	for _, d := range m.Dependencies {
		if d.Id == c.Application.Id {
			d.LinkDirection = "both"
			return
		}
	}
	status, reason := c.Status()
	a := &Application{
		Id:         c.Application.Id,
		Custom:     c.Application.Custom,
		Status:     c.Application.Status,
		Icon:       c.Application.ApplicationType().Icon(),
		Indicators: model.CalcIndicators(c.Application),
		Labels:     c.Application.Labels(),

		LinkDirection:    "to",
		LinkStatus:       status,
		LinkStatusReason: reason,
	}
	m.Clients = append(m.Clients, a)

	requests := c.GetConnectionsRequestsSum(nil).Last()
	latency := c.GetConnectionsRequestsLatency(nil).Last()
	if !timeseries.IsNaN(requests) {
		a.LinkWeight = requests
	}
	bytesSent := c.BytesSent.Last()
	bytesReceived := c.BytesReceived.Last()
	a.LinkStats = utils.FormatLinkStats(requests, latency, bytesSent, bytesReceived, reason)
}
