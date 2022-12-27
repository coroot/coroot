package alerts

import (
	"context"
	"fmt"
	"github.com/coroot/coroot/auditor"
	"github.com/coroot/coroot/cache"
	"github.com/coroot/coroot/constructor"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
	"time"
)

type Alert struct {
	ProjectId     db.ProjectId
	ApplicationId model.ApplicationId
	Incident      *db.Incident
	Reports       []*model.AuditReport
}

type AlertManager struct {
	db    *db.DB
	cache *cache.Cache
}

func NewAlertManager(db *db.DB, cache *cache.Cache) *AlertManager {
	return &AlertManager{db: db, cache: cache}
}

func (mgr *AlertManager) Start(checkInterval time.Duration) {
	go func() {
		for range time.Tick(checkInterval) {
			projects, err := mgr.db.GetProjects()
			if err != nil {
				klog.Errorln("failed to get projects:", err)
				continue
			}
			for _, project := range projects {
				mgr.checkProject(project)
			}
		}
	}()
}

func (mgr *AlertManager) checkProject(project *db.Project) {
	t := time.Now()
	var apps int
	defer func() {
		klog.Infof("%s: checked %d apps in %s", project.Id, apps, time.Since(t).Truncate(time.Millisecond))
	}()

	world, err := mgr.loadWorld(project)
	if err != nil {
		klog.Errorln("failed to load world:", err)
		return
	}

	auditor.Audit(world)

	for _, app := range world.Applications {
		status := app.SLOStatus()
		if status == model.UNKNOWN {
			continue
		}
		apps++
		incident, err := mgr.db.CreateOrUpdateIncident(project.Id, app.Id, timeseries.Now(), status)
		if err != nil {
			klog.Errorln(err)
			continue
		}
		if incident == nil {
			continue
		}
		if ok := mgr.sendAlert(project, app, incident); ok {
			if err := mgr.db.MarkIncidentAsSent(project.Id, app.Id, incident, timeseries.Now()); err != nil {
				klog.Errorln(err)
			}
		}
	}
}

func (mgr *AlertManager) loadWorld(project *db.Project) (*model.World, error) {
	cc := mgr.cache.GetCacheClient(project)
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
	return constructor.New(mgr.db, project, cc).LoadWorld(context.Background(), from, to, step, nil)
}

func (mgr *AlertManager) sendAlert(project *db.Project, app *model.Application, incident *db.Incident) bool {
	alert := Alert{ProjectId: project.Id, ApplicationId: app.Id, Incident: incident, Reports: app.Reports}
	sent := false
	if cfg := project.Settings.Integrations.Slack; cfg != nil && cfg.Enabled {
		err := NewSlack(cfg.Token).SendAlert(project.Settings.Integrations.BaseUrl, cfg.DefaultChannel, alert)
		if err != nil {
			klog.Errorln("slack error:", err)
		} else {
			klog.Infoln("alert successfully sent to the slack channel")
			sent = true
		}
	}
	return sent
}
