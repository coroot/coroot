package profiling

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/coroot/coroot/clickhouse"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"golang.org/x/exp/maps"
	"k8s.io/klog"
)

type View struct {
	Status   model.Status `json:"status"`
	Message  string       `json:"message"`
	Services []Service    `json:"services"`
	Profiles []Meta       `json:"profiles"`
	Profile  Profile      `json:"profile"`
	Chart    *model.Chart `json:"chart"`
}

type Service struct {
	Name   string `json:"name"`
	Linked bool   `json:"linked"`
}

type Meta struct {
	Type model.ProfileType `json:"type"`
	Name string            `json:"name"`
}

type Query struct {
	Type model.ProfileType `json:"type"`
	From timeseries.Time   `json:"from"`
	To   timeseries.Time   `json:"to"`
	Mode string            `json:"mode"`
}

type Profile struct {
	Type       model.ProfileType     `json:"type"`
	FlameGraph *model.FlameGraphNode `json:"flamegraph"`
	Diff       bool                  `json:"diff"`
}

func Render(ctx context.Context, ch *clickhouse.Client, app *model.Application, appSettings *db.ApplicationSettings, query url.Values, wCtx timeseries.Context) *View {
	if ch == nil {
		return nil
	}

	var q Query
	var category model.ProfileCategory
	if s := query.Get("query"); s != "" {
		switch s {
		case model.ProfileCategoryCPU, model.ProfileCategoryMemory:
			category = model.ProfileCategory(s)
		default:
			if err := json.Unmarshal([]byte(s), &q); err != nil {
				klog.Warningln(err)
			}
		}
	}
	if q.From == 0 {
		q.From = wCtx.From
	}
	if q.To == 0 {
		q.To = wCtx.To
	}

	v := &View{}

	profileTypes, err := ch.GetProfileTypes(ctx)
	if err != nil {
		klog.Errorln(err)
		v.Status = model.WARNING
		v.Message = fmt.Sprintf("clickhouse error: %s", err)
		return v
	}

	service := ""
	if appSettings != nil && appSettings.Profiling != nil {
		service = appSettings.Profiling.Service
	} else {
		service = model.GuessService(maps.Keys(profileTypes), app.Id)
	}

	for s := range profileTypes {
		if !strings.HasPrefix(s, "/") {
			v.Services = append(v.Services, Service{Name: s, Linked: s == service})
		}
	}
	sort.Slice(v.Services, func(i, j int) bool {
		return v.Services[i].Name < v.Services[j].Name
	})

	for _, p := range profileTypes[service] {
		v.Profiles = append(v.Profiles, Meta{Type: p, Name: model.Profiles[p].Name})
	}

	if len(v.Profiles) == 0 {
		v.Status = model.UNKNOWN
		v.Message = "No profiles found"
		return v
	}
	sort.Slice(v.Profiles, func(i, j int) bool {
		return v.Profiles[i].Name < v.Profiles[j].Name
	})

	if q.Type == "" && category != model.ProfileCategoryNone {
		var featured model.ProfileType
		for _, p := range profileTypes[service] {
			pm := model.Profiles[p]
			if pm.Category != category {
				continue
			}
			if featured == "" && pm.Featured {
				featured = p
			}
			if q.Type == "" {
				q.Type = p
			}
		}
		if featured != "" {
			q.Type = featured
		}
	}
	if q.Type == "" {
		q.Type = v.Profiles[0].Type
	}

	services := utils.NewStringSet()
	for _, i := range app.Instances {
		for _, c := range i.Containers {
			services.Add(model.ContainerIdToServiceName(c.Id))
		}
	}

	v.Chart = getChart(app, q.Type, wCtx)
	v.Profile = Profile{Type: q.Type, Diff: q.Mode == "diff"}
	v.Profile.FlameGraph, err = ch.GetProfile(ctx, q.From, q.To, services.Items(), q.Type, v.Profile.Diff)
	if err != nil {
		klog.Errorln(err)
		v.Status = model.WARNING
		v.Message = fmt.Sprintf("clickhouse error: %s", err)
		return v
	}
	if v.Profile.FlameGraph == nil {
		v.Status = model.UNKNOWN
		v.Message = "No profiles found"
		return v
	}

	v.Status = model.OK
	v.Message = "OK"

	return v
}

func getChart(app *model.Application, typ model.ProfileType, ctx timeseries.Context) *model.Chart {
	var chart *model.Chart
	var containerToSeriesF func(c *model.Container) *timeseries.TimeSeries
	category := model.Profiles[typ].Category
	switch category {
	case model.ProfileCategoryCPU:
		chart = model.NewChart(ctx, "CPU usage by instance, cores").Stacked()
		containerToSeriesF = func(c *model.Container) *timeseries.TimeSeries { return c.CpuUsage }
	case model.ProfileCategoryMemory:
		chart = model.NewChart(ctx, "Memory (RSS) usage by instance, bytes").Stacked()
		containerToSeriesF = func(c *model.Container) *timeseries.TimeSeries { return c.MemoryRss }
	default:
		return nil
	}
	for _, i := range app.Instances {
		agg := timeseries.NewAggregate(timeseries.NanSum)
		for _, c := range i.Containers {
			agg.Add(containerToSeriesF(c))
		}
		chart.AddSeries(i.Name, agg)
	}
	events := model.EventsToAnnotations(app.Events, ctx)
	incidents := model.IncidentsToAnnotations(app.Incidents, ctx)
	return chart.AddAnnotation(events...).AddAnnotation(incidents...)
}
