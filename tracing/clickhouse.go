package tracing

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2"
	"net"
	"time"
)

type ClickhouseClientConfig struct {
	Protocol      string
	Address       string
	TlsEnable     bool
	TlsSkipVerify bool
	User          string
	Password      string
	Database      string
	TracesTable   string
	LogsTable     string
	DialContext   func(ctx context.Context, addr string) (net.Conn, error)
}

func NewClickhouseClientConfig(address, user, password string) ClickhouseClientConfig {
	return ClickhouseClientConfig{
		Protocol:    "native",
		Address:     address,
		User:        user,
		Password:    password,
		Database:    "default",
		TracesTable: "otel_traces",
		LogsTable:   "otel_logs",
	}
}

type ClickhouseClient struct {
	config ClickhouseClientConfig
	conn   clickhouse.Conn
}

func NewClickhouseClient(config ClickhouseClientConfig) (*ClickhouseClient, error) {
	opts := &clickhouse.Options{
		Addr: []string{config.Address},
		Auth: clickhouse.Auth{
			Database: config.Database,
			Username: config.User,
			Password: config.Password,
		},
		Compression: &clickhouse.Compression{Method: clickhouse.CompressionLZ4},
		DialTimeout: 10 * time.Second,
	}
	switch config.Protocol {
	case "native":
		opts.Protocol = clickhouse.Native
	case "http":
		opts.Protocol = clickhouse.HTTP
	default:
		return nil, fmt.Errorf("unknown protocol: %s", config.Protocol)
	}
	if config.TracesTable == "" {
		return nil, fmt.Errorf("empty traces table name")
	}
	if config.LogsTable == "" {
		return nil, fmt.Errorf("empty traces table name")
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
	return &ClickhouseClient{config: config, conn: conn}, nil
}

func (c *ClickhouseClient) Ping(ctx context.Context) error {
	return c.conn.Ping(ctx)
}
