package watchers

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func collectRumSignals(app *model.Application, world *model.World) []model.RumSignal {
	var out []model.RumSignal
	seen := map[model.ApplicationId]bool{}

	consider := func(rumApp *model.Application) {
		if rumApp == nil || rumApp.RumStats == nil || seen[rumApp.Id] {
			return
		}
		seen[rumApp.Id] = true
		stats := rumApp.RumStats
		svc := rumApp.Id.Name
		if rumApp.Settings != nil && rumApp.Settings.Rum != nil && rumApp.Settings.Rum.Service != "" {
			svc = rumApp.Settings.Rum.Service
		}
		add := func(check model.CheckConfig, ts *timeseries.TimeSeries, fmtValue func(float32) string) {
			if ts == nil {
				return
			}
			v := ts.Last()
			if timeseries.IsNaN(v) || v <= check.DefaultThreshold {
				return
			}
			msg := check.Title + " degraded"
			if fmtValue != nil {
				msg = check.Title + " " + fmtValue(v) + " > " + fmtValue(check.DefaultThreshold)
			}
			out = append(out, model.RumSignal{
				Service:   svc,
				Check:     string(check.Id),
				Value:     v,
				Threshold: check.DefaultThreshold,
				Message:   msg,
			})
		}
		ms := func(v float32) string { return model.CheckUnit("").FormatValue(v) + " ms" }
		add(model.Checks.RumLcpP75, stats.LcpP75, ms)
		add(model.Checks.RumInpP75, stats.InpP75, ms)
		add(model.Checks.RumClsP75, stats.ClsP75, func(v float32) string { return model.CheckUnit("").FormatValue(v) })
		add(model.Checks.RumJsErrors, stats.JsErrorsPs, nil)
		if stats.FetchErrorPct > model.Checks.RumFetchErrors.DefaultThreshold {
			out = append(out, model.RumSignal{
				Service:   svc,
				Check:     string(model.Checks.RumFetchErrors.Id),
				Value:     stats.FetchErrorPct,
				Threshold: model.Checks.RumFetchErrors.DefaultThreshold,
				Message:   model.Checks.RumFetchErrors.Title + " elevated",
			})
		}
	}

	if app.Id.Kind == model.ApplicationKindRumClient {
		consider(app)
		return out
	}
	consider(app)
	for _, rumApp := range world.Applications {
		if rumApp.Id.Kind != model.ApplicationKindRumClient {
			continue
		}
		for _, u := range rumApp.Upstreams {
			if u.RemoteApplication != nil && u.RemoteApplication.Id == app.Id {
				consider(rumApp)
				break
			}
		}
	}
	return out
}
