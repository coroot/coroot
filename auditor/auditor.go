package auditor

import (
	"sort"
	"time"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"k8s.io/klog"
)

type appAuditor struct {
	w        *model.World
	p        *db.Project
	app      *model.Application
	reports  []*model.AuditReport
	detailed bool
}

type Profile struct {
	Stages map[string]float32 `json:"stages"`
}

type Stages map[string]time.Duration

func (ss Stages) stage(name string, f func()) {
	if ss == nil {
		f()
		return
	}
	t := time.Now()
	f()
	ss[name] += time.Since(t)
}

func Audit(w *model.World, p *db.Project, generateDetailedReportFor *model.Application, prof *Profile) {
	start := time.Now()
	ncs := nodeConsumersByNode{}

	var stages Stages
	if prof != nil {
		stages = Stages{}
	}
	for _, app := range w.Applications {
		a := &appAuditor{
			w:        w,
			p:        p,
			app:      app,
			detailed: app == generateDetailedReportFor,
		}
		stages.stage("slo", a.slo)
		stages.stage("instances", a.instances)
		stages.stage("cpu", func() { a.cpu(ncs) })
		stages.stage("memory", func() { a.memory(ncs) })
		stages.stage("storage", a.storage)
		stages.stage("gpu", a.gpu)
		stages.stage("network", a.network)
		stages.stage("dns", a.dns)
		stages.stage("postgres", a.postgres)
		stages.stage("mysql", a.mysql)
		stages.stage("redis", a.redis)
		stages.stage("mongodb", a.mongodb)
		stages.stage("memcached", a.memcached)
		stages.stage("jvm", a.jvm)
		stages.stage("dotnet", a.dotnet)
		stages.stage("python", a.python)
		stages.stage("nodejs", a.nodejs)
		stages.stage("logs", a.logs)
		stages.stage("deployments", a.deployments)

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
	}

	if prof != nil {
		prof.Stages = map[string]float32{}
		for name, duration := range stages {
			d := float32(duration.Seconds())
			if d > prof.Stages[name] {
				prof.Stages[name] = d
			}
		}
	}

	klog.Infof("%s: audited %d apps in %s", p.Id, len(w.Applications), time.Since(start).Truncate(time.Millisecond))
}

func (a *appAuditor) addReport(name model.AuditReportName) *model.AuditReport {
	r := model.NewAuditReport(a.app, a.w.Ctx, a.w.CheckConfigs, name, a.detailed)
	a.reports = append(a.reports, r)
	return r
}

func (a *appAuditor) delReport(name model.AuditReportName) {
	for i, r := range a.reports {
		if r.Name == name {
			a.reports = append(a.reports[:i], a.reports[i+1:]...)
			return
		}
	}
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
		if w.Heatmap != nil {
			if w.Heatmap.IsEmpty() {
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
