package deployments

import (
	"context"
	"fmt"
	"github.com/coroot/coroot/cache"
	"github.com/coroot/coroot/constructor"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/notifications"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"k8s.io/klog"
	"math"
	"sort"
	"time"
)

type Watcher struct {
	db    *db.DB
	cache *cache.Cache
}

func NewWatcher(db *db.DB, cache *cache.Cache) *Watcher {
	return &Watcher{db: db, cache: cache}
}

func (w *Watcher) Start(interval time.Duration) {
	go func() {
		for range time.Tick(interval) {
			projects, err := w.db.GetProjects()
			if err != nil {
				klog.Errorln("failed to get projects:", err)
				continue
			}
			for _, project := range projects {
				world, cacheTo := w.discoverAndSaveDeployments(project)
				if world == nil {
					continue
				}
				w.snapshotDeploymentMetrics(project, world.Applications)
				w.sendNotifications(project, world, cacheTo)
			}
		}
	}()
}

func (w *Watcher) discoverAndSaveDeployments(project *db.Project) (*model.World, timeseries.Time) {
	t := time.Now()
	var apps int
	defer func() {
		klog.Infof("%s: checked %d apps in %s", project.Id, apps, time.Since(t).Truncate(time.Millisecond))
	}()

	cacheClient, cacheTo, err := w.getCacheClient(project)
	if err != nil {
		klog.Errorln("failed to get cache client:", err)
		return nil, cacheTo
	}
	step := project.Prometheus.RefreshInterval
	to := cacheTo
	from := to.Add(-timeseries.Hour)
	world, err := constructor.New(w.db, project, cacheClient).LoadWorld(context.Background(), from, to, step, nil)
	if err != nil {
		klog.Errorln("failed to load world:", err)
		return nil, cacheTo
	}

	for _, app := range world.Applications {
		if app.Id.Kind != model.ApplicationKindDeployment {
			continue
		}
		apps++

		deployments := calcDeployments(app)

		if len(app.Deployments) == 0 && len(deployments) == 0 {
			if err := w.db.SaveApplicationDeployment(project.Id, calcInitialDeployment(app, cacheTo)); err != nil {
				klog.Errorln("failed to save deployment:", err)
			}
			continue
		}
		for _, d := range deployments {
			var known *model.ApplicationDeployment
			for _, dd := range app.Deployments {
				if dd.Name == d.Name && dd.StartedAt == d.StartedAt {
					known = dd
					break
				}
			}
			if known == nil || known.FinishedAt != d.FinishedAt {
				if err := w.db.SaveApplicationDeployment(project.Id, d); err != nil {
					klog.Errorln("failed to save deployment:", err)
					return nil, cacheTo
				}
			}
			if known == nil {
				klog.Infof("new deployment detected for %s: %s", app.Id, d.Name)
				app.Deployments = append(app.Deployments, d)
			}
		}
	}
	return world, cacheTo
}

func (w *Watcher) snapshotDeploymentMetrics(project *db.Project, applications []*model.Application) {
	if len(applications) == 0 {
		return
	}
	cacheClient, cacheTo, err := w.getCacheClient(project)
	if err != nil {
		klog.Errorln("failed to get cache client:", err)
		return
	}
	step := project.Prometheus.RefreshInterval
	for _, app := range applications {
		for i, d := range app.Deployments {
			if d.MetricsSnapshot != nil || d.FinishedAt.IsZero() {
				continue
			}
			from := d.FinishedAt.Add(model.ApplicationDeploymentMetricsSnapshotShift).Truncate(step)
			to := from.Add(model.ApplicationDeploymentMetricsSnapshotWindow).Truncate(step)
			nextOrNow := cacheTo
			if i < len(app.Deployments)-1 {
				nextOrNow = app.Deployments[i+1].StartedAt
			}
			if to.After(nextOrNow) {
				continue
			}
			world, err := constructor.New(w.db, project, cacheClient).LoadWorld(context.Background(), from, to, step, nil)
			if err != nil {
				klog.Errorln("failed to load world:", err)
				continue
			}
			a := world.GetApplication(d.ApplicationId)
			if a == nil {
				klog.Warningln("unknown application:", d.ApplicationId)
				continue
			}
			d.MetricsSnapshot = calcMetricsSnapshot(a, from, to, step)
			if err := w.db.SaveApplicationDeploymentMetricsSnapshot(project.Id, d); err != nil {
				klog.Errorln("failed to save metrics snapshot:", err)
				continue
			}
		}
	}
}

func (w *Watcher) sendNotifications(project *db.Project, world *model.World, now timeseries.Time) {
	slack := notifications.NewSlack(project)
	settings := project.Settings
	for _, app := range world.Applications {
		notificationsEnabled := settings.ApplicationCategorySettings[app.Category].NotifyOfDeployments
		for _, ds := range model.CalcApplicationDeploymentStatuses(app, world.CheckConfigs, now) {
			d := ds.Deployment
			if d.Notifications == nil {
				d.Notifications = &model.ApplicationDeploymentNotifications{}
			}
			if d.Notifications.State >= ds.State {
				continue
			}
			if notificationsEnabled && now.Sub(d.StartedAt) < timeseries.Day {
				if err := slack.SendDeployment(ds); err != nil {
					klog.Errorln("slack error:", err)
					continue
				}
			}
			d.Notifications.State = ds.State
			if err := w.db.SaveApplicationDeploymentNotifications(project.Id, d); err != nil {
				klog.Errorln(err)
			}
		}
	}
}

func (w *Watcher) getCacheClient(project *db.Project) (*cache.Client, timeseries.Time, error) {
	cc := w.cache.GetCacheClient(project)
	cacheTo, err := cc.GetTo()
	if err != nil {
		return nil, 0, err
	}
	if cacheTo.IsZero() {
		return nil, 0, fmt.Errorf("cache is empty")
	}
	return cc, cacheTo, nil
}

func calcDeployments(app *model.Application) []*model.ApplicationDeployment {
	if app.Id.Kind != model.ApplicationKindDeployment || len(app.Instances) == 0 {
		return nil
	}

	lifeSpans := map[string]*timeseries.AggregatedTimeseries{}
	images := map[string]*utils.StringSet{}
	for _, instance := range app.Instances {
		if instance.Pod == nil || instance.Pod.ReplicaSet == "" {
			continue
		}
		rs := instance.Pod.ReplicaSet
		lifeSpans[rs] = timeseries.Merge(lifeSpans[rs], instance.Pod.LifeSpan, timeseries.NanSum)
		if images[rs] == nil {
			images[rs] = utils.NewStringSet()
		}
		for _, container := range instance.Containers {
			images[rs].Add(container.Image)
		}
	}
	if len(lifeSpans) == 0 {
		return nil
	}

	iters := map[string]timeseries.Iterator{}
	for name, ts := range lifeSpans {
		iters[name] = timeseries.Iter(ts)
	}
	var rssOverTime []rss
	done := false
	for {
		names := make([]string, 0, len(lifeSpans))
		var t timeseries.Time
		var v float64
		for name, iter := range iters {
			if !iter.Next() {
				done = true
				break
			}
			t, v = iter.Value()
			if v > 0 {
				names = append(names, name)
			}
		}
		if done {
			break
		}
		if len(names) == 0 {
			continue
		}
		sort.Strings(names)
		rssOverTime = append(rssOverTime, rss{time: t, names: names})
	}

	var deployments []*model.ApplicationDeployment
	var deployment *model.ApplicationDeployment
	prev := ""
	for _, rss := range rssOverTime {
		switch len(rss.names) {
		case 0:
		case 1:
			curr := rss.names[0]
			if prev == "" {
				prev = curr
				continue
			}
			if deployment != nil {
				if curr == deployment.Name {
					deployment.FinishedAt = rss.time
				}
				deployment = nil
			}
			if prev == curr {
				continue
			}
			if deployment == nil {
				deployment = &model.ApplicationDeployment{ApplicationId: app.Id, Name: curr, StartedAt: rss.time}
				deployments = append(deployments, deployment)
			}
			deployment.FinishedAt = rss.time
			deployment = nil
			prev = curr
		default:
			if prev == "" {
				continue
			}
			if deployment == nil {
				name := ""
				for _, n := range rss.names { // get some new name
					if n != prev {
						name = n
						break
					}
				}
				deployment = &model.ApplicationDeployment{ApplicationId: app.Id, Name: name, StartedAt: rss.time}
				deployments = append(deployments, deployment)
				prev = name
			}
		}
	}

	for _, d := range deployments {
		if images[d.Name] != nil {
			d.Details = &model.ApplicationDeploymentDetails{
				ContainerImages: images[d.Name].Items(),
			}
		}
	}

	return deployments
}

func calcInitialDeployment(app *model.Application, now timeseries.Time) *model.ApplicationDeployment {
	name := ""
	images := utils.NewStringSet()
	for _, i := range app.Instances {
		if i.Pod != nil && i.Pod.ReplicaSet != "" {
			name = i.Pod.ReplicaSet
		}
		for _, c := range i.Containers {
			if c.Image != "" {
				images.Add(c.Image)
			}
		}
	}
	res := &model.ApplicationDeployment{
		ApplicationId: app.Id,
		Name:          name,
		StartedAt:     now,
		FinishedAt:    now,
	}
	if images.Len() > 0 {
		res.Details = &model.ApplicationDeploymentDetails{ContainerImages: images.Items()}
	}
	res.Notifications = &model.ApplicationDeploymentNotifications{
		State: model.ApplicationDeploymentStateSummary,
	}
	return res
}

func calcMetricsSnapshot(app *model.Application, from, to timeseries.Time, step timeseries.Duration) *model.MetricsSnapshot {
	ms := model.MetricsSnapshot{Timestamp: to, Duration: to.Sub(from), Latency: map[string]int64{}}
	for _, sli := range app.AvailabilitySLIs {
		ms.Requests = sumR(sli.TotalRequests, step)
		ms.Errors = sumR(sli.FailedRequests, step)
		break
	}
	for _, sli := range app.LatencySLIs {
		for _, h := range sli.Histogram {
			ms.Latency[fmt.Sprintf("%.3f", h.Le)] = sumR(h.TimeSeries, step)
		}
		break
	}
	cpuUsage := timeseries.Aggregate(timeseries.NanSum)
	memUsage := timeseries.Aggregate(timeseries.NanSum)
	oomKills := timeseries.Aggregate(timeseries.NanSum)
	restarts := timeseries.Aggregate(timeseries.NanSum)
	logErrors := timeseries.Aggregate(timeseries.NanSum)
	logWarnings := timeseries.Aggregate(timeseries.NanSum)
	for _, i := range app.Instances {
		for _, c := range i.Containers {
			cpuUsage.AddInput(c.CpuUsage)
			memUsage.AddInput(c.MemoryRss)
			restarts.AddInput(c.Restarts)
			oomKills.AddInput(c.OOMKills)
			for level, ts := range i.LogMessagesByLevel {
				switch level {
				case model.LogLevelCritical, model.LogLevelError:
					logErrors.AddInput(ts)
				case model.LogLevelWarning:
					logWarnings.AddInput(ts)
				}
			}
		}
	}
	ms.CPUUsage = float32(sumRF(cpuUsage, step))
	if lr := timeseries.NewLinearRegression(memUsage); lr != nil {
		ms.MemoryLeak = int64(lr.Calc(from.Add(timeseries.Hour)) - lr.Calc(from))
	}
	ms.OOMKills = sum(oomKills)
	ms.Restarts = sum(restarts)
	ms.LogErrors = sum(logErrors)
	ms.LogWarnings = sum(logWarnings)
	return &ms
}

func sumR(ts timeseries.TimeSeries, step timeseries.Duration) int64 {
	return int64(sumRF(ts, step))
}

func sumRF(ts timeseries.TimeSeries, step timeseries.Duration) float64 {
	return sumF(ts) * float64(step/timeseries.Second)
}

func sum(ts timeseries.TimeSeries) int64 {
	return int64(sumF(ts))
}

func sumF(ts timeseries.TimeSeries) float64 {
	v := timeseries.Reduce(timeseries.NanSum, ts)
	if math.IsNaN(v) {
		return 0
	}
	return v
}

type rss struct {
	time  timeseries.Time
	names []string
}
