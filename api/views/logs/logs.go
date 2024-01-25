package logs

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
	"github.com/coroot/logparser"
	"golang.org/x/exp/maps"
	"k8s.io/klog"
)

const (
	viewMessages = "messages"
	viewPatterns = "patterns"

	defaultLimit = 100
)

type View struct {
	Status     model.Status      `json:"status"`
	Message    string            `json:"message"`
	Sources    []model.LogSource `json:"sources"`
	Source     model.LogSource   `json:"source"`
	Services   []string          `json:"services"`
	Service    string            `json:"service"`
	Views      []string          `json:"views"`
	View       string            `json:"view"`
	Severities []string          `json:"severities"`
	Severity   []string          `json:"severity"`
	Chart      *model.Chart      `json:"chart"`
	Entries    []Entry           `json:"entries"`
	Patterns   []*Pattern        `json:"patterns"`
	Limit      int               `json:"limit"`
}

type Pattern struct {
	Severity string                `json:"severity"`
	Sample   string                `json:"sample"`
	Messages *timeseries.Aggregate `json:"messages"`
	Sum      uint64                `json:"sum"`
	Chart    *model.Chart          `json:"chart"`
	Hash     string                `json:"hash"`

	pattern       *logparser.Pattern
	similarHashes *utils.StringSet
	sumByInstance map[string]*timeseries.Aggregate
}

type Entry struct {
	Timestamp  int64             `json:"timestamp"`
	Severity   string            `json:"severity"`
	Message    string            `json:"message"`
	Attributes map[string]string `json:"attributes"`
}

type Query struct {
	Source   model.LogSource `json:"source"`
	View     string          `json:"view"`
	Severity []string        `json:"severity"`
	Search   string          `json:"search"`
	Hash     string          `json:"hash"`
	Limit    int             `json:"limit"`
}

func Render(ctx context.Context, ch *clickhouse.Client, app *model.Application, appSettings *db.ApplicationSettings, query url.Values, w *model.World) *View {
	v := &View{}

	var q Query
	if s := query.Get("query"); s != "" {
		if err := json.Unmarshal([]byte(s), &q); err != nil {
			klog.Warningln(err)
		}
	}
	if q.Limit <= 0 {
		q.Limit = defaultLimit
	}

	patterns := getPatterns(app)

	if ch == nil {
		v.Status = model.UNKNOWN
		v.Message = "Clickhouse integration is not configured"
		v.View = viewPatterns
		renderPatterns(v, patterns, w.Ctx)
		return v
	}

	v.View = q.View
	if v.View == "" {
		v.View = viewMessages
	}
	v.Views = append(v.Views, viewMessages)
	renderEntries(ctx, v, ch, app, appSettings, w, q, patterns)
	if v.Source == model.LogSourceAgent {
		v.Views = append(v.Views, viewPatterns)
		if v.View == viewPatterns {
			renderPatterns(v, patterns, w.Ctx)
		}
	}
	return v
}

func renderEntries(ctx context.Context, v *View, ch *clickhouse.Client, app *model.Application, appSettings *db.ApplicationSettings, w *model.World, q Query, patterns map[string]map[string]*Pattern) {
	services, err := ch.GetServicesFromLogs(ctx)
	if err != nil {
		klog.Errorln(err)
		v.Status = model.WARNING
		v.Message = fmt.Sprintf("Clickhouse error: %s", err)
		return
	}

	service := ""
	if appSettings != nil && appSettings.Logs != nil {
		service = appSettings.Logs.Service
	} else {
		service = model.GuessService(maps.Keys(services), app.Id)
	}
	for s := range services {
		if strings.HasPrefix(s, "/") {
			v.Sources = append(v.Sources, model.LogSourceAgent)
		} else {
			v.Services = append(v.Services, s)
			if s == service {
				v.Service = s
				v.Sources = append(v.Sources, model.LogSourceOtel)
			}
		}
	}
	sort.Strings(v.Services)

	if len(v.Sources) == 0 {
		v.Status = model.UNKNOWN
		v.Message = "No logs found"
		return
	}

	v.Source = q.Source
	if v.Source == "" {
		if v.Service != "" {
			v.Source = model.LogSourceOtel
		} else {
			v.Source = model.LogSourceAgent
		}
	}
	v.Severity = q.Severity

	var histogram map[string]*timeseries.TimeSeries
	var entries []*model.LogEntry
	switch v.Source {
	case model.LogSourceOtel:
		v.Message = fmt.Sprintf("Using OpenTelemetry logs of <i>%s</i>", service)
		v.Severities = services[v.Service]
		if len(v.Severity) == 0 {
			v.Severity = v.Severities
		}
		if v.View == viewMessages {
			histogram, err = ch.GetServiceLogsHistogram(ctx, w.Ctx.From, w.Ctx.To, w.Ctx.Step, service, v.Severity, q.Search)
			if err == nil {
				entries, err = ch.GetServiceLogs(ctx, w.Ctx.From, w.Ctx.To, service, v.Severity, q.Search, q.Limit)
			}
		}
	case model.LogSourceAgent:
		v.Message = "Using container logs"
		containers := map[string][]string{}
		severities := utils.NewStringSet()
		for _, i := range app.Instances {
			for _, c := range i.Containers {
				s := model.ContainerIdToServiceName(c.Id)
				containers[s] = append(containers[s], c.Id)
				severities.Add(services[s]...)
			}
		}
		v.Severities = severities.Items()
		if len(v.Severity) == 0 {
			v.Severity = v.Severities
		}
		if v.View == viewMessages {
			var hashes []string
			if q.Hash != "" {
				hashes = getSimilarHashes(patterns, q.Hash)
			}
			histogram, err = ch.GetContainerLogsHistogram(ctx, w.Ctx.From, w.Ctx.To, w.Ctx.Step, containers, v.Severity, hashes, q.Search)
			if err == nil {
				entries, err = ch.GetContainerLogs(ctx, w.Ctx.From, w.Ctx.To, containers, v.Severity, hashes, q.Search, q.Limit)
			}
		}
	}
	if err != nil {
		klog.Errorln(err)
		v.Status = model.WARNING
		v.Message = fmt.Sprintf("Clickhouse error: %s", err)
		return
	}

	v.Status = model.OK

	if len(histogram) > 0 {
		v.Chart = model.NewChart(w.Ctx, "").Column()
		v.Chart.Flags = "severity"
		for severity, ts := range histogram {
			v.Chart.AddSeries(severity, ts)
		}
	}

	for _, e := range entries {
		entry := Entry{
			Timestamp:  e.Timestamp.UnixMilli(),
			Severity:   e.Severity,
			Message:    e.Body,
			Attributes: map[string]string{},
		}
		for name, value := range e.LogAttributes {
			if name != "" && value != "" {
				entry.Attributes[name] = value
			}
		}
		for name, value := range e.ResourceAttributes {
			if name != "" && value != "" {
				entry.Attributes[name] = value
			}
		}
		if e.TraceId != "" {
			entry.Attributes["trace.id"] = e.TraceId
		}
		v.Entries = append(v.Entries, entry)
	}
	if len(v.Entries) >= q.Limit {
		v.Limit = q.Limit
	}
}

func renderPatterns(v *View, patterns map[string]map[string]*Pattern, ctx timeseries.Context) {
	bySeverity := map[string]*timeseries.Aggregate{}
	for severity, byHash := range patterns {
		bySeverity[severity] = timeseries.NewAggregate(timeseries.NanSum)
		for _, p := range byHash {
			bySeverity[severity].Add(p.Messages.Get())
			v.Patterns = append(v.Patterns, p)
			p.Chart = model.NewChart(ctx, "").Column()
			for name, ts := range p.sumByInstance {
				p.Chart.AddSeries(name, ts)
			}
		}
	}
	sort.Slice(v.Patterns, func(i, j int) bool {
		return v.Patterns[i].Sum > v.Patterns[j].Sum
	})

	if len(bySeverity) > 0 {
		v.Chart = model.NewChart(ctx, "").Column()
		v.Chart.Flags = "severity"
		for severity, ts := range bySeverity {
			v.Chart.AddSeries(severity, ts.Get())
		}
	}
}

func getPatterns(app *model.Application) map[string]map[string]*Pattern {
	res := map[string]map[string]*Pattern{}
	for _, instance := range app.Instances {
		for level, msgs := range instance.LogMessages {
			severity := string(level)
			for hash, pattern := range msgs.Patterns {
				events := pattern.Messages.Reduce(timeseries.NanSum)
				if timeseries.IsNaN(events) || events == 0 {
					continue
				}
				if res[severity] == nil {
					res[severity] = map[string]*Pattern{}
				}
				p := res[severity][hash]
				if p == nil {
					for _, pp := range res[severity] {
						if pp.pattern.WeakEqual(pattern.Pattern) {
							p = pp
							break
						}
					}
					if p == nil {
						p = &Pattern{
							pattern:       pattern.Pattern,
							Severity:      severity,
							Sample:        pattern.Sample,
							Messages:      timeseries.NewAggregate(timeseries.NanSum),
							Hash:          hash,
							similarHashes: utils.NewStringSet(),
							sumByInstance: map[string]*timeseries.Aggregate{},
						}
						res[severity][hash] = p
					}
				}
				p.Sum += uint64(events)
				p.Messages.Add(pattern.Messages)
				p.similarHashes.Add(hash)
				if p.sumByInstance[instance.Name] == nil {
					p.sumByInstance[instance.Name] = timeseries.NewAggregate(timeseries.NanSum)
				}
				p.sumByInstance[instance.Name].Add(pattern.Messages)
			}
		}
	}
	return res
}

func getSimilarHashes(patterns map[string]map[string]*Pattern, hash string) []string {
	set := utils.NewStringSet()
	for _, ps := range patterns {
		for _, p := range ps {
			if p.similarHashes.Has(hash) {
				set.Add(p.similarHashes.Items()...)
			}
		}
	}
	return set.Items()
}
