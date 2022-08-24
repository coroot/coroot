package cache

import (
	"github.com/coroot/coroot-focus/timeseries"
	"k8s.io/klog"
	"os"
	"time"
)

func (c *Cache) gc() {
	if c.cfg.GC == nil {
		return
	}
	for range time.Tick(c.cfg.GC.Interval) {
		klog.Infoln("starting cache GC")
		now := time.Now()
		c.lock.RLock()

		minTs := timeseries.Time(now.Add(-c.cfg.GC.TTL).Unix())

		toDelete := map[string][]string{}

		for queryHash, qData := range c.data {
			for path, chunk := range qData.chunksOnDisk {
				if chunk.ts.Add(chunk.duration) < minTs {
					toDelete[queryHash] = append(toDelete[queryHash], path)
				}
			}
		}
		c.lock.RUnlock()

		c.lock.Lock()
		for queryHash, chunks := range toDelete {
			qData := c.data[queryHash]
			for _, path := range chunks {
				klog.Infoln("deleting obsolete chunk:", path)
				if err := os.Remove(path); err != nil {
					klog.Errorf("failed to delete chunk %s: %s", path, err)
				} else {
					delete(qData.chunksOnDisk, path)
				}
			}
			if len(qData.chunksOnDisk) == 0 {
				delete(c.data, queryHash)
			}
		}
		c.lock.Unlock()
		klog.Infof("GC done in %s", time.Since(now))
	}
}
