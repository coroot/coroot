package constructor

import (
	"sort"
	"strconv"
	"strings"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
)

func (c *Constructor) loadSLIs(w *model.World, metrics map[string][]model.MetricValues) {
	builtinAvailabilityCur := builtinAvailability(metrics[qRecordingRuleInboundRequestsTotal])
	builtinAvailabilityRaw := builtinAvailability(metrics[qRecordingRuleInboundRequestsTotal+"_raw"])
	builtinLatencyCur := builtinLatency(metrics[qRecordingRuleInboundRequestsHistogram])
	builtinLatencyRaw := builtinLatency(metrics[qRecordingRuleInboundRequestsHistogram+"_raw"])

	customAvailabilityCur := map[model.ApplicationId]availabilitySlis{}
	customAvailabilityRaw := map[model.ApplicationId]availabilitySlis{}
	customLatencyCur := map[model.ApplicationId][]model.HistogramBucket{}
	customLatencyRaw := map[model.ApplicationId][]model.HistogramBucket{}
	loadCustomSLIs(metrics, customAvailabilityCur, customAvailabilityRaw, customLatencyCur, customLatencyRaw)

	for _, app := range w.Applications {
		availabilityCfg, _ := w.CheckConfigs.GetAvailability(app.Id)
		if availabilityCfg.Custom {
			cur, raw := customAvailabilityCur[app.Id], customAvailabilityRaw[app.Id]
			app.AvailabilitySLIs = append(app.AvailabilitySLIs, &model.AvailabilitySLI{
				Config:        availabilityCfg,
				TotalRequests: cur.total, TotalRequestsRaw: raw.total,
				FailedRequests: cur.failed, FailedRequestsRaw: raw.failed,
			})
		} else {
			cur, raw := builtinAvailabilityCur[app.Id], builtinAvailabilityRaw[app.Id]
			if !cur.total.IsEmpty() || !raw.total.IsEmpty() {
				app.AvailabilitySLIs = append(app.AvailabilitySLIs, &model.AvailabilitySLI{
					Config:        availabilityCfg,
					TotalRequests: cur.total, TotalRequestsRaw: raw.total,
					FailedRequests: cur.failed, FailedRequestsRaw: raw.failed,
				})
			}
		}

		latencyCfg, _ := w.CheckConfigs.GetLatency(app.Id, app.Category)
		if latencyCfg.Custom {
			cur, raw := customLatencyCur[app.Id], customLatencyRaw[app.Id]
			app.LatencySLIs = append(app.LatencySLIs, &model.LatencySLI{
				Config:    latencyCfg,
				Histogram: cur, HistogramRaw: raw,
			})
		} else {
			cur, raw := builtinLatencyCur[app.Id], builtinLatencyRaw[app.Id]
			if len(cur) > 0 || len(raw) > 0 {
				app.LatencySLIs = append(app.LatencySLIs, &model.LatencySLI{
					Config:    latencyCfg,
					Histogram: cur, HistogramRaw: raw,
				})
			}
		}
	}
}

func loadCustomSLIs(metrics map[string][]model.MetricValues,
	availabilityCur, availabilityRaw map[model.ApplicationId]availabilitySlis,
	latencyCur, latencyRaw map[model.ApplicationId][]model.HistogramBucket,
) {
	for queryName, values := range metrics {
		if len(values) == 0 || !strings.HasPrefix(queryName, qApplicationCustomSLI) {
			continue
		}
		parts := strings.Split(queryName, "/")
		if len(parts) != 3 {
			continue
		}
		appId, _ := model.NewApplicationIdFromString(parts[1])
		switch parts[2] {
		case "total_requests":
			a := availabilityCur[appId]
			a.total = values[0].Values
			availabilityCur[appId] = a
		case "total_requests_raw":
			a := availabilityRaw[appId]
			a.total = values[0].Values
			availabilityRaw[appId] = a
		case "failed_requests":
			a := availabilityCur[appId]
			a.failed = values[0].Values
			availabilityCur[appId] = a
		case "failed_requests_raw":
			a := availabilityRaw[appId]
			a.failed = values[0].Values
			availabilityRaw[appId] = a
		case "requests_histogram":
			latencyCur[appId] = histogramBuckets(values)
		case "requests_histogram_raw":
			latencyRaw[appId] = histogramBuckets(values)
		}
	}
}

type availabilitySlis struct {
	total  *timeseries.TimeSeries
	failed *timeseries.TimeSeries
}

func builtinAvailability(values []model.MetricValues) map[model.ApplicationId]availabilitySlis {
	if len(values) == 0 {
		return nil
	}
	byApp := map[model.ApplicationId]map[string]*timeseries.TimeSeries{}
	for _, mv := range values {
		appId, err := model.NewApplicationIdFromString(mv.Labels["application"])
		if err != nil {
			klog.Warningln(err)
			continue
		}
		status := mv.Labels["status"]
		if byApp[appId] == nil {
			byApp[appId] = map[string]*timeseries.TimeSeries{}
		}
		byApp[appId][status] = mv.Values
	}
	res := map[model.ApplicationId]availabilitySlis{}
	for appId, byStatus := range byApp {
		total := timeseries.NewAggregate(timeseries.NanSum)
		failed := timeseries.NewAggregate(timeseries.NanSum)
		for status, ts := range byStatus {
			total.Add(ts)
			if model.IsRequestStatusFailed(status) {
				failed.Add(ts)
			}
		}
		res[appId] = availabilitySlis{total: total.Get(), failed: failed.Get()}
	}
	return res
}

func builtinLatency(values []model.MetricValues) map[model.ApplicationId][]model.HistogramBucket {
	if len(values) == 0 {
		return nil
	}

	byApp := map[model.ApplicationId][]model.MetricValues{}
	for _, mv := range values {
		appId, err := model.NewApplicationIdFromString(mv.Labels["application"])
		if err != nil {
			klog.Warningln(err)
			continue
		}
		byApp[appId] = append(byApp[appId], mv)
	}

	res := map[model.ApplicationId][]model.HistogramBucket{}
	for appId, mvs := range byApp {
		res[appId] = histogramBuckets(mvs)
	}
	return res
}

func histogramBuckets(values []model.MetricValues) []model.HistogramBucket {
	buckets := make([]model.HistogramBucket, 0, len(values))
	for _, m := range values {
		le, err := strconv.ParseFloat(m.Labels["le"], 64)
		if err != nil {
			klog.Warningln(err)
			continue
		}
		buckets = append(buckets, model.HistogramBucket{Le: float32(le), TimeSeries: m.Values})
	}
	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].Le < buckets[j].Le
	})
	return buckets
}
