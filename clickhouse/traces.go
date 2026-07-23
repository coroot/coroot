package clickhouse

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"golang.org/x/exp/maps"
	"k8s.io/klog"
)

var (
	HistogramBuckets    = []float64{0, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000, math.Inf(1)}
	HistogramNextBucket = map[float32]float32{}
)

func init() {
	for i, b := range HistogramBuckets[:len(HistogramBuckets)-2] {
		HistogramNextBucket[float32(b)] = float32(HistogramBuckets[i+1])
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
	if c.useTracesHistogram(ctx, q, q.Ctx.From) {
		filter, filterArgs := q.RootSpansFilter("Root = 1")
		return c.getSpansHistogram(ctx, q, filter, filterArgs, true)
	}
	filter, filterArgs := q.RootSpansFilter("ParentSpanId = ''")
	return c.getSpansHistogram(ctx, q, filter, filterArgs, false)
}

func (c *Client) GetRootSpans(ctx context.Context, q SpanQuery) ([]*model.TraceSpan, error) {
	filter, filterArgs := q.RootSpansFilter("ParentSpanId = ''")
	return c.getSpans(ctx, q, "", filter, filterArgs)
}

func (c *Client) GetTraceSpanStats(ctx context.Context, q SpanQuery) (map[model.TraceSpanKey]*model.TraceSpanStats, error) {
	if q.DurFrom == 0 && q.DurTo == 0 && !q.Errors && c.useTracesHistogram(ctx, q, q.TsFrom) {
		filter, filterArgs := q.RootSpansFilter("Root = 1")
		return c.getTraceSpanStats(ctx, q, filter, filterArgs, true)
	}
	filter, filterArgs := q.RootSpansFilter("ParentSpanId = ''")
	return c.getTraceSpanStats(ctx, q, filter, filterArgs, false)
}

func (c *Client) GetTraceErrors(ctx context.Context, q SpanQuery) (map[model.TraceSpanKey]*model.TraceErrorsStat, error) {
	return c.getTraceErrors(ctx, q)
}

func (c *Client) GetSpansByServiceNameHistogram(ctx context.Context, q SpanQuery) ([]model.HistogramBucket, error) {
	filter, filterArgs := q.SpansByServiceNameFilter()
	return c.getSpansHistogram(ctx, q, filter, filterArgs, c.useTracesHistogram(ctx, q, q.Ctx.From))
}

func (c *Client) GetSpansByServiceName(ctx context.Context, q SpanQuery) ([]*model.TraceSpan, error) {
	filter, filterArgs := q.SpansByServiceNameFilter()
	return c.getSpans(ctx, q, "", filter, filterArgs)
}

func (c *Client) GetInboundSpansHistogram(ctx context.Context, q SpanQuery, clients []string, listens []model.Listen) ([]model.HistogramBucket, error) {
	if len(listens) == 0 {
		return nil, nil
	}
	fromMV := c.useTracesHistogram(ctx, q, q.Ctx.From)
	filter, filterArgs := inboundSpansFilter(clients, listens, fromMV)
	return c.getSpansHistogram(ctx, q, filter, filterArgs, fromMV)
}

func (c *Client) GetInboundSpans(ctx context.Context, q SpanQuery, clients []string, listens []model.Listen) ([]*model.TraceSpan, error) {
	if len(listens) == 0 {
		return nil, nil
	}
	filter, filterArgs := inboundSpansFilter(clients, listens, false)
	return c.getSpans(ctx, q, "", filter, filterArgs)
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
	var minTs, maxTs time.Time
	err := c.QueryRow(ctx,
		"SELECT min(Start), max(End)+1 FROM @@table_otel_traces_trace_id_ts@@ WHERE TraceId IN (@traceIds)",
		clickhouse.Named("traceIds", maps.Keys(traceIds)),
	).Scan(&minTs, &maxTs)
	if err != nil {
		return nil, err
	}
	q := SpanQuery{
		TsFrom: timeseries.TimeFromStandard(minTs),
		TsTo:   timeseries.TimeFromStandard(maxTs),
	}
	return c.getSpans(ctx, q, "",
		[]string{"TraceId IN (@traceIds)", "(TraceId, SpanId) IN (@ids)"},
		[]any{
			clickhouse.Named("traceIds", maps.Keys(traceIds)),
			clickhouse.Named("ids", ids),
		},
	)
}

func (c *Client) GetSpansByTraceId(ctx context.Context, traceId string) ([]*model.TraceSpan, error) {
	var minTs, maxTs time.Time
	err := c.QueryRow(ctx,
		"SELECT min(Start), max(End)+1 FROM @@table_otel_traces_trace_id_ts@@ WHERE TraceId = @traceId",
		clickhouse.Named("traceId", traceId),
	).Scan(&minTs, &maxTs)
	if err != nil {
		return nil, err
	}
	q := SpanQuery{
		TsFrom: timeseries.TimeFromStandard(minTs),
		TsTo:   timeseries.TimeFromStandard(maxTs),
	}
	return c.getSpans(ctx, q, "Timestamp",
		[]string{"TraceId = @traceId"},
		[]any{
			clickhouse.Named("traceId", traceId),
		},
	)
}

func (c *Client) getOtelTracesServiceName(ctx context.Context, world *model.World, app *model.Application) (string, error) {
	if app.Settings != nil && app.Settings.Tracing != nil {
		return app.Settings.Tracing.Service, nil
	}
	services, err := c.GetServicesFromTraces(ctx, world.Ctx.From)
	if err != nil {
		return "", err
	}
	var otelServices []string
	for _, s := range services {
		if !strings.HasPrefix(s, "/") {
			otelServices = append(otelServices, s)
		}
	}
	return model.GuessService(otelServices, world, app), nil
}

func (c *Client) GetTracesViolatingSLOs(ctx context.Context, from, to timeseries.Time, world *model.World, app *model.Application) (*model.Trace, *model.Trace, error) {
	serviceName, err := c.getOtelTracesServiceName(ctx, world, app)
	if err != nil || serviceName == "" {
		return nil, nil, err
	}

	sq := SpanQuery{
		Ctx:    world.Ctx,
		TsFrom: from,
		TsTo:   to,
		Limit:  1,
	}

	sq.AddFilter("ServiceName", "=", serviceName)

	sq.Errors = true
	var errorTrace, slowTrace *model.Trace

	if errorTrace, err = c.getTrace(ctx, sq); err != nil {
		return nil, nil, err
	}

	if len(app.LatencySLIs) > 0 && app.LatencySLIs[0].Config.ObjectivePercentage > 0 {
		sq.Errors = false
		sq.DurFrom = time.Duration(float64(app.LatencySLIs[0].Config.ObjectiveBucket) * float64(time.Second))
		if slowTrace, err = c.getTrace(ctx, sq); err != nil {
			return nil, nil, err
		}
	}

	return errorTrace, slowTrace, nil
}

func (c *Client) getTrace(ctx context.Context, sq SpanQuery) (*model.Trace, error) {
	spans, err := c.GetSpansByServiceName(ctx, sq)
	if err != nil {
		return nil, err
	}
	if len(spans) == 0 || spans[0].TraceId == "" {
		return nil, nil
	}
	if spans, err = c.GetSpansByTraceId(ctx, spans[0].TraceId); err != nil {
		return nil, err
	}
	return &model.Trace{Spans: spans}, nil
}

func (c *Client) getSpansHistogram(ctx context.Context, q SpanQuery, filters []string, filterArgs []any, fromMV bool) ([]model.HistogramBucket, error) {
	step := q.Ctx.Step
	if r := step % 60; r != 0 {
		step += 60 - r
	}
	from := q.Ctx.From
	to := q.Ctx.To.Add(step)

	filters = append(filters, "Timestamp BETWEEN @from AND @to")
	filterArgs = append(filterArgs,
		clickhouse.Named("step", int(step)),
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.NanoSeconds),
	)

	var query string
	if fromMV {
		query = "SELECT toStartOfInterval(Timestamp, INTERVAL @step second), Bucket, sum(Total), sum(Failed)"
		query += " FROM @@table_otel_traces_histogram@@"
	} else {
		filterArgs = append(filterArgs, clickhouse.Named("buckets", HistogramBuckets[:len(HistogramBuckets)-1]))
		query = "SELECT toStartOfInterval(Timestamp, INTERVAL @step second), roundDown(Duration/1000000, @buckets), count(1), countIf(StatusCode = 'STATUS_CODE_ERROR')"
		query += " FROM @@table_otel_traces@@"
	}
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
	for i := 1; i < len(HistogramBuckets); i++ {
		ts := byBucket[HistogramBuckets[i-1]]
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
			Le:         float32(HistogramBuckets[i] / 1000),
			TimeSeries: ts,
		})
	}
	return res, nil
}

func (c *Client) getTraceSpanStats(ctx context.Context, q SpanQuery, filters []string, filterArgs []any, fromMV bool) (map[model.TraceSpanKey]*model.TraceSpanStats, error) {
	if fromMV {
		filters = append(filters, "Timestamp >= @tsFrom AND Timestamp < @tsTo")
	} else {
		filters = append(filters, "Timestamp BETWEEN @tsFrom AND @tsTo")
	}
	filterArgs = append(filterArgs,
		clickhouse.DateNamed("tsFrom", q.TsFrom.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("tsTo", q.TsTo.ToStandard(), clickhouse.NanoSeconds),
	)

	var query string
	if fromMV {
		query = "SELECT ServiceName, SpanName, Bucket, sum(Total), sum(Failed)"
		query += " FROM @@table_otel_traces_histogram@@"
	} else {
		filterArgs = append(filterArgs, clickhouse.Named("buckets", HistogramBuckets[:len(HistogramBuckets)-1]))
		durFilter, durFilterArgs := q.DurationFilter()
		if durFilter != "" {
			filters = append(filters, durFilter)
			filterArgs = append(filterArgs, durFilterArgs...)
		}
		query = "SELECT ServiceName, SpanName, roundDown(Duration/1000000, @buckets), count(1), countIf(StatusCode = 'STATUS_CODE_ERROR')"
		query += " FROM @@table_otel_traces@@"
	}
	query += " WHERE " + strings.Join(filters, " AND ")
	query += " GROUP BY 1, 2, 3"

	rows, err := c.Query(ctx, query, filterArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bucket float64
	var total, failed uint64

	res := map[model.TraceSpanKey]*model.TraceSpanStats{}

	var k model.TraceSpanKey
	for rows.Next() {
		if err = rows.Scan(&k.ServiceName, &k.SpanName, &bucket, &total, &failed); err != nil {
			return nil, err
		}
		stats := res[k]
		if stats == nil {
			stats = &model.TraceSpanStats{TraceSpanKey: k, Histogram: map[float32]float32{}}
			res[k] = stats
		}
		stats.Total += float32(total)
		stats.Failed += float32(failed)
		stats.Histogram[float32(bucket)] = float32(total)
	}
	return res, nil
}

var spanSliceWindows = []timeseries.Duration{timeseries.Hour, 2 * timeseries.Hour, 4 * timeseries.Hour}

func (c *Client) getSpans(ctx context.Context, q SpanQuery, orderBy string, filters []string, filterArgs []any) ([]*model.TraceSpan, error) {
	if orderBy != "" || q.Limit <= 0 || q.TsFrom.IsZero() || q.TsTo.IsZero() {
		return c.querySpans(ctx, q, orderBy, filters, filterArgs)
	}
	var res []*model.TraceSpan
	to := q.TsTo
	for _, d := range spanSliceWindows {
		from := to.Add(-d)
		if from <= q.TsFrom {
			break
		}
		sq := q
		sq.TsFrom = from
		sq.TsTo = to
		sq.Limit = q.Limit - len(res)
		spans, err := c.querySpans(ctx, sq, "", append([]string{}, filters...), append([]any{}, filterArgs...))
		if err != nil {
			return nil, err
		}
		res = append(res, spans...)
		if len(res) >= q.Limit {
			return res, nil
		}
		to = from
	}
	sq := q
	sq.TsTo = to
	sq.Limit = q.Limit - len(res)
	spans, err := c.querySpans(ctx, sq, "", filters, filterArgs)
	if err != nil {
		return nil, err
	}
	return append(res, spans...), nil
}

func (c *Client) querySpans(ctx context.Context, q SpanQuery, orderBy string, filters []string, filterArgs []any) ([]*model.TraceSpan, error) {
	tsFilter := "Timestamp >= @tsFrom AND Timestamp < @tsTo"
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

	cond := strings.Join(filters, " AND ")
	query := "SELECT Timestamp, TraceId, SpanId, ParentSpanId, SpanName, ServiceName, Duration, StatusCode, StatusMessage, ResourceAttributes, SpanAttributes, Events.Timestamp, Events.Name, Events.Attributes"
	query += " FROM @@table_otel_traces@@"
	query += " WHERE " + cond
	if orderBy == "" && q.Limit > 0 {
		cutoff := "SELECT min(Timestamp) FROM (SELECT Timestamp FROM @@table_otel_traces@@ WHERE " + cond + " ORDER BY Timestamp DESC LIMIT " + fmt.Sprint(q.Limit) + ")"
		query += " AND Timestamp >= (" + cutoff + ")"
		orderBy = "Timestamp DESC"
	}
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
		s.ClusterName = c.Project().Name
		res = append(res, &s)
	}
	return res, nil
}

func (c *Client) getTraces(ctx context.Context, from, to timeseries.Time, filters []string, filterArgs []any) ([]*model.Trace, error) {
	const limit = 200
	cond := strings.Join(filters, " AND ")
	var sliceBounds [][2]timeseries.Time
	sliceTo := to
	for _, d := range spanSliceWindows {
		sliceFrom := sliceTo.Add(-d)
		if sliceFrom <= from {
			break
		}
		sliceBounds = append(sliceBounds, [2]timeseries.Time{sliceFrom, sliceTo})
		sliceTo = sliceFrom
	}
	sliceBounds = append(sliceBounds, [2]timeseries.Time{from, sliceTo})

	var groups [][]string
	seen := map[string]bool{}
	remaining := limit
	for _, b := range sliceBounds {
		query := "SELECT count(1), groupArray(distinct TraceId)"
		query += " FROM (SELECT TraceId FROM @@table_otel_traces@@ WHERE " + cond
		query += " AND Timestamp >= @sliceFrom AND Timestamp < @sliceTo ORDER BY Timestamp DESC LIMIT " + fmt.Sprint(remaining) + ")"
		args := append(append([]any{}, filterArgs...),
			clickhouse.DateNamed("sliceFrom", b[0].ToStandard(), clickhouse.NanoSeconds),
			clickhouse.DateNamed("sliceTo", b[1].ToStandard(), clickhouse.NanoSeconds),
		)
		var cnt uint64
		var ids []string
		t := time.Now()
		if err := c.QueryRow(ctx, query, args...).Scan(&cnt, &ids); err != nil {
			return nil, err
		}
		klog.Infof("trace ids query took %s, returned %d traces: %s, args: %v", time.Since(t).Truncate(time.Millisecond), len(ids), query, args)
		if cnt == 0 {
			continue
		}
		var group []string
		for _, id := range ids {
			if !seen[id] {
				seen[id] = true
				group = append(group, id)
			}
		}
		if len(group) > 0 {
			groups = append(groups, group)
		}
		remaining -= int(cnt)
		if remaining <= 0 {
			break
		}
	}

	res := map[string]*model.Trace{}
	for _, traceIds := range groups {
		if err := c.getTraceSpans(ctx, from, to, traceIds, res); err != nil {
			return nil, err
		}
	}
	return maps.Values(res), nil
}

func (c *Client) getTraceSpans(ctx context.Context, from, to timeseries.Time, traceIds []string, res map[string]*model.Trace) error {
	var minTs, maxTs time.Time
	err := c.QueryRow(ctx,
		"SELECT min(Start), max(End)+1 FROM @@table_otel_traces_trace_id_ts@@ WHERE TraceId IN (@traceIds)",
		clickhouse.Named("traceIds", traceIds),
	).Scan(&minTs, &maxTs)
	if err != nil {
		return err
	}
	if minTs.IsZero() || minTs.Unix() <= 0 {
		minTs, maxTs = from.ToStandard(), to.ToStandard().Add(time.Second)
	}
	query := `
SELECT Timestamp, TraceId, SpanId, ParentSpanId, SpanName, ServiceName, Duration, StatusCode, StatusMessage, ResourceAttributes, SpanAttributes, Events.Timestamp, Events.Name, Events.Attributes
FROM @@table_otel_traces@@
WHERE Timestamp BETWEEN @from AND @to AND TraceId IN @traceIds`
	rows, err := c.Query(ctx, query,
		clickhouse.DateNamed("from", minTs, clickhouse.NanoSeconds),
		clickhouse.DateNamed("to", maxTs, clickhouse.NanoSeconds),
		clickhouse.Named("traceIds", traceIds),
	)
	if err != nil {
		return err
	}
	defer rows.Close()

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
			return err
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
	return nil
}

func (c *Client) getTraceErrors(ctx context.Context, q SpanQuery) (map[model.TraceSpanKey]*model.TraceErrorsStat, error) {
	filters, filterArgs := q.RootSpansFilter("ParentSpanId = ''")
	q.Errors = true
	durFilter, durFilterArgs := q.DurationFilter()
	if durFilter != "" {
		filters = append(filters, durFilter)
		filterArgs = append(filterArgs, durFilterArgs...)
	}
	traces, err := c.getTraces(ctx, q.TsFrom, q.TsTo, filters, filterArgs)
	if err != nil {
		return nil, err
	}

	errors := map[model.TraceSpanKey]*model.TraceErrorsStat{}
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
			k := model.TraceSpanKey{
				ServiceName: s.ServiceName,
				SpanName:    s.Name,
				LabelsHash:  ls.Hash(),
			}
			if errors[k] == nil {
				errors[k] = &model.TraceErrorsStat{
					TraceSpanKey:  k,
					Labels:        ls,
					SampleTraceId: s.TraceId,
					SampleError:   s.ErrorMessage(),
				}
			}
			errors[k].Count++
			total++
		}
	}
	return errors, nil
}

func (c *Client) GetSelectionAndBaselineTraces(ctx context.Context, q SpanQuery) ([]*model.Trace, []*model.Trace, error) {
	filters, filterArgs := q.RootSpansFilter("ParentSpanId = ''")

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
		selectionTraces, err = c.getTraces(ctx, q.TsFrom, q.TsTo, append(filters, selectionFilter), filterArgs)
		if err != nil {
			return nil, nil, err
		}
		if !q.Diff {
			return selectionTraces, nil, nil
		}
	}

	if selectionFilter != "" {
		filters = append(filters, fmt.Sprintf("NOT (%s)", selectionFilter))
	}
	baselineTraces, err := c.getTraces(ctx, q.Ctx.From, q.Ctx.To, filters, filterArgs)
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

func (q *SpanQuery) RootSpansFilter(rootCondition string) ([]string, []any) {
	filter, args := q.Filter()
	filter = append(filter, rootCondition)
	filter = append(filter, "NOT startsWith(ServiceName, '/')")
	if len(q.ExcludePeerAddrs) > 0 {
		filter = append(filter, "NetSockPeerAddr NOT IN (@addrs)")
		args = append(args, clickhouse.Named("addrs", q.ExcludePeerAddrs))
	}
	return filter, args
}

var (
	tracesHistogramMVLock      sync.Mutex
	tracesHistogramMVCreatedAt = map[db.ProjectId]time.Time{}
)

func (c *Client) tracesHistogramCreatedAt(ctx context.Context) time.Time {
	tracesHistogramMVLock.Lock()
	t, ok := tracesHistogramMVCreatedAt[c.project.Id]
	tracesHistogramMVLock.Unlock()
	if ok {
		return t
	}
	err := c.conn.QueryRow(ctx,
		"SELECT metadata_modification_time FROM system.tables WHERE database = currentDatabase() AND name = 'otel_traces_histogram_mv'",
	).Scan(&t)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			klog.Warningln(err)
		}
		return time.Time{}
	}
	tracesHistogramMVLock.Lock()
	tracesHistogramMVCreatedAt[c.project.Id] = t
	tracesHistogramMVLock.Unlock()
	return t
}

func (c *Client) useTracesHistogram(ctx context.Context, q SpanQuery, from timeseries.Time) bool {
	createdAt := c.tracesHistogramCreatedAt(ctx)
	return q.filtersOnHistogramDimensions() && !createdAt.IsZero() && from.ToStandard().After(createdAt)
}

func (q *SpanQuery) filtersOnHistogramDimensions() bool {
	for _, f := range q.Filters {
		switch f.Field {
		case "ServiceName", "SpanName", "SpanKind", "NetSockPeerAddr":
		default:
			return false
		}
	}
	return true
}

func (q *SpanQuery) SpansByServiceNameFilter() ([]string, []any) {
	filter, args := q.Filter()
	filter = append(filter, "(SpanKind = 'SPAN_KIND_SERVER' OR SpanKind = 'SPAN_KIND_CONSUMER')")
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

func inboundSpansFilter(clients []string, listens []model.Listen, fromMV bool) ([]string, []any) {
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
	peerName, peerPort := "SpanAttributes['net.peer.name']", "SpanAttributes['net.peer.port']"
	if fromMV {
		peerName, peerPort = "NetPeerName", "NetPeerPort"
	}
	filter := []string{
		"ServiceName IN (@services)",
		fmt.Sprintf("(%s IN (@ips) OR (%s, %s) IN (@addrs))", peerName, peerName, peerPort),
	}
	args := []any{
		clickhouse.Named("services", clients),
		clickhouse.Named("ips", maps.Keys(ips)),
		clickhouse.Named("addrs", addrs),
	}
	return filter, args
}
