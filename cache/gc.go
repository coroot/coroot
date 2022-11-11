package cache

import (
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/timeseries"
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
		for projectId, byQuery := range c.byProject {
			toDeleteInProject := map[string][]string{}
			for queryHash, qData := range byQuery {
				for path, chunk := range qData.chunksOnDisk {
					if chunk.From.Add(timeseries.Duration(chunk.PointsCount)*chunk.Step) < minTs {
						toDeleteInProject[queryHash] = append(toDeleteInProject[queryHash], path)
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
			for queryHash, chunks := range toDeleteInProject {
				qData := c.byProject[projectId][queryHash]
				for _, path := range chunks {
					klog.Infoln("deleting obsolete chunk:", path)
					if err := os.Remove(path); err != nil {
						klog.Errorf("failed to delete chunk %s: %s", path, err)
					} else {
						delete(qData.chunksOnDisk, path)
					}
				}
				if len(qData.chunksOnDisk) == 0 {
					delete(c.byProject[projectId], queryHash)
				}
			}
		}

		c.lock.Unlock()
		klog.Infof("GC done in %s", time.Since(now))
	}
}
