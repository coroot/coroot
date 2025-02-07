package collector

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/ch-go"
	"github.com/ClickHouse/ch-go/chpool"
	chproto "github.com/ClickHouse/ch-go/proto"
	"github.com/coroot/coroot/db"
	"golang.org/x/exp/maps"
)

type ClickhouseClient struct {
	pool    *chpool.Pool
	cluster string
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
	cluster, err := c.getCluster(ctx)
	if err != nil {
		return nil, err
	}
	c.cluster = cluster
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

func (c *ClickhouseClient) getCluster(ctx context.Context) (string, error) {
	var exists chproto.ColUInt8
	q := ch.Query{Body: "EXISTS system.zookeeper", Result: chproto.Results{{Name: "result", Data: &exists}}}
	if err := c.pool.Do(ctx, q); err != nil {
		return "", err
	}
	if exists.Row(0) != 1 {
		return "", nil
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
		return "", err
	}
	switch {
	case len(clusters) == 0:
		return "", nil
	case len(clusters) == 1:
		return maps.Keys(clusters)[0], nil
	case clusters["coroot"]:
		return "coroot", nil
	case clusters["default"]:
		return "default", nil
	}
	return "", fmt.Errorf(`multiple ClickHouse clusters found, but neither "coroot" nor "default" cluster found`)
}
