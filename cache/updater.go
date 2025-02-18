package cache

import (
	"context"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/coroot/coroot/cache/chunk"
	"github.com/coroot/coroot/constructor"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/prom"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	promModel "github.com/prometheus/common/model"
	"k8s.io/klog"
)

const (
	QueryConcurrency   = 10
	BackFillInterval   = 4 * timeseries.Hour
	MinRefreshInterval = timeseries.Minute
	queryTimeout       = 5 * time.Minute
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
			promClient, _ := c.getPrometheusClient(project)
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
	step, err := getScrapeInterval(promClient)
	if err != nil {
		klog.Errorln(err)
	}

	c.lock.Lock()
	if projData := c.byProject[projectId]; projData == nil {
		projData = newProjectData()
		projData.step = step
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

		if promClient, _ = c.getPrometheusClient(project); promClient != nil {
			si, err := getScrapeInterval(promClient)
			if err != nil {
				klog.Errorln(err)
			} else if si != step {
				step = si
				c.lock.Lock()
				if c.byProject[projectId] == nil {
					c.lock.Unlock()
					klog.Warningln("unknown project:", projectId)
					return
				}
				c.byProject[projectId].step = step
				c.lock.Unlock()
			}
			wg := sync.WaitGroup{}
			tasks := make(chan *PrometheusQueryState)
			to := now.Add(-step)
			for i := 0; i < QueryConcurrency; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for state := range tasks {
						c.download(to, promClient, project.Id, step, state)
					}
				}()
			}
			for _, q := range queries {
				tasks <- states[q]
			}
			close(tasks)
			wg.Wait()

			c.processRecordingRules(to, project, step, states)

			select {
			case c.updates <- project.Id:
			default:
			}
		}
		duration := time.Since(start)
		klog.Infof("%s: cache updated in %s", projectId, duration.Truncate(time.Millisecond))
		refreshInterval := step
		if refreshInterval < MinRefreshInterval {
			refreshInterval = MinRefreshInterval
		}
		time.Sleep(refreshInterval.ToStandard() - duration)
	}
}

func (c *Cache) download(to timeseries.Time, promClient *prom.Client, projectId db.ProjectId, step timeseries.Duration, state *PrometheusQueryState) {
	hash, jitter := QueryId(projectId, state.Query)
	pointsCount := int(chunk.Size / step)
	from := state.LastTs
	if to.Sub(from) > BackFillInterval {
		from = to.Add(-BackFillInterval)
	}
	for _, i := range calcIntervals(from, step, to, jitter) {
		ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
		vs, err := promClient.QueryRange(ctx, state.Query, i.chunkTs, i.toTs, step)
		cancel()
		if err != nil {
			state.LastError = err.Error()
			if err = c.saveState(state); err != nil {
				klog.Errorln("failed to save query state:", err)
			}
			return
		}
		for _, v := range vs {
			delete(v.Labels, promModel.MetricNameLabel)
		}
		chunkEnd := i.chunkTs.Add(timeseries.Duration(pointsCount-1) * step)
		finalized := chunkEnd == i.toTs
		err = c.writeChunk(projectId, hash, i.chunkTs, pointsCount, step, finalized, vs)
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

func (c *Cache) writeChunk(projectId db.ProjectId, queryHash string, from timeseries.Time, pointsCount int, step timeseries.Duration, finalized bool, metrics []*model.MetricValues) error {
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
	f, err := os.CreateTemp(dir, file)
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
		Created:     timeseries.Now(),
	}
	return nil
}

func (c *Cache) processRecordingRules(to timeseries.Time, project *db.Project, step timeseries.Duration, states map[string]*PrometheusQueryState) {
	var from timeseries.Time
	for query, state := range states {
		if constructor.RecordingRules[query] == nil {
			continue
		}
		if from.IsZero() || state.LastTs.Before(from) {
			from = state.LastTs
		}
	}
	if to.Sub(from) > BackFillInterval {
		from = to.Add(-BackFillInterval)
	}
	jitter := chunkJitter(project.Id, "")
	intervals := calcIntervals(from, step, to, jitter)
	if len(intervals) == 0 {
		return
	}
	cacheClient := c.GetCacheClient(project.Id)
	pointsCount := int(chunk.Size / step)
	for _, i := range intervals {
		ctr := constructor.New(c.db, project, cacheClient, nil, constructor.OptionLoadPerConnectionHistograms, constructor.OptionDoNotLoadRawSLIs, constructor.OptionLoadContainerLogs)
		world, err := ctr.LoadWorld(context.TODO(), i.chunkTs, i.toTs, step, nil)
		if err != nil {
			klog.Errorln("failed to load world:", err)
			return
		}
		chunkEnd := i.chunkTs.Add(timeseries.Duration(pointsCount-1) * step)
		finalized := chunkEnd == i.toTs
		for name, rule := range constructor.RecordingRules {
			hash := queryHash(name)
			mvs := rule(project, world)
			err = c.writeChunk(project.Id, hash, i.chunkTs, pointsCount, step, finalized, mvs)
			if err != nil {
				klog.Errorln("failed to save chunk:", err)
				return
			}
			state := states[name]
			state.LastTs = i.toTs
			state.LastError = ""
			err = c.saveState(state)
			if err != nil {
				klog.Errorln("failed to save state:", err)
				return
			}
		}
	}
}

type interval struct {
	chunkTs, toTs timeseries.Time
}

func (i interval) String() string {
	format := "2006-01-02T15:04:05"
	return fmt.Sprintf(`(%s %s)`, i.chunkTs.ToStandard().Format(format), i.toTs.ToStandard().Format(format))
}

func calcIntervals(lastSavedTime timeseries.Time, scrapeInterval timeseries.Duration, now timeseries.Time, jitter timeseries.Duration) []interval {
	to := now.Truncate(scrapeInterval)
	from := lastSavedTime.Add(scrapeInterval)
	if to <= from {
		return nil
	}
	var res []interval
	for f := from.Add(-jitter).Truncate(chunk.Size).Add(jitter).Truncate(scrapeInterval); f < to; f = f.Add(chunk.Size) {
		i := interval{chunkTs: f, toTs: f.Add(chunk.Size)}
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

func getScrapeInterval(promClient *prom.Client) (timeseries.Duration, error) {
	step, _ := promClient.GetStep(0, 0)
	if step == 0 {
		klog.Warningln("step is zero")
		step = MinRefreshInterval
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
