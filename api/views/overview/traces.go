package overview

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/coroot/coroot/clickhouse"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"golang.org/x/exp/maps"
	"k8s.io/klog"
)

const (
	spansLimit      = 100
	attrValuesLimit = 10
)

type Traces struct {
	Message   string                     `json:"message"`
	Error     string                     `json:"error"`
	Heatmap   *model.Heatmap             `json:"heatmap"`
	Traces    []Span                     `json:"traces"`
	Limit     int                        `json:"limit"`
	Trace     []Span                     `json:"trace"`
	Summary   *model.TraceSpanSummary    `json:"summary"`
	AttrStats []model.TraceSpanAttrStats `json:"attr_stats"`
	Errors    []model.TraceErrorsStat    `json:"errors"`
	Latency   *model.Profile             `json:"latency"`
}

type Span struct {
	Service    string                 `json:"service"`
	TraceId    string                 `json:"trace_id"`
	Id         string                 `json:"id"`
	ParentId   string                 `json:"parent_id"`
	Name       string                 `json:"name"`
	Timestamp  int64                  `json:"timestamp"`
	Duration   float64                `json:"duration"`
	Status     model.TraceSpanStatus  `json:"status"`
	Details    model.TraceSpanDetails `json:"details"`
	Attributes map[string]string      `json:"attributes"`
	Events     []Event                `json:"events"`
	Cluster    string                 `json:"cluster"`
}

type Event struct {
	Timestamp  int64             `json:"timestamp"`
	Name       string            `json:"name"`
	Attributes map[string]string `json:"attributes"`
}

type Query struct {
	View    string          `json:"view"`
	TsFrom  timeseries.Time `json:"ts_from"`
	TsTo    timeseries.Time `json:"ts_to"`
	DurFrom string          `json:"dur_from"`
	DurTo   string          `json:"dur_to"`

	TraceId    string   `json:"trace_id"`
	Filters    []Filter `json:"filters"`
	IncludeAux bool     `json:"include_aux"`
	Diff       bool     `json:"diff"`

	durFrom time.Duration
	durTo   time.Duration
	errors  bool
}

type Filter struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value string `json:"value"`
}

func renderTraces(ctx context.Context, chs []*clickhouse.Client, w *model.World, query string) *Traces {
	res := &Traces{}

	if len(chs) == 0 {
		res.Message = "Clickhouse integration is not configured."
		return res
	}

	q := parseQuery(query, w.Ctx)

	sq := clickhouse.SpanQuery{Ctx: w.Ctx}

	for _, f := range q.Filters {
		sq.AddFilter(f.Field, f.Op, f.Value)
	}

	if !q.IncludeAux {
		sq.ExcludePeerAddrs = getMonitoringAndControlPlanePodIps(w)
		sq.AddFilter("SpanName", "!~", "GET /(health[z]*|metrics|debug/.+|actuator/.+)")
	}

	byLe := map[float32]*timeseries.Aggregate{}

	for _, ch := range chs {
		histogram, err := ch.GetRootSpansHistogram(ctx, sq)
		if err != nil {
			klog.Errorln(err)
			res.Error = fmt.Sprintf("Clickhouse error: %s", err)
			return res
		}
		for _, h := range histogram {
			agg := byLe[h.Le]
			if agg == nil {
				agg = timeseries.NewAggregate(timeseries.NanSum)
				byLe[h.Le] = agg
			}
			agg.Add(h.TimeSeries)
		}
	}
	if len(byLe) > 1 {
		hist := make([]model.HistogramBucket, 0, len(byLe)-1)
		var errors *timeseries.Aggregate
		for le, agg := range byLe {
			if le == 0 {
				errors = agg
			} else {
				hist = append(hist, model.HistogramBucket{Le: le, TimeSeries: agg.Get()})
			}
		}
		sort.Slice(hist, func(i, j int) bool { return hist[i].Le < hist[j].Le })
		res.Heatmap = model.NewHeatmap(w.Ctx, "Latency & Errors heatmap, requests per second")
		for _, h := range model.HistogramSeries(hist, 0, 0) {
			res.Heatmap.AddSeries(h.Name, h.Title, h.Data, h.Threshold, h.Value)
		}
		res.Heatmap.AddSeries("errors", "errors", errors, "", "err")
	} else {
		services := utils.NewStringSet()
		for _, ch := range chs {
			svcs, err := ch.GetServicesFromTraces(ctx, w.Ctx.From)
			if err != nil {
				klog.Errorln(err)
				res.Error = fmt.Sprintf("Clickhouse error: %s", err)
				return res
			}
			services.Add(svcs...)
		}

		var otelTracesFound bool
		for _, s := range services.Items() {
			if !strings.HasPrefix(s, "/") {
				otelTracesFound = true
				break
			}
		}
		if !otelTracesFound {
			res.Message = "not_found"
			return res
		}
	}

	sq.TsFrom = q.TsFrom
	if sq.TsFrom == 0 {
		sq.TsFrom = sq.Ctx.From
	}
	sq.TsTo = q.TsTo
	if sq.TsTo == 0 {
		sq.TsTo = sq.Ctx.To
	}
	sq.DurFrom = q.durFrom
	sq.DurTo = q.durTo
	sq.Errors = q.errors
	sq.Limit = spansLimit
	sq.Diff = q.Diff

	var overallSpans []*model.TraceSpan
	overallErrors := map[model.TraceSpanKey]*model.TraceErrorsStat{}
	overallSpanStats := map[model.TraceSpanKey]*model.TraceSpanStats{}
	totalStats := &model.TraceSpanStats{Histogram: map[float32]float32{}}
	totalErrors := float32(0)
	var overallSelectionTraces, overallBaselineTraces []*model.Trace

	for _, ch := range chs {
		switch {
		case q.TraceId != "":
			spans, err := ch.GetSpansByTraceId(ctx, q.TraceId)
			if err != nil {
				klog.Errorln(err)
				res.Error = fmt.Sprintf("Clickhouse error: %s", err)
				return res
			}
			overallSpans = append(overallSpans, spans...)
		case q.View == "traces":
			spans, err := ch.GetRootSpans(ctx, sq)
			if err != nil {
				klog.Errorln(err)
				res.Error = fmt.Sprintf("Clickhouse error: %s", err)
				return res
			}
			overallSpans = append(overallSpans, spans...)
		case q.View == "attributes":
			sq.Limit = attrValuesLimit
			sq.Diff = true
			selectionTraces, baselineTraces, err := ch.GetSelectionAndBaselineTraces(ctx, sq)
			if err != nil {
				klog.Errorln(err)
				res.Error = fmt.Sprintf("Clickhouse error: %s", err)
				return res
			}
			overallSelectionTraces = append(overallSelectionTraces, selectionTraces...)
			overallBaselineTraces = append(overallBaselineTraces, baselineTraces...)
		case q.View == "errors":
			errors, err := ch.GetTraceErrors(ctx, sq)
			if err != nil {
				klog.Errorln(err)
				res.Error = fmt.Sprintf("Clickhouse error: %s", err)
				return res
			}
			for k, v := range errors {
				totalErrors += v.Count
				existing := overallErrors[k]
				if existing == nil {
					overallErrors[k] = v
				} else {
					existing.Count += v.Count
				}
			}
		case q.View == "latency":
			selectionTraces, baselineTraces, err := ch.GetSelectionAndBaselineTraces(ctx, sq)
			if err != nil {
				klog.Errorln(err)
				res.Error = fmt.Sprintf("Clickhouse error: %s", err)
				return res
			}
			overallSelectionTraces = append(overallSelectionTraces, selectionTraces...)
			overallBaselineTraces = append(overallBaselineTraces, baselineTraces...)
		default:
			stats, err := ch.GetTraceSpanStats(ctx, sq)
			if err != nil {
				klog.Errorln(err)
				res.Error = fmt.Sprintf("Clickhouse error: %s", err)
				return res
			}
			for k, v := range stats {
				s := overallSpanStats[k]
				if s == nil {
					overallSpanStats[k] = v
				} else {
					s.Failed += v.Failed
					s.Total += v.Total
					for b, vv := range v.Histogram {
						s.Histogram[b] = vv
					}
				}
				totalStats.Total += v.Total
				totalStats.Failed += v.Failed
				for b, vv := range v.Histogram {
					totalStats.Histogram[b] = vv
				}
			}
		}

	}
	switch q.View {
	case "attributes":
		res.AttrStats = spanAttrStats(overallSelectionTraces, overallBaselineTraces, sq.Limit)
	case "latency":
		fgBase := getTraceLatencyFlamegraph(overallBaselineTraces)
		fgComp := getTraceLatencyFlamegraph(overallSelectionTraces)

		res.Latency = &model.Profile{Type: "::nanoseconds"}
		if q.Diff {
			fgBase.Diff(fgComp)
			res.Latency.FlameGraph = fgBase
			res.Latency.Diff = true
		} else {
			if sq.IsSelectionDefined() {
				res.Latency.FlameGraph = fgComp
			} else {
				res.Latency.FlameGraph = fgBase
			}
		}
	case "errors":
		for _, v := range overallErrors {
			v.Count /= totalErrors
			res.Errors = append(res.Errors, *v)
		}
	default:
		if len(overallSpanStats) > 0 {
			res.Summary = &model.TraceSpanSummary{}
			duration := sq.TsTo.Sub(sq.TsFrom)
			klog.Infoln(duration)
			quantiles := []float32{0.5, 0.95, 0.99}
			for _, v := range overallSpanStats {
				v.DurationQuantiles = getQuantiles(v.Histogram, quantiles)
				v.Failed /= v.Total
				v.Total /= float32(duration)
				res.Summary.Stats = append(res.Summary.Stats, *v)
			}
			v := *totalStats
			v.DurationQuantiles = getQuantiles(v.Histogram, quantiles)
			v.Failed /= v.Total
			v.Total /= float32(duration)
			res.Summary.Overall = v
		}
	}

	if len(overallSpans) == spansLimit {
		res.Limit = spansLimit
	}

	for _, s := range overallSpans {
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
		}
		ss.Cluster = s.ClusterName
		ss.Attributes["Cluster"] = s.ClusterName
		for name, value := range s.ResourceAttributes {
			ss.Attributes[name] = value
		}
		for name, value := range s.SpanAttributes {
			ss.Attributes[name] = value
		}
		for _, e := range s.Events {
			ss.Events = append(ss.Events, Event{
				Timestamp:  e.Timestamp.UnixMilli(),
				Name:       e.Name,
				Attributes: e.Attributes,
			})
		}
		if q.TraceId != "" {
			res.Trace = append(res.Trace, ss)
		} else {
			if len(res.Traces) >= spansLimit {
				res.Limit = spansLimit
				break
			}
			res.Traces = append(res.Traces, ss)
		}
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

func getTraceLatencyFlamegraph(traces []*model.Trace) *model.FlameGraphNode {
	if len(traces) == 0 {
		return nil
	}
	byParent := map[string][]*model.TraceSpan{}
	for _, t := range traces {
		for _, s := range t.Spans {
			byParent[s.ParentSpanId] = append(byParent[s.ParentSpanId], s)
		}
	}

	root := &model.FlameGraphNode{Name: "total"}
	addChildrenSpans(root, byParent, "")
	for _, ch := range root.Children {
		root.Total += ch.Total
		if root.Data == nil {
			root.Data = ch.Data
		}
	}
	root.Self = 0
	return root

}

func addChildrenSpans(node *model.FlameGraphNode, byParent map[string][]*model.TraceSpan, parentId string) {
	spans := byParent[parentId]
	if len(spans) == 0 {
		return
	}

	durations := map[*model.TraceSpan]int64{}
	if parentId == "" {
		for _, s := range spans {
			durations[s] = s.Duration.Nanoseconds()
		}
	} else {
		intervalSet := map[int64]bool{}
		for _, s := range spans {
			intervalSet[s.Timestamp.UnixNano()] = true
			intervalSet[s.Timestamp.Add(s.Duration).UnixNano()] = true
		}
		intervals := maps.Keys(intervalSet)
		sort.Slice(intervals, func(i, j int) bool { return intervals[i] < intervals[j] })
		for i := range intervals[:len(intervals)-1] {
			from, to := intervals[i], intervals[i+1]
			var ss []*model.TraceSpan
			for _, s := range spans {
				if s.Timestamp.UnixNano() <= from && s.Timestamp.Add(s.Duration).UnixNano() >= to {
					ss = append(ss, s)
				}
			}
			for _, s := range ss {
				durations[s] += (to - from) / int64(len(ss))
			}
		}
	}

	for _, s := range spans {
		var child *model.FlameGraphNode
		name := s.ServiceName + ": " + s.Name + " " + s.Labels().String()
		for _, n := range node.Children {
			if n.Name == name {
				child = n
				break
			}
		}
		if child == nil {
			child = &model.FlameGraphNode{Name: name, ColorBy: s.ServiceName, Data: map[string]string{"trace_id": s.TraceId}}
			node.Children = append(node.Children, child)
		}
		child.Total += durations[s]
		addChildrenSpans(child, byParent, s.SpanId)
	}
	sort.Slice(node.Children, func(i, j int) bool {
		return node.Children[i].Name < node.Children[j].Name
	})
	node.Self = node.Total
	for _, ch := range node.Children {
		node.Self -= ch.Total
	}
}

func spanAttrStats(selectionTraces, baselineTraces []*model.Trace, limit int) []model.TraceSpanAttrStats {
	type Attr struct{ name, value string }
	type Counts struct {
		selection, baseline float32
		sampleTraceId       string
	}
	attrs := map[Attr]*Counts{}
	for _, t := range selectionTraces {
		for _, s := range t.Spans {
			for name, value := range s.SpanAttributes {
				a := Attr{name: name, value: value}
				if attrs[a] == nil {
					attrs[a] = &Counts{sampleTraceId: s.TraceId}
				}
				attrs[a].selection++
			}
			for name, value := range s.ResourceAttributes {
				a := Attr{name: name, value: value}
				if attrs[a] == nil {
					attrs[a] = &Counts{sampleTraceId: s.TraceId}
				}
				attrs[a].selection++
			}
		}
	}
	for _, t := range baselineTraces {
		for _, s := range t.Spans {
			for name, value := range s.SpanAttributes {
				a := Attr{name: name, value: value}
				if attrs[a] == nil {
					attrs[a] = &Counts{sampleTraceId: s.TraceId}
				}
				attrs[a].baseline++
			}
			for name, value := range s.ResourceAttributes {
				a := Attr{name: name, value: value}
				if attrs[a] == nil {
					attrs[a] = &Counts{sampleTraceId: s.TraceId}
				}
				attrs[a].baseline++
			}
		}
	}
	byName := map[string][]*model.TraceSpanAttrStatsValue{}
	for attr, counts := range attrs {
		byName[attr.name] = append(byName[attr.name], &model.TraceSpanAttrStatsValue{
			Name:          attr.value,
			Selection:     counts.selection,
			Baseline:      counts.baseline,
			SampleTraceId: counts.sampleTraceId,
		})
	}
	var res []model.TraceSpanAttrStats
	maxDiff := map[string]float32{}
	for name, values := range byName {
		var total Counts
		for _, v := range values {
			total.selection += v.Selection
			total.baseline += v.Baseline
		}
		for _, v := range values {
			if total.selection > 0 {
				v.Selection /= total.selection
			}
			if total.baseline > 0 {
				v.Baseline /= total.baseline
			}
			diff := v.Selection - v.Baseline
			if diff > maxDiff[name] {
				maxDiff[name] = diff
			}
		}
		sort.Slice(values, func(i, j int) bool {
			vi, vj := values[i], values[j]
			return vi.Selection+vi.Baseline > vj.Selection+vj.Baseline
		})
		if len(values) > limit {
			values = values[:limit]
		}
		res = append(res, model.TraceSpanAttrStats{Name: name, Values: values})
	}
	sort.Slice(res, func(i, j int) bool {
		ri, rj := res[i], res[j]
		return maxDiff[ri.Name] > maxDiff[rj.Name]
	})
	return res
}

type histBucket struct {
	ge, count float32
}

func getQuantiles(buckets map[float32]float32, quantiles []float32) []float32 {
	hist := make([]histBucket, 0, len(buckets))
	for k, v := range buckets {
		hist = append(hist, histBucket{ge: k, count: v})
	}
	sort.Slice(hist, func(i, j int) bool {
		return hist[i].ge < hist[j].ge
	})
	var total float32
	for _, b := range hist {
		total += b.count
	}
	var res []float32
	for _, q := range quantiles {
		target := q * total
		var sum float32
		for _, b := range hist {
			if sum+b.count < target {
				sum += b.count
				continue
			}
			bNext := clickhouse.HistogramNextBucket[b.ge]
			v := b.ge
			if bNext > 0 && b.count > 0 {
				v += (bNext - b.ge) * (target - sum) / b.count
			}
			res = append(res, v)
			break
		}
	}
	return res
}
