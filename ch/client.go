package ch

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/ClickHouse/ch-go"
	"github.com/ClickHouse/ch-go/chpool"
	chproto "github.com/ClickHouse/ch-go/proto"
	"github.com/coroot/coroot/config"
	"github.com/coroot/coroot/db"
	"golang.org/x/exp/maps"
	"k8s.io/klog"
)

const (
	dialTimeout    = 10 * time.Second
	authHeader     = "X-API-Key"
	ProtocolCoroot = "coroot"
)

type LowLevelClient struct {
	pool    *chpool.Pool
	cluster string
	cloud   bool
}

func NewLowLevelClient(ctx context.Context, cfg *db.IntegrationClickhouse) (*LowLevelClient, error) {
	var err error
	var dialer *Dialer
	if cfg.Protocol == ProtocolCoroot {
		var err error
		origConfig := cfg
		if cfg, err = GetConfigFromRemoteCoroot(origConfig); err != nil {
			return nil, err
		}
		dialer = GetRemoteCorootDialer(origConfig)
	}

	opts := ch.Options{
		Address:          cfg.Addr,
		Database:         cfg.Database,
		User:             cfg.Auth.User,
		Password:         cfg.Auth.Password,
		Compression:      ch.CompressionLZ4,
		ReadTimeout:      30 * time.Second,
		DialTimeout:      dialTimeout,
		HandshakeTimeout: dialTimeout,
	}
	if cfg.TlsEnable {
		opts.TLS = &tls.Config{
			InsecureSkipVerify: cfg.TlsSkipVerify,
		}
	}
	if dialer != nil {
		opts.Dialer = dialer
	}
	pool, err := chpool.Dial(context.Background(), chpool.Options{ClientOptions: opts})
	if err != nil {
		return nil, err
	}
	c := &LowLevelClient{pool: pool}
	if err = c.info(ctx, opts.Address); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *LowLevelClient) Exec(ctx context.Context, query string) error {
	var result chproto.Results
	if c.cluster != "" {
		query = strings.ReplaceAll(query, "@cluster", c.cluster)
		query = strings.ReplaceAll(query, "@on_cluster", "ON CLUSTER "+c.cluster)
	} else {
		query = strings.ReplaceAll(query, "@on_cluster", "")
	}

	return c.pool.Do(ctx, ch.Query{
		Body: query,
		OnResult: func(ctx context.Context, block chproto.Block) error {
			return nil
		},
		Result: result.Auto(),
	})
}

func (c *LowLevelClient) Do(ctx context.Context, q ch.Query) (err error) {
	q.Body = ReplaceTables(q.Body, c.cluster != "")
	return c.pool.Do(ctx, q)
}

func (c *LowLevelClient) Close() {
	if c != nil && c.pool != nil {
		c.pool.Close()
	}
}

func (c *LowLevelClient) info(ctx context.Context, address string) error {
	var exists chproto.ColUInt8
	q := ch.Query{Body: "EXISTS system.zookeeper", Result: chproto.Results{{Name: "result", Data: &exists}}}
	if err := c.pool.Do(ctx, q); err != nil {
		return err
	}
	if exists.Row(0) != 1 {
		return nil
	}

	var modeStr chproto.ColStr
	q = ch.Query{
		Body:   "SELECT value FROM system.settings WHERE name = 'cloud_mode_engine'",
		Result: chproto.Results{{Name: "value", Data: &modeStr}},
	}
	if err := c.pool.Do(ctx, q); err != nil {
		return err
	}
	if modeStr.Rows() > 0 {
		mode, _ := strconv.ParseUint(modeStr.Row(0), 10, 64)
		if mode >= 2 {
			klog.Infoln(address, "is a ClickHouse cloud instance")
			c.cloud = true
			return nil
		}
	}
	var clusterCol chproto.ColStr
	clusters := map[string]bool{}
	q = ch.Query{
		Body: "SHOW CLUSTERS",
		Result: chproto.Results{
			{Name: "cluster", Data: &clusterCol},
		},
		OnResult: func(ctx context.Context, block chproto.Block) error {
			return clusterCol.ForEach(func(i int, s string) error {
				clusters[s] = true
				return nil
			})
		},
	}
	if err := c.pool.Do(ctx, q); err != nil {
		return err
	}
	switch {
	case len(clusters) == 0:
	case len(clusters) == 1:
		c.cluster = maps.Keys(clusters)[0]
	case clusters["coroot"]:
		c.cluster = "coroot"
	case clusters["default"]:
		c.cluster = "default"
	default:
		return fmt.Errorf(`multiple ClickHouse clusters found, but neither "coroot" nor "default" cluster found`)
	}
	return nil
}

func (c *LowLevelClient) CreateDB(ctx context.Context, name string) error {
	return c.Exec(ctx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s @on_cluster", name))
}

func (c *LowLevelClient) GetInfo() (ClickHouseInfo, error) {
	return ClickHouseInfo{Name: c.cluster, Cloud: c.cloud}, nil
}

type ClickHouseInfo struct {
	Name  string
	Cloud bool
}

func (ci ClickHouseInfo) UseDistributed() bool {
	return !ci.Cloud && ci.Name != ""
}

func (c *LowLevelClient) Migrate(ctx context.Context, cfg config.CollectorConfig) error {
	for _, t := range tables {
		t = strings.ReplaceAll(t, "@ttl_traces", fmt.Sprintf("%d", cfg.TracesTTL))
		t = strings.ReplaceAll(t, "@ttl_logs", fmt.Sprintf("%d", cfg.LogsTTL))
		t = strings.ReplaceAll(t, "@ttl_profiles", fmt.Sprintf("%d", cfg.ProfilesTTL))
		t = strings.ReplaceAll(t, "@ttl_metrics", fmt.Sprintf("%d", cfg.MetricsTTL))
		if c.cluster != "" {
			t = strings.ReplaceAll(t, "@merge_tree", "ReplicatedMergeTree('/clickhouse/tables/{shard}/{database}/{table}', '{replica}')")
			t = strings.ReplaceAll(t, "@replacing_merge_tree", "ReplicatedReplacingMergeTree('/clickhouse/tables/{shard}/{database}/{table}', '{replica}')")
		} else {
			t = strings.ReplaceAll(t, "@merge_tree", "MergeTree()")
			t = strings.ReplaceAll(t, "@replacing_merge_tree", "ReplacingMergeTree()")
		}
		err := c.Exec(ctx, t)
		if err != nil {
			return err
		}
	}
	if c.cluster != "" {
		for _, t := range distributedTables {
			err := c.Exec(ctx, t)
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
TTL toDateTime(Timestamp) + toIntervalSecond(@ttl_logs)
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
TTL toDateTime(LastSeen) + toIntervalSecond(@ttl_logs)
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
TTL toDateTime(Timestamp) + toIntervalSecond(@ttl_traces)
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
TTL toDateTime(Start) + toIntervalSecond(@ttl_traces)
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
TTL toDateTime(LastSeen) + toIntervalSecond(@ttl_traces)
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
TTL toDateTime(LastSeen) + toIntervalSecond(@ttl_profiles)
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
TTL toDateTime(Start) + toIntervalSecond(@ttl_profiles)
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
TTL toDateTime(LastSeen) + toIntervalSecond(@ttl_profiles)
PARTITION BY toDate(LastSeen)`,

		`
CREATE MATERIALIZED VIEW IF NOT EXISTS profiling_profiles_mv @on_cluster TO profiling_profiles AS
SELECT ServiceName, Type, max(End) AS LastSeen FROM profiling_samples group by ServiceName, Type`,

		`
CREATE TABLE IF NOT EXISTS metrics @on_cluster (
	Timestamp DateTime64(3, 'UTC') CODEC(Delta, ZSTD(1)),
	MetricHash UInt64 CODEC(ZSTD(1)),
    MetricName LowCardinality(String) CODEC(ZSTD(1)),
	Labels Map(LowCardinality(String), String) CODEC(ZSTD(1)),
	Value Float64 CODEC(ZSTD(1)),
	INDEX idx_metric_name MetricName TYPE bloom_filter(0.001) GRANULARITY 1,
	INDEX idx_labels_key mapKeys(Labels) TYPE bloom_filter(0.01) GRANULARITY 1,
	INDEX idx_labels_value mapValues(Labels) TYPE bloom_filter(0.01) GRANULARITY 1
) ENGINE @merge_tree
PARTITION BY toDate(Timestamp)
ORDER BY (MetricName, MetricHash, toUnixTimestamp(Timestamp))
TTL toDateTime(Timestamp) + toIntervalSecond(@ttl_metrics)
SETTINGS index_granularity = 8192`,

		`
CREATE TABLE IF NOT EXISTS metrics_metadata @on_cluster (
    MetricFamilyName LowCardinality(String) CODEC(ZSTD(1)),
    Type LowCardinality(String) CODEC(ZSTD(1)),
    Help String CODEC(ZSTD(1)),
    Unit LowCardinality(String) CODEC(ZSTD(1)),
) ENGINE @replacing_merge_tree
ORDER BY MetricFamilyName
SETTINGS index_granularity = 8192`,
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

		`CREATE TABLE IF NOT EXISTS metrics_distributed ON CLUSTER @cluster AS metrics
		ENGINE = Distributed(@cluster, currentDatabase(), metrics, MetricHash)`,

		`CREATE TABLE IF NOT EXISTS metrics_metadata_distributed ON CLUSTER @cluster AS metrics_metadata
		ENGINE = Distributed(@cluster, currentDatabase(), metrics_metadata, sipHash64(MetricFamilyName))`,
	}
)

func ReplaceTables(query string, distributed bool) string {
	tbls := []string{
		"otel_logs", "otel_logs_service_name_severity_text",
		"otel_traces", "otel_traces_trace_id_ts", "otel_traces_service_name",
		"profiling_stacks", "profiling_samples", "profiling_profiles",
		"metrics", "metrics_metadata",
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

func GetConfigFromRemoteCoroot(cfg *db.IntegrationClickhouse) (*db.IntegrationClickhouse, error) {
	tr := &http.Transport{}
	if cfg.TlsEnable && cfg.TlsSkipVerify {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	httpClient := &http.Client{
		Transport: tr,
		Timeout:   dialTimeout,
	}
	scheme := "http"
	if cfg.TlsEnable {
		scheme = "https"
	}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s://%s/api/clickhouse-config", scheme, cfg.Addr), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(authHeader, cfg.Auth.Password)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got \"%s\" from remote Coroot", resp.Status)
	}
	var res db.IntegrationClickhouse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode clickhouse config: %w", err)
	}
	return &res, nil
}

type Dialer struct {
	cfg *db.IntegrationClickhouse
}

func (d *Dialer) Dial(ctx context.Context, address string) (net.Conn, error) {
	return d.DialContext(ctx, "tcp", address)
}

func (d *Dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(d.cfg.Addr)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTimeout("tcp", d.cfg.Addr, dialTimeout)
	if err != nil {
		return nil, err
	}
	if d.cfg.TlsEnable {
		conn = tls.Client(conn, &tls.Config{ServerName: host, InsecureSkipVerify: d.cfg.TlsSkipVerify})
	}

	payload := fmt.Sprintf(
		"CONNECT /api/clickhouse-connect HTTP/1.1\r\nHost: %s\r\n%s: %s\r\n\r\n",
		host,
		headerKey(authHeader),
		headerValue(d.cfg.Auth.Password),
	)
	if _, err = conn.Write([]byte(payload)); err != nil {
		conn.Close()
		return nil, err
	}
	var status uint32
	if err = binary.Read(conn, binary.LittleEndian, &status); err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("failed to connect to clickhouse: %d", status)
	}
	return conn, nil
}

func GetRemoteCorootDialer(cfg *db.IntegrationClickhouse) *Dialer {
	return &Dialer{cfg: cfg}
}

var newlineReplaces = strings.NewReplacer("\n", " ", "\r", " ")

func headerKey(s string) string {
	return textproto.CanonicalMIMEHeaderKey(s)
}
func headerValue(s string) string {
	return textproto.TrimString(strings.TrimSpace(newlineReplaces.Replace(s)))
}
