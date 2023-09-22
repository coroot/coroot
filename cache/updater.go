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
	"io/ioutil"
	"k8s.io/klog"
	"math"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"
)

const (
	QueryConcurrency = 10
	BackFillInterval = 4 * timeseries.Hour

	defaultRefreshInterval = 15 * timeseries.Second
	queryTimeout           = 5 * time.Minute
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
			promClient, _ := c.promClientFactory(project)
			if promClient == nil {
				continue
			}
			ids[project.Id] = true
			_, ok := workers.Load(project.Id)
			workers.Store(project.Id, project)
			if !ok {
				go c.updaterWorker(workers, project.Id, promClient)
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

func (c *Cache) updaterWorker(projects *sync.Map, projectId db.ProjectId, promClient *prom.Client) {
	refreshInterval, err := getRefreshInterval(promClient)
	if err != nil {
		klog.Errorln(err)
	}
	c.lock.Lock()
	if projData := c.byProject[projectId]; projData == nil {
		projData = newProjectData()
		projData.refreshInterval = refreshInterval
		c.byProject[projectId] = projData
		projectDir := path.Join(c.cfg.Path, string(projectId))
		if err := utils.CreateDirectoryIfNotExists(projectDir); err != nil {
			c.lock.Unlock()
			klog.Errorln(err)
			return
		}
	}
	c.lock.Unlock()
	for {
		start := time.Now()
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

		if promClient, _ = c.promClientFactory(project); promClient != nil {
			ri, err := getRefreshInterval(promClient)
			if err != nil {
				klog.Errorln(err)
			} else if ri != refreshInterval {
				refreshInterval = ri
				c.lock.Lock()
				if c.byProject[projectId] == nil {
					c.lock.Unlock()
					klog.Warningln("unknown project:", projectId)
					return
				}
				c.byProject[projectId].refreshInterval = refreshInterval
				c.lock.Unlock()
			}
			wg := sync.WaitGroup{}
			tasks := make(chan *PrometheusQueryState)
			now = timeseries.Now()
			for i := 0; i < QueryConcurrency; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for state := range tasks {
						c.download(now, promClient, project.Id, refreshInterval, state)
					}
				}()
			}
			for _, q := range queries {
				tasks <- states[q]
			}
			close(tasks)
			wg.Wait()

			c.processRecordingRules(now, project, refreshInterval, states)
		}
		duration := time.Since(start)
		klog.Infof("worker iteration for %s completed in %s", projectId, duration)
		time.Sleep(refreshInterval.ToStandard() - duration)
	}
}

func (c *Cache) download(now timeseries.Time, promClient prom.Querier, projectId db.ProjectId, step timeseries.Duration, state *PrometheusQueryState) {
	queryHash, jitter := QueryId(projectId, state.Query)
	pointsCount := int(chunkSize / step)
	from := state.LastTs
	to := now.Add(-step)
	if to.Sub(from) > BackFillInterval {
		from = to.Add(-BackFillInterval)
	}
	for _, i := range calcIntervals(from, step, to, jitter) {
		ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
		vs, err := promClient.QueryRange(ctx, state.Query, i.chunkTs, i.toTs, step)
		cancel()
		if err != nil {
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
		chunkEnd := i.chunkTs.Add(timeseries.Duration(pointsCount-1) * step)
		finalized := chunkEnd == i.toTs
		err = c.writeChunk(projectId, queryHash, i.chunkTs, pointsCount, step, finalized, vs)
		if err != nil {
			klog.Errorln("failed to save chunk:", err)
			return
		}

		state.LastTs = i.toTs
		state.LastError = ""
		err = c.saveState(state)
		if err != nil {
			klog.Errorln("failed to save state:", err)
			return
		}
	}
}

func (c *Cache) writeChunk(projectId db.ProjectId, queryHash string, from timeseries.Time, pointsCount int, step timeseries.Duration, finalized bool, metrics []model.MetricValues) error {
	c.lock.Lock()
	projData := c.byProject[projectId]
	if projData == nil {
		return fmt.Errorf("unknown project: %s", projectId)
	}
	qData := projData.queries[queryHash]
	if qData == nil {
		qData = newQueryData()
		projData.queries[queryHash] = qData
	}
	c.lock.Unlock()

	projectDir := path.Join(c.cfg.Path, string(projectId))
	chunkFilePath := path.Join(projectDir, fmt.Sprintf(
		"%s-%s-%d-%d-%d.db",
		projectId, queryHash, from, pointsCount, step))
	dir, file := filepath.Split(chunkFilePath)
	if dir == "" {
		dir = "."
	}
	f, err := ioutil.TempFile(dir, file)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}()
	if err = chunk.Write(f, from, pointsCount, step, finalized, metrics); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}

	c.lock.Lock()
	defer c.lock.Unlock()
	if err = os.Rename(f.Name(), chunkFilePath); err != nil {
		return err
	}
	qData.chunksOnDisk[chunkFilePath] = &chunk.Meta{
		Path:        chunkFilePath,
		From:        from,
		PointsCount: uint32(pointsCount),
		Step:        step,
		Finalized:   finalized,
	}
	return nil
}

func (c *Cache) processRecordingRules(now timeseries.Time, project *db.Project, step timeseries.Duration, states map[string]*PrometheusQueryState) {
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
	cacheClient := c.GetCacheClient(project.Id)
	promClient := &recordingRulesProcessor{db: c.db, project: project, cacheClient: cacheClient, cacheTo: cacheTo}
	for rr := range constructor.RecordingRules {
		c.download(now, promClient, project.Id, step, states[rr])
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
	ctr := constructor.New(p.db, p.project, p.cacheClient, nil, constructor.OptionLoadPerConnectionHistograms, constructor.OptionDoNotLoadRawSLIs)
	world, err := ctr.LoadWorld(ctx, from, to, step, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to load world: %w", err)
	}
	return recordingRule(p.project, world), nil
}

func (p *recordingRulesProcessor) GetStep(from, to timeseries.Time) (timeseries.Duration, error) {
	return p.cacheClient.GetStep(from, to)
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

func getRefreshInterval(promClient *prom.Client) (timeseries.Duration, error) {
	step, _ := promClient.GetStep(0, 0)
	if step == 0 {
		klog.Warningln("step is zero")
		step = defaultRefreshInterval
	}
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	to := timeseries.Now()
	from := to.Add(-timeseries.Hour)
	query := fmt.Sprintf("timestamp(node_info)-%d", from)
	mvs, err := promClient.QueryRange(ctx, query, from, to, step)
	if err != nil {
		return step, err
	}
	var minDelta float32
	for _, mv := range mvs {
		mv.Values.Reduce(func(t timeseries.Time, v1 float32, v2 float32) float32 {
			delta := v2 - v1
			if delta > 0 && (delta < minDelta || minDelta == 0) {
				minDelta = delta
			}
			return v2
		})
	}
	scrapeInterval := timeseries.Duration(math.Round(float64(minDelta)/float64(step)) * float64(step))
	if scrapeInterval > step {
		return scrapeInterval, nil
	}
	return step, nil
}
