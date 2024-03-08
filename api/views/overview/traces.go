package overview

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/coroot/coroot/utils"

	"github.com/coroot/coroot/timeseries"

	"golang.org/x/exp/maps"

	"k8s.io/klog"

	"github.com/coroot/coroot/clickhouse"
	"github.com/coroot/coroot/model"
)

const (
	spansLimit      = 100
	attrValuesLimit = 10
)

type Traces struct {
	Message string                     `json:"message"`
	Heatmap *model.Heatmap             `json:"heatmap"`
	Spans   []Span                     `json:"spans"`
	Limit   int                        `json:"limit"`
	Stats   []model.TraceSpanStatsAttr `json:"stats"`
}

type Span struct {
	Service    string                 `json:"service"`
	TraceId    string                 `json:"trace_id"`
	Id         string                 `json:"id"`
	ParentId   string                 `json:"parent_id"`
	Name       string                 `json:"name"`
	Timestamp  int64                  `json:"timestamp"`
	Duration   float64                `json:"duration"`
	Client     string                 `json:"client"`
	Status     model.TraceSpanStatus  `json:"status"`
	Details    model.TraceSpanDetails `json:"details"`
	Attributes map[string]string      `json:"attributes"`
	Events     []model.TraceSpanEvent `json:"events"`
}

type Query struct {
	View    string          `json:"view"`
	TraceId string          `json:"trace_id"`
	TsFrom  timeseries.Time `json:"ts_from"`
	TsTo    timeseries.Time `json:"ts_to"`
	DurFrom string          `json:"dur_from"`
	DurTo   string          `json:"dur_to"`

	durFrom time.Duration
	durTo   time.Duration
	errors  bool
}

func renderTraces(ctx context.Context, ch *clickhouse.Client, w *model.World, query string) *Traces {
	res := &Traces{}

	if ch == nil {
		res.Message = "Clickhouse integration is not configured"
		return res
	}

	q := parseQuery(query, w.Ctx)

	ignoredPeerAddrs := getMonitoringAndControlPlanePodIps(w)

	histogram, err := ch.GetRootSpansHistogram(ctx, ignoredPeerAddrs, w.Ctx.From, w.Ctx.To, w.Ctx.Step)
	if err != nil {
		klog.Errorln(err)
		res.Message = fmt.Sprintf("Clickhouse error: %s", err)
		return res
	}
	if len(histogram) > 1 {
		res.Heatmap = model.NewHeatmap(w.Ctx, "Latency & Errors heatmap, requests per second")
		for _, h := range model.HistogramSeries(histogram[1:], 0, 0) {
			res.Heatmap.AddSeries(h.Name, h.Title, h.Data, h.Threshold, h.Value)
		}
		res.Heatmap.AddSeries("errors", "errors", histogram[0].TimeSeries, "", "err")
	}

	var spans []*model.TraceSpan
	switch {
	case q.TraceId != "":
		spans, err = ch.GetSpansByTraceId(ctx, q.TraceId)
	case q.View == "investigation" && (q.TsFrom != 0 && q.TsTo != 0):
		res.Stats, err = ch.GetSpanAttrsStat(ctx, w.Ctx.From, w.Ctx.To, q.TsFrom, q.TsTo, q.durFrom, q.durTo, q.errors, attrValuesLimit)
	default:
		from, to := q.TsFrom, q.TsTo
		if from == 0 {
			from = w.Ctx.From
		}
		if to == 0 {
			to = w.Ctx.To
		}
		spans, err = ch.GetRootSpans(ctx, ignoredPeerAddrs, from, to, q.durFrom, q.durTo, q.errors, spansLimit)
	}

	if err != nil {
		klog.Errorln(err)
		res.Message = fmt.Sprintf("Clickhouse error: %s", err)
		return res
	}

	if len(spans) == spansLimit {
		res.Limit = spansLimit
	}

	for _, s := range spans {
		ss := Span{
			Service:    s.ServiceName,
			TraceId:    s.TraceId,
			Id:         s.SpanId,
			ParentId:   s.ParentSpanId,
			Name:       s.Name,
			Timestamp:  s.Timestamp.UnixMilli(),
			Duration:   s.Duration.Seconds() * 1000,
			Status:     s.Status(),
			Attributes: map[string]string{},
			Details:    s.Details(),
			Events:     s.Events,
		}
		for name, value := range s.ResourceAttributes {
			ss.Attributes[name] = value
		}
		for name, value := range s.SpanAttributes {
			ss.Attributes[name] = value
		}
		res.Spans = append(res.Spans, ss)
	}

	return res
}

func getMonitoringAndControlPlanePodIps(w *model.World) []string {
	res := map[string]bool{}
	for _, a := range w.Applications {
		if a.Category.Monitoring() || a.Category.ControlPlane() {
			for _, i := range a.Instances {
				for l := range i.TcpListens {
					if ip := net.ParseIP(l.IP); ip != nil && !ip.IsLoopback() {
						res[l.IP] = true
					}
				}
			}
		}
	}
	return maps.Keys(res)
}

func parseQuery(query string, ctx timeseries.Context) Query {
	var res Query
	if query != "" {
		if err := json.Unmarshal([]byte(query), &res); err != nil {
			klog.Warningln(err)
		}
	}
	res.durFrom = utils.ParseHeatmapDuration(res.DurFrom)
	res.durTo = utils.ParseHeatmapDuration(res.DurTo)
	res.errors = res.DurFrom == "inf" || res.DurTo == "err"
	return res
}
