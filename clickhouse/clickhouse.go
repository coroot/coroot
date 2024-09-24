package clickhouse

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
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
