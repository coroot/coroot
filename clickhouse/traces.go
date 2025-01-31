package clickhouse

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"golang.org/x/exp/maps"
)

var (
	histogramBuckets    = []float64{0, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000, math.Inf(1)}
	histogramNextBucket = map[float32]float32{}
)

func init() {
	for i, b := range histogramBuckets[:len(histogramBuckets)-2] {
		histogramNextBucket[float32(b)] = float32(histogramBuckets[i+1])
	}
}

func (c *Client) GetServicesFromTraces(ctx context.Context, from timeseries.Time) ([]string, error) {
	rows, err := c.Query(ctx, "SELECT DISTINCT ServiceName FROM @@table_otel_traces_service_name@@ WHERE LastSeen >= @from",
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.NanoSeconds),
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	var res []string
	for rows.Next() {
		var app string
		if err = rows.Scan(&app); err != nil {
			return nil, err
		}
		res = append(res, app)
	}
	return res, nil
}

func (c *Client) GetRootSpansHistogram(ctx context.Context, q SpanQuery) ([]model.HistogramBucket, error) {
	filter, filterArgs := q.RootSpansFilter()
	return c.getSpansHistogram(ctx, q, filter, filterArgs)
}

func (c *Client) GetRootSpans(ctx context.Context, q SpanQuery) ([]*model.TraceSpan, error) {
	filter, filterArgs := q.RootSpansFilter()
	return c.getSpans(ctx, q, "", "Timestamp DESC", filter, filterArgs)
}

func (c *Client) GetRootSpansSummary(ctx context.Context, q SpanQuery) (*model.TraceSpanSummary, error) {
	filter, filterArgs := q.RootSpansFilter()
	return c.getSpansSummary(ctx, q, filter, filterArgs)
}

func (c *Client) GetSpanAttrStats(ctx context.Context, q SpanQuery) ([]model.TraceSpanAttrStats, error) {
	return c.getSpanAttrStats(ctx, q)
}

func (c *Client) GetTraceErrors(ctx context.Context, q SpanQuery) ([]model.TraceErrorsStat, error) {
	return c.getTraceErrors(ctx, q)
}

func (c *Client) GetTraceLatencyProfile(ctx context.Context, q SpanQuery) (*model.Profile, error) {
	selectionTraces, baselineTraces, err := c.getSelectionAndBaselineTraces(ctx, q)
	if err != nil {
		return nil, err
	}

	fgBase := getTraceLatencyFlamegraph(baselineTraces)
	fgComp := getTraceLatencyFlamegraph(selectionTraces)

	profile := &model.Profile{Type: "::nanoseconds"}
	if q.Diff {
		fgBase.Diff(fgComp)
		profile.FlameGraph = fgBase
		profile.Diff = true
	} else {
		if q.IsSelectionDefined() {
			profile.FlameGraph = fgComp
		} else {
			profile.FlameGraph = fgBase
		}
	}

	return profile, nil
}

func (c *Client) GetSpansByServiceNameHistogram(ctx context.Context, q SpanQuery) ([]model.HistogramBucket, error) {
	filter, filterArgs := q.SpansByServiceNameFilter()
	return c.getSpansHistogram(ctx, q, filter, filterArgs)
}

func (c *Client) GetSpansByServiceName(ctx context.Context, q SpanQuery) ([]*model.TraceSpan, error) {
	filter, filterArgs := q.SpansByServiceNameFilter()
	return c.getSpans(ctx, q, "", "Timestamp DESC", filter, filterArgs)
}

func (c *Client) GetInboundSpansHistogram(ctx context.Context, q SpanQuery, clients []string, listens []model.Listen) ([]model.HistogramBucket, error) {
	if len(listens) == 0 {
		return nil, nil
	}
	filter, filterArgs := inboundSpansFilter(clients, listens)
	return c.getSpansHistogram(ctx, q, filter, filterArgs)
}

func (c *Client) GetInboundSpans(ctx context.Context, q SpanQuery, clients []string, listens []model.Listen) ([]*model.TraceSpan, error) {
	if len(listens) == 0 {
		return nil, nil
	}
	filter, filterArgs := inboundSpansFilter(clients, listens)
	return c.getSpans(ctx, q, "", "Timestamp DESC", filter, filterArgs)
}

func (c *Client) GetParentSpans(ctx context.Context, spans []*model.TraceSpan) ([]*model.TraceSpan, error) {
	traceIds := map[string]bool{}
	var ids []clickhouse.GroupSet
	for _, s := range spans {
		if s.ParentSpanId != "" {
			ids = append(ids, clickhouse.GroupSet{Value: []any{s.TraceId, s.ParentSpanId}})
			traceIds[s.TraceId] = true
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	var q SpanQuery
	return c.getSpans(ctx, q,
		"@traceIds as traceIds, (SELECT min(Start) as start, max(End)+1 as end FROM @@table_otel_traces_trace_id_ts@@ WHERE TraceId IN (traceIds)) as ts",
		"",
		[]string{"Timestamp BETWEEN ts.start AND ts.end", "TraceId IN (traceIds)", "(TraceId, SpanId) IN (@ids)"},
		[]any{
			clickhouse.Named("traceIds", maps.Keys(traceIds)),
			clickhouse.Named("ids", ids),
		},
	)
}

func (c *Client) GetSpansByTraceId(ctx context.Context, traceId string) ([]*model.TraceSpan, error) {
	var q SpanQuery
	return c.getSpans(ctx, q,
		"(SELECT min(Start) as start, max(End)+1 as end FROM @@table_otel_traces_trace_id_ts@@ WHERE TraceId = @traceId) as ts",
		"Timestamp",
		[]string{"TraceId = @traceId", "Timestamp BETWEEN ts.start AND ts.end"},
		[]any{
			clickhouse.Named("traceId", traceId),
		},
	)
}

func (c *Client) getSpansHistogram(ctx context.Context, q SpanQuery, filters []string, filterArgs []any) ([]model.HistogramBucket, error) {
	step := q.Ctx.Step
	from := q.Ctx.From
	to := q.Ctx.To.Add(step)

	filters = append(filters, "Timestamp BETWEEN @from AND @to")
	filterArgs = append(filterArgs,
		clickhouse.Named("step", step),
		clickhouse.Named("buckets", histogramBuckets[:len(histogramBuckets)-1]),
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.NanoSeconds),
	)

	query := "SELECT toStartOfInterval(Timestamp, INTERVAL @step second), roundDown(Duration/1000000, @buckets), count(1), countIf(StatusCode = 'STATUS_CODE_ERROR')"
	query += " FROM @@table_otel_traces@@"
	query += " WHERE " + strings.Join(filters, " AND ")
	query += " GROUP BY 1, 2"

	rows, err := c.Query(ctx, query, filterArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var t time.Time
	var bucket float64
	var total, failed uint64
	byBucket := map[float64]*timeseries.TimeSeries{}
	errors := map[timeseries.Time]uint64{}
	for rows.Next() {
		if err = rows.Scan(&t, &bucket, &total, &failed); err != nil {
			return nil, err
		}
		if byBucket[bucket] == nil {
			byBucket[bucket] = timeseries.New(from, int(to.Sub(from)/step), step)
		}
		ts := timeseries.Time(t.Unix())
		byBucket[bucket].Set(ts, float32(total)/float32(step))
		errors[ts] += failed
	}

	if len(byBucket) == 0 {
		return nil, nil
	}

	res := []model.HistogramBucket{
		{TimeSeries: timeseries.New(from, int(to.Sub(from)/step), step)}, // errors
	}
	for ts, count := range errors {
		res[0].TimeSeries.Set(ts, float32(count)/float32(step))
	}
	for i := 1; i < len(histogramBuckets); i++ {
		ts := byBucket[histogramBuckets[i-1]]
		if ts.IsEmpty() {
			ts = timeseries.New(from, int(to.Sub(from)/step), step)
		}
		if len(res) > 0 {
			ts = timeseries.Aggregate2(res[len(res)-1].TimeSeries, ts, func(x, y float32) float32 {
				if timeseries.IsNaN(x) {
					return y
				}
				if timeseries.IsNaN(y) {
					return x
				}
				return x + y
			})
		}
		res = append(res, model.HistogramBucket{
			Le:         float32(histogramBuckets[i] / 1000),
			TimeSeries: ts,
		})
	}
	return res, nil
}

func (c *Client) getSpansSummary(ctx context.Context, q SpanQuery, filters []string, filterArgs []any) (*model.TraceSpanSummary, error) {
	filters = append(filters,
		"Timestamp BETWEEN @tsFrom AND @tsTo",
	)
	filterArgs = append(filterArgs,
		clickhouse.DateNamed("tsFrom", q.TsFrom.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("tsTo", q.TsTo.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.Named("buckets", histogramBuckets[:len(histogramBuckets)-1]),
	)
	durFilter, durFilterArgs := q.DurationFilter()
	if durFilter != "" {
		filters = append(filters, durFilter)
		filterArgs = append(filterArgs, durFilterArgs...)
	}

	query := "SELECT ServiceName, SpanName, roundDown(Duration/1000000, @buckets), count(1), countIf(StatusCode = 'STATUS_CODE_ERROR')"
	query += " FROM @@table_otel_traces@@"
	query += " WHERE " + strings.Join(filters, " AND ")
	query += " GROUP BY 1, 2, 3"

	rows, err := c.Query(ctx, query, filterArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var serviceName, spanName string
	var bucket float64
	var total, failed uint64
	type key struct {
		serviceName, spanName string
	}
	totalByKey := map[key]uint64{}
	failedByKey := map[key]uint64{}
	histByKey := map[key][]histBucket{}
	for rows.Next() {
		if err = rows.Scan(&serviceName, &spanName, &bucket, &total, &failed); err != nil {
			return nil, err
		}
		k := key{serviceName: serviceName, spanName: spanName}
		totalByKey[k] += total
		failedByKey[k] += failed
		histByKey[k] = append(histByKey[k], histBucket{ge: float32(bucket), count: float32(total)})
	}

	if len(totalByKey) == 0 {
		return nil, nil
	}

	res := &model.TraceSpanSummary{}
	duration := q.TsTo.Sub(q.TsFrom)
	quantiles := []float32{0.5, 0.95, 0.99}
	totalHist := map[float32]float32{}
	for k := range totalByKey {
		res.Stats = append(res.Stats, model.TraceSpanStats{
			ServiceName:       k.serviceName,
			SpanName:          k.spanName,
			Total:             float32(totalByKey[k]) / float32(duration),
			Failed:            float32(failedByKey[k]) / float32(totalByKey[k]),
			DurationQuantiles: getQuantiles(histByKey[k], quantiles),
		})
		res.Overall.Total += float32(totalByKey[k])
		res.Overall.Failed += float32(failedByKey[k])
		for _, b := range histByKey[k] {
			totalHist[b.ge] += b.count
		}
	}
	var hist []histBucket
	for ge, count := range totalHist {
		hist = append(hist, histBucket{ge: ge, count: count})
	}
	res.Overall.Failed /= res.Overall.Total
	res.Overall.Total /= float32(duration)
	res.Overall.DurationQuantiles = getQuantiles(hist, quantiles)

	return res, nil
}

func (c *Client) getSpans(ctx context.Context, q SpanQuery, with string, orderBy string, filters []string, filterArgs []any) ([]*model.TraceSpan, error) {
	tsFilter := "Timestamp BETWEEN @tsFrom AND @tsTo"
	tsFilterArgs := []any{
		clickhouse.DateNamed("tsFrom", q.TsFrom.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("tsTo", q.TsTo.ToStandard(), clickhouse.NanoSeconds),
	}
	if !q.TsFrom.IsZero() && !q.TsTo.IsZero() {
		filters = append(filters, tsFilter)
		filterArgs = append(filterArgs, tsFilterArgs...)
	}
	durFilter, durFilterArgs := q.DurationFilter()
	if durFilter != "" {
		filters = append(filters, durFilter)
		filterArgs = append(filterArgs, durFilterArgs...)
	}

	query := ""
	if with != "" {
		query += "WITH " + with
	}
	query += " SELECT Timestamp, TraceId, SpanId, ParentSpanId, SpanName, ServiceName, Duration, StatusCode, StatusMessage, ResourceAttributes, SpanAttributes, Events.Timestamp, Events.Name, Events.Attributes"
	query += " FROM @@table_otel_traces@@"
	query += " WHERE " + strings.Join(filters, " AND ")
	if orderBy != "" {
		query += " ORDER BY " + orderBy
	}
	if q.Limit > 0 {
		query += " LIMIT " + fmt.Sprint(q.Limit)
	}

	rows, err := c.Query(ctx, query, filterArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*model.TraceSpan
	for rows.Next() {
		var s model.TraceSpan
		var eventsTimestamp []time.Time
		var eventsName []string
		var eventsAttributes []map[string]string
		if err = rows.Scan(
			&s.Timestamp, &s.TraceId, &s.SpanId, &s.ParentSpanId, &s.Name, &s.ServiceName,
			&s.Duration, &s.StatusCode, &s.StatusMessage,
			&s.ResourceAttributes, &s.SpanAttributes,
			&eventsTimestamp, &eventsName, &eventsAttributes,
		); err != nil {
			return nil, err
		}
		l := len(eventsTimestamp)
		if l > 0 && l == len(eventsName) && l == len(eventsAttributes) {
			s.Events = make([]model.TraceSpanEvent, l)
			for i := range eventsTimestamp {
				s.Events[i].Timestamp = eventsTimestamp[i]
				s.Events[i].Name = eventsName[i]
				s.Events[i].Attributes = eventsAttributes[i]
			}
		}
		res = append(res, &s)
	}
	return res, nil
}

func (c *Client) getTraces(ctx context.Context, filters []string, filterArgs []any) ([]*model.Trace, error) {
	query := fmt.Sprintf(`
WITH (
	SELECT min(Timestamp) AS start, max(Timestamp)+1 AS end, groupArray(distinct TraceId) AS ids 
	FROM (SELECT TraceId, Timestamp FROM @@table_otel_traces@@ WHERE %s ORDER BY Timestamp LIMIT 1000)
) AS t
SELECT Timestamp, TraceId, SpanId, ParentSpanId, SpanName, ServiceName, Duration, StatusCode, StatusMessage, ResourceAttributes, SpanAttributes, Events.Timestamp, Events.Name, Events.Attributes
FROM @@table_otel_traces@@
WHERE Timestamp BETWEEN t.start AND t.end AND has(coalesce(t.ids, []), TraceId)`, strings.Join(filters, " AND "))
	rows, err := c.Query(ctx, query, filterArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := map[string]*model.Trace{}
	var eventsTimestamp []time.Time
	var eventsName []string
	var eventsAttributes []map[string]string
	for rows.Next() {
		var s model.TraceSpan
		err = rows.Scan(
			&s.Timestamp, &s.TraceId, &s.SpanId, &s.ParentSpanId, &s.Name, &s.ServiceName, &s.Duration,
			&s.StatusCode, &s.StatusMessage, &s.ResourceAttributes, &s.SpanAttributes,
			&eventsTimestamp, &eventsName, &eventsAttributes)
		if err != nil {
			return nil, err
		}
		if res[s.TraceId] == nil {
			res[s.TraceId] = &model.Trace{}
		}

		if l := len(eventsTimestamp); l > 0 && l == len(eventsName) && l == len(eventsAttributes) {
			s.Events = make([]model.TraceSpanEvent, l)
			for i := range eventsTimestamp {
				s.Events[i].Timestamp = eventsTimestamp[i]
				s.Events[i].Name = eventsName[i]
				s.Events[i].Attributes = eventsAttributes[i]
			}
		}
		res[s.TraceId].Spans = append(res[s.TraceId].Spans, &s)
	}
	return maps.Values(res), nil
}

func (c *Client) getSpanAttrStats(ctx context.Context, q SpanQuery) ([]model.TraceSpanAttrStats, error) {
	q.Diff = true
	selectionTraces, baselineTraces, err := c.getSelectionAndBaselineTraces(ctx, q)
	if err != nil {
		return nil, err
	}

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
		if len(values) > q.Limit {
			values = values[:q.Limit]
		}
		res = append(res, model.TraceSpanAttrStats{Name: name, Values: values})
	}
	sort.Slice(res, func(i, j int) bool {
		ri, rj := res[i], res[j]
		return maxDiff[ri.Name] > maxDiff[rj.Name]
	})
	return res, nil
}

func (c *Client) getTraceErrors(ctx context.Context, q SpanQuery) ([]model.TraceErrorsStat, error) {
	filters, filterArgs := q.RootSpansFilter()
	filters = append(filters, "Timestamp BETWEEN @from AND @to")
	filterArgs = append(filterArgs,
		clickhouse.DateNamed("from", q.TsFrom.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("to", q.TsTo.ToStandard(), clickhouse.NanoSeconds),
	)
	q.Errors = true
	durFilter, durFilterArgs := q.DurationFilter()
	if durFilter != "" {
		filters = append(filters, durFilter)
		filterArgs = append(filterArgs, durFilterArgs...)
	}
	traces, err := c.getTraces(ctx, filters, filterArgs)
	if err != nil {
		return nil, err
	}

	type Key struct {
		serviceName string
		spanName    string
		labelsHash  uint64
	}
	errors := map[Key]*model.TraceErrorsStat{}
	var total float32
	for _, t := range traces {
		parents := map[string]string{}
		errorSpans := map[string]*model.TraceSpan{}
		for _, s := range t.Spans {
			parents[s.SpanId] = s.ParentSpanId
			if s.StatusCode == "STATUS_CODE_ERROR" {
				errorSpans[s.SpanId] = s
			}
		}
		for _, s := range errorSpans {
			parentId := s.ParentSpanId
			for parentId != "" {
				delete(errorSpans, parentId)
				parentId = parents[parentId]
			}
		}

		for _, s := range errorSpans {
			ls := s.Labels()
			k := Key{
				serviceName: s.ServiceName,
				spanName:    s.Name,
				labelsHash:  ls.Hash(),
			}
			if errors[k] == nil {
				errors[k] = &model.TraceErrorsStat{
					ServiceName:   s.ServiceName,
					SpanName:      s.Name,
					Labels:        ls,
					SampleTraceId: s.TraceId,
					SampleError:   s.ErrorMessage(),
				}
			}
			errors[k].Count++
			total++
		}
	}

	var res []model.TraceErrorsStat
	for _, v := range errors {
		v.Count /= total
		res = append(res, *v)
	}

	return res, nil
}

func (c *Client) getSelectionAndBaselineTraces(ctx context.Context, q SpanQuery) ([]*model.Trace, []*model.Trace, error) {
	filters, filterArgs := q.RootSpansFilter()

	var err error
	var selectionFilter string
	var selectionTraces []*model.Trace
	if q.IsSelectionDefined() {
		selectionFilter = "Timestamp BETWEEN @tsFrom AND @tsTo"
		filterArgs = append(filterArgs,
			clickhouse.DateNamed("tsFrom", q.TsFrom.ToStandard(), clickhouse.NanoSeconds),
			clickhouse.DateNamed("tsTo", q.TsTo.ToStandard(), clickhouse.NanoSeconds),
		)
		durFilter, durFilterArgs := q.DurationFilter()
		if durFilter != "" {
			selectionFilter += " AND " + durFilter
			filterArgs = append(filterArgs, durFilterArgs...)
		}
		selectionTraces, err = c.getTraces(ctx, append(filters, selectionFilter), filterArgs)
		if err != nil {
			return nil, nil, err
		}
		if !q.Diff {
			return selectionTraces, nil, nil
		}
	}

	filters = append(filters, "Timestamp BETWEEN @from AND @to")
	filterArgs = append(filterArgs,
		clickhouse.DateNamed("from", q.Ctx.From.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("to", q.Ctx.To.ToStandard(), clickhouse.NanoSeconds),
	)
	if selectionFilter != "" {
		filters = append(filters, fmt.Sprintf("NOT (%s)", selectionFilter))
	}
	baselineTraces, err := c.getTraces(ctx, filters, filterArgs)
	if err != nil {
		return nil, nil, err
	}
	return selectionTraces, baselineTraces, nil
}

type SpanFilter struct {
	Field string
	Op    string
	Value string
}

type SpanQuery struct {
	Ctx timeseries.Context

	TsFrom  timeseries.Time
	TsTo    timeseries.Time
	DurFrom time.Duration
	DurTo   time.Duration
	Errors  bool

	Limit int

	Filters          []SpanFilter
	ExcludePeerAddrs []string

	Diff bool
}

func (q *SpanQuery) AddFilter(field, op, value string) {
	q.Filters = append(q.Filters, SpanFilter{Field: field, Op: op, Value: value})
}

func (q *SpanQuery) IsSelectionDefined() bool {
	return q.TsFrom > q.Ctx.From || q.TsTo < q.Ctx.To || q.DurFrom > 0 || q.DurTo > 0 || q.Errors
}

func (q *SpanQuery) DurationFilter() (string, []any) {
	var filter string
	switch {
	case q.DurFrom > 0 && q.DurTo > 0 && q.Errors:
		filter = "(Duration BETWEEN @durFrom AND @durTo OR StatusCode = 'STATUS_CODE_ERROR')"
	case q.DurFrom == 0 && q.DurTo > 0 && q.Errors:
		filter = "(Duration <= @durTo OR StatusCode = 'STATUS_CODE_ERROR')"
	case q.DurFrom > 0 && q.DurTo == 0 && q.Errors:
		filter = "(Duration >= @durFrom OR StatusCode = 'STATUS_CODE_ERROR')"
	case q.DurFrom == 0 && q.DurTo == 0 && q.Errors:
		filter = "StatusCode = 'STATUS_CODE_ERROR'"
	case q.DurFrom > 0 && q.DurTo > 0 && !q.Errors:
		filter = "Duration BETWEEN @durFrom AND @durTo"
	case q.DurFrom == 0 && q.DurTo > 0 && !q.Errors:
		filter = "Duration <= @durTo"
	case q.DurFrom > 0 && q.DurTo == 0 && !q.Errors:
		filter = "Duration >= @durFrom"
	}
	var args []any
	if q.DurFrom > 0 {
		args = append(args, clickhouse.Named("durFrom", q.DurFrom.Nanoseconds()))
	}
	if q.DurTo > 0 {
		args = append(args, clickhouse.Named("durTo", q.DurTo.Nanoseconds()))
	}
	return filter, args
}

func (q *SpanQuery) RootSpansFilter() ([]string, []any) {
	filter, args := q.Filter()
	filter = append(filter, "ParentSpanId = ''")
	filter = append(filter, "NOT startsWith(ServiceName, '/')")
	if len(q.ExcludePeerAddrs) > 0 {
		filter = append(filter, "NetSockPeerAddr NOT IN (@addrs)")
		args = append(args, clickhouse.Named("addrs", q.ExcludePeerAddrs))
	}
	return filter, args
}

func (q *SpanQuery) SpansByServiceNameFilter() ([]string, []any) {
	filter, args := q.Filter()
	filter = append(filter, "SpanKind = 'SPAN_KIND_SERVER'")
	if len(q.ExcludePeerAddrs) > 0 {
		filter = append(filter, "NetSockPeerAddr NOT IN (@addrs)")
		args = append(args, clickhouse.Named("addrs", q.ExcludePeerAddrs))
	}
	return filter, args
}

func (q *SpanQuery) Filter() ([]string, []any) {
	var filter []string
	var args []any
	for i, f := range q.Filters {
		if strings.ContainsFunc(f.Field, func(r rune) bool { return !unicode.IsLetter(r) }) {
			continue
		}
		var expr string
		switch f.Op {
		case "=":
			expr = "%s = @%s"
		case "!=":
			expr = "%s != @%s"
		case "~":
			expr = "match(%s, @%s)"
		case "!~":
			expr = "NOT match(%s, @%s)"
		default:
			continue
		}
		name := fmt.Sprintf("filter_%d", i)
		expr = fmt.Sprintf(expr, f.Field, name)
		filter = append(filter, expr)
		args = append(args, clickhouse.Named(name, f.Value))
	}
	return filter, args
}

func inboundSpansFilter(clients []string, listens []model.Listen) ([]string, []any) {
	ips := map[string]bool{}
	for _, l := range listens {
		if l.Port == "0" {
			ips[l.IP] = true
		}
	}
	var addrs []clickhouse.GroupSet
	for _, l := range listens {
		addrs = append(addrs, clickhouse.GroupSet{Value: []any{l.IP, l.Port}})
	}
	filter := []string{
		"ServiceName IN (@services)",
		"(SpanAttributes['net.peer.name'] IN (@ips) OR (SpanAttributes['net.peer.name'], SpanAttributes['net.peer.port']) IN (@addrs))",
	}
	args := []any{
		clickhouse.Named("services", clients),
		clickhouse.Named("ips", maps.Keys(ips)),
		clickhouse.Named("addrs", addrs),
	}
	return filter, args
}

type histBucket struct {
	ge, count float32
}

func getQuantiles(hist []histBucket, quantiles []float32) []float32 {
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
			bNext := histogramNextBucket[b.ge]
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
