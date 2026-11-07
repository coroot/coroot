package constructor

import (
	"sort"
	"strconv"
	"strings"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
)

func (c *Constructor) loadSLIs(w *model.World, metrics map[string][]*model.MetricValues, project *db.Project) {
	inboundAvailabilityAgg := inboundAvailability(w, metrics[qRecordingRuleApplicationL7InboundRequests+"_avg"], project)
	inboundAvailabilityRaw := inboundAvailability(w, metrics[qRecordingRuleApplicationL7InboundRequests+"_raw"], project)
	inboundLatencyAgg := inboundLatency(w, metrics[qRecordingRuleApplicationL7InboundHistogram+"_avg"], project)
	inboundLatencyRaw := inboundLatency(w, metrics[qRecordingRuleApplicationL7InboundHistogram+"_raw"], project)
	builtinAvailabilityAgg := builtinAvailability(w, metrics[qRecordingRuleApplicationL7Requests+"_avg"], project)
	builtinAvailabilityRaw := builtinAvailability(w, metrics[qRecordingRuleApplicationL7Requests+"_raw"], project)
	builtinLatencyAgg := builtinLatency(w, metrics[qRecordingRuleApplicationL7Histogram+"_avg"], project)
	builtinLatencyRaw := builtinLatency(w, metrics[qRecordingRuleApplicationL7Histogram+"_raw"], project)
	llmLatencyAgg := inboundLatency(w, metrics[qRecordingRuleApplicationLLMInboundHistogram+"_avg"], project)
	llmLatencyRaw := inboundLatency(w, metrics[qRecordingRuleApplicationLLMInboundHistogram+"_raw"], project)

	customAvailabilityAgg := map[model.ApplicationId]availabilitySlis{}
	customAvailabilityRaw := map[model.ApplicationId]availabilitySlis{}
	customLatencyAgg := map[model.ApplicationId][]model.HistogramBucket{}
	customLatencyRaw := map[model.ApplicationId][]model.HistogramBucket{}
	loadCustomSLIs(metrics, customAvailabilityAgg, customLatencyAgg, project, "_avg")
	loadCustomSLIs(metrics, customAvailabilityRaw, customLatencyRaw, project, "_raw")

	for _, app := range w.Applications {
		availabilityCfg, _ := w.CheckConfigs.GetAvailability(app.Id)
		if availabilityCfg.Custom {
			agg := customAvailabilityAgg[app.Id]
			raw := customAvailabilityRaw[app.Id]
			if agg.failed.IsEmpty() {
				agg.failed = agg.total.WithNewValue(timeseries.NaN)
			}
			app.AvailabilitySLIs = append(app.AvailabilitySLIs, &model.AvailabilitySLI{
				Config:        availabilityCfg,
				TotalRequests: agg.total, TotalRequestsRaw: raw.total,
				FailedRequests: agg.failed, FailedRequestsRaw: raw.failed,
			})
		} else {
			agg, ok := inboundAvailabilityAgg[app.Id]
			raw := inboundAvailabilityRaw[app.Id]
			if !ok || agg.total.IsEmpty() {
				agg = builtinAvailabilityAgg[app.Id]
				raw = builtinAvailabilityRaw[app.Id]
			}
			if agg.failed.IsEmpty() {
				agg.failed = agg.total.WithNewValue(timeseries.NaN)
			}
			if !agg.total.IsEmpty() {
				app.AvailabilitySLIs = append(app.AvailabilitySLIs, &model.AvailabilitySLI{
					Config:        availabilityCfg,
					TotalRequests: agg.total, TotalRequestsRaw: raw.total,
					FailedRequests: agg.failed, FailedRequestsRaw: raw.failed,
				})
			}
		}

		latencyCfg, _ := w.CheckConfigs.GetLatency(app.Id, app.Category)
		if latencyCfg.Custom {
			app.LatencySLIs = append(app.LatencySLIs, &model.LatencySLI{
				Config:    latencyCfg,
				Histogram: customLatencyAgg[app.Id], HistogramRaw: customLatencyRaw[app.Id],
			})
		} else {
			agg := inboundLatencyAgg[app.Id]
			raw := inboundLatencyRaw[app.Id]
			if len(agg) == 0 {
				agg = builtinLatencyAgg[app.Id]
				raw = builtinLatencyRaw[app.Id]
			}
			if len(agg) > 0 {
				app.LatencySLIs = append(app.LatencySLIs, &model.LatencySLI{
					Config:    latencyCfg,
					Histogram: agg, HistogramRaw: raw,
				})
			}
		}
		if agg := llmLatencyAgg[app.Id]; len(agg) > 0 {
			cfg, _ := w.CheckConfigs.GetLLMLatency(app.Id)
			app.LLMLatencySLI = &model.LatencySLI{Config: cfg, Histogram: agg, HistogramRaw: llmLatencyRaw[app.Id]}
		}
	}
}

func inboundAvailability(w *model.World, values []*model.MetricValues, project *db.Project) map[model.ApplicationId]availabilitySlis {
	if len(values) == 0 {
		return nil
	}
	byApp := map[model.ApplicationId]map[string]*timeseries.TimeSeries{}
	for _, mv := range values {
		appId, err := model.NewApplicationIdFromString(mv.Labels["app"], project.ClusterId())
		if err != nil {
			klog.Warningln(err)
			continue
		}
		if w.GetApplication(appId) == nil {
			continue
		}
		status := mv.Labels["status"]
		if byApp[appId] == nil {
			byApp[appId] = map[string]*timeseries.TimeSeries{}
		}
		byApp[appId][status] = merge(byApp[appId][status], mv.Values, timeseries.NanSumOrNaN)
	}
	res := map[model.ApplicationId]availabilitySlis{}
	for appId, byStatus := range byApp {
		total := timeseries.NewAggregate(timeseries.NanSumOrNaN)
		failed := timeseries.NewAggregate(timeseries.NanSumOrNaN)
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

func inboundLatency(w *model.World, values []*model.MetricValues, project *db.Project) map[model.ApplicationId][]model.HistogramBucket {
	if len(values) == 0 {
		return nil
	}
	byApp := map[model.ApplicationId]map[string]*timeseries.TimeSeries{}
	for _, mv := range values {
		appId, err := model.NewApplicationIdFromString(mv.Labels["app"], project.ClusterId())
		if err != nil {
			klog.Warningln(err)
			continue
		}
		if w.GetApplication(appId) == nil {
			continue
		}
		buckets := byApp[appId]
		if buckets == nil {
			buckets = map[string]*timeseries.TimeSeries{}
			byApp[appId] = buckets
		}
		leStr := mv.Labels["le"]
		buckets[leStr] = merge(buckets[leStr], mv.Values, timeseries.NanSum)
	}
	res := map[model.ApplicationId][]model.HistogramBucket{}
	for appId, buckets := range byApp {
		res[appId] = histogramBuckets(buckets)
	}
	return res
}

func loadCustomSLIs(metrics map[string][]*model.MetricValues,
	availabilityRaw map[model.ApplicationId]availabilitySlis,
	latencyRaw map[model.ApplicationId][]model.HistogramBucket,
	project *db.Project,
	suffix string,
) {
	for queryName, values := range metrics {
		if len(values) == 0 || !strings.HasPrefix(queryName, qApplicationCustomSLI) {
			continue
		}
		parts := strings.Split(queryName, "/")
		if len(parts) != 3 {
			continue
		}
		appId, _ := model.NewApplicationIdFromString(parts[1], project.ClusterId())
		switch parts[2] {
		case "total_requests" + suffix:
			a := availabilityRaw[appId]
			a.total = values[0].Values
			availabilityRaw[appId] = a
		case "failed_requests" + suffix:
			a := availabilityRaw[appId]
			a.failed = values[0].Values
			availabilityRaw[appId] = a
		case "requests_histogram" + suffix:
			buckets := map[string]*timeseries.TimeSeries{}
			for _, mv := range values {
				leStr := mv.Labels["le"]
				buckets[leStr] = mv.Values
			}
			latencyRaw[appId] = histogramBuckets(buckets)
		}
	}
}

type availabilitySlis struct {
	total  *timeseries.TimeSeries
	failed *timeseries.TimeSeries
}

func builtinAvailability(w *model.World, values []*model.MetricValues, project *db.Project) map[model.ApplicationId]availabilitySlis {
	if len(values) == 0 {
		return nil
	}
	byApp := map[model.ApplicationId]map[string]*timeseries.TimeSeries{}
	for _, mv := range values {
		dstId, err := model.NewApplicationIdFromString(mv.Labels["dest"], project.ClusterId())
		if err != nil {
			klog.Warningln(err)
			continue
		}
		srcId, err := model.NewApplicationIdFromString(mv.Labels["app"], project.ClusterId())
		if err != nil {
			klog.Warningln(err)
			continue
		}
		status := mv.Labels["status"]
		src := w.GetApplication(srcId)
		dst := w.GetApplication(dstId)
		if src == nil || dst == nil {
			continue
		}
		if !dst.Category.Auxiliary() && src.Category.Auxiliary() {
			continue
		}
		if byApp[dstId] == nil {
			byApp[dstId] = map[string]*timeseries.TimeSeries{}
		}
		byApp[dstId][status] = merge(byApp[dstId][status], mv.Values, timeseries.NanSumOrNaN)
	}
	res := map[model.ApplicationId]availabilitySlis{}
	for appId, byStatus := range byApp {
		total := timeseries.NewAggregate(timeseries.NanSumOrNaN)
		failed := timeseries.NewAggregate(timeseries.NanSumOrNaN)
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

func builtinLatency(w *model.World, values []*model.MetricValues, project *db.Project) map[model.ApplicationId][]model.HistogramBucket {
	if len(values) == 0 {
		return nil
	}

	byApp := map[model.ApplicationId]map[string]*timeseries.TimeSeries{}
	for _, mv := range values {
		appId, err := model.NewApplicationIdFromString(mv.Labels["app"], project.ClusterId())
		if err != nil {
			klog.Warningln(err)
			continue
		}
		app := w.GetApplication(appId)
		if app == nil {
			continue
		}
		buckets := byApp[appId]
		if buckets == nil {
			buckets = map[string]*timeseries.TimeSeries{}
			byApp[appId] = buckets
		}
		leStr := mv.Labels["le"]
		buckets[leStr] = merge(buckets[leStr], mv.Values, timeseries.NanSum)
	}
	res := map[model.ApplicationId][]model.HistogramBucket{}
	for appId, buckets := range byApp {
		res[appId] = histogramBuckets(buckets)
	}
	return res
}

func histogramBuckets(values map[string]*timeseries.TimeSeries) []model.HistogramBucket {
	buckets := make([]model.HistogramBucket, 0, len(values))
	for leStr, ts := range values {
		le, err := strconv.ParseFloat(leStr, 64)
		if err != nil {
			klog.Warningln(err)
			continue
		}
		buckets = append(buckets, model.HistogramBucket{Le: float32(le), TimeSeries: ts})
	}
	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].Le < buckets[j].Le
	})
	return buckets
}
