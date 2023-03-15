package cache

import (
	"context"
	"fmt"
	"github.com/coroot/coroot/cache/chunk"
	"github.com/coroot/coroot/constructor"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/prom"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	promModel "github.com/prometheus/common/model"
	"k8s.io/klog"
	"path"
	"sync"
	"time"
)

const (
	QueryConcurrency = 10
	BackFillInterval = 4 * timeseries.Hour
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
			if project.Prometheus.Url == "" {
				continue
			}
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
		t := timeseries.Now()
		klog.Infoln("worker iteration for", projectId)
		p, ok := projects.Load(projectId)
		if !ok {
			klog.Infoln("stopping worker for project:", projectId)
			return
		}

		project := p.(*db.Project)
		states, err := c.loadStates(projectId)
		if err != nil {
			klog.Errorln("could not get query states:", err)
			return
		}
		checkConfigs, err := c.db.GetCheckConfigs(projectId)
		if err != nil {
			klog.Errorln("could not get check configs:", err)
			return
		}

		var queries []string
		for _, q := range constructor.QUERIES {
			queries = append(queries, q)
		}
		for appId := range checkConfigs {
			availabilityCfg, _ := checkConfigs.GetAvailability(appId)
			if availabilityCfg.Custom {
				queries = append(queries, availabilityCfg.Total(), availabilityCfg.Failed())
			}
			latencyCfg, _ := checkConfigs.GetLatency(appId, model.CalcApplicationCategory(appId, project.Settings.ApplicationCategories))
			if latencyCfg.Custom {
				queries = append(queries, latencyCfg.Histogram())
			}
		}

		var recordingRules []string
		for q := range constructor.RecordingRules {
			recordingRules = append(recordingRules, q)
		}

		actualQueries := map[string]bool{}
		now := timeseries.Now()
		for _, q := range append(queries, recordingRules...) {
			actualQueries[q] = true
			state := states[q]
			if state == nil {
				state = &PrometheusQueryState{ProjectId: projectId, Query: q, LastTs: now.Add(-BackFillInterval)}
				if err := c.saveState(state); err != nil {
					klog.Errorln("failed to create query state:", err)
					return
				}
				states[q] = state
			}
		}

		for q, s := range states {
			if actualQueries[q] {
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
		now = timeseries.Now()
		for i := 0; i < QueryConcurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for state := range tasks {
					c.download(now, promClient, project, state)
				}
			}()
		}
		for _, q := range queries {
			tasks <- states[q]
		}
		close(tasks)
		wg.Wait()

		c.processRecordingRules(now, project, states)

		refreshInterval := project.Prometheus.RefreshInterval
		if refreshInterval < c.refreshIntervalMin {
			refreshInterval = c.refreshIntervalMin
		}
		time.Sleep(time.Duration(refreshInterval-timeseries.Since(t)) * time.Second)
	}
}

func (c *Cache) download(now timeseries.Time, promClient prom.Client, project *db.Project, state *PrometheusQueryState) {
	queryHash, jitter := QueryId(project.Id, state.Query)
	step := project.Prometheus.RefreshInterval
	pointsCount := int(chunkSize / step)
	for _, i := range calcIntervals(state.LastTs, step, now.Add(-step), jitter) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		vs, err := promClient.QueryRange(ctx, state.Query, i.chunkTs, i.toTs, step)
		cancel()
		if err != nil {
			klog.Errorln(err)
			if constructor.RecordingRules[state.Query] == nil {
				state.LastError = err.Error()
			}
			if err := c.saveState(state); err != nil {
				klog.Errorln("failed to save query state:", err)
			}
			return
		}
		for _, v := range vs {
			delete(v.Labels, promModel.MetricNameLabel)
		}
		c.lock.Lock()
		chunkEnd := i.chunkTs.Add(timeseries.Duration(pointsCount-1) * step)
		finalized := chunkEnd == i.toTs

		err = c.writeChunk(project.Id, queryHash, i.chunkTs, pointsCount, step, finalized, vs)
		c.lock.Unlock()
		if err != nil {
			klog.Errorln("failed to save chunk:", err)
			return
		}

		state.LastTs = i.toTs
		state.LastError = ""
		c.lock.Lock()
		err = c.saveState(state)
		c.lock.Unlock()
		if err != nil {
			klog.Errorln("failed to save state:", err)
			return
		}
	}
}

func (c *Cache) writeChunk(projectID db.ProjectId, queryHash string, from timeseries.Time, pointsCount int, step timeseries.Duration, finalized bool, metrics []model.MetricValues) error {
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
		projectID, queryHash, from, pointsCount, step))
	qData.chunksOnDisk[chunkFilePath] = &chunk.Meta{
		Path:        chunkFilePath,
		From:        from,
		PointsCount: uint32(pointsCount),
		Step:        step,
		Finalized:   finalized,
	}

	return chunk.WriteV2(chunkFilePath, from, pointsCount, step, finalized, metrics)
}

func (c *Cache) processRecordingRules(now timeseries.Time, project *db.Project, states map[string]*PrometheusQueryState) {
	var cacheTo timeseries.Time
	for query, state := range states {
		if constructor.RecordingRules[query] != nil {
			continue
		}
		if cacheTo.IsZero() || cacheTo.After(state.LastTs) {
			cacheTo = state.LastTs
		}
	}
	if cacheTo.IsZero() {
		return
	}
	cacheClient := c.GetCacheClient(project)
	promClient := &recordingRulesProcessor{db: c.db, project: project, cacheClient: cacheClient, cacheTo: cacheTo}
	for rr := range constructor.RecordingRules {
		c.download(now, promClient, project, states[rr])
	}
}

type recordingRulesProcessor struct {
	db          *db.DB
	project     *db.Project
	cacheClient *Client
	cacheTo     timeseries.Time
}

func (p *recordingRulesProcessor) QueryRange(ctx context.Context, query string, from, to timeseries.Time, step timeseries.Duration) ([]model.MetricValues, error) {
	recordingRule := constructor.RecordingRules[query]
	if recordingRule == nil {
		return nil, fmt.Errorf("unknown recording rule: %s", query)
	}
	if p.cacheTo.Before(to) {
		return nil, fmt.Errorf("cache is outdated")
	}
	c := constructor.New(p.db, p.project, p.cacheClient, constructor.OptionLoadPerConnectionHistograms, constructor.OptionDoNotLoadRawSLIs)
	world, err := c.LoadWorld(ctx, from, to, step, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to load world: %w", err)
	}
	return recordingRule(p.project, world), nil
}

func (p *recordingRulesProcessor) Ping(_ context.Context) error {
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
