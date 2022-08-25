package cache

import (
	"bytes"
	"context"
	"fmt"
	"github.com/coroot/coroot-focus/constructor"
	"github.com/coroot/coroot-focus/db"
	"github.com/coroot/coroot-focus/timeseries"
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
	BackFillInterval       = 2 * time.Hour
)

func (c *Cache) updater() {
	for {
		klog.Infoln("refreshing cache")
		now := time.Now()
		func() {
			byQuery, err := c.db.LoadStates()
			if err != nil {
				klog.Errorln("could not get query states:", err)
				return
			}
			queries := constructor.QUERIES
			actualQueries := map[string]*db.PrometheusQueryState{}
			for _, q := range queries {
				state := byQuery[q]
				if state == nil {
					state = &db.PrometheusQueryState{Query: q, LastTs: now.Add(-BackFillInterval).Unix()}
					if err := c.db.SaveState(state); err != nil {
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
				if err := c.db.DeleteState(s); err != nil {
					klog.Warningln("failed to delete obsolete query state:", err)
					continue
				}
			}

			wg := sync.WaitGroup{}
			tasks := make(chan *db.PrometheusQueryState)
			for i := 0; i < QueryConcurrency; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for state := range tasks {
						c.download(context.Background(), state)
					}
				}()
			}
			for _, state := range actualQueries {
				tasks <- state
			}
			close(tasks)
			wg.Wait()
		}()
		duration := time.Since(now)
		klog.Infof("cache refreshed in %dms", duration.Milliseconds())
		time.Sleep(c.scrapeInterval - duration)
	}
}

func (c *Cache) download(ctx context.Context, state *db.PrometheusQueryState) {
	buf := &bytes.Buffer{}
	queryHash := hash(state.Query)
	jitter := queryJitter(queryHash)

	for _, i := range calcIntervals(time.Unix(state.LastTs, 0), c.scrapeInterval, time.Now().Add(-c.scrapeInterval), jitter) {
		promCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		vs, err := c.promClient.QueryRange(promCtx, state.Query, i.chunkTs, i.toTs, c.scrapeInterval)
		cancel()
		if err != nil {
			state.LastError = err.Error()
			if err := c.db.SaveState(state); err != nil {
				klog.Errorln("failed to save query state:", err)
			}
			return
		}

		chunk := NewChunk(
			timeseries.Time(i.chunkTs.Unix()),
			timeseries.Time(i.toTs.Unix()),
			timeseries.Duration(i.chunkDuration.Seconds()),
			timeseries.Duration(c.scrapeInterval.Seconds()),
			buf,
		)
		for _, v := range vs {
			delete(v.Labels, promModel.MetricNameLabel)
			if err = chunk.WriteMetric(v); err != nil {
				klog.Errorln("failed to write metric to chunk:", err)
				return
			}
		}
		c.lock.Lock()
		err = c.saveChunk(queryHash, chunk)
		c.lock.Unlock()

		if err != nil {
			klog.Errorln("failed to save chunk:", err)
			return
		}
		state.LastTs = i.toTs.Unix()
		state.LastError = ""
		if err := c.db.SaveState(state); err != nil {
			klog.Errorln("failed to save state:", err)
			return
		}
	}
}

func (c *Cache) saveChunk(queryHash string, chunk *Chunk) error {
	qData, ok := c.data[queryHash]
	if !ok {
		qData = newQueryData()
		c.data[queryHash] = qData
	}
	chunkFilePath := path.Join(c.cfg.Path, fmt.Sprintf(
		"%s-%d-%d-%d.db",
		queryHash, chunk.from, chunk.duration, chunk.step))
	if err := atomic.WriteFile(chunkFilePath, chunk.buf); err != nil {
		return err
	}
	chunkInfo := &ChunkInfo{
		path:     chunkFilePath,
		ts:       chunk.from,
		duration: chunk.duration,
		step:     chunk.step,
		lastTs:   chunk.to,
	}
	qData.chunksOnDisk[chunkInfo.path] = chunkInfo
	return nil
}

type interval struct {
	chunkTs, toTs time.Time
	chunkDuration time.Duration
}

func (i interval) String() string {
	format := "2006-01-02T15:04:05"
	return fmt.Sprintf(`(%s, %d, %s)`, i.chunkTs.Format(format), int(i.chunkDuration.Seconds()), i.toTs.Format(format))
}

func calcIntervals(lastSavedTime time.Time, scrapeInterval time.Duration, now time.Time, jitter time.Duration) []interval {
	to := now.Truncate(scrapeInterval)
	from := lastSavedTime.Add(scrapeInterval)
	if to.Before(from) {
		return nil
	}
	from = from.Truncate(scrapeInterval)
	var res []interval
	for f := from.Add(-jitter).Truncate(chunkSize).Add(jitter); !f.After(to); f = f.Add(chunkSize) {
		i := interval{chunkTs: f, chunkDuration: chunkSize, toTs: f.Add(chunkSize)}
		if i.toTs.After(to) {
			i.toTs = to
		}
		i.toTs = i.toTs.Add(-scrapeInterval)
		if i.chunkTs.After(i.toTs) {
			continue
		}
		res = append(res, i)
	}
	return res
}
