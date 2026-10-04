package ch

var (
	rumTables = []string{
		`
CREATE TABLE IF NOT EXISTS rum_spans @on_cluster (
     Timestamp DateTime64(9) CODEC(Delta, ZSTD(1)),
     TraceId String CODEC(ZSTD(1)),
     SpanId String CODEC(ZSTD(1)),
     ParentSpanId String CODEC(ZSTD(1)),
     TraceState String CODEC(ZSTD(1)),
     SpanName LowCardinality(String) CODEC(ZSTD(1)),
     SpanKind LowCardinality(String) CODEC(ZSTD(1)),
     ServiceName LowCardinality(String) CODEC(ZSTD(1)),
     ResourceAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
     SpanAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
     Duration Int64 CODEC(T64, ZSTD(1)),
     StatusCode LowCardinality(String) CODEC(ZSTD(1)),
     StatusMessage String CODEC(ZSTD(1)),
     Events Nested (
         Timestamp DateTime64(9),
         Name LowCardinality(String),
         Attributes Map(LowCardinality(String), String)
     ) CODEC(ZSTD(1)),
     Links Nested (
         TraceId String,
         SpanId String,
         TraceState String,
         Attributes Map(LowCardinality(String), String)
     ) CODEC(ZSTD(1)),
     PagePath LowCardinality(String) CODEC(ZSTD(1)),
     SessionId String CODEC(ZSTD(1)),
     BrowserName LowCardinality(String) MATERIALIZED coalesce(nullIf(ResourceAttributes['browser.name'], ''), '') CODEC(ZSTD(1)),
     OsName LowCardinality(String) MATERIALIZED coalesce(nullIf(ResourceAttributes['os.name'], ''), '') CODEC(ZSTD(1)),
     DeviceType LowCardinality(String) MATERIALIZED coalesce(nullIf(ResourceAttributes['device.type'], ''), '') CODEC(ZSTD(1)),
     Country LowCardinality(String) MATERIALIZED coalesce(
        nullIf(SpanAttributes['geo.country'], ''),
        nullIf(ResourceAttributes['geo.country'], ''),
        '') CODEC(ZSTD(1)),
     ServiceVersion LowCardinality(String) MATERIALIZED coalesce(nullIf(ResourceAttributes['service.version'], ''), '') CODEC(ZSTD(1)),
     INDEX idx_trace_id TraceId TYPE bloom_filter(0.001) GRANULARITY 1,
     INDEX idx_session_id SessionId TYPE bloom_filter(0.001) GRANULARITY 1,
     INDEX idx_span_attr_key mapKeys(SpanAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
     INDEX idx_duration Duration TYPE minmax GRANULARITY 1
) ENGINE @merge_tree
TTL toDateTime(Timestamp) + toIntervalSecond(@ttl_rum)
PARTITION BY toDate(Timestamp)
ORDER BY (ServiceName, SpanName, toUnixTimestamp(Timestamp), TraceId)
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1`,

		`
CREATE TABLE IF NOT EXISTS rum_events @on_cluster (
     Timestamp DateTime64(9) CODEC(Delta, ZSTD(1)),
     ServiceName LowCardinality(String) CODEC(ZSTD(1)),
     TraceId String CODEC(ZSTD(1)),
     SpanId String CODEC(ZSTD(1)),
     SessionId String CODEC(ZSTD(1)),
     PagePath LowCardinality(String) CODEC(ZSTD(1)),
     EventType LowCardinality(String) CODEC(ZSTD(1)),
     Value Float64 CODEC(Gorilla, ZSTD(1)),
     Rating LowCardinality(String) CODEC(ZSTD(1)),
     BrowserName LowCardinality(String) CODEC(ZSTD(1)),
     OsName LowCardinality(String) CODEC(ZSTD(1)),
     DeviceType LowCardinality(String) CODEC(ZSTD(1)),
     Country LowCardinality(String) MATERIALIZED coalesce(nullIf(Attributes['geo.country'], ''), '') CODEC(ZSTD(1)),
     ServiceVersion LowCardinality(String) MATERIALIZED coalesce(nullIf(Attributes['service.version'], ''), '') CODEC(ZSTD(1)),
     Attributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
     INDEX idx_trace_id TraceId TYPE bloom_filter(0.001) GRANULARITY 1,
     INDEX idx_session_id SessionId TYPE bloom_filter(0.001) GRANULARITY 1
) ENGINE @merge_tree
TTL toDateTime(Timestamp) + toIntervalSecond(@ttl_rum)
PARTITION BY toDate(Timestamp)
ORDER BY (ServiceName, EventType, PagePath, toUnixTimestamp(Timestamp), TraceId)
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1`,

		// Upgrade existing rum_spans/rum_events before any MVs that read PagePath/Country.
		// Legacy installs had MATERIALIZED PagePath/SessionId; collector now inserts them as regular columns.
		`DROP TABLE IF EXISTS rum_page_stats_1m_mv @on_cluster`,
		`DROP TABLE IF EXISTS rum_spans_histogram_mv @on_cluster`,
		`DROP TABLE IF EXISTS rum_events_1m_mv @on_cluster`,
		`ALTER TABLE rum_spans @on_cluster DROP INDEX IF EXISTS idx_session_id`,
		`ALTER TABLE rum_spans @on_cluster DROP COLUMN IF EXISTS PagePath`,
		`ALTER TABLE rum_spans @on_cluster ADD COLUMN IF NOT EXISTS PagePath LowCardinality(String) CODEC(ZSTD(1))`,
		`ALTER TABLE rum_spans @on_cluster DROP COLUMN IF EXISTS SessionId`,
		`ALTER TABLE rum_spans @on_cluster ADD COLUMN IF NOT EXISTS SessionId String CODEC(ZSTD(1))`,
		`ALTER TABLE rum_spans @on_cluster ADD COLUMN IF NOT EXISTS Country LowCardinality(String) MATERIALIZED coalesce(nullIf(SpanAttributes['geo.country'], ''), nullIf(ResourceAttributes['geo.country'], ''), '') CODEC(ZSTD(1))`,
		`ALTER TABLE rum_spans @on_cluster ADD COLUMN IF NOT EXISTS ServiceVersion LowCardinality(String) MATERIALIZED coalesce(nullIf(ResourceAttributes['service.version'], ''), '') CODEC(ZSTD(1))`,
		`ALTER TABLE rum_spans @on_cluster ADD INDEX IF NOT EXISTS idx_session_id SessionId TYPE bloom_filter(0.001) GRANULARITY 1`,
		`ALTER TABLE rum_events @on_cluster DROP INDEX IF EXISTS idx_session_id`,
		`ALTER TABLE rum_events @on_cluster DROP COLUMN IF EXISTS SessionId`,
		`ALTER TABLE rum_events @on_cluster ADD COLUMN IF NOT EXISTS SessionId String CODEC(ZSTD(1))`,
		`ALTER TABLE rum_events @on_cluster ADD COLUMN IF NOT EXISTS Country LowCardinality(String) MATERIALIZED coalesce(nullIf(Attributes['geo.country'], ''), '') CODEC(ZSTD(1))`,
		`ALTER TABLE rum_events @on_cluster ADD COLUMN IF NOT EXISTS ServiceVersion LowCardinality(String) MATERIALIZED coalesce(nullIf(Attributes['service.version'], ''), '') CODEC(ZSTD(1))`,
		`ALTER TABLE rum_events @on_cluster ADD INDEX IF NOT EXISTS idx_session_id SessionId TYPE bloom_filter(0.001) GRANULARITY 1`,

		`
CREATE TABLE IF NOT EXISTS rum_spans_histogram @on_cluster (
     ServiceName LowCardinality(String) CODEC(ZSTD(1)),
     SpanName LowCardinality(String) CODEC(ZSTD(1)),
     PagePath LowCardinality(String) CODEC(ZSTD(1)),
     Timestamp DateTime CODEC(Delta, ZSTD(1)),
     Bucket Float64 CODEC(ZSTD(1)),
     Total UInt64 CODEC(ZSTD(1)),
     Failed UInt64 CODEC(ZSTD(1))
) ENGINE @summing_merge_tree
TTL Timestamp + toIntervalSecond(@ttl_rum_agg)
PARTITION BY toDate(Timestamp)
ORDER BY (ServiceName, SpanName, PagePath, Timestamp, Bucket)
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1`,

		`
CREATE MATERIALIZED VIEW IF NOT EXISTS rum_spans_histogram_mv @on_cluster TO rum_spans_histogram AS
SELECT
    ServiceName, SpanName, PagePath,
    toDateTime(toStartOfMinute(Timestamp)) AS Timestamp,
    roundDown(Duration/1000000, [0,5,10,25,50,100,250,500,1000,2500,5000,10000]) AS Bucket,
    count(1) AS Total,
    countIf(StatusCode = 'STATUS_CODE_ERROR') AS Failed
FROM rum_spans
GROUP BY ServiceName, SpanName, PagePath, Timestamp, Bucket`,

		`
CREATE TABLE IF NOT EXISTS rum_web_vitals_hist @on_cluster (
     ServiceName LowCardinality(String) CODEC(ZSTD(1)),
     EventType LowCardinality(String) CODEC(ZSTD(1)),
     PagePath LowCardinality(String) CODEC(ZSTD(1)),
     Timestamp DateTime CODEC(Delta, ZSTD(1)),
     Bucket Float64 CODEC(ZSTD(1)),
     Total UInt64 CODEC(ZSTD(1))
) ENGINE @summing_merge_tree
TTL Timestamp + toIntervalSecond(@ttl_rum_agg)
PARTITION BY toDate(Timestamp)
ORDER BY (ServiceName, EventType, PagePath, Timestamp, Bucket)
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1`,

		`
CREATE MATERIALIZED VIEW IF NOT EXISTS rum_web_vitals_hist_mv @on_cluster TO rum_web_vitals_hist AS
SELECT
    ServiceName, EventType, PagePath,
    toDateTime(toStartOfMinute(Timestamp)) AS Timestamp,
    roundDown(Value, [0,100,250,500,1000,2500,4000,10000]) AS Bucket,
    count(1) AS Total
FROM rum_events
WHERE EventType IN ('lcp','inp','cls','ttfb','fcp')
GROUP BY ServiceName, EventType, PagePath, Timestamp, Bucket`,

		`
CREATE TABLE IF NOT EXISTS rum_service_name @on_cluster (
    ServiceName LowCardinality(String) CODEC(ZSTD(1)),
    LastSeen DateTime64(9) CODEC(Delta, ZSTD(1))
)
ENGINE @replacing_merge_tree
PRIMARY KEY (ServiceName)
TTL toDateTime(LastSeen) + toIntervalSecond(@ttl_rum_agg)
PARTITION BY toDate(LastSeen)`,

		`
CREATE MATERIALIZED VIEW IF NOT EXISTS rum_service_name_mv @on_cluster TO rum_service_name AS
SELECT ServiceName, max(Timestamp) AS LastSeen FROM rum_spans group by ServiceName`,

		`
CREATE TABLE IF NOT EXISTS rum_service_edges @on_cluster (
     ClientService LowCardinality(String) CODEC(ZSTD(1)),
     ServerService LowCardinality(String) CODEC(ZSTD(1)),
     Timestamp DateTime CODEC(Delta, ZSTD(1)),
     Requests UInt64 CODEC(ZSTD(1)),
     Failed UInt64 CODEC(ZSTD(1)),
     DurationSum Float64 CODEC(ZSTD(1))
) ENGINE @summing_merge_tree
TTL Timestamp + toIntervalSecond(@ttl_rum_agg)
PARTITION BY toDate(Timestamp)
ORDER BY (ClientService, ServerService, Timestamp)
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1`,

		`
CREATE MATERIALIZED VIEW IF NOT EXISTS rum_service_edges_mv @on_cluster TO rum_service_edges AS
SELECT
    ServiceName AS ClientService,
    coalesce(
        nullIf(SpanAttributes['peer.service'], ''),
        nullIf(SpanAttributes['server.address'], ''),
        nullIf(SpanAttributes['http.host'], ''),
        nullIf(SpanAttributes['net.peer.name'], ''),
        ''
    ) AS ServerService,
    toDateTime(toStartOfMinute(Timestamp)) AS Timestamp,
    count(1) AS Requests,
    countIf(StatusCode = 'STATUS_CODE_ERROR') AS Failed,
    sum(Duration/1000000) AS DurationSum
FROM rum_spans
WHERE SpanKind IN ('SPAN_KIND_CLIENT', 'SPAN_KIND_PRODUCER')
  AND coalesce(nullIf(SpanAttributes['peer.service'], ''), nullIf(SpanAttributes['server.address'], ''), nullIf(SpanAttributes['http.host'], ''), nullIf(SpanAttributes['net.peer.name'], ''), '') != ''
GROUP BY ClientService, ServerService, Timestamp`,

		`
CREATE TABLE IF NOT EXISTS rum_page_stats_1m @on_cluster (
     ServiceName LowCardinality(String) CODEC(ZSTD(1)),
     PagePath LowCardinality(String) CODEC(ZSTD(1)),
     BrowserName LowCardinality(String) CODEC(ZSTD(1)),
     OsName LowCardinality(String) CODEC(ZSTD(1)),
     DeviceType LowCardinality(String) CODEC(ZSTD(1)),
     Country LowCardinality(String) CODEC(ZSTD(1)),
     Version LowCardinality(String) CODEC(ZSTD(1)),
     Timestamp DateTime CODEC(Delta, ZSTD(1)),
     Views SimpleAggregateFunction(sum, UInt64),
     Errors SimpleAggregateFunction(sum, UInt64),
     DurationQ AggregateFunction(quantileTDigest, Float64),
     Sessions AggregateFunction(uniq, String)
) ENGINE @aggregating_merge_tree
TTL Timestamp + toIntervalSecond(@ttl_rum_agg)
PARTITION BY toDate(Timestamp)
ORDER BY (ServiceName, PagePath, BrowserName, OsName, DeviceType, Country, Version, Timestamp)
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1`,

		`
CREATE MATERIALIZED VIEW IF NOT EXISTS rum_page_stats_1m_mv @on_cluster TO rum_page_stats_1m AS
SELECT
    ServiceName,
    PagePath,
    BrowserName,
    OsName,
    DeviceType,
    Country,
    ServiceVersion AS Version,
    toDateTime(toStartOfMinute(Timestamp)) AS Timestamp,
    count(1) AS Views,
    countIf(StatusCode = 'STATUS_CODE_ERROR') AS Errors,
    quantileTDigestState(Duration/1000000) AS DurationQ,
    uniqState(SessionId) AS Sessions
FROM rum_spans
WHERE (
    SpanName ILIKE '%document%Load%'
    OR SpanName ILIKE '%navigation%'
    OR SpanName = 'routeChange'
    OR SpanName = 'softNavigation'
    OR SpanName ILIKE '%page_view%'
    OR SpanName ILIKE '%pageview%'
)
GROUP BY ServiceName, PagePath, BrowserName, OsName, DeviceType, Country, Version, Timestamp`,

		`
CREATE TABLE IF NOT EXISTS rum_events_1m @on_cluster (
     ServiceName LowCardinality(String) CODEC(ZSTD(1)),
     EventType LowCardinality(String) CODEC(ZSTD(1)),
     PagePath LowCardinality(String) CODEC(ZSTD(1)),
     BrowserName LowCardinality(String) CODEC(ZSTD(1)),
     OsName LowCardinality(String) CODEC(ZSTD(1)),
     DeviceType LowCardinality(String) CODEC(ZSTD(1)),
     Country LowCardinality(String) CODEC(ZSTD(1)),
     Version LowCardinality(String) CODEC(ZSTD(1)),
     Timestamp DateTime CODEC(Delta, ZSTD(1)),
     Count SimpleAggregateFunction(sum, UInt64),
     ValueQ AggregateFunction(quantileTDigest, Float64),
     Good SimpleAggregateFunction(sum, UInt64),
     NeedsImprovement SimpleAggregateFunction(sum, UInt64),
     Poor SimpleAggregateFunction(sum, UInt64)
) ENGINE @aggregating_merge_tree
TTL Timestamp + toIntervalSecond(@ttl_rum_agg)
PARTITION BY toDate(Timestamp)
ORDER BY (ServiceName, EventType, PagePath, BrowserName, OsName, DeviceType, Country, Version, Timestamp)
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1`,

		`
CREATE MATERIALIZED VIEW IF NOT EXISTS rum_events_1m_mv @on_cluster TO rum_events_1m AS
SELECT
    ServiceName,
    EventType,
    PagePath,
    BrowserName,
    OsName,
    DeviceType,
    Country,
    ServiceVersion AS Version,
    toDateTime(toStartOfMinute(Timestamp)) AS Timestamp,
    count(1) AS Count,
    quantileTDigestStateIf(Value, Value > 0) AS ValueQ,
    countIf(Rating = 'good') AS Good,
    countIf(Rating = 'needs-improvement') AS NeedsImprovement,
    countIf(Rating = 'poor') AS Poor
FROM rum_events
GROUP BY ServiceName, EventType, PagePath, BrowserName, OsName, DeviceType, Country, Version, Timestamp`,

		`
CREATE TABLE IF NOT EXISTS rum_replay_segments @on_cluster (
     Timestamp DateTime64(9) CODEC(Delta, ZSTD(1)),
     ServiceName LowCardinality(String) CODEC(ZSTD(1)),
     SessionId String CODEC(ZSTD(1)),
     TraceId String CODEC(ZSTD(1)),
     Seq UInt32 CODEC(ZSTD(1)),
     Payload String CODEC(ZSTD(3)),
     INDEX idx_session SessionId TYPE bloom_filter(0.001) GRANULARITY 1
) ENGINE @merge_tree
TTL toDateTime(Timestamp) + toIntervalSecond(@ttl_rum_replay)
PARTITION BY toDate(Timestamp)
ORDER BY (ServiceName, SessionId, Seq, toUnixTimestamp(Timestamp))
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1`,
	}

	rumDistributedTables = []string{
		`CREATE TABLE IF NOT EXISTS rum_spans_distributed ON CLUSTER @cluster AS rum_spans
			ENGINE = Distributed(@cluster, currentDatabase(), rum_spans, cityHash64(TraceId))`,

		`CREATE TABLE IF NOT EXISTS rum_events_distributed ON CLUSTER @cluster AS rum_events
			ENGINE = Distributed(@cluster, currentDatabase(), rum_events, cityHash64(TraceId))`,

		`CREATE TABLE IF NOT EXISTS rum_spans_histogram_distributed ON CLUSTER @cluster AS rum_spans_histogram
			ENGINE = Distributed(@cluster, currentDatabase(), rum_spans_histogram, rand())`,

		`CREATE TABLE IF NOT EXISTS rum_web_vitals_hist_distributed ON CLUSTER @cluster AS rum_web_vitals_hist
			ENGINE = Distributed(@cluster, currentDatabase(), rum_web_vitals_hist, rand())`,

		`CREATE TABLE IF NOT EXISTS rum_service_name_distributed ON CLUSTER @cluster AS rum_service_name
			ENGINE = Distributed(@cluster, currentDatabase(), rum_service_name)`,

		`CREATE TABLE IF NOT EXISTS rum_service_edges_distributed ON CLUSTER @cluster AS rum_service_edges
			ENGINE = Distributed(@cluster, currentDatabase(), rum_service_edges, rand())`,

		`CREATE TABLE IF NOT EXISTS rum_page_stats_1m_distributed ON CLUSTER @cluster AS rum_page_stats_1m
			ENGINE = Distributed(@cluster, currentDatabase(), rum_page_stats_1m, rand())`,

		`CREATE TABLE IF NOT EXISTS rum_events_1m_distributed ON CLUSTER @cluster AS rum_events_1m
			ENGINE = Distributed(@cluster, currentDatabase(), rum_events_1m, rand())`,

		`CREATE TABLE IF NOT EXISTS rum_replay_segments_distributed ON CLUSTER @cluster AS rum_replay_segments
			ENGINE = Distributed(@cluster, currentDatabase(), rum_replay_segments, rand())`,
	}

	rumTableNames = []string{
		"rum_spans", "rum_events", "rum_spans_histogram", "rum_web_vitals_hist", "rum_service_name", "rum_service_edges",
		"rum_page_stats_1m", "rum_events_1m", "rum_replay_segments",
	}
)
