package cache

import (
	"crypto/md5"
	"database/sql"
	"fmt"
	"github.com/coroot/coroot/cache/chunk"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/prometheus/client_golang/prometheus"
	"hash/fnv"
	"io/ioutil"
	"k8s.io/klog"
	"path"
	"strings"
	"sync"
	"time"
)

const (
	chunkSize = timeseries.Hour
)

type Cache struct {
	lock      sync.RWMutex
	byProject map[db.ProjectId]map[string]*queryData
	db        *db.DB
	state     *sql.DB
	cfg       Config

	refreshIntervalMin timeseries.Duration

	pendingCompactions prometheus.Gauge
	compactedChunks    *prometheus.CounterVec
}

type queryData struct {
	chunksOnDisk map[string]*chunk.Meta
}

func newQueryData() *queryData {
	return &queryData{
		chunksOnDisk: map[string]*chunk.Meta{},
	}
}

func NewCache(cfg Config, database *db.DB) (*Cache, error) {
	if err := utils.CreateDirectoryIfNotExists(cfg.Path); err != nil {
		return nil, err
	}

	state, err := openStateDB(path.Join(cfg.Path, "db.sqlite"))
	if err != nil {
		return nil, err
	}

	cache := &Cache{
		cfg:       cfg,
		byProject: map[db.ProjectId]map[string]*queryData{},
		db:        database,
		state:     state,

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

func (c *Cache) initCacheIndexFromDir() error {
	t := time.Now()
	files, err := ioutil.ReadDir(c.cfg.Path)
	if err != nil {
		return err
	}
	for _, f := range files {
		if !f.IsDir() {
			continue
		}
		projectId := f.Name()
		projectDir := path.Join(c.cfg.Path, projectId)
		projFiles, err := ioutil.ReadDir(projectDir)
		if err != nil {
			return err
		}
		byProject := map[string]*queryData{}
		c.byProject[db.ProjectId(projectId)] = byProject

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
			byQuery, ok := byProject[queryId]
			if !ok {
				byQuery = newQueryData()
				byProject[queryId] = byQuery
			}
			byQuery.chunksOnDisk[meta.Path] = meta
		}
	}
	klog.Infof("cache loaded from disk in %s", time.Since(t))
	return nil
}

func hash(query string) string {
	return fmt.Sprintf(`%x`, md5.Sum([]byte(query)))
}

func chunkJitter(projectId db.ProjectId, queryHash string) timeseries.Duration {
	queryKey := fmt.Sprintf("%s-%s", projectId, queryHash)
	h := fnv.New32a()
	_, _ = h.Write([]byte(queryKey))
	return timeseries.Duration(h.Sum32()%uint32(chunkSize/timeseries.Minute)) * timeseries.Minute
}

func QueryId(projectId db.ProjectId, query string) (string, timeseries.Duration) {
	queryHash := hash(query)
	jitter := chunkJitter(projectId, queryHash)
	return queryHash, jitter
}
