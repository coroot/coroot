package incidents

import (
	"context"
	"fmt"
	"github.com/coroot/coroot/auditor"
	"github.com/coroot/coroot/cache"
	"github.com/coroot/coroot/constructor"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/notifications"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
	"time"
)

type Watcher struct {
	db       *db.DB
	cache    *cache.Cache
	notifier *notifications.IncidentNotifier
}

func NewWatcher(db *db.DB, cache *cache.Cache, notifier *notifications.IncidentNotifier) *Watcher {
	return &Watcher{db: db, cache: cache, notifier: notifier}
}

func (w *Watcher) Start(checkInterval time.Duration) {
	go func() {
		for range time.Tick(checkInterval) {
			projects, err := w.db.GetProjects()
			if err != nil {
				klog.Errorln("failed to get projects:", err)
				continue
			}
			for _, project := range projects {
				w.checkProject(project)
			}
		}
	}()
}

func (w *Watcher) checkProject(project *db.Project) {
	t := time.Now()
	var apps int
	defer func() {
		klog.Infof("%s: checked %d apps in %s", project.Id, apps, time.Since(t).Truncate(time.Millisecond))
	}()

	world, err := w.loadWorld(project)
	if err != nil {
		klog.Errorln("failed to load world:", err)
		return
	}

	auditor.Audit(world, project)

	for _, app := range world.Applications {
		status := app.SLOStatus()
		if status == model.UNKNOWN {
			continue
		}
		apps++
		now := timeseries.Now()
		incident, err := w.db.CreateOrUpdateIncident(project.Id, app.Id, now, status)
		if err != nil {
			klog.Errorln(err)
			continue
		}
		if incident == nil {
			continue
		}
		w.notifier.Enqueue(project, app, incident, now)
	}
}

func (w *Watcher) loadWorld(project *db.Project) (*model.World, error) {
	cc := w.cache.GetCacheClient(project)
	cacheTo, err := cc.GetTo()
	if err != nil {
		return nil, err
	}
	if cacheTo.IsZero() {
		return nil, fmt.Errorf("cache is empty")
	}
	step := project.Prometheus.RefreshInterval
	to := cacheTo.Truncate(step)
	from := to.Add(-timeseries.Hour)
	return constructor.New(w.db, project, cc, nil).LoadWorld(context.Background(), from, to, step, nil)
}
