package overview

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

type Application struct {
	Id         model.ApplicationId       `json:"id"`
	Category   model.ApplicationCategory `json:"category"`
	Labels     model.Labels              `json:"labels"`
	Status     model.Status              `json:"status"`
	Indicators []model.Indicator         `json:"indicators"`

	Upstreams   []Link `json:"upstreams"`
	Downstreams []Link `json:"downstreams"`
}

type Link struct {
	Id     model.ApplicationId `json:"id"`
	Status model.Status        `json:"status"`
	Stats  []string            `json:"stats"`
	Weight float32             `json:"weight"`
}

func renderServiceMap(w *model.World) []*Application {
	var apps []*Application
	used := map[model.ApplicationId]bool{}
	for _, a := range w.Applications {
		app := Application{
			Id:          a.Id,
			Category:    a.Category,
			Labels:      a.Labels(),
			Status:      a.Status,
			Indicators:  model.CalcIndicators(a),
			Upstreams:   []Link{},
			Downstreams: []Link{},
		}

		upstreams := map[model.ApplicationId]struct {
			status      model.Status
			connections []*model.Connection
		}{}
		downstreams := map[model.ApplicationId]bool{}
		for _, i := range a.Instances {
			if i.IsObsolete() {
				continue
			}
			for _, u := range i.Upstreams {
				if u.IsObsolete() || u.RemoteApplication == nil || u.RemoteApplication == a {
					continue
				}
				status := u.Status()
				s := upstreams[u.RemoteApplication.Id]
				if status >= s.status {
					s.status = status
				}
				s.connections = append(s.connections, u)
				upstreams[u.RemoteApplication.Id] = s
			}
		}
		for _, d := range a.Downstreams {
			if d.IsObsolete() || d.Instance.OwnerId == app.Id {
				continue
			}
			downstreams[d.Instance.OwnerId] = true
		}

		for id, s := range upstreams {
			l := Link{Id: id, Status: s.status}
			requests := model.GetConnectionsRequestsSum(s.connections, nil).Last()
			latency := model.GetConnectionsRequestsLatency(s.connections, nil).Last()
			if !timeseries.IsNaN(requests) {
				l.Weight = requests
				l.Stats = append(l.Stats, utils.FormatFloat(requests)+" rps")
			}
			if !timeseries.IsNaN(latency) {
				l.Stats = append(l.Stats, utils.FormatLatency(latency))
			}
			app.Upstreams = append(app.Upstreams, l)
			used[a.Id] = true
			used[id] = true
		}

		for id := range downstreams {
			app.Downstreams = append(app.Downstreams, Link{Id: id})
			used[a.Id] = true
			used[id] = true
		}

		apps = append(apps, &app)
	}
	var appsUsed []*Application
	for _, a := range apps {
		if !used[a.Id] {
			continue
		}
		appsUsed = append(appsUsed, a)
	}
	return appsUsed
}
