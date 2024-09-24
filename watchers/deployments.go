package watchers

import (
	"context"
	"fmt"
	"sort"
	"time"

	cloud_pricing "github.com/coroot/coroot/cloud-pricing"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/notifications"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"k8s.io/klog"
)

const (
	sendTimeout = 30 * time.Second
)

type Deployments struct {
	db      *db.DB
	pricing *cloud_pricing.Manager
}

func NewDeployments(db *db.DB, pricing *cloud_pricing.Manager) *Deployments {
	return &Deployments{db: db, pricing: pricing}
}

func (w *Deployments) Check(project *db.Project, world *model.World) {
	start := time.Now()
	apps := w.discoverAndSaveDeployments(project, world)
	w.snapshotDeploymentMetrics(project, world)
	w.sendNotifications(project, world)
	klog.Infof("%s: checked %d apps in %s", project.Id, apps, time.Since(start).Truncate(time.Millisecond))
}

func (w *Deployments) discoverAndSaveDeployments(project *db.Project, world *model.World) int {
	var apps int
	for _, app := range world.Applications {
		if app.Id.Kind != model.ApplicationKindDeployment {
			continue
		}
		apps++

		for _, d := range calcDeployments(app) {
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
					return apps
				}
			}
			if known == nil {
				klog.Infof("new deployment detected for %s: %s", app.Id, d.Name)
				app.Deployments = append(app.Deployments, d)
			}
		}
	}
	return apps
}

func (w *Deployments) snapshotDeploymentMetrics(project *db.Project, world *model.World) {
	now := world.Ctx.To
	step := world.Ctx.Step
	for _, app := range world.Applications {
		for i, d := range app.Deployments {
			if d.MetricsSnapshot != nil || d.FinishedAt.IsZero() {
				continue
			}
			from := d.FinishedAt.Add(model.ApplicationDeploymentMetricsSnapshotShift)
			to := from.Add(model.ApplicationDeploymentMetricsSnapshotWindow)
			nextOrNow := now
			if i < len(app.Deployments)-1 {
				nextOrNow = app.Deployments[i+1].StartedAt
			}
			if to.After(nextOrNow) {
				continue
			}
			d.MetricsSnapshot = calcMetricsSnapshot(app, from, to, step)
			if err := w.db.SaveApplicationDeploymentMetricsSnapshot(project.Id, d); err != nil {
				klog.Errorln("failed to save metrics snapshot:", err)
				continue
			}
		}
	}
}

func (w *Deployments) sendNotifications(project *db.Project, world *model.World) {
	integrations := project.Settings.Integrations
	categorySettings := project.Settings.ApplicationCategorySettings
	now := world.Ctx.To
	for _, app := range world.Applications {
		if !categorySettings[app.Category].NotifyOfDeployments {
			continue
		}
		for _, ds := range model.CalcApplicationDeploymentStatuses(app, world.CheckConfigs, now) {
			d := ds.Deployment
			if now.Sub(d.StartedAt) > timeseries.Day {
				continue
			}
			if d.Notifications == nil {
				d.Notifications = &model.ApplicationDeploymentNotifications{}
			}
			if d.Notifications.State >= ds.State {
				continue
			}
			needSave := false
			if cfg := integrations.Slack; cfg != nil && cfg.Deployments && d.Notifications.Slack.State < ds.State {
				client := notifications.NewSlack(cfg.Token, cfg.DefaultChannel)
				ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
				err := client.SendDeployment(ctx, project, ds)
				cancel()
				if err != nil {
					klog.Errorln(err)
				} else {
					d.Notifications.Slack.State = ds.State
					needSave = true
				}
			}
			if cfg := integrations.Teams; cfg != nil && cfg.Deployments && d.Notifications.Teams.State < ds.State {
				client := notifications.NewTeams(cfg.WebhookUrl)
				ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
				err := client.SendDeployment(ctx, project, ds)
				cancel()
				if err != nil {
					klog.Errorln(err)
				} else {
					d.Notifications.Teams.State = ds.State
					needSave = true
				}
			}
			if cfg := integrations.Webhook; cfg != nil && cfg.Deployments && d.Notifications.Webhook.State < ds.State {
				client := notifications.NewWebhook(cfg)
				ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
				err := client.SendDeployment(ctx, project, ds)
				cancel()
				if err != nil {
					klog.Errorln(err)
				} else {
					d.Notifications.Webhook.State = ds.State
					needSave = true
				}
			}
			if !needSave {
				continue
			}
			if err := w.db.SaveApplicationDeploymentNotifications(project.Id, d); err != nil {
				klog.Errorln(err)
			}
		}
	}
}

func calcDeployments(app *model.Application) []*model.ApplicationDeployment {
	if app.Id.Kind != model.ApplicationKindDeployment || len(app.Instances) == 0 {
		return nil
	}

	lifeSpans := map[string]*timeseries.Aggregate{}
	images := map[string]*utils.StringSet{}
	for _, instance := range app.Instances {
		if instance.Pod == nil || instance.Pod.ReplicaSet == "" {
			continue
		}
		rs := instance.Pod.ReplicaSet
		ts := lifeSpans[rs]
		if ts == nil {
			ts = timeseries.NewAggregate(timeseries.NanSum)
			lifeSpans[rs] = ts
		}
		ts.Add(instance.Pod.LifeSpan)
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

	iters := map[string]*timeseries.Iterator{}
	for name, agg := range lifeSpans {
		iter := agg.Get().Iter()
		iters[name] = iter
	}
	var rssOverTime []replicaSets
	done := false
	for {
		names := make([]string, 0, len(lifeSpans))
		var t timeseries.Time
		var v float32
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
		rssOverTime = append(rssOverTime, replicaSets{time: t, names: names})
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

func calcMetricsSnapshot(app *model.Application, from, to timeseries.Time, step timeseries.Duration) *model.MetricsSnapshot {
	ms := model.MetricsSnapshot{Timestamp: to, Duration: to.Sub(from), Latency: map[string]int64{}}
	for _, sli := range app.AvailabilitySLIs {
		ms.Requests = int64(sumRate(sli.TotalRequests, from, to, step))
		ms.Errors = int64(sumRate(sli.FailedRequests, from, to, step))
		break
	}
	for _, sli := range app.LatencySLIs {
		for _, h := range sli.Histogram {
			ms.Latency[fmt.Sprintf("%.3f", h.Le)] = int64(sumRate(h.TimeSeries, from, to, step))
		}
		break
	}
	cpuUsage := timeseries.NewAggregate(timeseries.NanSum)
	memUsage := timeseries.NewAggregate(timeseries.NanSum)
	oomKills := timeseries.NewAggregate(timeseries.NanSum)
	restarts := timeseries.NewAggregate(timeseries.NanSum)
	logErrors := timeseries.NewAggregate(timeseries.NanSum)
	logWarnings := timeseries.NewAggregate(timeseries.NanSum)

	for level, msgs := range app.LogMessages {
		switch level {
		case model.LogLevelCritical, model.LogLevelError:
			logErrors.Add(msgs.Messages)
		case model.LogLevelWarning:
			logWarnings.Add(msgs.Messages)
		}
	}

	for _, i := range app.Instances {
		for _, c := range i.Containers {
			cpuUsage.Add(c.CpuUsage)
			memUsage.Add(c.MemoryRss)
			restarts.Add(c.Restarts)
			oomKills.Add(c.OOMKills)
		}
	}
	ms.CPUUsage = sumRate(cpuUsage.Get(), from, to, step)
	if totalMem := memUsage.Get(); !totalMem.IsEmpty() {
		if lr := timeseries.NewLinearRegression(totalMem.Map(timeseries.ZeroToNan)); lr != nil {
			s := lr.Calc(from.Add(-timeseries.Hour))
			e := lr.Calc(from)
			if s > 0 && e > 0 {
				ms.MemoryLeakPercent = (e - s) / s * 100
			}
		}
		s := totalMem.Reduce(timeseries.NanSum)
		c := totalMem.Map(timeseries.Defined).Reduce(timeseries.NanSum)
		if c > 0 && s > 0 {
			ms.MemoryUsage = int64(s / c)
		}
	}
	ms.OOMKills = int64(sum(oomKills.Get(), from, to))
	ms.Restarts = int64(sum(restarts.Get(), from, to))
	ms.LogErrors = int64(sum(logErrors.Get(), from, to))
	ms.LogWarnings = int64(sum(logWarnings.Get(), from, to))
	return &ms
}

func sum(ts *timeseries.TimeSeries, from, to timeseries.Time) float32 {
	s := ts.Reduce(func(t timeseries.Time, s float32, v float32) float32 {
		if timeseries.IsNaN(s) {
			s = 0
		}
		if t.Before(from) || t.After(to) || timeseries.IsNaN(v) {
			return s
		}
		return s + v
	})
	if timeseries.IsNaN(s) {
		return 0
	}
	return s
}

func sumRate(ts *timeseries.TimeSeries, from, to timeseries.Time, step timeseries.Duration) float32 {
	return sum(ts, from, to) * float32(step/timeseries.Second)
}

type replicaSets struct {
	time  timeseries.Time
	names []string
}
