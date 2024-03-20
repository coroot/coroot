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
	filter, filterArgs := q.RootSpansFilter()
	return c.getSpanAttrStats(ctx, q, filter, filterArgs)
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
		"@traceIds as traceIds, (SELECT min(Start) FROM otel_traces_trace_id_ts WHERE TraceId IN (traceIds)) as start, (SELECT max(End) + 1 FROM otel_traces_trace_id_ts WHERE TraceId IN (traceIds)) as end",
		"",
		[]string{"Timestamp BETWEEN start AND end", "TraceId IN (traceIds)", "(TraceId, SpanId) IN (@ids)"},
		[]any{
			clickhouse.Named("traceIds", maps.Keys(traceIds)),
			clickhouse.Named("ids", ids),
		},
	)
}

func (c *Client) GetSpansByTraceId(ctx context.Context, traceId string) ([]*model.TraceSpan, error) {
	var q SpanQuery
	return c.getSpans(ctx, q,
		"(SELECT min(Start) FROM otel_traces_trace_id_ts WHERE TraceId = @traceId) as start, (SELECT max(End) + 1 FROM otel_traces_trace_id_ts WHERE TraceId = @traceId) as end",
		"Timestamp",
		[]string{"TraceId = @traceId", "Timestamp BETWEEN start AND end"},
		[]any{
			clickhouse.Named("traceId", traceId),
		},
	)
}

func (c *Client) getSpansHistogram(ctx context.Context, q SpanQuery, filters []string, filterArgs []any) ([]model.HistogramBucket, error) {
	step := q.Ctx.Step
	from := q.Ctx.From
	to := q.Ctx.To.Add(step)

	tsFilter := "Timestamp BETWEEN @from AND @to"
	filters = append(filters, tsFilter)
	filterArgs = append(filterArgs,
		clickhouse.Named("step", step),
		clickhouse.Named("buckets", histogramBuckets[:len(histogramBuckets)-1]),
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.NanoSeconds),
	)

	var with string
	var join string

	if q.Attribute != nil {
		attrFilter, attrFilterArgs := q.SpansByAttributeFilter()
		filterArgs = append(filterArgs, attrFilterArgs...)
		with = "t AS ("
		with += "SELECT DISTINCT TraceId"
		with += " FROM otel_traces"
		with += " WHERE " + strings.Join([]string{tsFilter, attrFilter}, " AND ")
		with += ")"
		join = "t USING(TraceId)"
	}

	var query string

	if with != "" {
		query += "WITH " + with
	}

	query += "SELECT toStartOfInterval(Timestamp, INTERVAL @step second), roundDown(Duration/1000000, @buckets), count(1), count(if(StatusCode = 'STATUS_CODE_ERROR', 1, NULL))"
	query += " FROM otel_traces"

	if join != "" {
		query += " JOIN " + join
	}

	query += " WHERE " + strings.Join(filters, " AND ")
	query += " GROUP BY 1, 2"

	rows, err := c.conn.Query(ctx, query, filterArgs...)
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

	query := "SELECT ServiceName, SpanName, roundDown(Duration/1000000, @buckets), count(1), count(if(StatusCode = 'STATUS_CODE_ERROR', 1, NULL))"
	query += " FROM otel_traces"
	query += " WHERE " + strings.Join(filters, " AND ")
	query += " GROUP BY 1, 2, 3"

	rows, err := c.conn.Query(ctx, query, filterArgs...)
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

	var join string

	if q.Attribute != nil {
		attrFilter, attrFilterArgs := q.SpansByAttributeFilter()
		filterArgs = append(filterArgs, attrFilterArgs...)
		with = "t AS ("
		with += "SELECT DISTINCT TraceId"
		with += " FROM otel_traces"
		with += " WHERE " + strings.Join([]string{tsFilter, attrFilter}, " AND ")
		with += ")"
		join = "t USING(TraceId)"
	}

	query := ""
	if with != "" {
		query += "WITH " + with
	}
	query += " SELECT Timestamp, TraceId, SpanId, ParentSpanId, SpanName, ServiceName, Duration, StatusCode, StatusMessage, ResourceAttributes, SpanAttributes, Events.Timestamp, Events.Name, Events.Attributes"
	query += " FROM otel_traces"
	if join != "" {
		query += " JOIN " + join
	}
	query += " WHERE " + strings.Join(filters, " AND ")
	if orderBy != "" {
		query += " ORDER BY " + orderBy
	}
	if q.Limit > 0 {
		query += " LIMIT " + fmt.Sprint(q.Limit)
	}

	rows, err := c.conn.Query(ctx, query, filterArgs...)
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

func (c *Client) getSpanAttrStats(ctx context.Context, q SpanQuery, filters []string, filterArgs []any) ([]model.TraceSpanAttrStats, error) {
	tsFilter := "Timestamp BETWEEN @from AND @to"
	filters = append(filters, tsFilter)
	filterArgs = append(filterArgs,
		clickhouse.DateNamed("from", q.Ctx.From.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("to", q.Ctx.To.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("tsFrom", q.TsFrom.ToStandard(), clickhouse.Seconds),
		clickhouse.DateNamed("tsTo", q.TsTo.ToStandard(), clickhouse.Seconds),
		clickhouse.Named("top", q.Limit),
	)

	isInSelection := "false"
	if q.IsSelectionDefined() {
		isInSelection = "Timestamp BETWEEN @tsFrom AND @tsTo"
		durFilter, durFilterArgs := q.DurationFilter()
		if durFilter != "" {
			isInSelection += " AND " + durFilter
			filterArgs = append(filterArgs, durFilterArgs...)
		}
	}

	traceFilters := strings.Join(filters, " AND ")
	traceIds := fmt.Sprintf("SELECT DISTINCT TraceId FROM otel_traces WHERE %s", traceFilters)
	if q.Attribute != nil {
		attrFilter, attrFilterArgs := q.SpansByAttributeFilter()
		filterArgs = append(filterArgs, attrFilterArgs...)
		traceIds = fmt.Sprintf(
			"SELECT DISTINCT TraceId FROM otel_traces JOIN (SELECT DISTINCT TraceId FROM otel_traces WHERE %s AND %s) t USING(TraceId) WHERE %s",
			tsFilter, attrFilter, traceFilters)
	}

	query := fmt.Sprintf(`
WITH t AS (
	%[1]s
), a AS (
    SELECT 'SpanName' as source, ('SpanName', SpanName) AS attribute, %[3]s AS selection, count(*) AS count
    FROM otel_traces JOIN t USING(TraceId) WHERE %[2]s GROUP BY attribute, selection
    UNION ALL
    SELECT 'StatusCode' as source, ('StatusCode', StatusCode) AS attribute, %[3]s AS selection, count(*) AS count
    FROM otel_traces JOIN t USING(TraceId) WHERE %[2]s GROUP BY attribute, selection
    UNION ALL
    SELECT 'StatusMessage' as source, ('StatusMessage', StatusMessage) AS attribute, %[3]s AS selection, count(*) AS count
    FROM otel_traces JOIN t USING(TraceId) WHERE %[2]s GROUP BY attribute, selection
    UNION ALL
    SELECT 'ResourceAttributes' as source, arrayJoin(ResourceAttributes) AS attribute, %[3]s AS selection, count(*) AS count
    FROM otel_traces JOIN t USING(TraceId) WHERE %[2]s GROUP BY attribute, selection
    UNION ALL
    SELECT 'SpanAttributes' as source, arrayJoin(SpanAttributes) AS attribute, %[3]s AS selection, count(*) AS count
    FROM otel_traces JOIN t USING(TraceId) WHERE %[2]s GROUP BY attribute, selection
), s AS (
    SELECT
        source,
        tupleElement(attribute, 1) AS name,
        tupleElement(attribute, 2) AS value,
        sum(if(a.selection, count, 0)) AS selection,
        sum(if(a.selection, 0, count)) AS baseline,
        sum(selection) OVER (PARTITION BY name) AS selection_total,
        sum(baseline) OVER (PARTITION BY name) AS baseline_total
    FROM a
    GROUP BY source, name, value
), p AS (
    SELECT
        source, name, value,
        if(selection_total=0, 0, selection / selection_total) AS selection,
        if(baseline_total=0, 0, baseline / baseline_total) AS baseline,
        row_number() OVER (PARTITION BY name ORDER BY selection+baseline DESC) AS top
    FROM s
)
SELECT source, name, value, selection, baseline FROM p WHERE top <= @top`,
		traceIds, tsFilter, isInSelection,
	)

	rows, err := c.conn.Query(ctx, query, filterArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type Attr struct{ source, name string }

	var attr Attr
	var value string
	var selection, baseline float64
	attrs := map[Attr][]*model.TraceSpanAttrStatsValue{}
	maxDiff := map[Attr]float64{}
	for rows.Next() {
		if err = rows.Scan(&attr.source, &attr.name, &value, &selection, &baseline); err != nil {
			return nil, err
		}
		diff := selection - baseline
		if diff > maxDiff[attr] {
			maxDiff[attr] = diff
		}
		attrs[attr] = append(attrs[attr], &model.TraceSpanAttrStatsValue{
			Name:      value,
			Selection: float32(selection),
			Baseline:  float32(baseline),
		})
	}
	var res []model.TraceSpanAttrStats
	for a, vs := range attrs {
		res = append(res, model.TraceSpanAttrStats{Source: a.source, Name: a.name, Values: vs})
	}
	sort.Slice(res, func(i, j int) bool {
		ri, rj := res[i], res[j]
		return maxDiff[Attr{source: ri.Source, name: ri.Name}] > maxDiff[Attr{source: rj.Source, name: rj.Name}]
	})

	return res, nil
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
	Attribute        *model.TraceSpanAttr
	ExcludePeerAddrs []string
}

func (q SpanQuery) IsSelectionDefined() bool {
	return q.TsFrom > q.Ctx.From || q.TsTo < q.Ctx.To || q.DurFrom > 0 || q.DurTo > 0 || q.Errors
}

func (q SpanQuery) DurationFilter() (string, []any) {
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

func (q SpanQuery) RootSpansFilter() ([]string, []any) {
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
		filter = append(filter, "NetSockPeerAddr NOT IN (@addrs)")
		args = append(args, clickhouse.Named("addrs", q.ExcludePeerAddrs))
	}
	return filter, args
}

func (q SpanQuery) SpansByServiceNameFilter() ([]string, []any) {
	filter := []string{
		"ServiceName = @serviceName",
		"SpanKind = 'SPAN_KIND_SERVER'",
	}
	args := []any{
		clickhouse.Named("serviceName", q.ServiceName),
	}
	if len(q.ExcludePeerAddrs) > 0 {
		filter = append(filter, "NetSockPeerAddr NOT IN (@addrs)")
		args = append(args, clickhouse.Named("addrs", q.ExcludePeerAddrs))
	}
	return filter, args
}

func (q SpanQuery) SpansByAttributeFilter() (string, []any) {
	var f string
	switch q.Attribute.Source {
	case "SpanName":
		f = "SpanName = @attr_value"
	case "StatusCode":
		f = "StatusCode = @attr_value"
	case "StatusMessage":
		f = "StatusMessage = @attr_value"
	case "ResourceAttributes":
		f = "ResourceAttributes[@attr_name] = @attr_value"
	case "SpanAttributes":
		f = "SpanAttributes[@attr_name] = @attr_value"
	}
	args := []any{
		clickhouse.Named("attr_name", q.Attribute.Name),
		clickhouse.Named("attr_value", q.Attribute.Value),
	}
	return f, args
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
