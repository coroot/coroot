package clickhouse

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
)

func finiteF64(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

type RumServiceEdge struct {
	ClientService string
	ServerService string
	Requests      float64
	Failed        float64
	AvgLatencyMs  float64
}

type RumEventStat struct {
	PagePath    string
	EventType   string
	Count       uint64
	P75         float64 // latency at selected percentile (ms)
	LcpP75      float64 // LCP at selected percentile (ms)
	InpP75      float64 // INP at selected percentile (ms)
	ErrorCount  uint64
	BrowserName string
	OsName      string
	DeviceType  string
}

type RumPageView struct {
	Timestamp time.Time
	TraceId   string
	SpanId    string
	SessionId string
	PagePath  string
	Duration  float64
	Status    string
	Name      string
	Lcp       float64
	Ttfb      float64
	Inp       float64
	Cls       float64
	Browser   string
	Os        string
	Device    string
	Country   string
	Version   string
}

type RumSummary struct {
	PageViews      uint64
	P75LoadMs      float64 // latency at selected percentile (legacy field name)
	P75LcpMs       float64
	P75TtfbMs      float64
	P75InpMs       float64
	P75Cls         float64
	ErrorCount     uint64
	FetchErrorPct  float32
	UniqueSessions uint64
}

func (c *Client) GetRumServices(ctx context.Context, from timeseries.Time) ([]string, error) {
	rows, err := c.Query(ctx, "SELECT DISTINCT ServiceName FROM @@table_rum_service_name@@ WHERE LastSeen >= @from",
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.NanoSeconds),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []string
	for rows.Next() {
		var s string
		if err = rows.Scan(&s); err != nil {
			return nil, err
		}
		res = append(res, s)
	}
	return res, nil
}

func (c *Client) GetRumServiceEdges(ctx context.Context, from, to timeseries.Time) ([]RumServiceEdge, error) {
	// NOTE: do not alias sum(Requests) as Requests — ClickHouse SummingMergeTree expands
	// that into nested aggregates when HAVING references the alias (ILLEGAL_AGGREGATION).
	q := `
SELECT ClientService, ServerService,
       toFloat64(sum(Requests)) AS reqs,
       toFloat64(sum(Failed)) AS fails,
       if(sum(Requests)=0, 0, sum(DurationSum)/sum(Requests)) AS AvgLatency
FROM @@table_rum_service_edges@@
WHERE Timestamp >= @from AND Timestamp <= @to
GROUP BY ClientService, ServerService
HAVING sum(Requests) > 0`
	rows, err := c.Query(ctx, q,
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.Seconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.Seconds),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []RumServiceEdge
	for rows.Next() {
		var e RumServiceEdge
		if err = rows.Scan(&e.ClientService, &e.ServerService, &e.Requests, &e.Failed, &e.AvgLatencyMs); err != nil {
			return nil, err
		}
		res = append(res, e)
	}
	return res, nil
}

func (c *Client) GetRumSpansHistogram(ctx context.Context, service string, from, to timeseries.Time, step timeseries.Duration) ([]model.HistogramBucket, error) {
	if r := step % 60; r != 0 {
		step += 60 - r
	}
	to = to.Add(step)
	q := `
SELECT
    toStartOfInterval(Timestamp, INTERVAL @step second),
    Bucket,
    sum(Total),
    sum(Failed)
FROM @@table_rum_spans_histogram@@
WHERE ServiceName = @service AND Timestamp BETWEEN @from AND @to
GROUP BY 1, 2`
	rows, err := c.Query(ctx, q,
		clickhouse.Named("service", service),
		clickhouse.Named("step", int(step)),
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.Seconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.Seconds),
	)
	if err != nil {
		klog.Warningln(err)
		q = `
SELECT
    toStartOfInterval(Timestamp, INTERVAL @step second),
    roundDown(Duration/1000000, @buckets),
    count(1),
    countIf(StatusCode = 'STATUS_CODE_ERROR')
FROM @@table_rum_spans@@
WHERE ServiceName = @service AND Timestamp BETWEEN @from AND @to
GROUP BY 1, 2`
		rows, err = c.Query(ctx, q,
			clickhouse.Named("service", service),
			clickhouse.Named("step", int(step)),
			clickhouse.Named("buckets", HistogramBuckets[:len(HistogramBuckets)-1]),
			clickhouse.DateNamed("from", from.ToStandard(), clickhouse.NanoSeconds),
			clickhouse.DateNamed("to", to.ToStandard(), clickhouse.NanoSeconds),
		)
		if err != nil {
			return nil, err
		}
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
		{TimeSeries: timeseries.New(from, int(to.Sub(from)/step), step)},
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

// GetRumRootSpansByServiceName returns recent RUM spans for the Tracing list:
// page/route roots, client HTTP, UI actions, and errors — not only documentLoad.
func (c *Client) GetRumRootSpansByServiceName(ctx context.Context, q SpanQuery) ([]*model.TraceSpan, error) {
	filters := []string{
		"Timestamp >= @tsFrom AND Timestamp < @tsTo",
		// Interesting entries for Tracing (exclude CWV leaf spans like lcp/ttfb).
		`(
			ParentSpanId = ''
			OR SpanKind = 'SPAN_KIND_CLIENT'
			OR StatusCode = 'STATUS_CODE_ERROR'
			OR startsWith(SpanName, 'ui.')
			OR SpanName IN ('routeChange', 'documentLoad', 'window.onerror', 'unhandledrejection')
		)`,
		`SpanName NOT IN ('lcp', 'ttfb', 'cls', 'inp', 'fcp', 'fid')`,
	}
	args := []any{
		clickhouse.DateNamed("tsFrom", q.TsFrom.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("tsTo", q.TsTo.ToStandard(), clickhouse.NanoSeconds),
	}
	for i, f := range q.Filters {
		if f.Field != "ServiceName" || f.Op != "=" {
			continue
		}
		filters = append(filters, fmt.Sprintf("ServiceName = @svc_%d", i))
		args = append(args, clickhouse.Named(fmt.Sprintf("svc_%d", i), f.Value))
	}
	if durFilter, durArgs := q.DurationFilter(); durFilter != "" {
		filters = append(filters, durFilter)
		args = append(args, durArgs...)
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 100
	}
	query := `
SELECT Timestamp, TraceId, SpanId, ParentSpanId, SpanName, ServiceName, Duration, StatusCode, StatusMessage, ResourceAttributes, SpanAttributes, Events.Timestamp, Events.Name, Events.Attributes
FROM @@table_rum_spans@@
WHERE ` + strings.Join(filters, " AND ") + `
ORDER BY Timestamp DESC
LIMIT ` + fmt.Sprint(limit)
	rows, err := c.Query(ctx, query, args...)
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

// GetRumWebVitalP75 keeps exact p75 for auditor/checks (stable alert thresholds).
func (c *Client) GetRumWebVitalP75(ctx context.Context, service, eventType string, from, to timeseries.Time, step timeseries.Duration) (*timeseries.TimeSeries, error) {
	return c.getRumWebVitalSeries(ctx, service, eventType, from, to, step, 0.75, RumFilter{}, true)
}

// GetRumWebVitalQuantile returns a TDigest percentile series for the RUM page (view-only).
func (c *Client) GetRumWebVitalQuantile(ctx context.Context, service, eventType string, from, to timeseries.Time, step timeseries.Duration, qLevel float64, f RumFilter) (*timeseries.TimeSeries, error) {
	return c.getRumWebVitalSeries(ctx, service, eventType, from, to, step, qLevel, f, false)
}

func (c *Client) getRumWebVitalSeries(ctx context.Context, service, eventType string, from, to timeseries.Time, step timeseries.Duration, qLevel float64, f RumFilter, exact bool) (*timeseries.TimeSeries, error) {
	// Prefer rollup table (TDigest merge). `exact` is kept for API compatibility but also uses rollups.
	_ = exact
	rf, rArgs := f.rollupFilterSQL("")
	q := `
SELECT
    toStartOfInterval(Timestamp, INTERVAL @step SECOND) AS ts,
    toFloat64(quantileTDigestMerge(@q)(ValueQ)) AS qv
FROM @@table_rum_events_1m@@
WHERE ServiceName = @service AND EventType = @eventType AND Timestamp >= @from AND Timestamp <= @to` + rf + `
GROUP BY ts
ORDER BY ts`
	args := append([]any{
		clickhouse.Named("service", service),
		clickhouse.Named("eventType", eventType),
		clickhouse.Named("step", int(step)),
		clickhouse.Named("q", qLevel),
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.Seconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.Seconds),
	}, rArgs...)
	rows, err := c.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ts := timeseries.New(from, int(to.Sub(from)/step)+1, step)
	for rows.Next() {
		var t time.Time
		var qv float64
		if err = rows.Scan(&t, &qv); err != nil {
			return nil, err
		}
		ts.Set(timeseries.TimeFromStandard(t), float32(finiteF64(qv)))
	}
	return ts, nil
}

func (c *Client) GetRumErrorRate(ctx context.Context, service string, from, to timeseries.Time, step timeseries.Duration) (*timeseries.TimeSeries, error) {
	return c.GetRumErrorRateFiltered(ctx, service, from, to, step, RumFilter{})
}

func (c *Client) GetRumErrorRateFiltered(ctx context.Context, service string, from, to timeseries.Time, step timeseries.Duration, f RumFilter) (*timeseries.TimeSeries, error) {
	rf, rArgs := f.rollupFilterSQL("")
	q := `
SELECT
    toStartOfInterval(Timestamp, INTERVAL @step SECOND) AS ts,
    sum(Count) AS errors
FROM @@table_rum_events_1m@@
WHERE ServiceName = @service AND Timestamp >= @from AND Timestamp <= @to
  AND EventType IN ('js_error','http_error')` + rf + `
GROUP BY ts
ORDER BY ts`
	args := append([]any{
		clickhouse.Named("service", service),
		clickhouse.Named("step", int(step)),
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.Seconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.Seconds),
	}, rArgs...)
	rows, err := c.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ts := timeseries.New(from, int(to.Sub(from)/step)+1, step)
	for rows.Next() {
		var t time.Time
		var errors uint64
		if err = rows.Scan(&t, &errors); err != nil {
			return nil, err
		}
		ts.Set(timeseries.TimeFromStandard(t), float32(errors)/float32(step))
	}
	return ts, nil
}

func (c *Client) GetRumPageViewRate(ctx context.Context, service string, from, to timeseries.Time, step timeseries.Duration) (*timeseries.TimeSeries, error) {
	return c.GetRumPageViewRateFiltered(ctx, service, from, to, step, RumFilter{})
}

func (c *Client) GetRumPageViewRateFiltered(ctx context.Context, service string, from, to timeseries.Time, step timeseries.Duration, f RumFilter) (*timeseries.TimeSeries, error) {
	rf, rArgs := f.rollupFilterSQL("")
	q := `
SELECT
    toStartOfInterval(Timestamp, INTERVAL @step SECOND) AS ts,
    sum(Views) AS views
FROM @@table_rum_page_stats_1m@@
WHERE ServiceName = @service AND Timestamp >= @from AND Timestamp <= @to` + rf + `
GROUP BY ts
ORDER BY ts`
	args := append([]any{
		clickhouse.Named("service", service),
		clickhouse.Named("step", int(step)),
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.Seconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.Seconds),
	}, rArgs...)
	rows, err := c.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ts := timeseries.New(from, int(to.Sub(from)/step)+1, step)
	for rows.Next() {
		var t time.Time
		var views uint64
		if err = rows.Scan(&t, &views); err != nil {
			return nil, err
		}
		ts.Set(timeseries.TimeFromStandard(t), float32(views)/float32(step))
	}
	return ts, nil
}

func (c *Client) GetRumTopPages(ctx context.Context, service string, from, to timeseries.Time, limit int) ([]RumEventStat, error) {
	return c.GetRumTopPagesFiltered(ctx, service, from, to, 0.75, RumFilter{}, limit)
}

func (c *Client) GetRumTopPagesFiltered(ctx context.Context, service string, from, to timeseries.Time, qLevel float64, f RumFilter, limit int) ([]RumEventStat, error) {
	if limit <= 0 {
		limit = 20
	}
	rf, rArgs := f.rollupFilterSQL("")
	q := `
SELECT
    s.PagePath,
    s.cnt,
    s.lat,
    s.errors,
    if(isNaN(ifNull(e.lcp_q, 0)), 0, ifNull(e.lcp_q, 0)) AS lcp_q,
    if(isNaN(ifNull(e.inp_q, 0)), 0, ifNull(e.inp_q, 0)) AS inp_q
FROM (
    SELECT
           PagePath,
           sum(Views) AS cnt,
           toFloat64(if(isNaN(quantileTDigestMerge(@q)(DurationQ)), 0, quantileTDigestMerge(@q)(DurationQ))) AS lat,
           sum(Errors) AS errors
    FROM @@table_rum_page_stats_1m@@
    WHERE ServiceName = @service AND Timestamp >= @from AND Timestamp <= @to` + rf + `
    GROUP BY PagePath
    HAVING PagePath != ''
) AS s
LEFT JOIN (
    SELECT
           PagePath,
           toFloat64(if(isNaN(quantileTDigestMergeIf(@q)(ValueQ, EventType = 'lcp')), 0, quantileTDigestMergeIf(@q)(ValueQ, EventType = 'lcp'))) AS lcp_q,
           toFloat64(if(isNaN(quantileTDigestMergeIf(@q)(ValueQ, EventType = 'inp')), 0, quantileTDigestMergeIf(@q)(ValueQ, EventType = 'inp'))) AS inp_q
    FROM @@table_rum_events_1m@@
    WHERE ServiceName = @service AND Timestamp >= @from AND Timestamp <= @to
      AND EventType IN ('lcp','inp')` + rf + `
    GROUP BY PagePath
    HAVING PagePath != ''
) AS e ON s.PagePath = e.PagePath
ORDER BY s.lat DESC
LIMIT @limit`
	args := append([]any{
		clickhouse.Named("service", service),
		clickhouse.Named("q", qLevel),
		clickhouse.Named("limit", limit),
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.Seconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.Seconds),
	}, rArgs...)
	rows, err := c.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []RumEventStat
	for rows.Next() {
		var s RumEventStat
		if err = rows.Scan(&s.PagePath, &s.Count, &s.P75, &s.ErrorCount, &s.LcpP75, &s.InpP75); err != nil {
			return nil, err
		}
		s.P75 = finiteF64(s.P75)
		s.LcpP75 = finiteF64(s.LcpP75)
		s.InpP75 = finiteF64(s.InpP75)
		res = append(res, s)
	}
	return res, nil
}

func (c *Client) GetRumBreakdown(ctx context.Context, service string, from, to timeseries.Time, dim string) ([]RumEventStat, error) {
	return c.GetRumBreakdownFiltered(ctx, service, from, to, dim, 0.75, RumFilter{})
}

func (c *Client) GetRumBreakdownFiltered(ctx context.Context, service string, from, to timeseries.Time, dim string, qLevel float64, f RumFilter) ([]RumEventStat, error) {
	col := "BrowserName"
	switch dim {
	case "os":
		col = "OsName"
	case "device":
		col = "DeviceType"
	case "browser":
		col = "BrowserName"
	default:
		return nil, fmt.Errorf("unsupported dimension: %s", dim)
	}
	rf, rArgs := f.rollupFilterSQL("")
	q := fmt.Sprintf(`
SELECT %s,
       sum(Views) AS cnt,
       toFloat64(if(isNaN(quantileTDigestMerge(@q)(DurationQ)), 0, quantileTDigestMerge(@q)(DurationQ))) AS lat,
       sum(Errors) AS errors
FROM @@table_rum_page_stats_1m@@
WHERE ServiceName = @service AND Timestamp >= @from AND Timestamp <= @to AND %s != ''%s
GROUP BY %s
ORDER BY cnt DESC
LIMIT 20`, col, col, rf, col)
	args := append([]any{
		clickhouse.Named("service", service),
		clickhouse.Named("q", qLevel),
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.Seconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.Seconds),
	}, rArgs...)
	rows, err := c.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []RumEventStat
	for rows.Next() {
		var name string
		var s RumEventStat
		if err = rows.Scan(&name, &s.Count, &s.P75, &s.ErrorCount); err != nil {
			return nil, err
		}
		s.P75 = finiteF64(s.P75)
		switch dim {
		case "os":
			s.OsName = name
		case "device":
			s.DeviceType = name
		default:
			s.BrowserName = name
		}
		res = append(res, s)
	}
	return res, nil
}

func (c *Client) GetRumPageViews(ctx context.Context, service string, from, to timeseries.Time, limit int) ([]RumPageView, error) {
	return c.GetRumPageViewsFiltered(ctx, service, from, to, RumFilter{}, nil, nil, limit)
}

func (c *Client) GetRumPageViewsFiltered(ctx context.Context, service string, from, to timeseries.Time, f RumFilter, sel *RumSelection, ex *RumExplorerFilter, limit int) ([]RumPageView, error) {
	if limit <= 0 {
		limit = 25
	}
	sf, sArgs := f.spanFilterSQL("")
	selSQL, selArgs := selectionSpanSQL(sel)
	var exSQL string
	var exArgs []any
	if ex != nil {
		exSQL, exArgs = ex.PageViewWhereSQL()
	}
	q := `
SELECT
    s.Timestamp,
    s.TraceId,
    s.SpanId,
    s.SessionId,
    s.PagePath,
    s.DurationMs,
    s.StatusCode,
    s.SpanName,
    ifNull(e.Lcp, 0),
    ifNull(e.Ttfb, 0),
    ifNull(e.Inp, 0),
    ifNull(e.Cls, 0),
    ifNull(e.BrowserName, s.BrowserName),
    s.OsName,
    ifNull(e.DeviceType, s.DeviceType),
    s.Country,
    s.Version
FROM (
    SELECT Timestamp, TraceId, SpanId, SessionId, BrowserName, OsName, DeviceType,
           coalesce(
             nullIf(PagePath, ''),
             nullIf(SpanAttributes['page.path'], ''),
             nullIf(SpanAttributes['page.url.path'], ''),
             nullIf(SpanAttributes['url.path'], '')
           ) AS PagePath,
           Duration/1000000 AS DurationMs, StatusCode, SpanName,
           coalesce(nullIf(SpanAttributes['geo.country'], ''), nullIf(ResourceAttributes['geo.country'], ''), 'unknown') AS Country,
           coalesce(nullIf(ResourceAttributes['service.version'], ''), nullIf(SpanAttributes['service.version'], ''), '') AS Version
    FROM @@table_rum_spans@@
    WHERE ServiceName = @service AND Timestamp >= @from AND Timestamp <= @to
      AND ` + pageViewSpanPredicate("") + sf + selSQL + `
) AS s
LEFT JOIN (
    SELECT
        TraceId,
        maxIf(Value, EventType = 'lcp' AND Value > 0) AS Lcp,
        maxIf(Value, EventType = 'ttfb' AND Value > 0) AS Ttfb,
        maxIf(Value, EventType = 'inp' AND Value > 0) AS Inp,
        maxIf(Value, EventType = 'cls') AS Cls,
        anyIf(BrowserName, BrowserName != '') AS BrowserName,
        anyIf(DeviceType, DeviceType != '') AS DeviceType
    FROM @@table_rum_events@@
    WHERE ServiceName = @service AND Timestamp >= @from AND Timestamp <= @to
      AND EventType IN ('lcp', 'ttfb', 'inp', 'cls')
      AND TraceId IN (
        SELECT TraceId FROM @@table_rum_spans@@
        WHERE ServiceName = @service AND Timestamp >= @from AND Timestamp <= @to
          AND ` + pageViewSpanPredicate("") + sf + selSQL + `
      )
    GROUP BY TraceId
) AS e ON s.TraceId = e.TraceId
WHERE 1` + exSQL + `
ORDER BY s.Timestamp DESC
LIMIT @limit`
	args := append([]any{
		clickhouse.Named("service", service),
		clickhouse.Named("limit", limit),
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.NanoSeconds),
	}, sArgs...)
	args = append(args, selArgs...)
	// span/sel args appear twice (s subquery + events TraceId IN); ClickHouse named args can be reused.
	args = append(args, exArgs...)
	rows, err := c.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []RumPageView
	for rows.Next() {
		var v RumPageView
		if err = rows.Scan(
			&v.Timestamp, &v.TraceId, &v.SpanId, &v.SessionId, &v.PagePath, &v.Duration, &v.Status, &v.Name,
			&v.Lcp, &v.Ttfb, &v.Inp, &v.Cls, &v.Browser, &v.Os, &v.Device, &v.Country, &v.Version,
		); err != nil {
			return nil, err
		}
		v.Duration = finiteF64(v.Duration)
		v.Lcp = finiteF64(v.Lcp)
		v.Ttfb = finiteF64(v.Ttfb)
		v.Inp = finiteF64(v.Inp)
		v.Cls = finiteF64(v.Cls)
		res = append(res, v)
	}
	return res, nil
}

func (c *Client) GetRumSummary(ctx context.Context, service string, from, to timeseries.Time) (*RumSummary, error) {
	return c.GetRumSummaryFiltered(ctx, service, from, to, 0.75, RumFilter{})
}

func (c *Client) GetRumSummaryFiltered(ctx context.Context, service string, from, to timeseries.Time, qLevel float64, f RumFilter) (*RumSummary, error) {
	rf, rArgs := f.rollupFilterSQL("")
	q := `
SELECT
    sum(Views) AS cnt,
    toFloat64(if(isNaN(quantileTDigestMerge(@q)(DurationQ)), 0, quantileTDigestMerge(@q)(DurationQ))) AS load_ms
FROM @@table_rum_page_stats_1m@@
WHERE ServiceName = @service AND Timestamp >= @from AND Timestamp <= @to` + rf
	args := append([]any{
		clickhouse.Named("service", service),
		clickhouse.Named("q", qLevel),
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.Seconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.Seconds),
	}, rArgs...)
	rows, err := c.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	s := &RumSummary{}
	if rows.Next() {
		if err = rows.Scan(&s.PageViews, &s.P75LoadMs); err != nil {
			return nil, err
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	s.P75LoadMs = finiteF64(s.P75LoadMs)

	vitalQ := func(eventType string) float64 {
		vq := `
SELECT toFloat64(if(isNaN(quantileTDigestMerge(@q)(ValueQ)), 0, quantileTDigestMerge(@q)(ValueQ)))
FROM @@table_rum_events_1m@@
WHERE ServiceName = @service AND EventType = @eventType AND Timestamp >= @from AND Timestamp <= @to` + rf
		vargs := append([]any{
			clickhouse.Named("service", service),
			clickhouse.Named("eventType", eventType),
			clickhouse.Named("q", qLevel),
			clickhouse.DateNamed("from", from.ToStandard(), clickhouse.Seconds),
			clickhouse.DateNamed("to", to.ToStandard(), clickhouse.Seconds),
		}, rArgs...)
		vrows, e := c.Query(ctx, vq, vargs...)
		if e != nil {
			return 0
		}
		defer vrows.Close()
		var v float64
		if vrows.Next() {
			_ = vrows.Scan(&v)
		}
		return finiteF64(v)
	}
	s.P75LcpMs = vitalQ("lcp")
	s.P75TtfbMs = vitalQ("ttfb")
	s.P75InpMs = vitalQ("inp")
	s.P75Cls = vitalQ("cls")

	eq := `
SELECT sum(Count)
FROM @@table_rum_events_1m@@
WHERE ServiceName = @service AND Timestamp >= @from AND Timestamp <= @to
  AND EventType IN ('js_error','http_error')` + rf
	erows, eerr := c.Query(ctx, eq, append([]any{
		clickhouse.Named("service", service),
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.Seconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.Seconds),
	}, rArgs...)...)
	if eerr == nil {
		defer erows.Close()
		if erows.Next() {
			_ = erows.Scan(&s.ErrorCount)
		}
	}

	if n, e := c.GetRumUniqueSessions(ctx, service, from, to, f); e == nil {
		s.UniqueSessions = n
	}

	edges, eerr := c.GetRumServiceEdges(ctx, from, to)
	if eerr == nil {
		var req, failed float64
		for _, e := range edges {
			if e.ClientService == service {
				req += e.Requests
				failed += e.Failed
			}
		}
		if req > 0 {
			s.FetchErrorPct = float32(failed / req * 100)
		}
	}
	return s, nil
}

func (c *Client) GetRumSpansByTraceId(ctx context.Context, traceId string) ([]*model.TraceSpan, error) {
	q := `
SELECT Timestamp, TraceId, SpanId, ParentSpanId, SpanName, ServiceName, Duration, StatusCode, StatusMessage, ResourceAttributes, SpanAttributes, Events.Timestamp, Events.Name, Events.Attributes
FROM @@table_rum_spans@@
WHERE TraceId = @traceId
ORDER BY Timestamp`
	rows, err := c.Query(ctx, q, clickhouse.Named("traceId", traceId))
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

func NewRumClientApplicationId(serviceName string) model.ApplicationId {
	return model.NewApplicationId(model.ClusterIdExternal, "frontend", model.ApplicationKindRumClient, serviceName)
}
