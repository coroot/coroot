package watchers

import (
	"context"
	"sync"
	"time"

	"github.com/coroot/coroot/cache"
	pricing "github.com/coroot/coroot/cloud-pricing"
	"github.com/coroot/coroot/constructor"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
)

func Start(database *db.DB, cache *cache.Cache, pricing *pricing.Manager, incidents *Incidents, checkDeployments bool) {
	var deployments *Deployments
	if checkDeployments {
		deployments = NewDeployments(database, pricing)
	}

	if incidents == nil && deployments == nil {
		return
	}

	projectChan := make(chan db.ProjectId, 1000)

	pending := map[db.ProjectId]bool{}
	pendingLock := sync.Mutex{}

	// Fast consumer goroutine - just receives and deduplicates
	go func() {
		for projectId := range cache.Updates() {
			pendingLock.Lock()
			if !pending[projectId] {
				pending[projectId] = true
				select {
				case projectChan <- projectId:
				default:
					// Channel full, skip this update
					pending[projectId] = false
				}
			}
			pendingLock.Unlock()
		}
		close(projectChan)
	}()

	go func() {
		for projectId := range projectChan {
			// Remove from pending set
			pendingLock.Lock()
			delete(pending, projectId)
			pendingLock.Unlock()

			if !database.GetPrimaryLock(context.TODO()) {
				klog.Infoln("not the primary replica: skipping")
				continue
			}

			start := time.Now()
			project, err := database.GetProject(projectId)
			if err != nil {
				klog.Errorln(err)
				continue
			}
			cacheClient := cache.GetCacheClient(project.Id)
			cacheTo, err := cacheClient.GetTo()
			if err != nil {
				klog.Errorln(err)
				continue
			}
			if cacheTo.IsZero() {
				continue
			}
			to := cacheTo
			from := to.Add(-timeseries.Hour)
			step, err := cacheClient.GetStep(from, to)
			if err != nil {
				klog.Errorln(err)
				continue
			}
			cacheClient.GetStatus()
			ctr := constructor.New(database, project, cacheClient, pricing)
			world, err := ctr.LoadWorld(context.TODO(), from, to, step, nil)
			if err != nil {
				klog.Errorln("failed to load world:", err)
				continue
			}

			wg := sync.WaitGroup{}
			if incidents != nil {
				wg.Add(1)
				go func() {
					defer wg.Done()
					incidents.Check(project, world)
				}()
			}
			if deployments != nil {
				wg.Add(1)
				go func() {
					defer wg.Done()
					deployments.Check(project, world)
				}()
			}
			wg.Wait()
			klog.Infof("%s: iteration done in %s", project.Id, time.Since(start).Truncate(time.Millisecond))
		}
	}()
}
