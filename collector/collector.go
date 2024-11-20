package collector

import (
	"context"
	"crypto/tls"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/ch-go"
	"github.com/ClickHouse/ch-go/chpool"
	chproto "github.com/ClickHouse/ch-go/proto"
	"github.com/coroot/coroot/cache"
	"github.com/coroot/coroot/db"
	"github.com/jpillora/backoff"
	"golang.org/x/exp/maps"
	"k8s.io/klog"
)

const (
	ApiKeyHeader = "X-API-Key"

	batchLimit   = 10000
	batchTimeout = 5 * time.Second
)

var (
	ErrProjectNotFound         = errors.New("project not found")
	ErrClickhouseNotConfigured = errors.New("clickhouse integration is not configured")
)

type chClient struct {
	pool    *chpool.Pool
	cluster string
}

type Collector struct {
	db               *db.DB
	cache            *cache.Cache
	globalClickHouse *db.IntegrationClickhouse
	globalPrometheus *db.IntegrationsPrometheus

	projects     map[db.ProjectId]*db.Project
	projectsLock sync.RWMutex

	clickhouseClients     map[db.ProjectId]*chClient
	clickhouseClientsLock sync.RWMutex

	traceBatches       map[db.ProjectId]*TracesBatch
	traceBatchesLock   sync.Mutex
	logBatches         map[db.ProjectId]*LogsBatch
	logBatchesLock     sync.Mutex
	profileBatches     map[db.ProjectId]*ProfilesBatch
	profileBatchesLock sync.Mutex
}

func New(database *db.DB, cache *cache.Cache, globalClickHouse *db.IntegrationClickhouse, globalPrometheus *db.IntegrationsPrometheus) *Collector {
	c := &Collector{
		db:                database,
		cache:             cache,
		globalClickHouse:  globalClickHouse,
		globalPrometheus:  globalPrometheus,
		clickhouseClients: map[db.ProjectId]*chClient{},
		traceBatches:      map[db.ProjectId]*TracesBatch{},
		profileBatches:    map[db.ProjectId]*ProfilesBatch{},
		logBatches:        map[db.ProjectId]*LogsBatch{},
	}

	c.updateProjects()
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			c.updateProjects()
		}
	}()

	for _, p := range c.projects {
		cfg := p.ClickHouseConfig(c.globalClickHouse)
		if cfg == nil {
			continue
		}
		go func(cfg *db.IntegrationClickhouse) {
			b := backoff.Backoff{Factor: 2, Min: time.Minute, Max: 10 * time.Minute}
			for {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				client, err := c.clickhouseConnect(ctx, cfg)
				if err == nil {
					err = c.migrate(ctx, client)
				}
				if client != nil {
					client.pool.Close()
				}
				cancel()
				if err == nil {
					return
				}
				d := b.Duration()
				klog.Errorf("failed to create clickhouse tables, next attempt in %s: %s", d.String(), err)
				time.Sleep(d)
			}
		}(cfg)
	}

	return c
}

func (c *Collector) updateProjects() {
	projects, err := c.db.GetProjects()
	if err != nil {
		klog.Errorln(err)
		return
	}
	c.projectsLock.Lock()
	defer c.projectsLock.Unlock()
	c.projects = map[db.ProjectId]*db.Project{}
	for _, p := range projects {
		c.projects[p.Id] = p
	}
}

func (c *Collector) getProject(id db.ProjectId) (*db.Project, error) {
	c.projectsLock.RLock()
	defer c.projectsLock.RUnlock()
	if id == "" {
		if len(c.projects) == 1 {
			return maps.Values(c.projects)[0], nil
		}
		for _, p := range c.projects {
			if p.Name == "default" {
				return p, nil
			}
		}
	}
	p := c.projects[id]
	if p == nil {
		return nil, ErrProjectNotFound
	}
	return p, nil
}

func (c *Collector) Close() {
	c.traceBatchesLock.Lock()
	defer c.traceBatchesLock.Unlock()
	for _, b := range c.traceBatches {
		b.Close()
	}
	c.logBatchesLock.Lock()
	defer c.logBatchesLock.Unlock()
	for _, b := range c.logBatches {
		b.Close()
	}
	c.profileBatchesLock.Lock()
	defer c.profileBatchesLock.Unlock()
	for _, b := range c.profileBatches {
		b.Close()
	}

	c.clickhouseClientsLock.Lock()
	defer c.clickhouseClientsLock.Unlock()
	for _, cl := range c.clickhouseClients {
		cl.pool.Close()
	}
}

func (c *Collector) UpdateClickhouseClient(ctx context.Context, projectId db.ProjectId, cfg *db.IntegrationClickhouse) error {
	c.updateProjects()
	c.deleteClickhouseClient(projectId)
	if cfg == nil {
		return nil
	}
	client, err := c.clickhouseConnect(ctx, cfg)
	if err != nil {
		return err
	}
	defer client.pool.Close()
	return c.migrate(ctx, client)
}

func (c *Collector) deleteClickhouseClient(projectId db.ProjectId) {
	c.clickhouseClientsLock.Lock()
	defer c.clickhouseClientsLock.Unlock()
	client := c.clickhouseClients[projectId]
	if client != nil {
		client.pool.Close()
	}
	delete(c.clickhouseClients, projectId)
}

func (c *Collector) clickhouseConnect(ctx context.Context, cfg *db.IntegrationClickhouse) (*chClient, error) {
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
	cluster, err := getCluster(ctx, pool)
	if err != nil {
		if cfg.Global && strings.Contains(err.Error(), "UNKNOWN_DATABASE") {
			pool.Close()
			opts.Database = cfg.InitialDatabase
			pool, err = chpool.Dial(context.Background(), chpool.Options{ClientOptions: opts})
			if err != nil {
				return nil, err
			}
			cluster, err = getCluster(ctx, pool)
			if err != nil {
				return nil, err
			}
			q := "CREATE DATABASE " + cfg.Database
			if cluster != "" {
				q += " ON CLUSTER " + cluster
			}
			var result chproto.Results
			err = pool.Do(ctx, ch.Query{
				Body: q,
				OnResult: func(ctx context.Context, block chproto.Block) error {
					return nil
				},
				Result: result.Auto(),
			})
			pool.Close()
			if err != nil {
				return nil, err
			}
			return c.clickhouseConnect(ctx, cfg)
		}
		return nil, err
	}
	return &chClient{pool: pool, cluster: cluster}, err
}

func (c *Collector) getClickhouseClient(projectId db.ProjectId) (*chClient, error) {
	c.clickhouseClientsLock.RLock()
	client := c.clickhouseClients[projectId]
	c.clickhouseClientsLock.RUnlock()

	if client != nil {
		return client, nil
	}
	project, err := c.getProject(projectId)
	if err != nil {
		return nil, err
	}

	cfg := project.ClickHouseConfig(c.globalClickHouse)
	if cfg == nil {
		return nil, ErrClickhouseNotConfigured
	}
	if client, err = c.clickhouseConnect(context.TODO(), cfg); err != nil {
		return nil, err
	}

	c.clickhouseClientsLock.Lock()
	c.clickhouseClients[projectId] = client
	c.clickhouseClientsLock.Unlock()

	return client, nil
}

func (c *Collector) clickhouseDo(ctx context.Context, projectId db.ProjectId, query ch.Query) error {
	client, err := c.getClickhouseClient(projectId)
	if err != nil {
		return err
	}
	query.Body = ReplaceTables(query.Body, client.cluster != "")
	err = client.pool.Do(ctx, query)
	if err != nil {
		c.deleteClickhouseClient(projectId)
		return err
	}
	return nil
}

func (c *Collector) getTracesBatch(projectId db.ProjectId) *TracesBatch {
	c.traceBatchesLock.Lock()
	defer c.traceBatchesLock.Unlock()
	b := c.traceBatches[projectId]
	if b == nil {
		b = NewTracesBatch(batchLimit, batchTimeout, func(query ch.Query) error {
			return c.clickhouseDo(context.TODO(), projectId, query)
		})
		c.traceBatches[projectId] = b
	}
	return b
}

func (c *Collector) getLogsBatch(projectId db.ProjectId) *LogsBatch {
	c.logBatchesLock.Lock()
	defer c.logBatchesLock.Unlock()
	b := c.logBatches[projectId]
	if b == nil {
		b = NewLogsBatch(batchLimit, batchTimeout, func(query ch.Query) error {
			return c.clickhouseDo(context.TODO(), projectId, query)
		})
		c.logBatches[projectId] = b
	}
	return b
}

func (c *Collector) getProfilesBatch(projectId db.ProjectId) *ProfilesBatch {
	c.profileBatchesLock.Lock()
	defer c.profileBatchesLock.Unlock()
	b := c.profileBatches[projectId]
	if b == nil {
		b = NewProfilesBatch(batchLimit, batchTimeout, func(query ch.Query) error {
			return c.clickhouseDo(context.TODO(), projectId, query)
		})
		c.profileBatches[projectId] = b
	}
	return b
}

func (c *Collector) IsClickhouseDistributed(projectId db.ProjectId) (bool, error) {
	client, err := c.getClickhouseClient(projectId)
	if err != nil {
		return false, err
	}
	return client.cluster != "", nil
}
