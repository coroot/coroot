package collector

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/ch-go"
	"github.com/ClickHouse/ch-go/chpool"
	chproto "github.com/ClickHouse/ch-go/proto"
	"github.com/coroot/coroot/cache"
	"github.com/coroot/coroot/db"
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

func (c *chClient) Close() {
	if c != nil && c.pool != nil {
		c.pool.Close()
	}
}

type Collector struct {
	db               *db.DB
	cache            *cache.Cache
	globalClickHouse *db.IntegrationClickhouse
	globalPrometheus *db.IntegrationPrometheus

	projects     map[db.ProjectId]*db.Project
	projectsLock sync.RWMutex

	migrationDone     map[db.ProjectId]bool
	migrationDoneLock sync.RWMutex

	clickhouseClients     map[db.ProjectId]*chClient
	clickhouseClientsLock sync.RWMutex

	traceBatches       map[db.ProjectId]*TracesBatch
	traceBatchesLock   sync.Mutex
	logBatches         map[db.ProjectId]*LogsBatch
	logBatchesLock     sync.Mutex
	profileBatches     map[db.ProjectId]*ProfilesBatch
	profileBatchesLock sync.Mutex
}

func New(database *db.DB, cache *cache.Cache, globalClickHouse *db.IntegrationClickhouse, globalPrometheus *db.IntegrationPrometheus) *Collector {
	c := &Collector{
		db:                database,
		cache:             cache,
		globalClickHouse:  globalClickHouse,
		globalPrometheus:  globalPrometheus,
		migrationDone:     map[db.ProjectId]bool{},
		clickhouseClients: map[db.ProjectId]*chClient{},
		traceBatches:      map[db.ProjectId]*TracesBatch{},
		profileBatches:    map[db.ProjectId]*ProfilesBatch{},
		logBatches:        map[db.ProjectId]*LogsBatch{},
	}

	c.updateProjects()
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			c.updateProjects()
		}
	}()

	go c.migrateProjects()

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

func (c *Collector) getProject(apiKey string) (*db.Project, error) {
	c.projectsLock.RLock()
	defer c.projectsLock.RUnlock()

	if apiKey == "" {
		if len(c.projects) == 1 {
			return maps.Values(c.projects)[0], nil
		}
		for _, p := range c.projects {
			if p.Name == "default" {
				return p, nil
			}
		}
	}

	for _, p := range c.projects {
		for _, k := range p.Settings.ApiKeys {
			if k.Key == apiKey {
				return p, nil
			}
		}
	}

	return nil, ErrProjectNotFound
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
		cl.Close()
	}
}

func (c *Collector) MigrateClickhouseDatabase(ctx context.Context, project *db.Project) error {
	if c.globalClickHouse == nil {
		return nil
	}
	c.updateProjects()
	return c.migrateProject(ctx, project)
}

func (c *Collector) UpdateClickhouseClient(ctx context.Context, project *db.Project) error {
	c.updateProjects()
	c.deleteClickhouseClient(project.Id)
	return c.migrateProject(ctx, project)
}

func (c *Collector) deleteClickhouseClient(projectId db.ProjectId) {
	c.clickhouseClientsLock.Lock()
	defer c.clickhouseClientsLock.Unlock()
	client := c.clickhouseClients[projectId]
	client.Close()
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

func (c *Collector) getClickhouseClient(project *db.Project) (*chClient, error) {
	c.clickhouseClientsLock.RLock()
	client := c.clickhouseClients[project.Id]
	c.clickhouseClientsLock.RUnlock()
	if client != nil {
		return client, nil
	}

	cfg := project.ClickHouseConfig(c.globalClickHouse)
	if cfg == nil {
		return nil, ErrClickhouseNotConfigured
	}
	var err error
	if client, err = c.clickhouseConnect(context.TODO(), cfg); err != nil {
		return nil, err
	}

	c.clickhouseClientsLock.Lock()
	c.clickhouseClients[project.Id] = client
	c.clickhouseClientsLock.Unlock()

	return client, nil
}

func (c *Collector) clickhouseDo(ctx context.Context, project *db.Project, query ch.Query) error {
	c.migrationDoneLock.RLock()
	done := c.migrationDone[project.Id]
	c.migrationDoneLock.RUnlock()
	if !done {
		return fmt.Errorf("clickhouse tables not ready for project %s", project.Id)
	}
	client, err := c.getClickhouseClient(project)
	if err != nil {
		return err
	}
	query.Body = ReplaceTables(query.Body, client.cluster != "")
	err = client.pool.Do(ctx, query)
	if err != nil {
		c.deleteClickhouseClient(project.Id)
		return err
	}
	return nil
}

func (c *Collector) getTracesBatch(project *db.Project) *TracesBatch {
	c.traceBatchesLock.Lock()
	defer c.traceBatchesLock.Unlock()
	b := c.traceBatches[project.Id]
	if b == nil {
		b = NewTracesBatch(batchLimit, batchTimeout, func(query ch.Query) error {
			return c.clickhouseDo(context.TODO(), project, query)
		})
		c.traceBatches[project.Id] = b
	}
	return b
}

func (c *Collector) getLogsBatch(project *db.Project) *LogsBatch {
	c.logBatchesLock.Lock()
	defer c.logBatchesLock.Unlock()
	b := c.logBatches[project.Id]
	if b == nil {
		b = NewLogsBatch(batchLimit, batchTimeout, func(query ch.Query) error {
			return c.clickhouseDo(context.TODO(), project, query)
		})
		c.logBatches[project.Id] = b
	}
	return b
}

func (c *Collector) getProfilesBatch(project *db.Project) *ProfilesBatch {
	c.profileBatchesLock.Lock()
	defer c.profileBatchesLock.Unlock()
	b := c.profileBatches[project.Id]
	if b == nil {
		b = NewProfilesBatch(batchLimit, batchTimeout, func(query ch.Query) error {
			return c.clickhouseDo(context.TODO(), project, query)
		})
		c.profileBatches[project.Id] = b
	}
	return b
}

func (c *Collector) IsClickhouseDistributed(project *db.Project) (bool, error) {
	client, err := c.getClickhouseClient(project)
	if err != nil {
		return false, err
	}
	return client.cluster != "", nil
}
