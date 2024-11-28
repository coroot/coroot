package watchers

import (
	"time"

	"github.com/coroot/coroot/auditor"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/notifications"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
)

type Incidents struct {
	db       *db.DB
	notifier *notifications.IncidentNotifier
}

func NewIncidents(db *db.DB) *Incidents {
	return &Incidents{db: db, notifier: notifications.NewIncidentNotifier(db)}
}

func (w *Incidents) Check(project *db.Project, world *model.World) {
	start := time.Now()

	auditor.Audit(world, project, nil, false)

	var apps int
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
	klog.Infof("%s: checked %d apps in %s", project.Id, apps, time.Since(start).Truncate(time.Millisecond))
}
