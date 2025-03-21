package overview

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

type Application struct {
	Id         model.ApplicationId       `json:"id"`
	Custom     bool                      `json:"custom"`
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
	appsById := map[model.ApplicationId]*Application{}
	for _, a := range w.Applications {
		app := &Application{
			Id:          a.Id,
			Custom:      a.Custom,
			Category:    a.Category,
			Labels:      a.Labels(),
			Status:      a.Status,
			Indicators:  model.CalcIndicators(a),
			Upstreams:   []Link{},
			Downstreams: []Link{},
		}
		appsById[a.Id] = app
		upstreams := map[model.ApplicationId]struct {
			status       model.Status
			statusReason string
			connection   *model.AppToAppConnection
		}{}
		downstreams := map[model.ApplicationId]bool{}
		for _, u := range a.Upstreams {
			if u.RemoteApplication == a {
				continue
			}
			status, statusReason := u.Status()
			s := upstreams[u.RemoteApplication.Id]
			if status >= s.status {
				s.status = status
				s.statusReason = statusReason
			}
			s.connection = u
			upstreams[u.RemoteApplication.Id] = s
		}
		for _, d := range a.Downstreams {
			if d.Application == a {
				continue
			}
			downstreams[d.Application.Id] = true
		}

		for id, s := range upstreams {
			l := Link{Id: id, Status: s.status}
			requests := s.connection.GetConnectionsRequestsSum(nil).Last()
			latency := s.connection.GetConnectionsRequestsLatency(nil).Last()
			if !timeseries.IsNaN(requests) {
				l.Weight = requests
			}
			l.Stats = utils.FormatLinkStats(requests, latency, s.connection.BytesSent.Last(), s.connection.BytesReceived.Last(), s.statusReason)
			app.Upstreams = append(app.Upstreams, l)
			used[a.Id] = true
			used[id] = true
		}

		for id := range downstreams {
			app.Downstreams = append(app.Downstreams, Link{Id: id})
			used[a.Id] = true
			used[id] = true
		}

		apps = append(apps, app)
	}
	var appsUsed []*Application
	for _, a := range apps {
		if !used[a.Id] {
			continue
		}
		if len(a.Upstreams) == 0 && len(a.Downstreams) > 0 {
			downstreamCategories := utils.NewStringSet()
			ca := appsById[a.Id]
			if ca != nil && ca.Category.Default() {
				for _, dId := range a.Downstreams {
					d := appsById[dId.Id]
					if d != nil {
						downstreamCategories.Add(string(d.Category))
					}
				}
				if downstreamCategories.Len() == 1 {
					ca.Category = model.ApplicationCategory(downstreamCategories.Items()[0])
				}
			}
		}
		appsUsed = append(appsUsed, a)
	}
	return appsUsed
}
