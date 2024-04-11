package collector

import (
	"context"
	"crypto/tls"
	"errors"
	"sync"
	"time"

	"github.com/ClickHouse/ch-go"
	"github.com/ClickHouse/ch-go/chpool"
	"github.com/coroot/coroot/db"
	"golang.org/x/exp/maps"
	"k8s.io/klog"
)

const (
	ApiKeyHeader = "X-API-Key"
)

var (
	ErrProjectNotFound         = errors.New("project not found")
	ErrClickhouseNotConfigured = errors.New("clickhouse integration is not configured")
)

type Collector struct {
	db *db.DB

	projects     map[db.ProjectId]*db.Project
	projectsLock sync.RWMutex

	clickhouseClients     map[db.ProjectId]*chpool.Pool
	clickhouseClientsLock sync.RWMutex
}

func New(database *db.DB) *Collector {
	c := &Collector{
		db:                database,
		clickhouseClients: map[db.ProjectId]*chpool.Pool{},
	}

	c.updateProjects()
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			c.updateProjects()
		}
	}()

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
	if id != "" {
		p := c.projects[id]
		if p == nil {
			return nil, ErrProjectNotFound
		}
		return p, nil
	}
	if len(c.projects) == 1 {
		return maps.Values(c.projects)[0], nil
	}
	for _, p := range c.projects {
		if p.Name == "default" {
			return p, nil
		}
	}
	return nil, ErrProjectNotFound
}

func (c *Collector) Close() {
	c.clickhouseClientsLock.Lock()
	defer c.clickhouseClientsLock.Unlock()
	for _, cl := range c.clickhouseClients {
		cl.Close()
	}
}

func (c *Collector) UpdateClickhouseClient(ctx context.Context, projectId db.ProjectId, cfg *db.IntegrationClickhouse) error {
	c.deleteClickhouseClient(projectId)
	return c.Migrate(ctx, cfg)
}

func (c *Collector) deleteClickhouseClient(projectId db.ProjectId) {
	c.clickhouseClientsLock.Lock()
	defer c.clickhouseClientsLock.Unlock()
	client := c.clickhouseClients[projectId]
	if client != nil {
		client.Close()
	}
	delete(c.clickhouseClients, projectId)
}

func (c *Collector) getClickhouseClient(projectId db.ProjectId) (*chpool.Pool, error) {
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

	cfg := project.Settings.Integrations.Clickhouse
	if cfg == nil {
		return nil, ErrClickhouseNotConfigured
	}
	opts := ch.Options{
		Address:          cfg.Addr,
		Database:         cfg.Database,
		User:             cfg.Auth.User,
		Password:         cfg.Auth.Password,
		Compression:      ch.CompressionLZ4,
		ReadTimeout:      30 * time.Second,
		DialTimeout:      10 * time.Second,
		HandshakeTimeout: 10 * time.Second,
		Settings: []ch.Setting{
			{Key: "async_insert", Value: "1"},
			{Key: "wait_for_async_insert", Value: "0"},
		},
	}
	if cfg.TlsEnable {
		opts.TLS = &tls.Config{
			InsecureSkipVerify: cfg.TlsSkipVerify,
		}
	}
	client, err = chpool.Dial(context.Background(), chpool.Options{
		ClientOptions: opts,
	})
	if err != nil {
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
	err = client.Do(ctx, query)
	if err != nil {
		c.deleteClickhouseClient(projectId)
		return err
	}
	return nil
}
