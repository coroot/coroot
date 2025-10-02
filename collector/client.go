package collector

import (
	"context"
	"crypto/tls"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ClickHouse/ch-go"
	"github.com/ClickHouse/ch-go/chpool"
	chproto "github.com/ClickHouse/ch-go/proto"
	"github.com/coroot/coroot/db"
	"golang.org/x/exp/maps"
	"k8s.io/klog"
)

type ClickhouseClient struct {
	pool    *chpool.Pool
	cluster string
	cloud   bool
}

func NewClickhouseClient(ctx context.Context, cfg *db.IntegrationClickhouse) (*ClickhouseClient, error) {
	opts := ch.Options{
		Address:          cfg.Addr,
		Database:         cfg.Database,
		User:             cfg.Auth.User,
		Password:         cfg.Auth.Password,
		Compression:      ch.CompressionLZ4,
		ReadTimeout:      30 * time.Second,
		DialTimeout:      10 * time.Second,
		HandshakeTimeout: 10 * time.Second,
	}
	if cfg.TlsEnable {
		opts.TLS = &tls.Config{
			InsecureSkipVerify: cfg.TlsSkipVerify,
		}
	}
	pool, err := chpool.Dial(context.Background(), chpool.Options{ClientOptions: opts})
	if err != nil {
		return nil, err
	}
	c := &ClickhouseClient{pool: pool}
	if err = c.info(ctx, opts.Address); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *ClickhouseClient) Exec(ctx context.Context, query string) error {
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

func (c *ClickhouseClient) Close() {
	if c != nil && c.pool != nil {
		c.pool.Close()
	}
}

func (c *ClickhouseClient) info(ctx context.Context, address string) error {
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
