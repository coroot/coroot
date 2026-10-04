package clickhouse

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/coroot/coroot/timeseries"
)

type RumErrorGroup struct {
	Message     string
	Type        string
	Count       uint64
	Sessions    uint64
	FirstSeen   time.Time
	LastSeen    time.Time
	TopPage     string
	SampleTrace string
}

type RumAjaxStat struct {
	Method      string
	URL         string
	PeerService string
	Count       uint64
	ErrorCount  uint64
	LatencyMs   float64
}

type RumSessionStat struct {
	SessionId  string
	StartedAt  time.Time
	DurationMs float64
	PageViews  uint64
	Errors     uint64
	Browser    string
	Os         string
	Device     string
	Country    string
	Version    string
}

type RumVitalBucket struct {
	EventType string
	Bucket    float64
	Total     uint64
}

type RumVitalRating struct {
	EventType string
	Good      uint64
	NeedsImp  uint64
	Poor      uint64
}

// GetRumErrors groups browser JS errors for the Errors table.
func (c *Client) GetRumErrors(ctx context.Context, service string, from, to timeseries.Time, f RumFilter, limit int) ([]RumErrorGroup, error) {
	if limit <= 0 {
		limit = 25
	}
	evFilter, evArgs := f.eventFilterSQL("")
	q := `
SELECT
  coalesce(nullIf(Attributes['exception.message'], ''), nullIf(Attributes['exception.type'], ''), 'Error') AS Msg,
  coalesce(nullIf(Attributes['exception.type'], ''), 'Error') AS Typ,
  count() AS Cnt,
  uniqExact(SessionId) AS Sessions,
  min(Timestamp) AS FirstSeen,
  max(Timestamp) AS LastSeen,
  anyHeavy(PagePath) AS TopPage,
  any(TraceId) AS SampleTrace
FROM @@table_rum_events@@
WHERE ServiceName = @service AND Timestamp >= @from AND Timestamp <= @to
  AND EventType = 'js_error'` + evFilter + `
GROUP BY Msg, Typ
ORDER BY Cnt DESC
LIMIT @limit`
	args := append([]any{
		clickhouse.Named("service", service),
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.Named("limit", limit),
	}, evArgs...)
	rows, err := c.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []RumErrorGroup
	for rows.Next() {
		var g RumErrorGroup
		if err = rows.Scan(&g.Message, &g.Type, &g.Count, &g.Sessions, &g.FirstSeen, &g.LastSeen, &g.TopPage, &g.SampleTrace); err != nil {
			return nil, err
		}
		res = append(res, g)
	}
	return res, nil
}

// GetRumAjaxStats returns client HTTP (fetch/XHR) aggregates.
func (c *Client) GetRumAjaxStats(ctx context.Context, service string, from, to timeseries.Time, qLevel float64, f RumFilter, limit int) ([]RumAjaxStat, error) {
	if limit <= 0 {
		limit = 25
	}
	sf, sArgs := f.spanFilterSQL("")
	q := `
SELECT
  coalesce(nullIf(SpanAttributes['http.method'], ''), splitByChar(' ', SpanName)[1], 'GET') AS Method,
  coalesce(
    nullIf(SpanAttributes['http.url'], ''),
    nullIf(SpanAttributes['url.full'], ''),
    SpanName
  ) AS Url,
  coalesce(
    nullIf(SpanAttributes['peer.service'], ''),
    nullIf(SpanAttributes['server.address'], ''),
    nullIf(SpanAttributes['http.host'], ''),
    ''
  ) AS Peer,
  count() AS Cnt,
  countIf(StatusCode = 'STATUS_CODE_ERROR') AS Errs,
  toFloat64(if(isNaN(quantileTDigest(@q)(Duration/1000000)), 0, quantileTDigest(@q)(Duration/1000000))) AS Lat
FROM @@table_rum_spans@@
WHERE ServiceName = @service AND Timestamp >= @from AND Timestamp <= @to
  AND SpanKind IN ('SPAN_KIND_CLIENT', 'SPAN_KIND_PRODUCER')` + sf + `
GROUP BY Method, Url, Peer
ORDER BY Cnt DESC
LIMIT @limit`
	args := append([]any{
		clickhouse.Named("service", service),
		clickhouse.Named("q", qLevel),
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.Named("limit", limit),
	}, sArgs...)
	rows, err := c.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []RumAjaxStat
	for rows.Next() {
		var s RumAjaxStat
		if err = rows.Scan(&s.Method, &s.URL, &s.PeerService, &s.Count, &s.ErrorCount, &s.LatencyMs); err != nil {
			return nil, err
		}
		s.LatencyMs = finiteF64(s.LatencyMs)
		res = append(res, s)
	}
	return res, nil
}

// GetRumSessions returns recent sessions for the Sessions table.
func (c *Client) GetRumSessions(ctx context.Context, service string, from, to timeseries.Time, f RumFilter, ex *RumExplorerFilter, limit int) ([]RumSessionStat, error) {
	if limit <= 0 {
		limit = 25
	}
	sf, sArgs := f.spanFilterSQL("")
	havingSQL, havingArgs := "", []any(nil)
	if ex != nil {
		havingSQL, havingArgs = ex.SessionHavingSQL()
	}
	q := `
SELECT
  SessionId,
  min(Timestamp) AS StartedAt,
  dateDiff('millisecond', min(Timestamp), max(Timestamp)) AS DurMs,
  countIf(` + pageViewSpanPredicate("") + `) AS PageViews,
  countIf(StatusCode = 'STATUS_CODE_ERROR') AS Errors,
  anyIf(BrowserName, BrowserName != '') AS Browser,
  anyIf(OsName, OsName != '') AS Os,
  anyIf(DeviceType, DeviceType != '') AS Device,
  coalesce(anyIf(SpanAttributes['geo.country'], SpanAttributes['geo.country'] != ''), 'unknown') AS Country,
  coalesce(anyIf(ResourceAttributes['service.version'], ResourceAttributes['service.version'] != ''), '') AS Version
FROM @@table_rum_spans@@
WHERE ServiceName = @service AND Timestamp >= @from AND Timestamp <= @to
  AND SessionId != ''` + sf + `
GROUP BY SessionId` + havingSQL + `
ORDER BY StartedAt DESC
LIMIT @limit`
	args := append([]any{
		clickhouse.Named("service", service),
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.Named("limit", limit),
	}, sArgs...)
	args = append(args, havingArgs...)
	rows, err := c.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []RumSessionStat
	for rows.Next() {
		var s RumSessionStat
		var durMs int64
		if err = rows.Scan(&s.SessionId, &s.StartedAt, &durMs, &s.PageViews, &s.Errors, &s.Browser, &s.Os, &s.Device, &s.Country, &s.Version); err != nil {
			return nil, err
		}
		s.DurationMs = float64(durMs)
		res = append(res, s)
	}
	return res, nil
}

// GetRumSessionSummary returns aggregate stats for a single session.
func (c *Client) GetRumSessionSummary(ctx context.Context, service, sessionId string, from, to timeseries.Time) (*RumSessionStat, error) {
	if sessionId == "" {
		return nil, nil
	}
	q := `
SELECT
  SessionId,
  min(Timestamp) AS StartedAt,
  dateDiff('millisecond', min(Timestamp), max(Timestamp)) AS DurMs,
  countIf(` + pageViewSpanPredicate("") + `) AS PageViews,
  countIf(StatusCode = 'STATUS_CODE_ERROR') AS Errors,
  anyIf(BrowserName, BrowserName != '') AS Browser,
  anyIf(OsName, OsName != '') AS Os,
  anyIf(DeviceType, DeviceType != '') AS Device,
  coalesce(anyIf(SpanAttributes['geo.country'], SpanAttributes['geo.country'] != ''), 'unknown') AS Country,
  coalesce(anyIf(ResourceAttributes['service.version'], ResourceAttributes['service.version'] != ''), '') AS Version
FROM @@table_rum_spans@@
WHERE Timestamp BETWEEN @from AND @to
  AND SessionId = @sid
  AND (@svc = '' OR ServiceName = @svc)
GROUP BY SessionId
LIMIT 1`
	rows, err := c.Query(ctx, q,
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.Named("sid", sessionId),
		clickhouse.Named("svc", service),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	var s RumSessionStat
	var durMs int64
	if err = rows.Scan(&s.SessionId, &s.StartedAt, &durMs, &s.PageViews, &s.Errors, &s.Browser, &s.Os, &s.Device, &s.Country, &s.Version); err != nil {
		return nil, err
	}
	s.DurationMs = float64(durMs)
	return &s, nil
}

// GetRumWebVitalsHist reads the rum_web_vitals_hist MV.
func (c *Client) GetRumWebVitalsHist(ctx context.Context, service string, from, to timeseries.Time, f RumFilter) ([]RumVitalBucket, error) {
	pagePred := ""
	var args []any
	args = append(args,
		clickhouse.Named("service", service),
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.Seconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.Seconds),
	)
	if f.PagePath != "" {
		pagePred = " AND PagePath = @rf_page"
		args = append(args, clickhouse.Named("rf_page", f.PagePath))
	}
	q := `
SELECT EventType, Bucket, sum(Total) AS Total
FROM @@table_rum_web_vitals_hist@@
WHERE ServiceName = @service AND Timestamp >= @from AND Timestamp <= @to` + pagePred + `
GROUP BY EventType, Bucket
ORDER BY EventType, Bucket`
	rows, err := c.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []RumVitalBucket
	for rows.Next() {
		var b RumVitalBucket
		if err = rows.Scan(&b.EventType, &b.Bucket, &b.Total); err != nil {
			return nil, err
		}
		res = append(res, b)
	}
	return res, nil
}

// GetRumWebVitalRatings returns good/needs-improvement/poor counts per vital.
func (c *Client) GetRumWebVitalRatings(ctx context.Context, service string, from, to timeseries.Time, f RumFilter) ([]RumVitalRating, error) {
	rf, rArgs := f.rollupFilterSQL("")
	q := `
SELECT
  EventType,
  sum(Good) AS Good,
  sum(NeedsImprovement) AS NeedsImp,
  sum(Poor) AS Poor
FROM @@table_rum_events_1m@@
WHERE ServiceName = @service AND Timestamp >= @from AND Timestamp <= @to
  AND EventType IN ('lcp','inp','cls','ttfb')` + rf + `
GROUP BY EventType`
	args := append([]any{
		clickhouse.Named("service", service),
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.Seconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.Seconds),
	}, rArgs...)
	rows, err := c.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []RumVitalRating
	for rows.Next() {
		var r RumVitalRating
		if err = rows.Scan(&r.EventType, &r.Good, &r.NeedsImp, &r.Poor); err != nil {
			return nil, err
		}
		res = append(res, r)
	}
	return res, nil
}

// GetRumUniqueSessions counts distinct session ids in the window.
func (c *Client) GetRumUniqueSessions(ctx context.Context, service string, from, to timeseries.Time, f RumFilter) (uint64, error) {
	rf, rArgs := f.rollupFilterSQL("")
	q := `
SELECT uniqMerge(Sessions)
FROM @@table_rum_page_stats_1m@@
WHERE ServiceName = @service AND Timestamp >= @from AND Timestamp <= @to` + rf
	args := append([]any{
		clickhouse.Named("service", service),
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.Seconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.Seconds),
	}, rArgs...)
	rows, err := c.Query(ctx, q, args...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var n uint64
	if rows.Next() {
		_ = rows.Scan(&n)
	}
	return n, nil
}

// GetRumOsBreakdown is a convenience wrapper.
func (c *Client) GetRumOsBreakdown(ctx context.Context, service string, from, to timeseries.Time, qLevel float64, f RumFilter) ([]RumEventStat, error) {
	return c.GetRumBreakdownFiltered(ctx, service, from, to, "os", qLevel, f)
}
