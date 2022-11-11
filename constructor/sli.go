package constructor

import (
	"context"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/prom"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
	"sort"
	"strconv"
)

func loadSLIs(ctx context.Context, w *model.World, prom prom.Client, rawStep timeseries.Duration, from, to timeseries.Time, step timeseries.Duration) {
	for appId := range w.CheckConfigs {
		app := w.GetApplication(appId)
		if app == nil {
			continue
		}
		rawFrom := to.Add(-model.MaxAlertRuleWindow)
		for _, cfg := range w.CheckConfigs.GetAvailability(appId) {
			qTotal, qFailed := cfg.Total(), cfg.Failed()
			sli := &model.AvailabilitySLI{
				Config:            cfg,
				TotalRequests:     queryAvailability(ctx, prom, qTotal, from, to, step),
				TotalRequestsRaw:  queryAvailability(ctx, prom, qTotal, rawFrom, to, rawStep),
				FailedRequests:    queryAvailability(ctx, prom, qFailed, from, to, step),
				FailedRequestsRaw: queryAvailability(ctx, prom, qFailed, rawFrom, to, rawStep),
			}
			app.AvailabilitySLIs = append(app.AvailabilitySLIs, sli)
		}
		for _, cfg := range w.CheckConfigs.GetLatency(appId) {
			q := cfg.Histogram()
			sli := &model.LatencySLI{
				Config:       cfg,
				Histogram:    queryLatency(ctx, prom, q, from, to, step),
				HistogramRaw: queryLatency(ctx, prom, q, rawFrom, to, rawStep),
			}
			app.LatencySLIs = append(app.LatencySLIs, sli)
		}
	}
}

func queryAvailability(ctx context.Context, prom prom.Client, query string, from, to timeseries.Time, step timeseries.Duration) timeseries.TimeSeries {
	values, err := prom.QueryRange(ctx, query, from, to, step)
	if err != nil {
		klog.Warningln(err)
		return nil
	}
	if len(values) == 0 {
		return nil
	}
	return values[0].Values
}

func queryLatency(ctx context.Context, prom prom.Client, query string, from, to timeseries.Time, step timeseries.Duration) []model.HistogramBucket {
	values, err := prom.QueryRange(ctx, query, from, to, step)
	if err != nil {
		klog.Warningln(err)
		return nil
	}
	buckets := make([]model.HistogramBucket, 0, len(values))
	for _, m := range values {
		le, err := strconv.ParseFloat(m.Labels["le"], 64)
		if err != nil {
			klog.Warningln(err)
			continue
		}
		buckets = append(buckets, model.HistogramBucket{Le: le, TimeSeries: m.Values})
	}
	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].Le < buckets[j].Le
	})
	return buckets
}
