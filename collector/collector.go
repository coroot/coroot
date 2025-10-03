package collector

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	chgo "github.com/ClickHouse/ch-go"
	"github.com/coroot/coroot/cache"
	"github.com/coroot/coroot/ch"
	"github.com/coroot/coroot/config"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/grpc"
	"golang.org/x/exp/maps"
	"k8s.io/klog"
)

const (
	ApiKeyHeader = "X-API-Key"

	batchLimit   = 10000
	batchTimeout = 5 * time.Second
)

var (
	ErrProjectNotFound         = errors.New("project not found (x-api-key header is missing or invalid)")
	ErrClickhouseNotConfigured = errors.New("clickhouse integration is not configured")
)

type Collector struct {
	cfg              config.CollectorConfig
	db               *db.DB
	cache            *cache.Cache
	globalClickHouse *db.IntegrationClickhouse
	globalPrometheus *db.IntegrationPrometheus

	projects     map[db.ProjectId]*db.Project
	projectsLock sync.RWMutex

	migrationDone     map[db.ProjectId]bool
	migrationDoneLock sync.RWMutex

	clickhouseClients     map[db.ProjectId]*ch.LowLevelClient
	clickhouseClientsLock sync.RWMutex

	traceBatches       map[db.ProjectId]*TracesBatch
	traceBatchesLock   sync.Mutex
	logBatches         map[db.ProjectId]*LogsBatch
	logBatchesLock     sync.Mutex
	profileBatches     map[db.ProjectId]*ProfilesBatch
	profileBatchesLock sync.Mutex
	metricsBatches     map[db.ProjectId]*MetricsBatch
	metricsBatchesLock sync.Mutex
}

func New(cfg config.CollectorConfig, database *db.DB, cache *cache.Cache, globalClickHouse *db.IntegrationClickhouse, globalPrometheus *db.IntegrationPrometheus, grpcServer *grpc.Server) *Collector {
	c := &Collector{
		cfg:               cfg,
		db:                database,
		cache:             cache,
		globalClickHouse:  globalClickHouse,
		globalPrometheus:  globalPrometheus,
		migrationDone:     map[db.ProjectId]bool{},
		clickhouseClients: map[db.ProjectId]*ch.LowLevelClient{},
		traceBatches:      map[db.ProjectId]*TracesBatch{},
		profileBatches:    map[db.ProjectId]*ProfilesBatch{},
		logBatches:        map[db.ProjectId]*LogsBatch{},
		metricsBatches:    map[db.ProjectId]*MetricsBatch{},
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

	c.registerGRPCServices(grpcServer)

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

	isEmptyKey := apiKey == ""
	if isEmptyKey {
		apiKey = strings.Repeat("0", 32)
	}

	for _, p := range c.projects {
		for _, k := range p.Settings.ApiKeys {
			if k.Key == apiKey {
				return p, nil
			}
		}
	}
	if isEmptyKey {
		var project *db.Project
		if len(c.projects) == 1 {
			project = maps.Values(c.projects)[0]
		} else {
			for _, p := range c.projects {
				if p.Name == "default" {
					project = p
					break
				}
			}
		}
		if project != nil {
			project.Settings.ApiKeys = append(project.Settings.ApiKeys, db.ApiKey{
				Key:         apiKey,
				Description: "Default project access (no API key required)",
			})
			if err := c.db.SaveProjectSettings(project); err != nil {
				return nil, err
			}
			return project, nil
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
	c.metricsBatchesLock.Lock()
	defer c.metricsBatchesLock.Unlock()
	for _, b := range c.metricsBatches {
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

func (c *Collector) getClickhouseClient(project *db.Project) (*ch.LowLevelClient, error) {
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
	if client, err = ch.NewLowLevelClient(context.TODO(), cfg); err != nil {
		return nil, err
	}

	c.clickhouseClientsLock.Lock()
	c.clickhouseClients[project.Id] = client
	c.clickhouseClientsLock.Unlock()

	return client, nil
}

func (c *Collector) clickhouseDo(ctx context.Context, project *db.Project, query chgo.Query) error {
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
	err = client.Do(ctx, query)
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
		b = NewTracesBatch(batchLimit, batchTimeout, func(query chgo.Query) error {
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
		b = NewLogsBatch(batchLimit, batchTimeout, func(query chgo.Query) error {
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
		b = NewProfilesBatch(batchLimit, batchTimeout, func(query chgo.Query) error {
			return c.clickhouseDo(context.TODO(), project, query)
		})
		c.profileBatches[project.Id] = b
	}
	return b
}

func (c *Collector) getMetricsBatch(project *db.Project) *MetricsBatch {
	c.metricsBatchesLock.Lock()
	defer c.metricsBatchesLock.Unlock()
	b := c.metricsBatches[project.Id]
	if b == nil {
		b = NewMetricsBatch(batchLimit, batchTimeout, func(query chgo.Query) error {
			return c.clickhouseDo(context.TODO(), project, query)
		})
		c.metricsBatches[project.Id] = b
	}
	return b
}

func (c *Collector) GetClickhouseClusterInfo(project *db.Project) (ch.ClickHouseInfo, error) {
	client, err := c.getClickhouseClient(project)
	if err != nil {
		return ch.ClickHouseInfo{}, err
	}
	return client.GetInfo()
}
