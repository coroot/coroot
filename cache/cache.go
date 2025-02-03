package cache

import (
	"database/sql"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/coroot/coroot/cache/chunk"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/prom"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog"
)

type Cache struct {
	cfg       Config
	byProject map[db.ProjectId]*projectData
	lock      sync.RWMutex
	db        *db.DB
	state     *sql.DB
	stateLock sync.Mutex

	globalPrometheus *db.IntegrationPrometheus

	updates chan db.ProjectId

	pendingCompactions prometheus.Gauge
	compactedChunks    *prometheus.CounterVec
}

func NewCache(cfg Config, database *db.DB, globalPrometheus *db.IntegrationPrometheus) (*Cache, error) {
	err := utils.CreateDirectoryIfNotExists(cfg.Path)
	if err != nil {
		return nil, err
	}
	state, err := db.NewSqlite(cfg.Path)
	if err != nil {
		return nil, err
	}
	err = state.Migrator().Migrate(&PrometheusQueryState{})
	if err != nil {
		return nil, err
	}

	cache := &Cache{
		cfg:       cfg,
		byProject: map[db.ProjectId]*projectData{},
		db:        database,
		state:     state.DB(),

		globalPrometheus: globalPrometheus,

		updates: make(chan db.ProjectId),

		pendingCompactions: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "coroot_pending_compactions",
			},
		),
		compactedChunks: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "coroot_compacted_chunks_total",
			},
			[]string{"src", "dst"},
		),
	}
	if err := cache.initCacheIndexFromDir(); err != nil {
		return nil, err
	}

	prometheus.MustRegister(cache.pendingCompactions)
	prometheus.MustRegister(cache.compactedChunks)

	go cache.updater()
	go cache.gc()
	go cache.compaction()
	return cache, nil
}

func (c *Cache) Updates() <-chan db.ProjectId {
	return c.updates
}

func (c *Cache) initCacheIndexFromDir() error {
	t := time.Now()
	files, err := os.ReadDir(c.cfg.Path)
	if err != nil {
		return err
	}
	for _, f := range files {
		if !f.IsDir() {
			continue
		}
		projectId := f.Name()
		projectDir := path.Join(c.cfg.Path, projectId)
		projFiles, err := os.ReadDir(projectDir)
		if err != nil {
			return err
		}
		projData := newProjectData()
		c.byProject[db.ProjectId(projectId)] = projData

		var metaFrom timeseries.Time
		for _, chunkFile := range projFiles {
			if !strings.HasSuffix(chunkFile.Name(), ".db") {
				continue
			}
			parts := strings.Split(chunkFile.Name(), "-")
			if len(parts) != 5 {
				continue
			}
			queryId := parts[1]
			meta, err := chunk.ReadMeta(path.Join(projectDir, chunkFile.Name()))
			if err != nil {
				klog.Errorln(err)
				continue
			}
			if meta.From > metaFrom {
				projData.step = meta.Step
				metaFrom = meta.From
			}
			qData, ok := projData.queries[queryId]
			if !ok {
				qData = newQueryData()
				projData.queries[queryId] = qData
			}
			qData.chunksOnDisk[meta.Path] = meta
		}
	}
	klog.Infof("loaded from disk in %s", time.Since(t).Truncate(time.Millisecond))
	return nil
}

func (c *Cache) getPrometheusClient(p *db.Project) (*prom.Client, error) {
	cfg := p.PrometheusConfig(c.globalPrometheus)
	if cfg.Url == "" {
		return nil, fmt.Errorf("prometheus is not configured")
	}

	cc := prom.NewClientConfig(cfg.Url, cfg.RefreshInterval)
	cc.BasicAuth = cfg.BasicAuth
	cc.TlsSkipVerify = cfg.TlsSkipVerify
	cc.ExtraSelector = cfg.ExtraSelector
	cc.CustomHeaders = cfg.CustomHeaders
	client, err := prom.NewClient(cc)
	if err != nil {
		return nil, err
	}
	return client, nil
}

type projectData struct {
	step    timeseries.Duration
	queries map[string]*queryData
}

func newProjectData() *projectData {
	return &projectData{
		queries: map[string]*queryData{},
	}
}

type queryData struct {
	chunksOnDisk map[string]*chunk.Meta
}

func newQueryData() *queryData {
	return &queryData{
		chunksOnDisk: map[string]*chunk.Meta{},
	}
}
