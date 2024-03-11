package clickhouse

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"golang.org/x/exp/maps"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
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

type SpanQuery struct {
	Ctx timeseries.Context

	TsFrom  timeseries.Time
	TsTo    timeseries.Time
	DurFrom time.Duration
	DurTo   time.Duration
	Errors  bool

	Limit int

	ServiceName      string
	SpanName         string
	ExcludePeerAddrs []string
}

func (q *SpanQuery) IsSelectionDefined() bool {
	return q.TsFrom > q.Ctx.From || q.TsTo < q.Ctx.To || q.DurFrom > 0 || q.DurTo > 0 || q.Errors
}

func (c *Client) GetServicesFromTraces(ctx context.Context) ([]string, error) {
	q := "SELECT DISTINCT ServiceName FROM otel_traces"
	rows, err := c.conn.Query(ctx, q)
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
	filter, filterArgs := rootSpansFilter(q)
	return c.getSpansHistogram(ctx, q.Ctx.From, q.Ctx.To, q.Ctx.Step, filter, filterArgs)
}

func (c *Client) GetRootSpans(ctx context.Context, q SpanQuery) ([]*model.TraceSpan, error) {
	filter, filterArgs := rootSpansFilter(q)
	return c.getSpans(ctx, q.TsFrom, q.TsTo, q.DurFrom, q.DurTo, q.Errors, "", "Duration DESC", q.Limit, filter, filterArgs)
}

func (c *Client) GetRootSpansSummary(ctx context.Context, q SpanQuery) (*model.TraceSpanSummary, error) {
	filter, filterArgs := rootSpansFilter(q)
	return c.getSpansSummary(ctx, q.TsFrom, q.TsTo, q.DurFrom, q.DurTo, q.Errors, filter, filterArgs)
}

func (c *Client) GetSpanAttrStats(ctx context.Context, q SpanQuery) ([]model.TraceSpanAttrStats, error) {
	return c.getSpanAttrStats(ctx, q)
}

func (c *Client) GetSpansByServiceNameHistogram(ctx context.Context, q SpanQuery) ([]model.HistogramBucket, error) {
	filter, filterArgs := spansByServiceNameFilter(q)
	return c.getSpansHistogram(ctx, q.Ctx.From, q.Ctx.To, q.Ctx.Step, filter, filterArgs)
}

func (c *Client) GetSpansByServiceName(ctx context.Context, q SpanQuery) ([]*model.TraceSpan, error) {
	filter, filterArgs := spansByServiceNameFilter(q)
	return c.getSpans(ctx, q.TsFrom, q.TsTo, q.DurFrom, q.DurTo, q.Errors, "", "Timestamp DESC", q.Limit, filter, filterArgs)
}

func (c *Client) GetInboundSpansHistogram(ctx context.Context, q SpanQuery, clients []string, listens []model.Listen) ([]model.HistogramBucket, error) {
	if len(listens) == 0 {
		return nil, nil
	}
	filter, filterArgs := inboundSpansFilter(clients, listens)
	return c.getSpansHistogram(ctx, q.Ctx.From, q.Ctx.To, q.Ctx.Step, filter, filterArgs)
}

func (c *Client) GetInboundSpans(ctx context.Context, q SpanQuery, clients []string, listens []model.Listen) ([]*model.TraceSpan, error) {
	if len(listens) == 0 {
		return nil, nil
	}
	filter, filterArgs := inboundSpansFilter(clients, listens)
	return c.getSpans(ctx, q.TsFrom, q.TsTo, q.DurFrom, q.DurTo, q.Errors, "", "Timestamp DESC", q.Limit, filter, filterArgs)
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
	return c.getSpans(ctx, 0, 0, 0, 0, false,
		"@traceIds as traceIds, (SELECT min(Start) FROM otel_traces_trace_id_ts WHERE TraceId IN (traceIds)) as start, (SELECT max(End) + 1 FROM otel_traces_trace_id_ts WHERE TraceId IN (traceIds)) as end",
		"", 0,
		[]string{"Timestamp BETWEEN start AND end", "TraceId IN (traceIds)", "(TraceId, SpanId) IN (@ids)"},
		[]any{
			clickhouse.Named("traceIds", maps.Keys(traceIds)),
			clickhouse.Named("ids", ids),
		},
	)
}

func (c *Client) GetSpansByTraceId(ctx context.Context, traceId string) ([]*model.TraceSpan, error) {
	return c.getSpans(ctx, 0, 0, 0, 0, false,
		"(SELECT min(Start) FROM otel_traces_trace_id_ts WHERE TraceId = @traceId) as start, (SELECT max(End) + 1 FROM otel_traces_trace_id_ts WHERE TraceId = @traceId) as end",
		"Timestamp", 0,
		[]string{"TraceId = @traceId", "Timestamp BETWEEN start AND end"},
		[]any{
			clickhouse.Named("traceId", traceId),
		},
	)
}

func (c *Client) getSpansHistogram(ctx context.Context, from, to timeseries.Time, step timeseries.Duration, filters []string, filterArgs []any) ([]model.HistogramBucket, error) {
	to = to.Add(step)

	q := "SELECT toStartOfInterval(Timestamp, INTERVAL @step second), roundDown(Duration/1000000, @buckets), count(1), count(if(StatusCode = 'STATUS_CODE_ERROR', 1, NULL))"
	filters = append(filters,
		"Timestamp BETWEEN @from AND @to",
	)
	filterArgs = append(filterArgs,
		clickhouse.Named("step", step),
		clickhouse.Named("buckets", histogramBuckets[:len(histogramBuckets)-1]),
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.NanoSeconds),
	)

	q += " FROM otel_traces"
	q += " WHERE " + strings.Join(filters, " AND ")
	q += " GROUP BY 1, 2"

	rows, err := c.conn.Query(ctx, q, filterArgs...)
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

func (c *Client) getSpansSummary(ctx context.Context, tsFrom, tsTo timeseries.Time, durFrom, durTo time.Duration, errors bool, filters []string, filterArgs []any) (*model.TraceSpanSummary, error) {
	filters = append(filters,
		"Timestamp BETWEEN @tsFrom AND @tsTo",
	)
	filterArgs = append(filterArgs,
		clickhouse.DateNamed("tsFrom", tsFrom.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("tsTo", tsTo.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.Named("buckets", histogramBuckets[:len(histogramBuckets)-1]),
	)
	durFilter, durFilterArgs := durationFilter(durFrom, durTo, errors)
	if durFilter != "" {
		filters = append(filters, durFilter)
		filterArgs = append(filterArgs, durFilterArgs...)
	}

	q := "SELECT ServiceName, SpanName, roundDown(Duration/1000000, @buckets), count(1), count(if(StatusCode = 'STATUS_CODE_ERROR', 1, NULL))"
	q += " FROM otel_traces"
	q += " WHERE " + strings.Join(filters, " AND ")
	q += " GROUP BY 1, 2, 3"

	rows, err := c.conn.Query(ctx, q, filterArgs...)
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
	duration := tsTo.Sub(tsFrom)
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

func (c *Client) getSpans(ctx context.Context, tsFrom, tsTo timeseries.Time, durFrom, durTo time.Duration, errors bool, with string, orderBy string, limit int, filters []string, filterArgs []any) ([]*model.TraceSpan, error) {
	if !tsFrom.IsZero() && !tsTo.IsZero() {
		filters = append(filters, "Timestamp BETWEEN @tsFrom AND @tsTo")
		filterArgs = append(filterArgs,
			clickhouse.DateNamed("tsFrom", tsFrom.ToStandard(), clickhouse.NanoSeconds),
			clickhouse.DateNamed("tsTo", tsTo.ToStandard(), clickhouse.NanoSeconds),
		)
	}
	durFilter, durFilterArgs := durationFilter(durFrom, durTo, errors)
	if durFilter != "" {
		filters = append(filters, durFilter)
		filterArgs = append(filterArgs, durFilterArgs...)
	}

	q := ""
	if with != "" {
		q += "WITH " + with
	}
	q += " SELECT Timestamp, TraceId, SpanId, ParentSpanId, SpanName, ServiceName, Duration, StatusCode, StatusMessage, ResourceAttributes, SpanAttributes, Events.Timestamp, Events.Name, Events.Attributes"
	q += " FROM otel_traces"
	q += " WHERE " + strings.Join(filters, " AND ")
	if orderBy != "" {
		q += " ORDER BY " + orderBy
	}
	if limit > 0 {
		q += " LIMIT " + fmt.Sprint(limit)
	}

	rows, err := c.conn.Query(ctx, q, filterArgs...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
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

func (c *Client) getSpanAttrStats(ctx context.Context, q SpanQuery) ([]model.TraceSpanAttrStats, error) {
	filters := []string{
		"Timestamp BETWEEN @from AND @to",
		"NOT startsWith(ServiceName, '/')",
	}
	args := []any{
		clickhouse.DateNamed("from", q.Ctx.From.ToStandard(), clickhouse.Seconds),
		clickhouse.DateNamed("to", q.Ctx.To.ToStandard(), clickhouse.Seconds),
		clickhouse.DateNamed("tsFrom", q.TsFrom.ToStandard(), clickhouse.Seconds),
		clickhouse.DateNamed("tsTo", q.TsTo.ToStandard(), clickhouse.Seconds),
		clickhouse.Named("top", q.Limit),
	}
	isInSelection := "false"
	if q.IsSelectionDefined() {
		isInSelection = "Timestamp BETWEEN @tsFrom AND @tsTo"
		durFilter, durFilterArgs := durationFilter(q.DurFrom, q.DurTo, q.Errors)
		if durFilter != "" {
			isInSelection += " AND " + durFilter
			args = append(args, durFilterArgs...)
		}
	}
	if q.ServiceName != "" {
		filters = append(filters, "ServiceName = @serviceName")
		args = append(args, clickhouse.Named("serviceName", q.ServiceName))
	}
	if q.SpanName != "" {
		filters = append(filters, "SpanName = @spanName")
		args = append(args, clickhouse.Named("spanName", q.SpanName))
	}
	query := fmt.Sprintf(`
WITH a AS (
    SELECT
        Name,
        Value,
        %s AS selection,
        sum(Count) AS count
    FROM otel_traces_attributes
    WHERE
        %s
    GROUP BY 1, 2, 3
), s AS (
    SELECT
        Name,
        Value,
        sum(if(a.selection, count, 0)) AS selection,
        sum(if(a.selection, 0, count)) AS baseline,
        sum(selection) OVER (PARTITION BY Name) AS selection_total,
        sum(baseline) OVER (PARTITION BY Name) AS baseline_total
    FROM a
    GROUP BY 1, 2
), t AS (
    SELECT
        Name,
        Value,
        if(selection_total=0, 0, selection / selection_total) AS selection,
        if(baseline_total=0, 0, baseline / baseline_total) AS baseline,
        row_number() OVER (PARTITION BY Name ORDER BY selection+baseline DESC) AS top
    FROM s
)
SELECT Name, Value, selection, baseline FROM t WHERE top <= @top`,
		isInSelection, strings.Join(filters, " AND "),
	)

	rows, err := c.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var name, value string
	var selection, baseline float64
	byName := map[string][]*model.TraceSpanAttrStatsValue{}
	maxDiffByName := map[string]float64{}
	for rows.Next() {
		if err = rows.Scan(&name, &value, &selection, &baseline); err != nil {
			return nil, err
		}
		diff := selection - baseline
		if diff > maxDiffByName[name] {
			maxDiffByName[name] = diff
		}
		byName[name] = append(byName[name], &model.TraceSpanAttrStatsValue{
			Name:      value,
			Selection: float32(selection),
			Baseline:  float32(baseline),
		})
	}
	var res []model.TraceSpanAttrStats
	for n, vs := range byName {
		res = append(res, model.TraceSpanAttrStats{Name: n, Values: vs})
	}
	sort.Slice(res, func(i, j int) bool {
		return maxDiffByName[res[i].Name] > maxDiffByName[res[j].Name]
	})

	return res, nil
}

func rootSpansFilter(q SpanQuery) ([]string, []any) {
	filter := []string{
		"SpanKind = 'SPAN_KIND_SERVER'",
		"ParentSpanId = ''",
	}
	var args []any
	if q.ServiceName != "" {
		filter = append(filter, "ServiceName = @serviceName")
		args = append(args, clickhouse.Named("serviceName", q.ServiceName))
	}
	if q.SpanName != "" {
		filter = append(filter, "SpanName = @spanName")
		args = append(args, clickhouse.Named("spanName", q.SpanName))
	}
	if len(q.ExcludePeerAddrs) > 0 {
		filter = append(filter, "SpanAttributes['net.sock.peer.addr'] NOT IN (@addrs)")
		args = append(args, clickhouse.Named("addrs", q.ExcludePeerAddrs))
	}
	return filter, args
}

func spansByServiceNameFilter(q SpanQuery) ([]string, []any) {
	filter := []string{
		"ServiceName = @serviceName",
		"SpanKind = 'SPAN_KIND_SERVER'",
	}
	args := []any{
		clickhouse.Named("serviceName", q.ServiceName),
	}
	if len(q.ExcludePeerAddrs) > 0 {
		filter = append(filter, "SpanAttributes['net.sock.peer.addr'] NOT IN (@addrs)")
		args = append(args, clickhouse.Named("addrs", q.ExcludePeerAddrs))
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

func durationFilter(durFrom, durTo time.Duration, errors bool) (string, []any) {
	var filter string
	switch {
	case durFrom > 0 && durTo > 0 && errors:
		filter = "(Duration BETWEEN @durFrom AND @durTo OR StatusCode = 'STATUS_CODE_ERROR')"
	case durFrom == 0 && durTo > 0 && errors:
		filter = "(Duration <= @durTo OR StatusCode = 'STATUS_CODE_ERROR')"
	case durFrom > 0 && durTo == 0 && errors:
		filter = "(Duration >= @durFrom OR StatusCode = 'STATUS_CODE_ERROR')"
	case durFrom == 0 && durTo == 0 && errors:
		filter = "StatusCode = 'STATUS_CODE_ERROR'"
	case durFrom > 0 && durTo > 0 && !errors:
		filter = "Duration BETWEEN @durFrom AND @durTo"
	case durFrom == 0 && durTo > 0 && !errors:
		filter = "Duration <= @durTo"
	case durFrom > 0 && durTo == 0 && !errors:
		filter = "Duration >= @durFrom"
	}
	var args []any
	if durFrom > 0 {
		args = append(args, clickhouse.Named("durFrom", durFrom.Nanoseconds()))
	}
	if durTo > 0 {
		args = append(args, clickhouse.Named("durTo", durTo.Nanoseconds()))
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
