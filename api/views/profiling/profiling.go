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
	Status     model.Status        `json:"status"`
	Message    string              `json:"message"`
	Services   []Service           `json:"services"`
	Profiles   []Meta              `json:"profiles"`
	Profile    *model.Profile      `json:"profile"`
	Chart      *model.Chart        `json:"chart"`
	Instances  []string            `json:"instances"`
	Containers map[string][]string `json:"containers,omitempty"`
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
	Type      model.ProfileType `json:"type"`
	From      timeseries.Time   `json:"from"`
	To        timeseries.Time   `json:"to"`
	Mode      string            `json:"mode"`
	Instance  string            `json:"instance"`
	Container string            `json:"container"`
}

func Render(ctx context.Context, ch *clickhouse.Client, app *model.Application, query url.Values, w *model.World) *View {
	if ch == nil {
		return &View{Status: model.WARNING, Message: "Clickhouse integration is not configured"}
	}

	var q Query
	var category model.ProfileCategory
	if s := query.Get("query"); s != "" {
		switch s {
		case model.ProfileCategoryCPU, model.ProfileCategoryMemory, model.ProfileCategoryLock:
			category = model.ProfileCategory(s)
		default:
			if err := json.Unmarshal([]byte(s), &q); err != nil {
				klog.Warningln(err)
			}
		}
	}
	if q.From == 0 {
		q.From = w.Ctx.From
	}
	if q.To == 0 {
		q.To = w.Ctx.To
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
		if s := model.GuessService(maps.Keys(profileTypes), w, app); len(services) == 0 && s != "" {
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

	chart, containers := getChart(app, q.Type, w.Ctx, q.Instance)
	v.Chart = chart
	v.Instances = maps.Keys(containers)
	sort.Strings(v.Instances)
	v.Containers = containers
	v.Profile = &model.Profile{Type: q.Type, Diff: q.Mode == "diff"}
	pq := clickhouse.ProfileQuery{
		Type:     q.Type,
		From:     q.From,
		To:       q.To,
		Diff:     v.Profile.Diff,
		Services: maps.Keys(services),
	}
	if q.Container != "" {
		// Filter by specific container ID
		pq.Containers = []string{q.Container}
	} else if q.Instance != "" {
		if model.Profiles[q.Type].NodeAgent {
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
	profile := model.Profiles[typ]

	var chart *model.Chart
	var containerToSeriesF func(c *model.Container) *timeseries.TimeSeries
	switch profile.Category {
	case model.ProfileCategoryCPU:
		chart = model.NewChart(ctx, "CPU usage by instance, cores")
		containerToSeriesF = func(c *model.Container) *timeseries.TimeSeries { return c.CpuUsage }
	case model.ProfileCategoryMemory:
		switch typ {
		case model.ProfileTypeJavaHeapAllocObjects:
			return getJvmChart(app, ctx, instance, "Allocation rate by instance, objects/second",
				func(jvm *model.Jvm) *timeseries.TimeSeries { return jvm.AllocObjects })
		case model.ProfileTypeJavaHeapAllocSpace:
			return getJvmChart(app, ctx, instance, "Allocation rate by instance, bytes/second",
				func(jvm *model.Jvm) *timeseries.TimeSeries { return jvm.AllocBytes })
		}
		chart = model.NewChart(ctx, "Memory (RSS) usage by instance, bytes")
		containerToSeriesF = func(c *model.Container) *timeseries.TimeSeries { return c.MemoryRss }
	case model.ProfileCategoryLock:
		switch typ {
		case model.ProfileTypeJavaLockContentions:
			return getJvmChart(app, ctx, instance, "Lock contentions by instance, per second",
				func(jvm *model.Jvm) *timeseries.TimeSeries { return jvm.LockContentions })
		case model.ProfileTypeJavaLockDelay:
			return getJvmChart(app, ctx, instance, "Lock wait time by instance, seconds/second",
				func(jvm *model.Jvm) *timeseries.TimeSeries { return jvm.LockTime })
		}
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

func getJvmChart(app *model.Application, ctx timeseries.Context, instance string, title string, seriesF func(jvm *model.Jvm) *timeseries.TimeSeries) (*model.Chart, map[string][]string) {
	chart := model.NewChart(ctx, title)
	containers := map[string][]string{}
	for _, i := range app.Instances {
		agg := timeseries.NewAggregate(timeseries.NanSum)
		hasData := false
		for _, jvm := range i.Jvms {
			if s := seriesF(jvm); s != nil {
				agg.Add(s)
				hasData = true
			}
		}
		for _, c := range i.Containers {
			containers[i.Name] = append(containers[i.Name], c.Id)
		}
		if hasData && (instance == "" || i.Name == instance) {
			chart.AddSeries(i.Name, agg)
		}
	}
	events := model.EventsToAnnotations(app.Events, ctx)
	incidents := model.IncidentsToAnnotations(app.Incidents, ctx)
	return chart.AddAnnotation(events...).AddAnnotation(incidents...), containers
}
