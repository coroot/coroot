package collector

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/coroot/coroot/db"
	"github.com/jpillora/backoff"
	"golang.org/x/exp/maps"
	"k8s.io/klog"
)

const (
	ttlDays = "7"
)

func (c *Collector) migrateProjects() {
	b := backoff.Backoff{Factor: 2, Min: time.Minute, Max: 10 * time.Minute}
	for {
		t := time.Now()
		c.projectsLock.Lock()
		projects := maps.Values(c.projects)
		c.projectsLock.Unlock()
		var failed bool
		for _, p := range projects {
			c.migrationDoneLock.Lock()
			done := c.migrationDone[p.Id]
			c.migrationDoneLock.Unlock()
			if done {
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			err := c.migrateProject(ctx, p)
			cancel()
			if err != nil {
				klog.Errorf("failed to create or update clickhouse tables for project %s: %s", p.Id, err)
				failed = true
				continue
			}
		}
		if failed {
			d := b.Duration()
			klog.Errorf("clickhouse tables migration failed, next attempt in %s", d.String())
			time.Sleep(d)
			continue
		}
		klog.Infof("clickhouse tables migration done in %s", time.Since(t).Truncate(time.Millisecond))
		return
	}
}

func (c *Collector) migrateProject(ctx context.Context, p *db.Project) error {
	cfg := p.ClickHouseConfig(c.globalClickHouse)
	if cfg == nil {
		return nil
	}
	if cfg.Global {
		initialDbCfg := *cfg
		initialDbCfg.Database = cfg.InitialDatabase
		if initialDbCfg.Database == "" {
			initialDbCfg.Database = "default"
		}
		client, err := NewClickhouseClient(ctx, &initialDbCfg)
		if err != nil {
			return err
		}
		defer client.Close()
		err = client.Exec(ctx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s @on_cluster", cfg.Database))
		if err != nil {
			return err
		}
	}
	client, err := NewClickhouseClient(ctx, cfg)
	if err != nil {
		return err
	}
	defer client.Close()
	err = c.migrate(ctx, client)
	if err == nil {
		c.migrationDoneLock.Lock()
		c.migrationDone[p.Id] = true
		c.migrationDoneLock.Unlock()
	}
	return err
}

func (c *Collector) migrate(ctx context.Context, client *ClickhouseClient) error {
	for _, t := range tables {
		t = strings.ReplaceAll(t, "@ttl_days", ttlDays)
		if client.cluster != "" {
			t = strings.ReplaceAll(t, "@merge_tree", "ReplicatedMergeTree('/clickhouse/tables/{shard}/{database}/{table}', '{replica}')")
			t = strings.ReplaceAll(t, "@replacing_merge_tree", "ReplicatedReplacingMergeTree('/clickhouse/tables/{shard}/{database}/{table}', '{replica}')")
		} else {
			t = strings.ReplaceAll(t, "@merge_tree", "MergeTree()")
			t = strings.ReplaceAll(t, "@replacing_merge_tree", "ReplacingMergeTree()")
		}
		err := client.Exec(ctx, t)
		if err != nil {
			return err
		}
	}
	if client.cluster != "" {
		for _, t := range distributedTables {
			err := client.Exec(ctx, t)
			if err != nil {
				return err
			}
		}

	}
	return nil
}

var (
	tables = []string{
		`
CREATE TABLE IF NOT EXISTS otel_logs @on_cluster (
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
) ENGINE @merge_tree
TTL toDateTime(Timestamp) + toIntervalDay(@ttl_days)
PARTITION BY toDate(Timestamp)
ORDER BY (ServiceName, SeverityText, toUnixTimestamp(Timestamp), TraceId)
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1
`,

		`
CREATE TABLE IF NOT EXISTS otel_logs_service_name_severity_text @on_cluster (
    ServiceName LowCardinality(String) CODEC(ZSTD(1)),
    SeverityText LowCardinality(String) CODEC(ZSTD(1)),
    LastSeen DateTime64(9) CODEC(Delta, ZSTD(1))
)
ENGINE @replacing_merge_tree
PRIMARY KEY (ServiceName, SeverityText)
TTL toDateTime(LastSeen) + toIntervalDay(@ttl_days)
PARTITION BY toDate(LastSeen)`,

		`
CREATE MATERIALIZED VIEW IF NOT EXISTS otel_logs_service_name_severity_text_mv @on_cluster TO otel_logs_service_name_severity_text AS
SELECT ServiceName, SeverityText, max(Timestamp) AS LastSeen FROM otel_logs group by ServiceName, SeverityText`,

		`
CREATE TABLE IF NOT EXISTS otel_traces @on_cluster (
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
) ENGINE @merge_tree
TTL toDateTime(Timestamp) + toIntervalDay(@ttl_days)
PARTITION BY toDate(Timestamp)
ORDER BY (ServiceName, SpanName, toUnixTimestamp(Timestamp), TraceId)
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1`,

		`ALTER TABLE otel_traces @on_cluster ADD COLUMN IF NOT EXISTS NetSockPeerAddr LowCardinality(String) MATERIALIZED SpanAttributes['net.sock.peer.addr'] CODEC(ZSTD(1))`,

		`
CREATE TABLE IF NOT EXISTS otel_traces_trace_id_ts @on_cluster (
     TraceId String CODEC(ZSTD(1)),
     Start DateTime64(9) CODEC(Delta, ZSTD(1)),
     End DateTime64(9) CODEC(Delta, ZSTD(1)),
     INDEX idx_trace_id TraceId TYPE bloom_filter(0.01) GRANULARITY 1
) ENGINE @merge_tree
TTL toDateTime(Start) + toIntervalDay(@ttl_days)
ORDER BY (TraceId, toUnixTimestamp(Start))
SETTINGS index_granularity=8192`,

		`
CREATE MATERIALIZED VIEW IF NOT EXISTS otel_traces_trace_id_ts_mv @on_cluster TO otel_traces_trace_id_ts AS
SELECT 
	TraceId,
	min(Timestamp) as Start,
	max(Timestamp) as End
FROM otel_traces
WHERE TraceId!=''
GROUP BY TraceId`,

		`
CREATE TABLE IF NOT EXISTS otel_traces_service_name @on_cluster (
    ServiceName LowCardinality(String) CODEC(ZSTD(1)),
    LastSeen DateTime64(9) CODEC(Delta, ZSTD(1))
)
ENGINE @replacing_merge_tree
PRIMARY KEY (ServiceName)
TTL toDateTime(LastSeen) + toIntervalDay(@ttl_days)
PARTITION BY toDate(LastSeen)`,

		`
CREATE MATERIALIZED VIEW IF NOT EXISTS otel_traces_service_name_mv @on_cluster TO otel_traces_service_name AS
SELECT ServiceName, max(Timestamp) AS LastSeen FROM otel_traces group by ServiceName`,

		`
CREATE TABLE IF NOT EXISTS profiling_stacks @on_cluster (
	ServiceName LowCardinality(String) CODEC(ZSTD(1)),
	Hash UInt64 CODEC(ZSTD(1)),
	LastSeen DateTime64(9) CODEC(Delta, ZSTD(1)),
	Stack Array(String) CODEC(ZSTD(1))
) 
ENGINE @replacing_merge_tree
PRIMARY KEY (ServiceName, Hash)
TTL toDateTime(LastSeen) + toIntervalDay(@ttl_days)
PARTITION BY toDate(LastSeen)
ORDER BY (ServiceName, Hash)`,

		`
CREATE TABLE IF NOT EXISTS profiling_samples @on_cluster (
	ServiceName LowCardinality(String) CODEC(ZSTD(1)),
    Type LowCardinality(String) CODEC(ZSTD(1)),
	Start DateTime64(9) CODEC(Delta, ZSTD(1)),
	End DateTime64(9) CODEC(Delta, ZSTD(1)),
	Labels Map(LowCardinality(String), String) CODEC(ZSTD(1)),
	StackHash UInt64 CODEC(ZSTD(1)),
	Value Int64 CODEC(ZSTD(1))
) ENGINE @merge_tree
TTL toDateTime(Start) + toIntervalDay(@ttl_days)
PARTITION BY toDate(Start)
ORDER BY (ServiceName, Type, toUnixTimestamp(Start), toUnixTimestamp(End))`,

		`
CREATE TABLE IF NOT EXISTS profiling_profiles @on_cluster (
    ServiceName LowCardinality(String) CODEC(ZSTD(1)),
    Type LowCardinality(String) CODEC(ZSTD(1)),
    LastSeen DateTime64(9) CODEC(Delta, ZSTD(1))
)
ENGINE @replacing_merge_tree
PRIMARY KEY (ServiceName, Type)
TTL toDateTime(LastSeen) + toIntervalDay(@ttl_days)
PARTITION BY toDate(LastSeen)`,

		`
CREATE MATERIALIZED VIEW IF NOT EXISTS profiling_profiles_mv @on_cluster TO profiling_profiles AS
SELECT ServiceName, Type, max(End) AS LastSeen FROM profiling_samples group by ServiceName, Type`,
	}

	distributedTables = []string{
		`CREATE TABLE IF NOT EXISTS otel_logs_distributed ON CLUSTER @cluster AS otel_logs
			ENGINE = Distributed(@cluster, currentDatabase(), otel_logs, rand())`,

		`CREATE TABLE IF NOT EXISTS otel_logs_service_name_severity_text_distributed ON CLUSTER @cluster AS otel_logs_service_name_severity_text
			ENGINE = Distributed(@cluster, currentDatabase(), otel_logs_service_name_severity_text)`,

		`CREATE TABLE IF NOT EXISTS otel_traces_distributed ON CLUSTER @cluster AS otel_traces
			ENGINE = Distributed(@cluster, currentDatabase(), otel_traces, cityHash64(TraceId))`,

		`CREATE TABLE IF NOT EXISTS otel_traces_trace_id_ts_distributed ON CLUSTER @cluster AS otel_traces_trace_id_ts
			ENGINE = Distributed(@cluster, currentDatabase(), otel_traces_trace_id_ts)`,

		`CREATE TABLE IF NOT EXISTS otel_traces_service_name_distributed ON CLUSTER @cluster AS otel_traces_service_name
			ENGINE = Distributed(@cluster, currentDatabase(), otel_traces_service_name)`,

		`CREATE TABLE IF NOT EXISTS profiling_stacks_distributed ON CLUSTER @cluster AS profiling_stacks
		ENGINE = Distributed(@cluster, currentDatabase(), profiling_stacks, Hash)`,

		`CREATE TABLE IF NOT EXISTS profiling_samples_distributed ON CLUSTER @cluster AS profiling_samples
		ENGINE = Distributed(@cluster, currentDatabase(), profiling_samples, StackHash)`,

		`CREATE TABLE IF NOT EXISTS profiling_profiles_distributed ON CLUSTER @cluster AS profiling_profiles
		ENGINE = Distributed(@cluster, currentDatabase(), profiling_profiles)`,
	}
)

func ReplaceTables(query string, distributed bool) string {
	tbls := []string{
		"otel_logs", "otel_logs_service_name_severity_text",
		"otel_traces", "otel_traces_trace_id_ts", "otel_traces_service_name",
		"profiling_stacks", "profiling_samples", "profiling_profiles",
	}
	for _, t := range tbls {
		placeholder := "@@table_" + t + "@@"
		if distributed {
			t += "_distributed"
		}
		query = strings.ReplaceAll(query, placeholder, t)
	}
	return query
}
