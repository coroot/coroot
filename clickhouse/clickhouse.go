package clickhouse

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/coroot/coroot/collector"
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
	config               ClientConfig
	conn                 clickhouse.Conn
	useDistributedTables bool
}

func NewClient(config ClientConfig, distributed bool) (*Client, error) {
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
	return &Client{config: config, conn: conn, useDistributedTables: distributed}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	return c.conn.Ping(ctx)
}

func (c *Client) Query(ctx context.Context, query string, args ...interface{}) (driver.Rows, error) {
	query = collector.ReplaceTables(query, c.useDistributedTables)
	return c.conn.Query(ctx, query, args...)
}

func (c *Client) QueryRow(ctx context.Context, query string, args ...interface{}) driver.Row {
	query = collector.ReplaceTables(query, c.useDistributedTables)
	return c.conn.QueryRow(ctx, query, args...)
}

func (c *Client) Close() error {
	return c.conn.Close()
}

type ClusterNode struct {
	Cluster    string `json:"cluster"`
	ShardNum   uint32 `json:"shard_num"`
	ReplicaNum uint32 `json:"replica_num"`
	HostName   string `json:"host_name"`
	Port       uint16 `json:"port"`
}

func (c *Client) GetClusterTopology(ctx context.Context) ([]ClusterNode, error) {
	clusterQuery := `
		SELECT DISTINCT replaceRegexpOne(engine_full, '^Distributed\\(''([^'']+)''.*', '\\1') as cluster_name
		FROM system.tables 
		WHERE engine = 'Distributed' 
			AND database = currentDatabase()
		LIMIT 1`

	var clusterName string
	row := c.conn.QueryRow(ctx, clusterQuery)
	if err := row.Scan(&clusterName); err != nil {
		return []ClusterNode{}, nil
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
	Name          string `json:"name"`
	Path          string `json:"path"`
	FreeSpace     uint64 `json:"free_space"`
	TotalSpace    uint64 `json:"total_space"`
	KeepFreeSpace uint64 `json:"keep_free_space"`
	Type          string `json:"type"`
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
			keep_free_space,
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
			&disk.KeepFreeSpace,
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
