package clickhouse

import (
	"context"
	"math"

	"github.com/ClickHouse/clickhouse-go/v2"
)

const (
	ttlDays = 7
)

func (c *Client) Migrate(ctx context.Context) error {
	var buckets []uint64
	for _, b := range histogramBuckets {
		if !math.IsInf(b, 0) {
			buckets = append(buckets, uint64(b*1000000))
		}
	}
	args := []any{
		clickhouse.Named("ttl_days", ttlDays),
		clickhouse.Named("histogram_buckets", buckets),
	}
	for _, t := range tables {
		err := c.conn.Exec(ctx, t, args...)
		if err != nil {
			return err
		}
	}
	return nil
}

var (
	tables = []string{
		`
CREATE TABLE IF NOT EXISTS otel_logs (
     Timestamp DateTime64(9) CODEC(Delta, ZSTD(1)),
     TraceId String CODEC(ZSTD(1)),
     SpanId String CODEC(ZSTD(1)),
     TraceFlags UInt32 CODEC(ZSTD(1)),
     SeverityText LowCardinality(String) CODEC(ZSTD(1)),
     SeverityNumber Int32 CODEC(ZSTD(1)),
     ServiceName LowCardinality(String) CODEC(ZSTD(1)),
     Body String CODEC(ZSTD(1)),
     ResourceAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
     LogAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
     INDEX idx_trace_id TraceId TYPE bloom_filter(0.001) GRANULARITY 1,
     INDEX idx_res_attr_key mapKeys(ResourceAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
     INDEX idx_res_attr_value mapValues(ResourceAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
     INDEX idx_log_attr_key mapKeys(LogAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
     INDEX idx_log_attr_value mapValues(LogAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
     INDEX idx_body Body TYPE tokenbf_v1(32768, 3, 0) GRANULARITY 1
) ENGINE MergeTree()
TTL toDateTime(Timestamp) + toIntervalDay(@ttl_days)
PARTITION BY toDate(Timestamp)
ORDER BY (ServiceName, SeverityText, toUnixTimestamp(Timestamp), TraceId)
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1
`,

		`
CREATE TABLE IF NOT EXISTS otel_traces (
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
     Duration Int64 CODEC(ZSTD(1)),
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
     INDEX idx_trace_id TraceId TYPE bloom_filter(0.001) GRANULARITY 1,
     INDEX idx_res_attr_key mapKeys(ResourceAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
     INDEX idx_res_attr_value mapValues(ResourceAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
     INDEX idx_span_attr_key mapKeys(SpanAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
     INDEX idx_span_attr_value mapValues(SpanAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
     INDEX idx_duration Duration TYPE minmax GRANULARITY 1
) ENGINE MergeTree()
TTL toDateTime(Timestamp) + toIntervalDay(@ttl_days)
PARTITION BY toDate(Timestamp)
ORDER BY (ServiceName, SpanName, toUnixTimestamp(Timestamp), TraceId)
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1`,

		`
CREATE TABLE IF NOT EXISTS otel_traces_trace_id_ts (
     TraceId String CODEC(ZSTD(1)),
     Start DateTime64(9) CODEC(Delta, ZSTD(1)),
     End DateTime64(9) CODEC(Delta, ZSTD(1)),
     INDEX idx_trace_id TraceId TYPE bloom_filter(0.01) GRANULARITY 1
) ENGINE MergeTree()
TTL toDateTime(Start) + toIntervalDay(@ttl_days)
ORDER BY (TraceId, toUnixTimestamp(Start))
SETTINGS index_granularity=8192`,

		`
CREATE MATERIALIZED VIEW IF NOT EXISTS otel_traces_trace_id_ts_mv TO otel_traces_trace_id_ts AS
SELECT 
	TraceId,
	min(Timestamp) as Start,
	max(Timestamp) as End
FROM otel_traces
WHERE TraceId!=''
GROUP BY TraceId`,

		`ALTER TABLE otel_traces ADD COLUMN IF NOT EXISTS NetSockPeerAddr LowCardinality(String) MATERIALIZED SpanAttributes['net.sock.peer.addr'] CODEC(ZSTD(1))`,
	}
)
