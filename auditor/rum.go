package auditor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func rumServiceName(app *model.Application) string {
	if app.Settings != nil && app.Settings.Rum != nil && app.Settings.Rum.Service != "" {
		return app.Settings.Rum.Service
	}
	if app.Id.Kind == model.ApplicationKindRumClient {
		return app.Id.Name
	}
	return ""
}

func (a *appAuditor) rum() {
	svc := rumServiceName(a.app)
	stats := a.app.RumStats
	if svc == "" && stats == nil {
		return
	}

	report := a.addReport(model.AuditReportRum)
	// Charts live in the /rum payload (AppRum). Keep checks visible on the report card.
	report.Custom = false

	if stats != nil {
		lcpCheck := report.CreateCheck(model.Checks.RumLcpP75)
		inpCheck := report.CreateCheck(model.Checks.RumInpP75)
		clsCheck := report.CreateCheck(model.Checks.RumClsP75)
		ttfbCheck := report.CreateCheck(model.Checks.RumTtfbP75)
		jsCheck := report.CreateCheck(model.Checks.RumJsErrors)
		fetchCheck := report.CreateCheck(model.Checks.RumFetchErrors)

		// Build charts for check widgets only (not added to report.Widgets).
		rateChart := model.NewChart(a.w.Ctx, "Page views, per second")
		if stats.PageViewsPs != nil {
			rateChart.AddSeries("views", stats.PageViewsPs)
		}
		errChart := model.NewChart(a.w.Ctx, "Browser errors, per second")
		if stats.JsErrorsPs != nil {
			errChart.AddSeries("errors", stats.JsErrorsPs)
		}
		jsCheck.AddWidget(&model.Widget{Chart: errChart})

		cwvChart := model.NewChart(a.w.Ctx, "Core Web Vitals p75 (ms)")
		if stats.LcpP75 != nil {
			cwvChart.AddSeries("lcp", stats.LcpP75)
		}
		if stats.InpP75 != nil {
			cwvChart.AddSeries("inp", stats.InpP75)
		}
		if stats.TtfbP75 != nil {
			cwvChart.AddSeries("ttfb", stats.TtfbP75)
		}
		cwvWidget := &model.Widget{Chart: cwvChart}
		lcpCheck.AddWidget(cwvWidget)
		inpCheck.AddWidget(cwvWidget)
		ttfbCheck.AddWidget(cwvWidget)

		clsChart := model.NewChart(a.w.Ctx, "CLS p75")
		if stats.ClsP75 != nil {
			clsChart.AddSeries("cls", stats.ClsP75)
		}
		clsCheck.AddWidget(&model.Widget{Chart: clsChart})

		_ = rateChart

		setIfFinite := func(ch *model.Check, ts *timeseries.TimeSeries) {
			if ts == nil {
				return
			}
			v := ts.Last()
			if timeseries.IsNaN(v) {
				return
			}
			ch.SetValue(v)
		}
		setIfFinite(lcpCheck, stats.LcpP75)
		setIfFinite(inpCheck, stats.InpP75)
		setIfFinite(clsCheck, stats.ClsP75)
		setIfFinite(ttfbCheck, stats.TtfbP75)
		setIfFinite(jsCheck, stats.JsErrorsPs)
		if stats.FetchErrorPct >= 0 {
			fetchCheck.SetValue(stats.FetchErrorPct)
		}
	}

	report.AddWidget(&model.Widget{Rum: &model.Rum{ApplicationId: a.app.Id}, Width: "100%"})
}
