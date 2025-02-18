package cache

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/coroot/coroot/cache/chunk"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
)

type CompactionTask struct {
	projectID db.ProjectId
	queryHash string
	dstChunk  timeseries.Time
	src       []*chunk.Meta
	compactor Compactor
}

func (ct CompactionTask) String() string {
	src := make([]string, 0, len(ct.src))
	for _, s := range ct.src {
		src = append(src, strconv.Itoa(int(s.From)))
	}
	return fmt.Sprintf(
		"compaction task %s [%s]:%d -> %d:%d",
		ct.queryHash, strings.Join(src, ","), ct.compactor.SrcChunkDuration, ct.dstChunk, ct.compactor.DstChunkDuration,
	)
}

func calcCompactionTasks(compactor Compactor, projectID db.ProjectId, queryHash string, chunks map[string]*chunk.Meta) []*CompactionTask {
	tasks := map[timeseries.Time]*CompactionTask{}
	for _, ch := range chunks {
		if timeseries.Duration(ch.PointsCount)*ch.Step != compactor.SrcChunkDuration {
			continue
		}
		if !ch.Finalized {
			continue
		}
		dstChunkTs := ch.From.Truncate(compactor.DstChunkDuration).Add(ch.Jitter())
		task := tasks[dstChunkTs]
		if task == nil {
			task = &CompactionTask{
				projectID: projectID,
				queryHash: queryHash,
				dstChunk:  dstChunkTs,
				compactor: compactor,
			}
			tasks[dstChunkTs] = task
		}
		task.src = append(task.src, ch)
	}
	res := make([]*CompactionTask, 0, len(tasks))
	for _, task := range tasks {
		if len(task.src) == int(compactor.DstChunkDuration/compactor.SrcChunkDuration) {
			res = append(res, task)
		}
	}
	return res
}

func (c *Cache) compaction() {
	cfg := DefaultCompactionConfig
	if c.cfg.Compaction != nil {
		cfg = *c.cfg.Compaction
	}

	if len(cfg.Compactors) == 0 {
		klog.Warningln("no compactors configured, deactivating compaction")
	}
	tasksCh := make(chan CompactionTask)

	for i := 0; i < cfg.WorkersNum; i++ {
		go func(ch <-chan CompactionTask) {
			klog.Infoln("compaction worker started")
			for t := range ch {
				if err := c.compact(t); err != nil {
					klog.Errorln(err)
					continue
				}
			}
		}(tasksCh)
	}

	for range time.Tick(cfg.Interval) {
		klog.Infoln("compaction iteration started")
		var tasks []*CompactionTask
		c.lock.RLock()

		for projectID, projData := range c.byProject {
			if projData == nil {
				continue
			}
			for hash, qData := range projData.queries {
				for _, cfg := range cfg.Compactors {
					tasks = append(tasks, calcCompactionTasks(cfg, projectID, hash, qData.chunksOnDisk)...)
				}
			}
		}
		c.lock.RUnlock()
		for i, t := range tasks {
			c.pendingCompactions.Set(float64(len(tasks) - i - 1))
			tasksCh <- *t
		}
	}
}

func (c *Cache) compact(t CompactionTask) error {
	if len(t.src) == 0 {
		return fmt.Errorf("no src chunks")
	}
	start := time.Now()
	metrics := map[uint64]*model.MetricValues{}
	sort.Slice(t.src, func(i, j int) bool {
		return t.src[i].From < t.src[j].From
	})
	var step timeseries.Duration
	for _, ch := range t.src {
		if ch.Step > step {
			step = ch.Step
		}
	}
	pointsCount := int(t.compactor.DstChunkDuration / step)
	for _, i := range t.src {
		if err := chunk.Read(i.Path, t.dstChunk, pointsCount, step, metrics, timeseries.FillAny); err != nil {
			return fmt.Errorf("failed to read metrics from src chunk while compaction: %s", err)
		}
	}

	dst := make([]*model.MetricValues, 0, len(metrics))
	for _, m := range metrics {
		dst = append(dst, m)
	}
	if err := c.writeChunk(t.projectID, t.queryHash, t.dstChunk, pointsCount, step, true, dst); err != nil {
		return err
	}

	c.lock.Lock()
	defer c.lock.Unlock()
	projData := c.byProject[t.projectID]
	if projData == nil {
		klog.Errorf("project data not found: %s", t.projectID)
	} else {
		qData := projData.queries[t.queryHash]
		if qData == nil {
			klog.Errorf("query data not found: %s-%s", t.projectID, t.queryHash)
		} else {
			for _, src := range t.src {
				if err := os.Remove(src.Path); err != nil {
					klog.Errorf("failed to delete chunk %s: %s", src.Path, err)
				}
				delete(qData.chunksOnDisk, src.Path)
			}
		}
	}
	c.compactedChunks.WithLabelValues(
		strconv.Itoa(int(t.compactor.SrcChunkDuration)),
		strconv.Itoa(int(t.compactor.DstChunkDuration)),
	).Inc()
	klog.Infoln(t.String(), "done in", time.Since(start))
	return nil
}
