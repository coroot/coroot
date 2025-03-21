package constructor

import (
	"sort"
	"strconv"
	"strings"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
)

func (c *Constructor) loadSLIs(w *model.World, metrics map[string][]*model.MetricValues) {
	builtinAvailabilityRaw := builtinAvailability(w, metrics[qRecordingRuleApplicationL7Requests+"_raw"])
	builtinLatencyRaw := builtinLatency(w, metrics[qRecordingRuleApplicationL7Histogram+"_raw"])

	customAvailabilityRaw := map[model.ApplicationId]availabilitySlis{}
	customLatencyRaw := map[model.ApplicationId][]model.HistogramBucket{}
	loadCustomSLIs(metrics, customAvailabilityRaw, customLatencyRaw)

	for _, app := range w.Applications {
		availabilityCfg, _ := w.CheckConfigs.GetAvailability(app.Id)
		if availabilityCfg.Custom {
			raw := customAvailabilityRaw[app.Id]
			app.AvailabilitySLIs = append(app.AvailabilitySLIs, &model.AvailabilitySLI{
				Config:        availabilityCfg,
				TotalRequests: aggregateSLI(w.Ctx, raw.total), TotalRequestsRaw: raw.total,
				FailedRequests: aggregateSLI(w.Ctx, raw.failed), FailedRequestsRaw: raw.failed,
			})
		} else {
			raw := builtinAvailabilityRaw[app.Id]
			if !raw.total.IsEmpty() {
				app.AvailabilitySLIs = append(app.AvailabilitySLIs, &model.AvailabilitySLI{
					Config:        availabilityCfg,
					TotalRequests: aggregateSLI(w.Ctx, raw.total), TotalRequestsRaw: raw.total,
					FailedRequests: aggregateSLI(w.Ctx, raw.failed), FailedRequestsRaw: raw.failed,
				})
			}
		}

		latencyCfg, _ := w.CheckConfigs.GetLatency(app.Id, app.Category)
		if latencyCfg.Custom {
			raw := customLatencyRaw[app.Id]
			app.LatencySLIs = append(app.LatencySLIs, &model.LatencySLI{
				Config:    latencyCfg,
				Histogram: aggregateHistogram(w.Ctx, raw), HistogramRaw: raw,
			})
		} else {
			raw := builtinLatencyRaw[app.Id]
			if len(raw) > 0 {
				app.LatencySLIs = append(app.LatencySLIs, &model.LatencySLI{
					Config:    latencyCfg,
					Histogram: aggregateHistogram(w.Ctx, raw), HistogramRaw: raw,
				})
			}
		}
	}
}

func loadCustomSLIs(metrics map[string][]*model.MetricValues,
	availabilityRaw map[model.ApplicationId]availabilitySlis,
	latencyRaw map[model.ApplicationId][]model.HistogramBucket,
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
		case "total_requests_raw":
			a := availabilityRaw[appId]
			a.total = values[0].Values
			availabilityRaw[appId] = a
		case "failed_requests_raw":
			a := availabilityRaw[appId]
			a.failed = values[0].Values
			availabilityRaw[appId] = a
		case "requests_histogram_raw":
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

func builtinAvailability(w *model.World, values []*model.MetricValues) map[model.ApplicationId]availabilitySlis {
	if len(values) == 0 {
		return nil
	}
	byApp := map[model.ApplicationId]map[string]*timeseries.TimeSeries{}
	for _, mv := range values {
		dstId, err := model.NewApplicationIdFromString(mv.Labels["dest"])
		if err != nil {
			klog.Warningln(err)
			continue
		}
		srcId, err := model.NewApplicationIdFromString(mv.Labels["app"])
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
		byApp[dstId][status] = merge(byApp[dstId][status], mv.Values, timeseries.NanSum)
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

func builtinLatency(w *model.World, values []*model.MetricValues) map[model.ApplicationId][]model.HistogramBucket {
	if len(values) == 0 {
		return nil
	}

	byApp := map[model.ApplicationId]map[string]*timeseries.TimeSeries{}
	for _, mv := range values {
		appId, err := model.NewApplicationIdFromString(mv.Labels["app"])
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

func aggregateHistogram(ctx timeseries.Context, raw []model.HistogramBucket) []model.HistogramBucket {
	res := make([]model.HistogramBucket, 0, len(raw))
	for _, b := range raw {
		res = append(res, model.HistogramBucket{
			Le:         b.Le,
			TimeSeries: aggregateSLI(ctx, b.TimeSeries),
		})
	}
	return res
}

func aggregateSLI(ctx timeseries.Context, raw *timeseries.TimeSeries) *timeseries.TimeSeries {
	from := ctx.From.Truncate(ctx.Step)
	to := ctx.To.Truncate(ctx.Step)
	resPoints := int(to.Sub(from)/ctx.Step + 1)
	res := timeseries.New(from, resPoints, ctx.Step)
	var sum, count float32
	tNext := from
	iter := raw.IterFrom(from)
	for iter.Next() {
		tRaw, vRaw := iter.Value()
		if t := tRaw.Truncate(ctx.Step); t > tNext {
			if count > 0 {
				res.Set(tNext, sum/count)
			}
			sum, count = 0., 0.
			tNext = t
		}
		if !timeseries.IsNaN(vRaw) {
			sum += vRaw
			count++
		}
	}
	if count > 0 {
		res.Set(tNext, sum/count)
	}
	return res
}
