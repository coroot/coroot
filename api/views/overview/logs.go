package overview

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/coroot/coroot/clickhouse"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
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
	MaxTs   string       `json:"max_ts"` // string because in JS: 1756993779510773600 === 1756993779510773500
}

type LogEntry struct {
	Application string            `json:"application"`
	Timestamp   int64             `json:"timestamp"`
	Severity    string            `json:"severity"`
	Color       string            `json:"color"`
	Message     string            `json:"message"`
	Attributes  map[string]string `json:"attributes"`
	TraceId     string            `json:"trace_id"`
	Cluster     string            `json:"cluster"`
}

type LogsQuery struct {
	View    string                 `json:"view"`
	Agent   bool                   `json:"agent"`
	Otel    bool                   `json:"otel"`
	Filters []clickhouse.LogFilter `json:"filters"`
	Limit   int                    `json:"limit"`
	Suggest *string                `json:"suggest,omitempty"`
	Since   string                 `json:"since"`
}

func renderLogs(ctx context.Context, chs []*clickhouse.Client, w *model.World, query string) *Logs {
	v := &Logs{}

	if len(chs) == 0 {
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
	lq := clickhouse.LogQuery{Ctx: w.Ctx, Limit: q.Limit}
	var clusterFilter *clickhouse.LogFilter

	for _, f := range q.Filters {
		if f.Name == "Cluster" {
			clusterFilter = &f
		} else {
			lq.Filters = append(lq.Filters, f)
		}
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
	var items []string
	suggest := utils.NewStringSet()
	bySeverity := map[model.Severity]*timeseries.Aggregate{}
	var overallEntries []*model.LogEntry

	for _, ch := range chs {
		if !clusterFilter.Matches(ch.Project().Name) {
			continue
		}
		if q.Suggest != nil {
			items, err = ch.GetLogFilters(ctx, lq, *q.Suggest)
			suggest.Add(items...)
		} else {
			histogram, err = ch.GetLogsHistogram(ctx, lq)
			for _, b := range histogram {
				agg := bySeverity[b.Severity]
				if agg == nil {
					agg = timeseries.NewAggregate(timeseries.NanSum)
					bySeverity[b.Severity] = agg
				}
				agg.Add(b.Timeseries)
			}
			if err == nil {
				if q.Since != "" {
					if i, _ := strconv.ParseInt(q.Since, 10, 64); i > 0 {
						lq.Since = time.Unix(0, i)
					}
				}
				entries, err = ch.GetLogs(ctx, lq)
				overallEntries = append(overallEntries, entries...)
			}
		}
		if err != nil {
			klog.Errorln(err)
			v.Error = fmt.Sprintf("Clickhouse error: %s", err)
			return v
		}
	}
	v.Suggest = suggest.Items()

	if len(bySeverity) > 0 {
		v.Chart = model.NewChart(w.Ctx, "").Column().Sorted()
		for severity, agg := range bySeverity {
			v.Chart.AddSeries(severity.String(), agg, severity.Color())
		}
	}

	v.renderEntries(overallEntries, w, q.Limit)
	return v
}

func (v *Logs) renderEntries(entries []*model.LogEntry, w *model.World, limit int) {
	if len(entries) == 0 {
		return
	}

	ss := utils.NewStringSet()
	for _, e := range entries {
		ss.Add(e.ServiceName)
	}
	services := ss.Items()

	type key struct {
		service   string
		clusterId string
	}

	apps := map[key]*model.Application{}
	for _, app := range w.Applications {
		for _, i := range app.Instances {
			for _, c := range i.Containers {
				apps[key{service: model.ContainerIdToServiceName(c.Id), clusterId: app.Id.ClusterId}] = app
			}
		}
		if settings := app.Settings; settings != nil && settings.Logs != nil && settings.Logs.Service != "" {
			apps[key{service: settings.Logs.Service, clusterId: app.Id.ClusterId}] = app
		} else if service := model.GuessService(services, w, app); service != "" {
			apps[key{service: service, clusterId: app.Id.ClusterId}] = app
		}
	}

	var maxTs int64
	for _, e := range entries {
		entry := LogEntry{
			Application: e.ServiceName,
			Timestamp:   e.Timestamp.UnixMilli(),
			Severity:    e.Severity.String(),
			Color:       e.Severity.Color(),
			Message:     e.Body,
			Attributes:  map[string]string{},
			TraceId:     e.TraceId,
			Cluster:     e.ClusterName,
		}
		if app := apps[key{service: e.ServiceName, clusterId: e.ClusterId}]; app != nil {
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
		entry.Attributes["Cluster"] = e.ClusterName
		v.Entries = append(v.Entries, entry)
		maxTs = max(maxTs, e.Timestamp.UnixNano())
	}
	sort.Slice(v.Entries, func(i, j int) bool {
		return v.Entries[i].Timestamp > v.Entries[j].Timestamp
	})
	if len(v.Entries) > limit {
		v.Entries = v.Entries[:limit]
	}
	if maxTs != 0 {
		v.MaxTs = strconv.FormatInt(maxTs, 10)
	}
}
