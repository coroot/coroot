package cache

import (
	"bytes"
	"context"
	"fmt"
	"github.com/coroot/coroot/constructor"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/prom"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/natefinch/atomic"
	promModel "github.com/prometheus/common/model"
	"k8s.io/klog"
	"path"
	"sync"
	"time"
)

const (
	ChunkVersion     uint8 = 2
	QueryConcurrency       = 10
	BackFillInterval       = 2 * timeseries.Hour
)

func (c *Cache) updater() {
	workers := &sync.Map{}
	for range time.Tick(time.Second) {
		projects, err := c.db.GetProjects()
		if err != nil {
			klog.Errorln("failed to get projects:", err)
			continue
		}
		ids := map[db.ProjectId]bool{}
		for _, project := range projects {
			ids[project.Id] = true
			_, ok := workers.Load(project.Id)
			workers.Store(project.Id, project)
			if !ok {
				go c.updaterWorker(workers, project.Id)
			}
		}
		workers.Range(func(key, value interface{}) bool {
			if !ids[key.(db.ProjectId)] {
				workers.Delete(key)
			}
			return true
		})
	}
}

func (c *Cache) updaterWorker(projects *sync.Map, projectId db.ProjectId) {
	for {
		klog.Infoln("worker iteration for", projectId)
		p, ok := projects.Load(projectId)
		if !ok {
			klog.Infoln("stopping worker for project:", projectId)
			return
		}
		project := p.(*db.Project)
		now := timeseries.Now()
		func() {
			byQuery, err := c.loadStates(projectId)
			if err != nil {
				klog.Errorln("could not get query states:", err)
				return
			}
			queries := constructor.QUERIES
			actualQueries := map[string]*PrometheusQueryState{}
			for _, q := range queries {
				state := byQuery[q]
				if state == nil {
					state = &PrometheusQueryState{ProjectId: projectId, Query: q, LastTs: now.Add(-BackFillInterval)}
					if err := c.saveState(state); err != nil {
						klog.Errorln("failed to create query state:", err)
						return
					}
				}
				actualQueries[q] = state
			}

			for q, s := range byQuery {
				if _, ok := actualQueries[q]; ok {
					continue
				}
				if err := c.deleteState(s); err != nil {
					klog.Warningln("failed to delete obsolete query state:", err)
					continue
				}
			}

			promClient := c.getPromClient(project)
			wg := sync.WaitGroup{}
			tasks := make(chan *PrometheusQueryState)
			for i := 0; i < QueryConcurrency; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for state := range tasks {
						c.download(context.Background(), promClient, project, state)
					}
				}()
			}
			for _, state := range actualQueries {
				tasks <- state
			}
			close(tasks)
			wg.Wait()
		}()
		refreshInterval := project.Prometheus.RefreshInterval
		if refreshInterval < c.refreshIntervalMin {
			refreshInterval = c.refreshIntervalMin
		}
		time.Sleep(time.Duration(refreshInterval-timeseries.Since(now)) * time.Second)
	}
}

func (c *Cache) download(ctx context.Context, promClient prom.Client, project *db.Project, state *PrometheusQueryState) {
	buf := &bytes.Buffer{}
	queryHash, jitter := QueryId(project.Id, state.Query)
	refreshInterval := project.Prometheus.RefreshInterval
	now := timeseries.Now()

	for _, i := range calcIntervals(state.LastTs, refreshInterval, now.Add(-refreshInterval), jitter) {
		promCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		vs, err := promClient.QueryRange(promCtx, state.Query, i.chunkTs, i.toTs, project.Prometheus.RefreshInterval)
		cancel()
		if err != nil {
			state.LastError = err.Error()
			if err := c.saveState(state); err != nil {
				klog.Errorln("failed to save query state:", err)
			}
			return
		}

		chunk := NewChunk(i.chunkTs, i.toTs, i.chunkDuration, project.Prometheus.RefreshInterval, buf)
		for _, v := range vs {
			delete(v.Labels, promModel.MetricNameLabel)
			if err = chunk.WriteMetric(v); err != nil {
				klog.Errorln("failed to write metric to chunk:", err)
				return
			}
		}
		c.lock.Lock()
		err = c.saveChunk(project.Id, queryHash, chunk)
		c.lock.Unlock()

		if err != nil {
			klog.Errorln("failed to save chunk:", err)
			return
		}
		state.LastTs = i.toTs
		state.LastError = ""
		if err := c.saveState(state); err != nil {
			klog.Errorln("failed to save state:", err)
			return
		}
	}
}

func (c *Cache) saveChunk(projectID db.ProjectId, queryHash string, chunk *Chunk) error {
	byProject, ok := c.byProject[projectID]
	projectDir := path.Join(c.cfg.Path, string(projectID))
	if !ok {
		byProject = map[string]*queryData{}
		c.byProject[projectID] = byProject
		if err := utils.CreateDirectoryIfNotExists(projectDir); err != nil {
			return err
		}
	}
	qData, ok := byProject[queryHash]
	if !ok {
		qData = newQueryData()
		byProject[queryHash] = qData
	}
	chunkFilePath := path.Join(projectDir, fmt.Sprintf(
		"%s-%s-%d-%d-%d.db",
		projectID, queryHash, chunk.from, chunk.duration, chunk.step))
	if err := atomic.WriteFile(chunkFilePath, chunk.buf); err != nil {
		return err
	}
	qData.chunksOnDisk[chunkFilePath] = &ChunkMeta{
		path:     chunkFilePath,
		startTs:  chunk.from,
		duration: chunk.duration,
		step:     chunk.step,
		lastTs:   chunk.to,
	}
	return nil
}

type interval struct {
	chunkTs, toTs timeseries.Time
	chunkDuration timeseries.Duration
}

func (i interval) String() string {
	format := "2006-01-02T15:04:05"
	return fmt.Sprintf(
		`(%s, %d, %s)`,
		i.chunkTs.ToStandard().Format(format), i.chunkDuration, i.toTs.ToStandard().Format(format),
	)
}

func calcIntervals(lastSavedTime timeseries.Time, scrapeInterval timeseries.Duration, now timeseries.Time, jitter timeseries.Duration) []interval {
	to := now.Truncate(scrapeInterval)
	from := lastSavedTime.Add(scrapeInterval)
	if to < from {
		return nil
	}
	from = from.Truncate(scrapeInterval)
	var res []interval
	for f := from.Add(-jitter).Truncate(chunkSize).Add(jitter); f < to; f = f.Add(chunkSize) {
		i := interval{chunkTs: f, chunkDuration: chunkSize, toTs: f.Add(chunkSize)}
		if i.toTs > to {
			i.toTs = to
		}
		i.toTs = i.toTs.Add(-scrapeInterval)
		if i.chunkTs > i.toTs {
			continue
		}
		res = append(res, i)
	}
	return res
}
