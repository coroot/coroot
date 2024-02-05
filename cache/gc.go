package cache

import (
	"os"
	"time"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
)

func (c *Cache) gc() {
	if c.cfg.GC == nil {
		return
	}
	for range time.Tick(c.cfg.GC.Interval) {
		now := time.Now()

		if projects, err := c.db.GetProjectNames(); err != nil {
			klog.Errorln("failed to get projects:", err)
		} else {
			c.lock.Lock()
			for projectId := range c.byProject {
				if _, ok := projects[projectId]; ok {
					continue
				}
				klog.Infoln("deleting obsolete project:", projectId)
				if err := c.deleteProject(projectId); err != nil {
					klog.Errorln("failed to delete project:", err)
					continue
				}
				delete(c.byProject, projectId)
			}
			c.lock.Unlock()
		}

		minTs := timeseries.Time(now.Add(-c.cfg.GC.TTL).Unix())
		toDelete := map[db.ProjectId]map[string][]string{}
		c.lock.RLock()
		for projectId, projData := range c.byProject {
			if projData == nil {
				continue
			}
			toDeleteInProject := map[string][]string{}
			for hash, qData := range projData.queries {
				for path, chunk := range qData.chunksOnDisk {
					if chunk.To() < minTs {
						toDeleteInProject[hash] = append(toDeleteInProject[hash], path)
					}
				}
			}
			if len(toDeleteInProject) > 0 {
				toDelete[projectId] = toDeleteInProject
			}
		}
		c.lock.RUnlock()

		c.lock.Lock()
		for projectId, toDeleteInProject := range toDelete {
			projData := c.byProject[projectId]
			if projData == nil {
				continue
			}
			for hash, chunks := range toDeleteInProject {
				qData := projData.queries[hash]
				for _, path := range chunks {
					klog.Infoln("deleting obsolete chunk:", path)
					if err := os.Remove(path); err != nil {
						klog.Errorf("failed to delete chunk %s: %s", path, err)
					} else {
						delete(qData.chunksOnDisk, path)
					}
				}
				if len(qData.chunksOnDisk) == 0 {
					delete(projData.queries, hash)
				}
			}
		}

		c.lock.Unlock()
		klog.Infof("GC done in %s", time.Since(now).Truncate(time.Millisecond))
	}
}
