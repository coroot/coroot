package profiling

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/coroot/coroot/clickhouse"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"golang.org/x/exp/maps"
	"k8s.io/klog"
)

type View struct {
	Status    model.Status   `json:"status"`
	Message   string         `json:"message"`
	Services  []Service      `json:"services"`
	Profiles  []Meta         `json:"profiles"`
	Profile   *model.Profile `json:"profile"`
	Chart     *model.Chart   `json:"chart"`
	Instances []string       `json:"instances"`
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
	Type     model.ProfileType `json:"type"`
	From     timeseries.Time   `json:"from"`
	To       timeseries.Time   `json:"to"`
	Mode     string            `json:"mode"`
	Instance string            `json:"instance"`
}

func Render(ctx context.Context, ch *clickhouse.Client, app *model.Application, query url.Values, wCtx timeseries.Context) *View {
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

	profileTypes, err := ch.GetProfileTypes(ctx, q.From)
	if err != nil {
		klog.Errorln(err)
		v.Status = model.WARNING
		v.Message = fmt.Sprintf("clickhouse error: %s", err)
		return v
	}

	services := map[string]bool{}
	if app.Settings != nil && app.Settings.Profiling != nil {
		services[app.Settings.Profiling.Service] = true
	} else {
		for _, i := range app.Instances {
			for _, c := range i.Containers {
				services[model.ContainerIdToServiceName(c.Id)] = true
			}
		}
		if s := model.GuessService(maps.Keys(profileTypes), app.Id); len(services) == 0 && s != "" {
			services[s] = true
		}
	}

	for s := range profileTypes {
		if !strings.HasPrefix(s, "/") {
			v.Services = append(v.Services, Service{Name: s, Linked: services[s]})
		}
	}
	sort.Slice(v.Services, func(i, j int) bool {
		return v.Services[i].Name < v.Services[j].Name
	})

	types := map[model.ProfileType]bool{}
	for s, pts := range profileTypes {
		if !services[s] {
			continue
		}
		for _, pt := range pts {
			types[pt] = true
		}
	}
	for pt := range types {
		v.Profiles = append(v.Profiles, Meta{Type: pt, Name: model.Profiles[pt].Name})
	}

	if len(v.Profiles) == 0 {
		v.Status = model.UNKNOWN
		v.Message = "No profiles found in ClickHouse"
		return v
	}
	sort.Slice(v.Profiles, func(i, j int) bool {
		return v.Profiles[i].Name < v.Profiles[j].Name
	})

	if q.Type == "" && category != model.ProfileCategoryNone {
		var featured model.ProfileType
		for _, p := range v.Profiles {
			pm := model.Profiles[p.Type]
			if pm.Category != category {
				continue
			}
			if featured == "" && pm.Featured {
				featured = p.Type
			}
			if q.Type == "" {
				q.Type = p.Type
			}
		}
		if featured != "" {
			q.Type = featured
		}
	}
	if q.Type == "" {
		q.Type = v.Profiles[0].Type
	}

	chart, containers := getChart(app, q.Type, wCtx, q.Instance)
	v.Chart = chart
	v.Instances = maps.Keys(containers)
	sort.Strings(v.Instances)
	v.Profile = &model.Profile{Type: q.Type, Diff: q.Mode == "diff"}
	pq := clickhouse.ProfileQuery{
		Type:     q.Type,
		From:     q.From,
		To:       q.To,
		Diff:     v.Profile.Diff,
		Services: maps.Keys(services),
	}
	if q.Instance != "" {
		if model.Profiles[q.Type].Ebpf {
			pq.Containers = containers[q.Instance]
		} else {
			pq.Namespace = app.Id.Namespace
			pq.Pod = q.Instance
		}
	}
	v.Profile.FlameGraph, err = ch.GetProfile(ctx, pq)
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

func getChart(app *model.Application, typ model.ProfileType, ctx timeseries.Context, instance string) (*model.Chart, map[string][]string) {
	var chart *model.Chart
	var containerToSeriesF func(c *model.Container) *timeseries.TimeSeries
	category := model.Profiles[typ].Category
	switch category {
	case model.ProfileCategoryCPU:
		chart = model.NewChart(ctx, "CPU usage by instance, cores")
		containerToSeriesF = func(c *model.Container) *timeseries.TimeSeries { return c.CpuUsage }
	case model.ProfileCategoryMemory:
		chart = model.NewChart(ctx, "Memory (RSS) usage by instance, bytes")
		containerToSeriesF = func(c *model.Container) *timeseries.TimeSeries { return c.MemoryRss }
	default:
		return nil, nil
	}
	containers := map[string][]string{}
	for _, i := range app.Instances {
		agg := timeseries.NewAggregate(timeseries.NanSum)
		for _, c := range i.Containers {
			agg.Add(containerToSeriesF(c))
			containers[i.Name] = append(containers[i.Name], c.Id)
		}
		if instance == "" || i.Name == instance {
			chart.AddSeries(i.Name, agg)
		}
	}
	events := model.EventsToAnnotations(app.Events, ctx)
	incidents := model.IncidentsToAnnotations(app.Incidents, ctx)
	return chart.AddAnnotation(events...).AddAnnotation(incidents...), containers
}
