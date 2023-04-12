package auditor

import (
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"sort"
)

type appAuditor struct {
	w       *model.World
	p       *db.Project
	app     *model.Application
	reports []*model.AuditReport
}

func Audit(w *model.World, p *db.Project) {
	ncs := nodeConsumersByNode{}

	for _, app := range w.Applications {
		a := &appAuditor{
			w:   w,
			p:   p,
			app: app,
		}
		a.slo()
		a.instances()
		a.cpu(ncs)
		a.memory(ncs)
		a.storage()
		a.network()
		a.postgres()
		a.redis()
		a.jvm()
		a.logs()
		a.deployments()

		for _, r := range a.reports {
			widgets := a.enrichWidgets(r.Widgets, app.Events)
			sort.SliceStable(widgets, func(i, j int) bool {
				return widgets[i].Table != nil
			})
			r.Widgets = widgets

			for _, ch := range r.Checks {
				ch.Calc()
				if ch.Status > r.Status {
					r.Status = ch.Status
				}
			}
			switch r.Name {
			case model.AuditReportPostgres, model.AuditReportRedis, model.AuditReportInstances, model.AuditReportSLO:
				if app.Status < r.Status {
					app.Status = r.Status
				}
			}
			app.Reports = append(app.Reports, r)
		}

		if p.Settings.Integrations.Pyroscope != nil {
			app.AddReport(model.AuditReportProfiling, &model.Widget{Profile: &model.Profile{ApplicationId: app.Id}, Width: "100%"})
		}
	}
}

func (a *appAuditor) addReport(name model.AuditReportName) *model.AuditReport {
	r := model.NewAuditReport(a.app, a.w.Ctx, a.w.CheckConfigs, name)
	a.reports = append(a.reports, r)
	return r
}

func (a *appAuditor) enrichWidgets(widgets []*model.Widget, events []*model.ApplicationEvent) []*model.Widget {
	annotations := model.EventsToAnnotations(events, a.w.Ctx)
	var res []*model.Widget
	for _, w := range widgets {
		if w.Chart != nil {
			if w.Chart.IsEmpty() {
				continue
			}
		}
		if w.ChartGroup != nil {
			var charts []*model.Chart
			for _, ch := range w.ChartGroup.Charts {
				if ch.IsEmpty() {
					continue
				}
				charts = append(charts, ch)
			}
			if len(charts) == 0 {
				continue
			}
			w.ChartGroup.Charts = charts
		}
		w.AddAnnotation(annotations...)
		res = append(res, w)
	}
	return res
}
