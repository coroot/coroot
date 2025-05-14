package overview

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/coroot/coroot/clickhouse"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/utils"
	"k8s.io/klog"
)

const (
	defaultLimit = 100
)

type Logs struct {
	Message string       `json:"message"`
	Error   string       `json:"error"`
	Chart   *model.Chart `json:"chart"`
	Entries []LogEntry   `json:"entries"`
	Suggest []string     `json:"suggest"`
}

type LogEntry struct {
	Application string            `json:"application"`
	Timestamp   int64             `json:"timestamp"`
	Severity    string            `json:"severity"`
	Color       string            `json:"color"`
	Message     string            `json:"message"`
	Attributes  map[string]string `json:"attributes"`
	TraceId     string            `json:"trace_id"`
}

type LogsQuery struct {
	View    string                 `json:"view"`
	Agent   bool                   `json:"agent"`
	Otel    bool                   `json:"otel"`
	Filters []clickhouse.LogFilter `json:"filters"`
	Limit   int                    `json:"limit"`
	Suggest *string                `json:"suggest,omitempty"`
}

func renderLogs(ctx context.Context, ch *clickhouse.Client, w *model.World, query string) *Logs {
	v := &Logs{}

	if ch == nil {
		v.Message = "Clickhouse integration is not configured."
		return v
	}

	var q LogsQuery
	if query != "" {
		if err := json.Unmarshal([]byte(query), &q); err != nil {
			klog.Warningln(err)
		}
	}
	if !q.Agent && !q.Otel {
		return v
	}
	if q.Limit <= 0 {
		q.Limit = defaultLimit
	}

	lq := clickhouse.LogQuery{
		Ctx:     w.Ctx,
		Filters: q.Filters,
		Limit:   q.Limit,
	}

	if !q.Agent || !q.Otel {
		if q.Agent {
			lq.Source = model.LogSourceAgent
		}
		if q.Otel {
			lq.Source = model.LogSourceOtel
		}
	}

	var histogram []model.LogHistogramBucket
	var entries []*model.LogEntry
	var err error
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
		v.Error = fmt.Sprintf("Clickhouse error: %s", err)
		return v
	}

	if len(histogram) > 0 {
		v.Chart = model.NewChart(w.Ctx, "").Column().Sorted()
		for _, b := range histogram {
			v.Chart.AddSeries(b.Severity.String(), b.Timeseries, b.Severity.Color())
		}
	}

	v.renderEntries(entries, w)

	return v
}

func (v *Logs) renderEntries(entries []*model.LogEntry, w *model.World) {
	if len(entries) == 0 {
		return
	}

	ss := utils.NewStringSet()
	for _, e := range entries {
		ss.Add(e.ServiceName)
	}
	services := ss.Items()

	apps := map[string]*model.Application{}
	for _, app := range w.Applications {
		for _, i := range app.Instances {
			for _, c := range i.Containers {
				apps[model.ContainerIdToServiceName(c.Id)] = app
			}
		}
		if settings := app.Settings; settings != nil && settings.Logs != nil && settings.Logs.Service != "" {
			apps[settings.Logs.Service] = app
		} else if service := model.GuessService(services, app.Id); service != "" {
			apps[service] = app
		}
	}

	for _, e := range entries {
		entry := LogEntry{
			Application: e.ServiceName,
			Timestamp:   e.Timestamp.UnixMilli(),
			Severity:    e.Severity.String(),
			Color:       e.Severity.Color(),
			Message:     e.Body,
			Attributes:  map[string]string{},
			TraceId:     e.TraceId,
		}
		if app := apps[e.ServiceName]; app != nil {
			entry.Application = app.Id.String()
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
}
