package logs

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/tracing"
	"github.com/coroot/coroot/utils"
	"github.com/coroot/logparser"
	"k8s.io/klog"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

const (
	viewMessages = "messages"
	viewPatterns = "patterns"

	defaultLimit = 100
)

type View struct {
	Status     model.Status     `json:"status"`
	Message    string           `json:"message"`
	Sources    []tracing.Source `json:"sources"`
	Source     tracing.Source   `json:"source"`
	Services   []string         `json:"services"`
	Service    string           `json:"service"`
	Views      []string         `json:"views"`
	View       string           `json:"view"`
	Severities []string         `json:"severities"`
	Severity   []string         `json:"severity"`
	Chart      *model.Chart     `json:"chart"`
	Entries    []Entry          `json:"entries"`
	Patterns   []*Pattern       `json:"patterns"`
	Limit      int              `json:"limit"`
}

type Source struct {
	Type     tracing.Source `json:"type"`
	Name     string         `json:"name"`
	Selected bool           `json:"selected"`
}

type Pattern struct {
	Severity string                `json:"severity"`
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
	Severity   string            `json:"severity"`
	Message    string            `json:"message"`
	Attributes map[string]string `json:"attributes"`
}

type Query struct {
	Source   tracing.Source `json:"source"`
	View     string         `json:"view"`
	Severity []string       `json:"severity"`
	Search   string         `json:"search"`
	Hash     []string       `json:"hash"`
	Limit    int            `json:"limit"`

	severity map[string]bool
	hash     map[string]bool
}

func Render(ctx context.Context, clickhouse *tracing.ClickhouseClient, app *model.Application, appSettings *db.ApplicationSettings, query url.Values, w *model.World) *View {
	v := &View{}

	var q Query
	if qs := query["query"]; len(qs) > 0 {
		if err := json.Unmarshal([]byte(qs[0]), &q); err != nil {
			klog.Warningln(err)
		}
		q.severity = map[string]bool{}
		for _, s := range q.Severity {
			q.severity[s] = true
		}
		q.hash = map[string]bool{}
		for _, h := range q.Hash {
			q.hash[h] = true
		}
	}
	if q.Limit == 0 {
		q.Limit = defaultLimit
	}

	v.View = q.View
	if v.View == "" {
		v.View = viewMessages
	}
	v.Views = append(v.Views, viewMessages)
	getLogs(ctx, v, clickhouse, app, appSettings, w, q)
	if v.Source == tracing.SourceAgent {
		v.Views = append(v.Views, viewPatterns)
		if v.View == viewPatterns {
			getPatterns(v, app, w, q)
		}
	}

	return v
}

func getLogs(ctx context.Context, v *View, clickhouse *tracing.ClickhouseClient, app *model.Application, appSettings *db.ApplicationSettings, w *model.World, q Query) {
	if clickhouse == nil {
		v.Status = model.UNKNOWN
		v.Message = "Clickhouse is not configured: %s"
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
		var ss []string
		for s := range services {
			ss = append(ss, s)
		}
		service = tracing.GuessService(ss, app.Id)
	}
	for s := range services {
		if strings.HasPrefix(s, "/") {
			v.Sources = append(v.Sources, tracing.SourceAgent)
		} else {
			v.Services = append(v.Services, s)
			if s == service {
				v.Service = s
				v.Sources = append(v.Sources, tracing.SourceOtel)
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
			v.Source = tracing.SourceOtel
		} else {
			v.Source = tracing.SourceAgent
		}
	}
	v.Severity = q.Severity

	var hist map[string]*timeseries.TimeSeries
	var entries []*tracing.LogEntry
	switch v.Source {
	case tracing.SourceOtel:
		v.Message = fmt.Sprintf("Using OpenTelemetry logs of <i>%s</i>", service)
		v.Severities = services[v.Service]
		if len(v.Severity) == 0 {
			v.Severity = v.Severities
		}
		if v.View == viewMessages {
			if len(q.Hash) == 0 && q.Search == "" {
				hist, err = clickhouse.GetServiceLogsHistogram(ctx, w.Ctx.From, w.Ctx.To, w.Ctx.Step, service, v.Severity)
			}
			if err == nil {
				entries, err = clickhouse.GetServiceLogs(ctx, w.Ctx.From, w.Ctx.To, service, v.Severity, q.Hash, q.Search, q.Limit)
			}
		}
	case tracing.SourceAgent:
		v.Message = "Using container logs"
		containerIds := map[string][]string{}
		for _, i := range app.Instances {
			for _, c := range i.Containers {
				s := containerIdToServiceName(c.Id)
				containerIds[s] = append(containerIds[s], c.Id)
			}
		}
		severities := utils.NewStringSet()
		for s := range containerIds {
			severities.Add(services[s]...)
		}
		v.Severities = severities.Items()
		if len(v.Severity) == 0 {
			v.Severity = v.Severities
		}
		if v.View == viewMessages {
			if len(q.Hash) == 0 && q.Search == "" {
				hist, err = clickhouse.GetContainerLogsHistogram(ctx, w.Ctx.From, w.Ctx.To, w.Ctx.Step, containerIds, v.Severity)
			}
			if err == nil {
				entries, err = clickhouse.GetContainerLogs(ctx, w.Ctx.From, w.Ctx.To, containerIds, v.Severity, q.Hash, q.Search, q.Limit)
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

	switch {
	case len(hist) > 0:
		v.Chart = model.NewChart(w.Ctx, "").Column()
		v.Chart.Flags = "severity"
		for severity, ts := range hist {
			v.Chart.AddSeries(severity, ts)
		}
	case len(q.Hash) > 0:
		v.Chart = model.NewChart(w.Ctx, "").Column()
		for name, ts := range sumByInstance(app, q) {
			v.Chart.AddSeries(name, ts)
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
		v.Entries = append(v.Entries, entry)
	}
	if len(v.Entries) >= q.Limit {
		v.Limit = q.Limit
	}
}

func getPatterns(v *View, app *model.Application, w *model.World, q Query) {
	patterns := map[string]map[string]*Pattern{}
	for _, instance := range app.Instances {
		for s, msgs := range instance.LogMessages {
			severity := string(s)
			if len(q.severity) > 0 && !q.severity[severity] {
				continue
			}
			for hash, pattern := range msgs.Patterns {
				events := pattern.Messages.Reduce(timeseries.NanSum)
				if timeseries.IsNaN(events) || events == 0 {
					continue
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

	bySeverity := map[string]*timeseries.Aggregate{}
	for _, p := range v.Patterns {
		if bySeverity[p.Severity] == nil {
			bySeverity[p.Severity] = timeseries.NewAggregate(timeseries.NanSum)
		}
		bySeverity[p.Severity].Add(p.Messages.Get())
		p.Chart = model.NewChart(w.Ctx, "").Column()
		for name, ts := range p.sumByInstance {
			p.Chart.AddSeries(name, ts)
		}
	}
	if len(bySeverity) > 0 {
		v.Chart = model.NewChart(w.Ctx, "").Column()
		v.Chart.Flags = "severity"
		for s, ts := range bySeverity {
			v.Chart.AddSeries(string(s), ts.Get())
		}
		sort.Slice(v.Patterns, func(i, j int) bool {
			return v.Patterns[i].Sum > v.Patterns[j].Sum
		})
	}
}

func sumByInstance(app *model.Application, q Query) map[string]*timeseries.Aggregate {
	res := map[string]*timeseries.Aggregate{}
	for _, instance := range app.Instances {
		for _, msgs := range instance.LogMessages {
			for hash, pattern := range msgs.Patterns {
				if !q.hash[hash] {
					continue
				}
				if res[instance.Name] == nil {
					res[instance.Name] = timeseries.NewAggregate(timeseries.NanSum)
				}
				res[instance.Name].Add(pattern.Messages)
			}
		}
	}
	return res
}

var (
	deploymentPodRegex  = regexp.MustCompile(`(/k8s/[a-z0-9-]+/[a-z0-9-]+)-[0-9a-f]{1,10}-[bcdfghjklmnpqrstvwxz2456789]{5}/.+`)
	daemonsetPodRegex   = regexp.MustCompile(`(/k8s/[a-z0-9-]+/[a-z0-9-]+)-[bcdfghjklmnpqrstvwxz2456789]{5}/.+`)
	statefulsetPodRegex = regexp.MustCompile(`(/k8s/[a-z0-9-]+/[a-z0-9-]+)-\d+/.+`)
)

func containerIdToServiceName(containerId string) string {
	if !strings.HasPrefix(containerId, "/k8s/") {
		return containerId
	}
	for _, r := range []*regexp.Regexp{deploymentPodRegex, daemonsetPodRegex, statefulsetPodRegex} {
		if g := r.FindStringSubmatch(containerId); len(g) == 2 {
			return g[1]
		}
	}
	return containerId
}
