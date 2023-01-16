package auditor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"sort"
	"strings"
)

type appAuditor struct {
	w       *model.World
	app     *model.Application
	reports []*model.AuditReport
}

func Audit(w *model.World) {
	for _, app := range w.Applications {
		a := &appAuditor{
			w:   w,
			app: app,
		}
		a.slo()
		a.instances()
		a.cpu()
		a.memory()
		a.storage()
		a.network()
		a.postgres()
		a.redis()
		a.jvm()
		a.logs()
		a.deployments()

		for _, r := range a.reports {
			widgets := enrichWidgets(r.Widgets, app.Events)
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
}

func (a *appAuditor) addReport(name model.AuditReportName) *model.AuditReport {
	r := model.NewAuditReport(a.app, a.w.Ctx, a.w.CheckConfigs, name)
	a.reports = append(a.reports, r)
	return r
}

func enrichWidgets(widgets []*model.Widget, events []*model.ApplicationEvent) []*model.Widget {
	var res []*model.Widget
	for _, w := range widgets {
		if w.Chart != nil {
			if len(w.Chart.Series) == 0 {
				continue
			}
			addAnnotations(events, w.Chart)
		}
		if w.ChartGroup != nil {
			var charts []*model.Chart
			for _, ch := range w.ChartGroup.Charts {
				if len(ch.Series) == 0 {
					continue
				}
				charts = append(charts, ch)
				addAnnotations(events, ch)
			}
			if len(charts) == 0 {
				continue
			}
			w.ChartGroup.Charts = charts
			w.ChartGroup.AutoFeatureChart()
		}
		if w.LogPatterns != nil {
			for _, p := range w.LogPatterns.Patterns {
				if p.Instances != nil {
					addAnnotations(events, p.Instances)
				}
			}
		}
		res = append(res, w)
	}
	return res
}

type annotation struct {
	start  timeseries.Time
	end    timeseries.Time
	events []*model.ApplicationEvent
}

func addAnnotations(events []*model.ApplicationEvent, chart *model.Chart) {
	if len(events) == 0 {
		return
	}
	var annotations []*annotation
	getLast := func() *annotation {
		if len(annotations) == 0 {
			return nil
		}
		return annotations[len(annotations)-1]
	}
	for _, e := range events {
		last := getLast()
		if last == nil || e.Start.Sub(last.start) > 3*chart.Ctx.Step {
			a := &annotation{start: e.Start, end: e.End, events: []*model.ApplicationEvent{e}}
			annotations = append(annotations, a)
			continue
		}
		last.events = append(last.events, e)
		last.end = e.End
	}
	for _, a := range annotations {
		sort.Slice(a.events, func(i, j int) bool {
			return a.events[i].Type < a.events[j].Type
		})
		icon := ""
		var msgs []string
		for _, e := range a.events {
			i := ""
			switch e.Type {
			case model.ApplicationEventTypeRollout:
				msgs = append(msgs, "deployment "+e.Details)
				i = "mdi-swap-horizontal-circle-outline"
			case model.ApplicationEventTypeSwitchover:
				msgs = append(msgs, "switchover "+e.Details)
				i = "mdi-database-sync-outline"
			case model.ApplicationEventTypeInstanceUp:
				msgs = append(msgs, e.Details+" is up")
				i = "mdi-alert-octagon-outline"
			case model.ApplicationEventTypeInstanceDown:
				msgs = append(msgs, e.Details+" is down")
				i = "mdi-alert-octagon-outline"
			}
			if icon == "" {
				icon = i
			}
		}
		chart.AddAnnotation(strings.Join(msgs, "<br>"), a.start, a.end, icon)
	}
}
