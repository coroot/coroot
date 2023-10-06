package logs

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/tracing"
	"github.com/coroot/logparser"
	"k8s.io/klog"
	"net/url"
	"sort"
	"strings"
)

const (
	limit = 1000
)

type View struct {
	Status   model.Status `json:"status"`
	Message  string       `json:"message"` // TODO: output if error
	Sources  []Source     `json:"sources"`
	Services []Service    `json:"services"`
	Chart    *model.Chart `json:"chart"`
	Patterns []*Pattern   `json:"patterns"`
	Entries  []Entry      `json:"entries"`
	Limit    int          `json:"limit"`
}

type Source struct {
	Type     tracing.Source `json:"type"`
	Name     string         `json:"name"`
	Selected bool           `json:"selected"`
}

type Service struct {
	Name   string `json:"name"`
	Linked bool   `json:"linked"`
}

type Pattern struct {
	Severity model.LogSeverity     `json:"severity"`
	Sample   string                `json:"sample"`
	Messages *timeseries.Aggregate `json:"messages"`
	Sum      uint64                `json:"sum"`
	Chart    *model.Chart          `json:"chart"`
	Hash     []string              `json:"hash"`

	pattern       *logparser.Pattern
	sumByInstance map[string]*timeseries.Aggregate
}

type Entry struct {
	Timestamp  int64             `json:"timestamp"`
	Severity   model.LogSeverity `json:"severity"`
	Message    string            `json:"message"`
	Attributes map[string]string `json:"attributes"`
}

type Query struct {
	Source   tracing.Source      `json:"source"`
	Severity []model.LogSeverity `json:"severity"`
	Search   string              `json:"search"`
	Hash     []string            `json:"hash"`

	severity map[model.LogSeverity]bool
	hash     map[string]bool
}

func Render(ctx context.Context, clickhouse *tracing.ClickhouseClient, app *model.Application, appSettings *db.ApplicationSettings, query url.Values, w *model.World) *View {
	v := &View{}

	var q Query
	if qs := query["query"]; len(qs) > 0 {
		if err := json.Unmarshal([]byte(qs[0]), &q); err != nil {
			klog.Warningln(err)
		}
		q.severity = map[model.LogSeverity]bool{}
		for _, s := range q.Severity {
			q.severity[s] = true
		}
		q.hash = map[string]bool{}
		for _, h := range q.Hash {
			q.hash[h] = true
		}
	}

	getChartAndPatterns(v, app, w, q)
	getLogs(ctx, v, clickhouse, app, appSettings, w, q)

	return v
}

func getChartAndPatterns(v *View, app *model.Application, w *model.World, q Query) {
	sumBySeverity := map[model.LogSeverity]*timeseries.Aggregate{}
	sumByInstance := map[string]*timeseries.Aggregate{}
	filterByPattern := len(q.hash) > 0
	patterns := map[model.LogSeverity]map[string]*Pattern{}
	for _, instance := range app.Instances {
		for severity, msgs := range instance.LogMessages {
			if len(q.severity) > 0 && !q.severity[severity] {
				continue
			}
			if !filterByPattern {
				if sumBySeverity[severity] == nil {
					sumBySeverity[severity] = timeseries.NewAggregate(timeseries.NanSum)
				}
				sumBySeverity[severity].Add(msgs.Messages)
			}

			for hash, pattern := range msgs.Patterns {
				events := pattern.Messages.Reduce(timeseries.NanSum)
				if timeseries.IsNaN(events) || events == 0 {
					continue
				}
				if filterByPattern && q.hash[hash] {
					if sumByInstance[instance.Name] == nil {
						sumByInstance[instance.Name] = timeseries.NewAggregate(timeseries.NanSum)
					}
					sumByInstance[instance.Name].Add(pattern.Messages)
				}
				if patterns[severity] == nil {
					patterns[severity] = map[string]*Pattern{}
				}
				p := patterns[severity][hash]
				if p == nil {
					for _, pp := range patterns[severity] {
						if pp.pattern.WeakEqual(pattern.Pattern) {
							p = pp
							p.Hash = append(p.Hash, hash)
							break
						}
					}
					if p == nil {
						p = &Pattern{
							pattern:       pattern.Pattern,
							Severity:      severity,
							Sample:        pattern.Sample,
							Messages:      timeseries.NewAggregate(timeseries.NanSum),
							Hash:          []string{hash},
							sumByInstance: map[string]*timeseries.Aggregate{},
						}
						patterns[severity][hash] = p
						v.Patterns = append(v.Patterns, p)
					}
				}
				p.Sum += uint64(events)
				p.Messages.Add(pattern.Messages)
				if p.sumByInstance[instance.Name] == nil {
					p.sumByInstance[instance.Name] = timeseries.NewAggregate(timeseries.NanSum)
				}
				p.sumByInstance[instance.Name].Add(pattern.Messages)
			}
		}
	}

	for _, p := range v.Patterns {
		p.Chart = model.NewChart(w.Ctx, "").Column()
		for name, ts := range p.sumByInstance {
			p.Chart.AddSeries(name, ts)
		}
	}
	sort.Slice(v.Patterns, func(i, j int) bool {
		return v.Patterns[i].Sum > v.Patterns[j].Sum
	})

	if !filterByPattern && len(sumBySeverity) > 0 {
		v.Chart = model.NewChart(w.Ctx, "").Column()
		for s, ts := range sumBySeverity {
			v.Chart.AddSeries(string(s), ts.Get())
		}
	}
	if filterByPattern && len(sumByInstance) > 0 {
		v.Chart = model.NewChart(w.Ctx, "").Column()
		for i, ts := range sumByInstance {
			v.Chart.AddSeries(i, ts.Get())
		}
	}
}

func getLogs(ctx context.Context, v *View, clickhouse *tracing.ClickhouseClient, app *model.Application, appSettings *db.ApplicationSettings, w *model.World, q Query) {
	if clickhouse == nil {
		// TODO: v.Message ?
		return
	}
	services, err := clickhouse.GetServiceNamesFromLogs(ctx)
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
		service = tracing.GuessService(services, app.Id)
	}
	var serviceFound, agentFound bool
	for _, s := range services {
		if s == "coroot-node-agent" {
			agentFound = true
		} else {
			if s == service {
				serviceFound = true
			}
			v.Services = append(v.Services, Service{Name: s, Linked: s == service})
		}
	}
	sort.Slice(v.Services, func(i, j int) bool {
		return v.Services[i].Name < v.Services[j].Name
	})

	if serviceFound {
		if q.Source == "" {
			q.Source = tracing.SourceOtel
		}
		v.Sources = append(v.Sources, Source{Type: tracing.SourceOtel, Name: "OpenTelemetry", Selected: q.Source == tracing.SourceOtel})
	}
	if agentFound {
		if q.Source == "" {
			q.Source = tracing.SourceAgent
		}
		v.Sources = append(v.Sources, Source{Type: tracing.SourceAgent, Name: "Container logs", Selected: q.Source == tracing.SourceAgent})
	}

	if !serviceFound && !agentFound {
		v.Status = model.UNKNOWN
		v.Message = "No logs found"
		return
	}

	var entries []*tracing.LogEntry
	switch q.Source {
	case tracing.SourceOtel:
		v.Message = fmt.Sprintf("Using OpenTelemetry logs of <i>%s</i>", service)
		entries, err = clickhouse.GetServiceLogs(ctx, w.Ctx.From, w.Ctx.To, service, q.Severity, q.Hash, q.Search, limit)
	case tracing.SourceAgent:
		v.Message = "Using container logs"
		var containerIds []string
		for _, i := range app.Instances {
			for _, c := range i.Containers {
				containerIds = append(containerIds, c.Id)
			}
		}
		entries, err = clickhouse.GetContainerLogs(ctx, w.Ctx.From, w.Ctx.To, containerIds, q.Severity, q.Hash, q.Search, limit)
	}
	if err != nil {
		klog.Errorln(err)
		v.Status = model.WARNING
		v.Message = fmt.Sprintf("Clickhouse error: %s", err)
		return
	}

	v.Status = model.OK

	for _, e := range entries {
		entry := Entry{
			Timestamp:  e.Timestamp.UnixMilli(),
			Severity:   model.LogSeverity(strings.ToLower(e.Severity)),
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
		v.Entries = append(v.Entries, entry)
	}
	if len(v.Entries) >= limit {
		v.Limit = limit
	}
}
