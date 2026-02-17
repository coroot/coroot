package watchers

import (
	"context"
	"sync"
	"time"

	"github.com/coroot/coroot/auditor"
	"github.com/coroot/coroot/cache"
	"github.com/coroot/coroot/clickhouse"
	pricing "github.com/coroot/coroot/cloud-pricing"
	"github.com/coroot/coroot/config"
	"github.com/coroot/coroot/constructor"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/timeseries"
	"golang.org/x/exp/maps"
	"k8s.io/klog"
)

func Start(database *db.DB, mcache *cache.Cache, pricing *pricing.Manager, incidents *Incidents, checkDeployments bool, globalClickHouse *db.IntegrationClickhouse, globalPrometheus *db.IntegrationPrometheus, spaceManagerCfg config.ClickHouseSpaceManager, logPatternEvaluator LogPatternEvaluator) {
	var deployments *Deployments
	if checkDeployments {
		deployments = NewDeployments(database, pricing)
	}

	alerts := NewAlerts(database, globalPrometheus, globalClickHouse, logPatternEvaluator)

	if incidents == nil && deployments == nil && alerts == nil {
		return
	}

	projectChan := make(chan db.ProjectId, 1000)

	pending := map[db.ProjectId]bool{}
	pendingLock := sync.Mutex{}
	lastSpaceManagerRun := time.Time{}

	// Fast consumer goroutine - just receives and deduplicates
	go func() {
		for projectId := range mcache.Updates() {
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
		// multi-cluster projects are skipped in the cache updater, so we need to check Incidents and Deployments by a ticker
		ticker := time.NewTicker(cache.MinRefreshInterval.ToStandard()).C

		for {
			select {
			case <-ticker:
				if !database.GetPrimaryLock(context.TODO()) {
					continue
				}
				projects, err := database.GetProjects()
				if err != nil {
					klog.Errorln(err)
				} else {
					for _, project := range projects {
						if project.Multicluster() {
							handleProjectUpdate(database, mcache, pricing, incidents, deployments, alerts, project.Id)
						}
					}
				}
			case projectId := <-projectChan:
				// Remove from pending set
				pendingLock.Lock()
				delete(pending, projectId)
				pendingLock.Unlock()

				if !database.GetPrimaryLock(context.TODO()) {
					klog.Infoln("not the primary replica: skipping")
					continue
				}

				handleProjectUpdate(database, mcache, pricing, incidents, deployments, alerts, projectId)

				if time.Since(lastSpaceManagerRun) >= time.Hour {
					lastSpaceManagerRun = time.Now()
					runSpaceManagerOnce(spaceManagerCfg, database, globalClickHouse)
				}
			}
		}
	}()
}

func handleProjectUpdate(database *db.DB, cache *cache.Cache, pricing *pricing.Manager, incidents *Incidents, deployments *Deployments, alerts *Alerts, projectId db.ProjectId) {
	start := time.Now()
	project, err := database.GetProject(projectId)
	if err != nil {
		klog.Errorln(err)
		return
	}

	cacheClients := map[db.ProjectId]constructor.Cache{}

	var (
		step     timeseries.Duration
		from, to timeseries.Time
	)

	if !project.Multicluster() {
		cacheClient := cache.GetCacheClient(project.Id)
		cacheTo, err := cacheClient.GetTo()
		if err != nil {
			klog.Errorln(err)
			return
		}
		if cacheTo.IsZero() {
			return
		}
		to = cacheTo
		from = to.Add(-timeseries.Hour)
		st, err := cacheClient.GetStep(from, to)
		if err != nil {
			klog.Errorln(err)
			return
		}
		if st > step {
			step = st
		}
		cacheClients[project.Id] = cacheClient
	} else {
		projects, err := database.GetProjects()
		if err != nil {
			klog.Errorln(err)
			return
		}
		for _, mp := range project.Settings.MemberProjects {
			p := projects[mp]
			if p == nil {
				klog.Warningln("member project not found:", mp)
				return
			}
			cacheClient := cache.GetCacheClient(p.Id)
			cacheTo, err := cacheClient.GetTo()
			if err != nil {
				klog.Errorln(err)
				return
			}
			if cacheTo.IsZero() || cacheTo.Before(from) {
				return
			}
			st, err := cacheClient.GetStep(from, to)
			if err != nil {
				klog.Errorln(err)
				return
			}
			if st > step {
				step = st
			}
			if to.IsZero() || cacheTo.Before(to) {
				to = cacheTo
				from = to.Add(-timeseries.Hour)
			}
			cacheClients[p.Id] = cacheClient
		}
	}

	ctr := constructor.New(database, project, cacheClients, pricing)
	world, err := ctr.LoadWorld(context.TODO(), from, to, step, nil)
	if err != nil {
		klog.Errorln("failed to load world:", err)
		return
	}
	auditor.Audit(world, project, nil, nil)

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
	if alerts != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			alerts.Check(project, world, from, to, step)
		}()
	}
	wg.Wait()
	klog.Infof("%s: iteration done in %s", project.Id, time.Since(start).Truncate(time.Millisecond))
}

func runSpaceManagerOnce(cfg config.ClickHouseSpaceManager, database *db.DB, globalClickHouse *db.IntegrationClickhouse) {
	if !cfg.Enabled {
		klog.Infof("clickhouse space manager disabled")
		return
	}
	projects, err := database.GetProjects()
	if err != nil {
		klog.Errorf("clickhouse space manager: failed to get projects: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := clickhouse.RunSpaceManagerForProjects(ctx, cfg, maps.Values(projects), globalClickHouse); err != nil {
		klog.Errorf("clickhouse space manager: failed: %v", err)
	}
}
