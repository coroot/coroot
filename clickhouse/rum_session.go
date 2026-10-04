package clickhouse

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/coroot/coroot/timeseries"
)

type RumSessionEvent struct {
	Timestamp   time.Time
	Kind        string // span | event
	Name        string
	TraceId     string
	SpanId      string
	PagePath    string
	DurationMs  float64
	Status      string
	ServiceName string
	Version     string
}

type RumVersionStat struct {
	Version    string
	Count      uint64
	P75Ms      float64
	ErrorCount uint64
}

// GetRumSessionTimeline returns up to limit events for a session id (spans + rum_events).
func (c *Client) GetRumSessionTimeline(ctx context.Context, service, sessionId string, from, to timeseries.Time, limit int) ([]RumSessionEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	q := `
SELECT Timestamp, 'span' AS Kind, SpanName AS Name, TraceId, SpanId, PagePath,
       Duration/1e6 AS DurationMs, StatusCode, ServiceName,
       coalesce(nullIf(ResourceAttributes['service.version'], ''), '') AS Version
FROM @@table_rum_spans@@
WHERE Timestamp BETWEEN @from AND @to
  AND SessionId = @sid
  AND (@svc = '' OR ServiceName = @svc)
UNION ALL
SELECT Timestamp, 'event' AS Kind, EventType AS Name, TraceId, SpanId, PagePath,
       Value AS DurationMs, Rating AS StatusCode, ServiceName,
       coalesce(nullIf(Attributes['service.version'], ''), '') AS Version
FROM @@table_rum_events@@
WHERE Timestamp BETWEEN @from AND @to
  AND SessionId = @sid
  AND (@svc = '' OR ServiceName = @svc)
ORDER BY Timestamp
LIMIT @limit`
	rows, err := c.Query(ctx, q,
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.Named("sid", sessionId),
		clickhouse.Named("svc", service),
		clickhouse.Named("limit", limit),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []RumSessionEvent
	for rows.Next() {
		var e RumSessionEvent
		if err = rows.Scan(&e.Timestamp, &e.Kind, &e.Name, &e.TraceId, &e.SpanId, &e.PagePath, &e.DurationMs, &e.Status, &e.ServiceName, &e.Version); err != nil {
			return nil, err
		}
		res = append(res, e)
	}
	return res, nil
}

// GetRumVersions returns service.version breakdown for CWV comparison.
func (c *Client) GetRumVersions(ctx context.Context, service, eventType string, from, to timeseries.Time) ([]RumVersionStat, error) {
	return c.GetRumVersionsFiltered(ctx, service, eventType, from, to, 0.75, RumFilter{})
}

func (c *Client) GetRumVersionsFiltered(ctx context.Context, service, eventType string, from, to timeseries.Time, qLevel float64, f RumFilter) ([]RumVersionStat, error) {
	rf, rArgs := f.rollupFilterSQL("")
	q := `
SELECT
  if(Version = '', 'unknown', Version) AS Version,
  sum(Count) AS Count,
  toFloat64(if(isNaN(quantileTDigestMerge(@q)(ValueQ)), 0, quantileTDigestMerge(@q)(ValueQ))) AS Lat,
  sum(Poor) AS Errors
FROM @@table_rum_events_1m@@
WHERE Timestamp BETWEEN @from AND @to
  AND ServiceName = @svc
  AND EventType = @et` + rf + `
GROUP BY Version
ORDER BY Count DESC
LIMIT 20`
	args := append([]any{
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.Seconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.Seconds),
		clickhouse.Named("svc", service),
		clickhouse.Named("et", eventType),
		clickhouse.Named("q", qLevel),
	}, rArgs...)
	rows, err := c.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []RumVersionStat
	for rows.Next() {
		var s RumVersionStat
		if err = rows.Scan(&s.Version, &s.Count, &s.P75Ms, &s.ErrorCount); err != nil {
			return nil, err
		}
		s.P75Ms = finiteF64(s.P75Ms)
		res = append(res, s)
	}
	return res, nil
}

// GetRumGeoBreakdown returns opt-in country aggregates from rum_events / spans.
func (c *Client) GetRumGeoBreakdown(ctx context.Context, service string, from, to timeseries.Time) ([]RumEventStat, error) {
	return c.GetRumGeoBreakdownFiltered(ctx, service, from, to, 0.75, RumFilter{})
}

func (c *Client) GetRumGeoBreakdownFiltered(ctx context.Context, service string, from, to timeseries.Time, qLevel float64, f RumFilter) ([]RumEventStat, error) {
	rf, rArgs := f.rollupFilterSQL("")
	q := `
SELECT
  if(Country = '', 'unknown', Country) AS Country,
  sum(Count) AS Count,
  toFloat64(if(isNaN(quantileTDigestMerge(@q)(ValueQ)), 0, quantileTDigestMerge(@q)(ValueQ))) AS Lat
FROM @@table_rum_events_1m@@
WHERE Timestamp BETWEEN @from AND @to
  AND ServiceName = @svc
  AND EventType IN ('lcp','ttfb','inp','cls','js_error')` + rf + `
GROUP BY Country
ORDER BY Count DESC
LIMIT 50`
	args := append([]any{
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.Seconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.Seconds),
		clickhouse.Named("svc", service),
		clickhouse.Named("q", qLevel),
	}, rArgs...)
	rows, err := c.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []RumEventStat
	for rows.Next() {
		var name string
		var count uint64
		var lat float64
		if err = rows.Scan(&name, &count, &lat); err != nil {
			return nil, err
		}
		res = append(res, RumEventStat{BrowserName: name, Count: count, P75: finiteF64(lat)})
	}
	if len(res) > 0 {
		return res, nil
	}
	// Fallback: page stats rollup when no CWV events yet.
	sq := `
SELECT
  if(Country = '', 'unknown', Country) AS Country,
  sum(Views) AS Count,
  toFloat64(if(isNaN(quantileTDigestMerge(@q)(DurationQ)), 0, quantileTDigestMerge(@q)(DurationQ))) AS Lat
FROM @@table_rum_page_stats_1m@@
WHERE Timestamp BETWEEN @from AND @to
  AND ServiceName = @svc` + rf + `
GROUP BY Country
ORDER BY Count DESC
LIMIT 50`
	sargs := append([]any{
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.Seconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.Seconds),
		clickhouse.Named("svc", service),
		clickhouse.Named("q", qLevel),
	}, rArgs...)
	srows, err := c.Query(ctx, sq, sargs...)
	if err != nil {
		return nil, err
	}
	defer srows.Close()
	for srows.Next() {
		var name string
		var count uint64
		var lat float64
		if err = srows.Scan(&name, &count, &lat); err != nil {
			return nil, err
		}
		res = append(res, RumEventStat{BrowserName: name, Count: count, P75: finiteF64(lat)})
	}
	return res, nil
}
