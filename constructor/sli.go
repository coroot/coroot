package constructor

import (
	"context"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/prom"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
)

func loadSLIs(ctx context.Context, w *model.World, prom prom.Client, from, to timeseries.Time, step timeseries.Duration) {
	for appId := range w.CheckConfigs {
		app := w.GetApplication(appId)
		if app == nil {
			continue
		}
		for _, cfg := range w.CheckConfigs.GetLatency(appId) {
			hist, err := prom.QueryRange(ctx, cfg.Histogram(), from, to, step)
			if err != nil {
				klog.Warningln(err)
				continue
			}
			avg, err := prom.QueryRange(ctx, cfg.Average(), from, to, step)
			if err != nil {
				klog.Warningln(err)
				continue
			}
			app.LatencySLIs = append(app.LatencySLIs, latencySLI(cfg, hist, avg))
		}
		for _, cfg := range w.CheckConfigs.GetAvailability(appId) {
			total, err := prom.QueryRange(ctx, cfg.Total(), from, to, step)
			if err != nil {
				klog.Warningln(err)
				continue
			}
			failed, err := prom.QueryRange(ctx, cfg.Failed(), from, to, step)
			if err != nil {
				klog.Warningln(err)
				continue
			}
			app.AvailabilitySLIs = append(app.AvailabilitySLIs, availabilitySLI(cfg, total, failed))
		}
	}
}

func latencySLI(cfg model.CheckConfigSLOLatency, histogram, average []model.MetricValues) *model.LatencySLI {
	sli := &model.LatencySLI{
		Config:    cfg,
		Histogram: map[string]timeseries.TimeSeries{},
	}
	for _, m := range histogram {
		le := m.Labels["le"]
		sli.Histogram[le] = update(sli.Histogram[le], m.Values)
		switch le {
		case cfg.ObjectiveBucket:
			sli.FastRequests = update(m.Values, m.Values)
		case "+Inf":
			sli.TotalRequests = update(sli.TotalRequests, m.Values)
		}
	}
	if len(average) == 1 {
		sli.Average = average[0].Values
	}
	return sli
}

func availabilitySLI(cfg model.CheckConfigSLOAvailability, total, failed []model.MetricValues) *model.AvailabilitySLI {
	sli := &model.AvailabilitySLI{
		Config: cfg,
	}
	if len(total) == 1 {
		sli.TotalRequests = total[0].Values
	}
	if len(failed) == 1 {
		sli.FailedRequests = failed[0].Values
	}
	return sli
}
