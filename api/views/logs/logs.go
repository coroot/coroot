package logs

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
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
	Status   model.Status      `json:"status"`
	Message  string            `json:"message"`
	Sources  []model.LogSource `json:"sources"`
	Source   model.LogSource   `json:"source"`
	Services []string          `json:"services"`
	Service  string            `json:"service"`
	View     string            `json:"view"`
	Chart    *model.Chart      `json:"chart"`
	Entries  []Entry           `json:"entries"`
	Patterns []*Pattern        `json:"patterns"`
	Limit    int               `json:"limit"`
	Suggest  []string          `json:"suggest"`
}

type Pattern struct {
	Severity string       `json:"severity"`
	Color    string       `json:"color"`
	Sample   string       `json:"sample"`
	Sum      uint64       `json:"sum"`
	Chart    *model.Chart `json:"chart"`
	Hash     string       `json:"hash"`
}

type Entry struct {
	Timestamp  int64             `json:"timestamp"`
	Severity   string            `json:"severity"`
	Color      string            `json:"color"`
	Message    string            `json:"message"`
	Attributes map[string]string `json:"attributes"`
	TraceId    string            `json:"trace_id"`
}

type Query struct {
	Source  model.LogSource        `json:"source"`
	View    string                 `json:"view"`
	Filters []clickhouse.LogFilter `json:"filters"`
	Limit   int                    `json:"limit"`
	Suggest *string                `json:"suggest,omitempty"`
}

func Render(ctx context.Context, ch *clickhouse.Client, app *model.Application, query url.Values, w *model.World) *View {
	v := &View{
		Status: model.OK,
	}

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

	v.View = cmp.Or(q.View, viewMessages)

	services, err := ch.GetServicesFromLogs(ctx, w.Ctx.From)
	if err != nil {
		klog.Errorln(err)
		v.Status = model.WARNING
		v.Message = fmt.Sprintf("Clickhouse error: %s", err)
		return v
	}

	var logsFromAgentFound bool
	var otelServices []string
	for _, s := range services {
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
	slices.Sort(v.Services)

	if len(v.Sources) == 0 {
		v.Status = model.UNKNOWN
		v.Message = "No logs found in ClickHouse"
		v.View = viewPatterns
		renderPatterns(v, app, w.Ctx)
		return v
	}

	v.Source = q.Source
	if v.Source == "" {
		if v.Service != "" {
			v.Source = model.LogSourceOtel
		} else {
			v.Source = model.LogSourceAgent
		}
	}
	switch v.Source {
	case model.LogSourceOtel:
		v.Message = fmt.Sprintf("Using OpenTelemetry logs of <i>%s</i>", otelService)
	case model.LogSourceAgent:
		v.Message = "Using container logs"
	}

	switch v.View {
	case viewPatterns:
		renderPatterns(v, app, w.Ctx)
	case viewMessages:
		renderEntries(ctx, v, ch, app, w, q, otelService)
	}

	return v
}

func renderEntries(ctx context.Context, v *View, ch *clickhouse.Client, app *model.Application, w *model.World, q Query, otelService string) {
	var err error
	lq := clickhouse.LogQuery{
		Ctx:     w.Ctx,
		Filters: q.Filters,
		Limit:   q.Limit,
	}
	switch v.Source {
	case model.LogSourceOtel:
		lq.Services = []string{otelService}
	case model.LogSourceAgent:
		lq.Services = getServices(app)
		hashes := utils.NewStringSet()
		for _, f := range q.Filters {
			if f.Name == "pattern.hash" {
				hashes.Add(getSimilarHashes(app, f.Value)...)
			}
		}
		for _, hash := range hashes.Items() {
			lq.Filters = append(lq.Filters, clickhouse.LogFilter{Name: "pattern.hash", Op: "=", Value: hash})
		}
	}

	var histogram []model.LogHistogramBucket
	var entries []*model.LogEntry
	if q.Suggest != nil {
		v.Suggest, err = ch.GetLogFilters(ctx, lq, *q.Suggest)
	} else {
		histogram, err = ch.GetLogsHistogram(ctx, lq)
		if err == nil {
			entries, err = ch.GetLogs(ctx, lq)
		}
	}

	if err != nil {
		klog.Errorln(err)
		v.Status = model.WARNING
		v.Message = fmt.Sprintf("Clickhouse error: %s", err)
		return
	}

	if len(histogram) > 0 {
		v.Chart = model.NewChart(w.Ctx, "").Column().Sorted()
		for _, b := range histogram {
			v.Chart.AddSeries(b.Severity.String(), b.Timeseries, b.Severity.Color())
		}
	}

	for _, e := range entries {
		entry := Entry{
			Timestamp:  e.Timestamp.UnixMilli(),
			Severity:   e.Severity.String(),
			Color:      e.Severity.Color(),
			Message:    e.Body,
			Attributes: map[string]string{},
			TraceId:    e.TraceId,
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

func renderPatterns(v *View, app *model.Application, ctx timeseries.Context) {
	bySeverity := map[model.Severity]*timeseries.Aggregate{}
	for severity, msgs := range app.LogMessages {
		for hash, pattern := range msgs.Patterns {
			sum := pattern.Messages.Reduce(timeseries.NanSum)
			if timeseries.IsNaN(sum) || sum == 0 {
				continue
			}
			if bySeverity[severity] == nil {
				bySeverity[severity] = timeseries.NewAggregate(timeseries.NanSum)
			}
			bySeverity[severity].Add(pattern.Messages)
			p := &Pattern{
				Severity: severity.String(),
				Color:    severity.Color(),
				Sample:   pattern.Sample,
				Sum:      uint64(sum),
				Chart:    model.NewChart(ctx, "").AddSeries(severity.String(), pattern.Messages, severity.Color()).Column().Legend(false),
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
			v.Chart.AddSeries(severity.String(), ts.Get(), severity.Color())
		}
	}
}

func getServices(app *model.Application) []string {
	res := utils.NewStringSet()
	for _, i := range app.Instances {
		for _, c := range i.Containers {
			res.Add(model.ContainerIdToServiceName(c.Id))
		}
	}
	return res.Items()
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
