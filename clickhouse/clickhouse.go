package clickhouse

import (
	"context"
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/coroot/coroot/ch"
	"github.com/coroot/coroot/db"
	"k8s.io/klog"
)

type ClientConfig struct {
	Protocol      string
	Address       string
	TlsEnable     bool
	TlsSkipVerify bool
	User          string
	Password      string
	Database      string
	DialContext   func(ctx context.Context, addr string) (net.Conn, error)
}

func NewClientConfig(address, user, password string) ClientConfig {
	return ClientConfig{
		Protocol: "native",
		Address:  address,
		User:     user,
		Password: password,
		Database: "default",
	}
}

type Client struct {
	config  ClientConfig
	conn    clickhouse.Conn
	chInfo  ch.ClickHouseInfo
	project *db.Project
}

func NewClient(config ClientConfig, chInfo ch.ClickHouseInfo, project *db.Project) (*Client, error) {
	opts := &clickhouse.Options{
		Addr: []string{config.Address},
		Auth: clickhouse.Auth{
			Database: config.Database,
			Username: config.User,
			Password: config.Password,
		},
		Compression: &clickhouse.Compression{Method: clickhouse.CompressionLZ4},
		DialTimeout: 10 * time.Second,
		//Debug:       true,
		//Debugf: func(format string, v ...interface{}) {
		//	klog.Infof(format, v...)
		//},
	}
	switch config.Protocol {
	case "native":
		opts.Protocol = clickhouse.Native
	case "http":
		opts.Protocol = clickhouse.HTTP
	default:
		return nil, fmt.Errorf("unknown protocol: %s", config.Protocol)
	}
	if config.DialContext != nil {
		opts.DialContext = config.DialContext
	}
	if config.TlsEnable {
		opts.TLS = &tls.Config{
			InsecureSkipVerify: config.TlsSkipVerify,
		}
	}
	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, err
	}
	return &Client{config: config, conn: conn, chInfo: chInfo, project: project}, nil
}

func (c *Client) Project() *db.Project {
	return c.project
}

func (c *Client) Ping(ctx context.Context) error {
	return c.conn.Ping(ctx)
}

func (c *Client) Query(ctx context.Context, query string, args ...interface{}) (driver.Rows, error) {
	query = ch.ReplaceTables(query, c.chInfo.UseDistributed())
	return c.conn.Query(ctx, query, args...)
}

func (c *Client) QueryRow(ctx context.Context, query string, args ...interface{}) driver.Row {
	query = ch.ReplaceTables(query, c.chInfo.UseDistributed())
	return c.conn.QueryRow(ctx, query, args...)
}

func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *Client) getClusterTopology(ctx context.Context) ([]ClusterNode, error) {
	var clusterName string
	if c.chInfo.Cloud {
		clusterName = "default"
	} else {
		clusterQuery := `
		SELECT DISTINCT replaceRegexpOne(engine_full, '^Distributed\\(''([^'']+)''.*', '\\1') as cluster_name
		FROM system.tables 
		WHERE engine = 'Distributed' 
			AND database = currentDatabase()
		LIMIT 1`
		row := c.conn.QueryRow(ctx, clusterQuery)
		if err := row.Scan(&clusterName); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return []ClusterNode{}, nil
			}
			return nil, err
		}
	}

	topologyQuery := `
		SELECT cluster, shard_num, replica_num, host_name, port 
		FROM system.clusters 
		WHERE cluster = ?
		ORDER BY shard_num, replica_num`

	rows, err := c.conn.Query(ctx, topologyQuery, clusterName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []ClusterNode
	for rows.Next() {
		var node ClusterNode
		if err := rows.Scan(&node.Cluster, &node.ShardNum, &node.ReplicaNum, &node.HostName, &node.Port); err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}

	return nodes, rows.Err()
}

func (c *Client) GetTableSizes(ctx context.Context) ([]TableInfo, error) {
	query := `
		SELECT 
			p.database,
			p.table,
			sum(p.bytes_on_disk) as bytes_on_disk,
			sum(p.data_uncompressed_bytes) as data_uncompressed_bytes,
			extract(
				t.create_table_query,
				'TTL .+\\+ (INTERVAL \\d+ [A-Z]+|toInterval\\w+\\(\\d+\\))'
			) AS ttl_expr,
			min(p.min_time) as data_since
		FROM system.parts p
		LEFT JOIN system.tables t ON p.database = t.database AND p.table = t.name
		WHERE p.active = 1 
		  	AND p.min_time > 0
			AND p.database = currentDatabase()
			AND p.engine NOT LIKE '%Distributed%'
			AND (p.table LIKE 'otel_%' OR p.table LIKE 'profiling_%' OR p.table LIKE 'metrics%')
		GROUP BY p.database, p.table, t.create_table_query
		ORDER BY p.table`

	rows, err := c.conn.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rawTables []TableInfo
	for rows.Next() {
		var table TableInfo

		var ttlExpr *string
		var dataSince *time.Time
		if err := rows.Scan(
			&table.Database,
			&table.Table,
			&table.BytesOnDisk,
			&table.DataUncompressedBytes,
			&ttlExpr,
			&dataSince,
		); err != nil {
			return nil, err
		}

		if ttlExpr != nil && *ttlExpr != "" {
			table.TTLInfo = *ttlExpr
			if seconds := parseTTLToSeconds(*ttlExpr); seconds > 0 {
				table.TTLSeconds = &seconds
			}
		}

		if dataSince != nil {
			table.DataSince = dataSince
		}

		if table.BytesOnDisk > 0 {
			table.CompressionRatio = float64(table.DataUncompressedBytes) / float64(table.BytesOnDisk)
		} else {
			table.CompressionRatio = 0
		}

		rawTables = append(rawTables, table)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rawTables, nil
}

func (c *Client) GetDiskInfo(ctx context.Context) ([]DiskInfo, error) {
	query := `
		SELECT 
			name,
			path,
			free_space,
			total_space,
			type
		FROM system.disks
		ORDER BY name`

	rows, err := c.conn.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var disks []DiskInfo
	for rows.Next() {
		var disk DiskInfo
		if err := rows.Scan(
			&disk.Name,
			&disk.Path,
			&disk.FreeSpace,
			&disk.TotalSpace,
			&disk.Type,
		); err != nil {
			return nil, err
		}
		disks = append(disks, disk)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return disks, nil
}

func (c *Client) IsCloud(ctx context.Context) (bool, error) {
	var cloudMode bool
	err := c.conn.QueryRow(ctx, "SELECT toBool(value) FROM system.settings WHERE name = 'cloud_mode';").Scan(&cloudMode)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return cloudMode, err
}

func parseTTLToSeconds(ttlExpr string) uint64 {
	ttlExpr = strings.TrimSpace(ttlExpr)

	intervalRegex := regexp.MustCompile(`INTERVAL\s+(\d+)\s+([A-Z]+)`)
	if matches := intervalRegex.FindStringSubmatch(ttlExpr); len(matches) == 3 {
		value, err := strconv.ParseUint(matches[1], 10, 64)
		if err != nil {
			return 0
		}
		unit := strings.ToUpper(matches[2])
		return convertIntervalToSeconds(value, unit)
	}

	toIntervalRegex := regexp.MustCompile(`toInterval([A-Za-z]+)\((\d+)\)`)
	if matches := toIntervalRegex.FindStringSubmatch(ttlExpr); len(matches) == 3 {
		value, err := strconv.ParseUint(matches[2], 10, 64)
		if err != nil {
			return 0
		}
		unit := strings.ToUpper(matches[1])
		return convertIntervalToSeconds(value, unit)
	}

	return 0
}

func convertIntervalToSeconds(value uint64, unit string) uint64 {
	switch unit {
	case "SECOND", "SECONDS":
		return value
	case "MINUTE", "MINUTES":
		return value * 60
	case "HOUR", "HOURS":
		return value * 3600
	case "DAY", "DAYS":
		return value * 86400
	case "WEEK", "WEEKS":
		return value * 604800
	case "MONTH", "MONTHS":
		return value * 2592000 // 30 days
	case "QUARTER", "QUARTERS":
		return value * 7776000 // 90 days
	case "YEAR", "YEARS":
		return value * 31536000 // 365 days
	default:
		return 0
	}
}

type TableInfo struct {
	Database              string     `json:"database"`
	Table                 string     `json:"table"`
	BytesOnDisk           uint64     `json:"bytes_on_disk"`
	DataUncompressedBytes uint64     `json:"data_uncompressed_bytes"`
	CompressionRatio      float64    `json:"compression_ratio"`
	TTLInfo               string     `json:"ttl_info,omitempty"`
	TTLSeconds            *uint64    `json:"ttl_seconds,omitempty"`
	DataSince             *time.Time `json:"data_since,omitempty"`
}

type DiskInfo struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	FreeSpace  uint64 `json:"free_space"`
	TotalSpace uint64 `json:"total_space"`
	Type       string `json:"type"`
}

type ServerDiskInfo struct {
	Addr  string     `json:"addr"`
	Disks []DiskInfo `json:"disks,omitempty"`
	Error string     `json:"error,omitempty"`
}

type ServerResult struct {
	Addr  string
	Data  interface{}
	Error error
}

type ClusterNode struct {
	Cluster    string `json:"cluster"`
	ShardNum   uint32 `json:"shard_num"`
	ReplicaNum uint32 `json:"replica_num"`
	HostName   string `json:"host_name"`
	Port       uint16 `json:"port"`
}

type ClusterInfo struct {
	Topology    []ClusterNode    `json:"topology,omitempty"`
	TableSizes  []TableInfo      `json:"table_sizes,omitempty"`
	ServerDisks []ServerDiskInfo `json:"server_disks,omitempty"`
}

func GetClusterInfo(ctx context.Context, cfg ClientConfig, info ch.ClickHouseInfo, project *db.Project) (*ClusterInfo, error) {
	ch, err := NewClient(cfg, info, project)
	if err != nil {
		return nil, err
	}
	ci := &ClusterInfo{}

	if ci.Topology, err = ch.getClusterTopology(ctx); err != nil {
		klog.Errorln("failed to get ClickHouse cluster topology:", err)
		return ci, nil
	}
	if ci.TableSizes, err = getClusterTableSizes(ctx, cfg, project, ci.Topology, ch, info); err != nil {
		klog.Errorln("failed to get ClickHouse table sizes:", err)
		return ci, nil
	}
	if ci.ServerDisks, err = getClusterServerDisks(ctx, cfg, project, ci.Topology, info); err != nil {
		klog.Errorln("failed to get ClickHouse server disks:", err)
		return ci, nil
	}
	return ci, nil
}

func executeOnAllServers(ctx context.Context, config ClientConfig, topology []ClusterNode, project *db.Project, operation func(*Client) (interface{}, error)) ([]ServerResult, error) {
	serverAddrs := make(map[string]bool)
	for _, node := range topology {
		serverAddrs[net.JoinHostPort(node.HostName, strconv.Itoa(int(node.Port)))] = true
	}
	if len(serverAddrs) == 0 {
		serverAddrs[config.Address] = true
	}

	type serverExecResult struct {
		addr   string
		result interface{}
		err    error
	}

	results := make([]ServerResult, 0, len(serverAddrs))
	resultsChan := make(chan serverExecResult, len(serverAddrs))

	var wg sync.WaitGroup
	for addr := range serverAddrs {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()

			clientConfig := config
			clientConfig.Address = addr

			client, err := NewClient(clientConfig, ch.ClickHouseInfo{}, project)
			if err != nil {
				resultsChan <- serverExecResult{addr: addr, err: err}
				return
			}
			defer client.Close()

			result, err := operation(client)
			resultsChan <- serverExecResult{addr: addr, result: result, err: err}
		}(addr)
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	for {
		select {
		case result, ok := <-resultsChan:
			if !ok {
				return results, nil
			}
			results = append(results, ServerResult{
				Addr:  result.addr,
				Data:  result.result,
				Error: result.err,
			})
		case <-ctx.Done():
			return results, ctx.Err()
		}
	}
}

func getClusterTableSizes(ctx context.Context, config ClientConfig, project *db.Project, topology []ClusterNode, ch *Client, info ch.ClickHouseInfo) ([]TableInfo, error) {
	if info.Cloud {
		allTables, err := ch.GetTableSizes(ctx)
		if err != nil {
			return nil, err
		}
		return aggregateTableStats(allTables), nil
	}
	results, err := executeOnAllServers(ctx, config, topology, project, func(client *Client) (interface{}, error) {
		return client.GetTableSizes(ctx)
	})
	if err != nil {
		return nil, err
	}

	var allTables []TableInfo
	for _, result := range results {
		if result.Error != nil {
			klog.Warningf("failed to get table sizes from server %s: %v", result.Addr, result.Error)
			continue
		}
		if tables, ok := result.Data.([]TableInfo); ok {
			allTables = append(allTables, tables...)
		}
	}
	return aggregateTableStats(allTables), nil
}

func getClusterServerDisks(ctx context.Context, config ClientConfig, project *db.Project, topology []ClusterNode, info ch.ClickHouseInfo) ([]ServerDiskInfo, error) {
	if info.Cloud {
		return nil, nil
	}
	results, err := executeOnAllServers(ctx, config, topology, project, func(client *Client) (interface{}, error) {
		return client.GetDiskInfo(ctx)
	})
	if err != nil {
		return nil, err
	}

	var servers []ServerDiskInfo
	for _, result := range results {
		server := ServerDiskInfo{
			Addr: result.Addr,
		}

		if result.Error != nil {
			klog.Warningf("failed to get disk info from server %s: %v", result.Addr, result.Error)
			server.Error = result.Error.Error()
		} else if disks, ok := result.Data.([]DiskInfo); ok {
			server.Disks = disks
		}

		servers = append(servers, server)
	}
	return servers, nil
}

func aggregateTableStats(tables []TableInfo) []TableInfo {
	agg := map[string]*TableInfo{}

	for _, table := range tables {
		var key string

		switch {
		case strings.HasPrefix(table.Table, "otel_logs"):
			key = "logs"
		case strings.HasPrefix(table.Table, "otel_traces"):
			key = "traces"
		case strings.HasPrefix(table.Table, "profiling_"):
			key = "profiling"
		case strings.HasPrefix(table.Table, "metrics"):
			key = "metrics"
		default:
			continue
		}

		ti := agg[key]
		if ti == nil {
			ti = &TableInfo{
				Table: key,
			}
			agg[key] = ti
		}

		ti.BytesOnDisk += table.BytesOnDisk
		ti.DataUncompressedBytes += table.DataUncompressedBytes

		if ti.TTLInfo == "" {
			ti.TTLInfo = table.TTLInfo
		}

		if ti.TTLSeconds == nil && table.TTLSeconds != nil {
			ti.TTLSeconds = table.TTLSeconds
		}

		if table.DataSince != nil {
			if ti.DataSince == nil || table.DataSince.Before(*ti.DataSince) {
				ti.DataSince = table.DataSince
			}
		}
		if ti.BytesOnDisk > 0 {
			ti.CompressionRatio = float64(ti.DataUncompressedBytes) / float64(ti.BytesOnDisk)
		}
	}
	res := make([]TableInfo, 0, len(agg))

	for _, ti := range agg {
		res = append(res, *ti)
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].Table < res[j].Table
	})
	return res
}
