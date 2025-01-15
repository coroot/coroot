package logs

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
	"github.com/coroot/coroot/utils"
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
	Severity string       `json:"severity"`
	Sample   string       `json:"sample"`
	Sum      uint64       `json:"sum"`
	Chart    *model.Chart `json:"chart"`
	Hash     string       `json:"hash"`
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

func Render(ctx context.Context, ch *clickhouse.Client, app *model.Application, query url.Values, w *model.World) *View {
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

	defer func() {
		if v.Chart != nil {
			events := model.EventsToAnnotations(app.Events, w.Ctx)
			incidents := model.IncidentsToAnnotations(app.Incidents, w.Ctx)
			v.Chart.AddAnnotation(events...).AddAnnotation(incidents...)
		}
	}()

	if ch == nil {
		v.Status = model.UNKNOWN
		v.Message = "Clickhouse integration is not configured"
		v.View = viewPatterns
		renderPatterns(v, app, w.Ctx)
		return v
	}

	v.View = q.View
	if v.View == "" {
		v.View = viewMessages
	}
	renderEntries(ctx, v, ch, app, w, q)

	if v.Status == model.UNKNOWN {
		v.View = viewPatterns
		renderPatterns(v, app, w.Ctx)
		return v
	}

	v.Views = append(v.Views, viewMessages)
	if v.Source == model.LogSourceAgent {
		v.Views = append(v.Views, viewPatterns)
		if v.View == viewPatterns {
			renderPatterns(v, app, w.Ctx)
		}
	}
	return v
}

func renderEntries(ctx context.Context, v *View, ch *clickhouse.Client, app *model.Application, w *model.World, q Query) {
	services, err := ch.GetServicesFromLogs(ctx, w.Ctx.From)
	if err != nil {
		klog.Errorln(err)
		v.Status = model.WARNING
		v.Message = fmt.Sprintf("Clickhouse error: %s", err)
		return
	}

	var logsFromAgentFound bool
	var otelServices []string
	for s := range services {
		if strings.HasPrefix(s, "/") {
			logsFromAgentFound = true
		} else {
			otelServices = append(otelServices, s)
		}
	}
	otelService := ""
	if app.Settings != nil && app.Settings.Logs != nil {
		otelService = app.Settings.Logs.Service
	} else {
		otelService = model.GuessService(otelServices, app.Id)
	}

	if logsFromAgentFound {
		v.Sources = append(v.Sources, model.LogSourceAgent)
	}

	for _, s := range otelServices {
		if s == otelService {
			v.Service = s
			v.Sources = append(v.Sources, model.LogSourceOtel)
		}
		v.Services = append(v.Services, s)
	}
	sort.Strings(v.Services)

	if len(v.Sources) == 0 {
		v.Status = model.UNKNOWN
		v.Message = "No logs found in ClickHouse"
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
		v.Message = fmt.Sprintf("Using OpenTelemetry logs of <i>%s</i>", otelService)
		v.Severities = services[v.Service]
		if len(v.Severity) == 0 {
			v.Severity = v.Severities
		}
		if v.View == viewMessages {
			histogram, err = ch.GetServiceLogsHistogram(ctx, w.Ctx.From, w.Ctx.To, w.Ctx.Step, otelService, v.Severity, q.Search)
			if err == nil {
				entries, err = ch.GetServiceLogs(ctx, w.Ctx.From, w.Ctx.To, otelService, v.Severity, q.Search, q.Limit)
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
				hashes = getSimilarHashes(app, q.Hash)
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

func renderPatterns(v *View, app *model.Application, ctx timeseries.Context) {
	bySeverity := map[string]*timeseries.Aggregate{}
	for level, msgs := range app.LogMessages {
		for hash, pattern := range msgs.Patterns {
			sum := pattern.Messages.Reduce(timeseries.NanSum)
			if timeseries.IsNaN(sum) || sum == 0 {
				continue
			}
			severity := string(level)
			if bySeverity[severity] == nil {
				bySeverity[severity] = timeseries.NewAggregate(timeseries.NanSum)
			}
			bySeverity[severity].Add(pattern.Messages)
			p := &Pattern{
				Severity: severity,
				Sample:   pattern.Sample,
				Sum:      uint64(sum),
				Chart:    model.NewChart(ctx, "").AddSeries(severity, pattern.Messages).Column().Legend(false),
				Hash:     hash,
			}
			v.Patterns = append(v.Patterns, p)
		}
	}
	sort.Slice(v.Patterns, func(i, j int) bool {
		return v.Patterns[i].Sum > v.Patterns[j].Sum
	})
	if len(bySeverity) > 0 {
		v.Chart = model.NewChart(ctx, "").Column()
		for severity, ts := range bySeverity {
			v.Chart.AddSeries(severity, ts.Get())
		}
	}
}

func getSimilarHashes(app *model.Application, hash string) []string {
	res := utils.NewStringSet()
	for _, msgs := range app.LogMessages {
		for _, pattern := range msgs.Patterns {
			if similar := pattern.SimilarPatternHashes; similar != nil {
				if similar.Has(hash) {
					res.Add(similar.Items()...)
				}
			}
		}
	}
	return res.Items()
}
