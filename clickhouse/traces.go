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

func (c *Client) GetRootSpansHistogram(ctx context.Context, ignoredPeerAddrs []string, from, to timeseries.Time, step timeseries.Duration) ([]model.HistogramBucket, error) {
	filter, filterArgs := rootSpansFilter(ignoredPeerAddrs)
	return c.getSpansHistogram(ctx, from, to, step, filter, filterArgs)
}

func (c *Client) GetRootSpans(ctx context.Context, ignoredPeerAddrs []string, tsFrom, tsTo timeseries.Time, durFrom, durTo time.Duration, errors bool, limit int) ([]*model.TraceSpan, error) {
	filter, filterArgs := rootSpansFilter(ignoredPeerAddrs)
	return c.getSpans(ctx, tsFrom, tsTo, durFrom, durTo, errors, "", "Duration DESC", limit, filter, filterArgs)
}

func (c *Client) GetSpanAttrsStat(ctx context.Context, from, to, tsFrom, tsTo timeseries.Time, durFrom, durTo time.Duration, errors bool, limit int) ([]model.TraceSpanStatsAttr, error) {
	return c.getSpansStat(ctx, from, to, tsFrom, tsTo, durFrom, durTo, errors, limit)
}

func (c *Client) GetSpansByServiceNameHistogram(ctx context.Context, name string, ignoredPeerAddrs []string, from, to timeseries.Time, step timeseries.Duration) ([]model.HistogramBucket, error) {
	filter, filterArgs := spansByServiceNameFilter(name, ignoredPeerAddrs)
	return c.getSpansHistogram(ctx, from, to, step, filter, filterArgs)
}

func (c *Client) GetSpansByServiceName(ctx context.Context, name string, ignoredPeerAddrs []string, tsFrom, tsTo timeseries.Time, durFrom, durTo time.Duration, errors bool, limit int) ([]*model.TraceSpan, error) {
	filter, filterArgs := spansByServiceNameFilter(name, ignoredPeerAddrs)
	return c.getSpans(ctx, tsFrom, tsTo, durFrom, durTo, errors, "", "Timestamp DESC", limit, filter, filterArgs)
}

func (c *Client) GetInboundSpansHistogram(ctx context.Context, clients []string, listens []model.Listen, from, to timeseries.Time, step timeseries.Duration) ([]model.HistogramBucket, error) {
	if len(listens) == 0 {
		return nil, nil
	}
	filter, filterArgs := inboundSpansFilter(clients, listens)
	return c.getSpansHistogram(ctx, from, to, step, filter, filterArgs)
}

func (c *Client) GetInboundSpans(ctx context.Context, clients []string, listens []model.Listen, tsFrom, tsTo timeseries.Time, durFrom, durTo time.Duration, errors bool, limit int) ([]*model.TraceSpan, error) {
	if len(listens) == 0 {
		return nil, nil
	}
	filter, filterArgs := inboundSpansFilter(clients, listens)
	return c.getSpans(ctx, tsFrom, tsTo, durFrom, durTo, errors, "", "Timestamp DESC", limit, filter, filterArgs)
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
		"Timestamp BETWEEN start AND end AND TraceId IN (traceIds) AND (TraceId, SpanId) IN (@ids)",
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
		"TraceId = @traceId AND Timestamp BETWEEN start AND end",
		[]any{
			clickhouse.Named("traceId", traceId),
		},
	)
}

func (c *Client) getSpansHistogram(ctx context.Context, from, to timeseries.Time, step timeseries.Duration, filter string, filterArgs []any) ([]model.HistogramBucket, error) {
	to = to.Add(step)
	buckets := []float64{0, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000, math.Inf(1)}

	q := "SELECT toStartOfInterval(Timestamp, INTERVAL @step second), roundDown(Duration/1000000, @buckets), count(1), count(if(StatusCode = 'STATUS_CODE_ERROR', 1, NULL))"
	filters := []string{
		"Timestamp BETWEEN @from AND @to",
	}
	args := []any{
		clickhouse.Named("step", step),
		clickhouse.Named("buckets", buckets[:len(buckets)-1]),
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.NanoSeconds),
	}
	if filter != "" {
		filters = append(filters, filter)
		args = append(args, filterArgs...)
	}

	q += " FROM otel_traces"
	q += " WHERE " + strings.Join(filters, " AND ")
	q += " GROUP BY 1, 2"

	rows, err := c.conn.Query(ctx, q, args...)
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
	for i := 1; i < len(buckets); i++ {
		ts := byBucket[buckets[i-1]]
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
			Le:         float32(buckets[i] / 1000),
			TimeSeries: ts,
		})
	}
	return res, nil
}

func (c *Client) getSpans(ctx context.Context, tsFrom, tsTo timeseries.Time, durFrom, durTo time.Duration, errors bool, with string, orderBy string, limit int, filter string, filterArgs []any) ([]*model.TraceSpan, error) {
	var filters []string
	var args []any

	if !tsFrom.IsZero() && !tsTo.IsZero() {
		filters = append(filters, "Timestamp BETWEEN @tsFrom AND @tsTo")
		args = append(args,
			clickhouse.DateNamed("tsFrom", tsFrom.ToStandard(), clickhouse.NanoSeconds),
			clickhouse.DateNamed("tsTo", tsTo.ToStandard(), clickhouse.NanoSeconds),
		)
	}
	durFilter, durFilterArgs := durationFilter(durFrom, durTo, errors)
	if durFilter != "" {
		filters = append(filters, durFilter)
		args = append(args, durFilterArgs...)
	}

	if filter != "" {
		filters = append(filters, filter)
		args = append(args, filterArgs...)
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

	rows, err := c.conn.Query(ctx, q, args...)
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

func (c *Client) getSpansStat(ctx context.Context, from, to, tsFrom, tsTo timeseries.Time, durFrom, durTo time.Duration, errors bool, limit int) ([]model.TraceSpanStatsAttr, error) {
	args := []any{
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.Seconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.Seconds),
		clickhouse.DateNamed("tsFrom", tsFrom.ToStandard(), clickhouse.Seconds),
		clickhouse.DateNamed("tsTo", tsTo.ToStandard(), clickhouse.Seconds),
		clickhouse.Named("top", limit),
	}
	durFilter, durFilterArgs := durationFilter(durFrom, durTo, errors)
	selectionCondition := "Timestamp BETWEEN @tsFrom AND @tsTo"
	if durFilter != "" {
		selectionCondition += " AND " + durFilter
		args = append(args, durFilterArgs...)
	}
	q := fmt.Sprintf(`
WITH a AS (
    SELECT
        Name,
        Value,
        %s AS selection,
        sum(Count) AS count
    FROM otel_traces_attributes
    WHERE
        Timestamp BETWEEN @from AND @to AND
        NOT startsWith(ServiceName, '/')
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
		selectionCondition,
	)

	rows, err := c.conn.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var name, value string
	var selection, baseline float64
	byName := map[string][]*model.TraceSpanStatsAttrValue{}
	maxDiffByName := map[string]float64{}
	for rows.Next() {
		if err = rows.Scan(&name, &value, &selection, &baseline); err != nil {
			return nil, err
		}
		diff := selection - baseline
		if diff > maxDiffByName[name] {
			maxDiffByName[name] = diff
		}
		byName[name] = append(byName[name], &model.TraceSpanStatsAttrValue{
			Name:      value,
			Selection: float32(selection),
			Baseline:  float32(baseline),
		})
	}
	var res []model.TraceSpanStatsAttr
	for n, vs := range byName {
		res = append(res, model.TraceSpanStatsAttr{Name: n, Values: vs})
	}
	sort.Slice(res, func(i, j int) bool {
		return maxDiffByName[res[i].Name] > maxDiffByName[res[j].Name]
	})

	return res, nil
}

func rootSpansFilter(ignoredPeerAddrs []string) (string, []any) {
	filter := `
		SpanKind = 'SPAN_KIND_SERVER' AND 
		ParentSpanId = '' AND
		SpanAttributes['net.sock.peer.addr'] NOT IN (@addrs)
	`
	args := []any{
		clickhouse.Named("addrs", ignoredPeerAddrs),
	}
	return filter, args
}

func spansByServiceNameFilter(serviceName string, ignoredPeerAddrs []string) (string, []any) {
	filter := `
		ServiceName = @name AND 
		SpanKind = 'SPAN_KIND_SERVER' AND 
		SpanAttributes['net.sock.peer.addr'] NOT IN (@addrs)
	`
	args := []any{
		clickhouse.Named("name", serviceName),
		clickhouse.Named("addrs", ignoredPeerAddrs),
	}
	return filter, args
}

func inboundSpansFilter(clients []string, listens []model.Listen) (string, []any) {
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
	filter := `
		ServiceName IN (@services) AND
		(SpanAttributes['net.peer.name'] IN (@ips) OR (SpanAttributes['net.peer.name'], SpanAttributes['net.peer.port']) IN (@addrs)) 
	`
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
