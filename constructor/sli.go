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
	rawFrom := to.Add(-model.MaxAlertRuleWindow)
	availabilityFromInboundConnections := loadAvailabilityFromInboundConnections(ctx, prom, from, to, step)
	availabilityFromInboundConnectionsRaw := loadAvailabilityFromInboundConnections(ctx, prom, rawFrom, to, rawStep)
	latencyFromInboundConnections := loadLatencyFromInboundConnections(ctx, prom, from, to, step)
	latencyFromInboundConnectionsRaw := loadLatencyFromInboundConnections(ctx, prom, rawFrom, to, rawStep)

	for _, app := range w.Applications {
		availability, _ := w.CheckConfigs.GetAvailability(app.Id)
		for _, cfg := range availability {
			sli := &model.AvailabilitySLI{Config: cfg}
			app.AvailabilitySLIs = append(app.AvailabilitySLIs, sli)
			if cfg.Custom {
				qTotal, qFailed := cfg.Total(), cfg.Failed()
				sli.TotalRequests = loadAvailabilityFromConfiguredSli(ctx, prom, qTotal, from, to, step)
				sli.TotalRequestsRaw = loadAvailabilityFromConfiguredSli(ctx, prom, qTotal, rawFrom, to, rawStep)
				sli.FailedRequests = loadAvailabilityFromConfiguredSli(ctx, prom, qFailed, from, to, step)
				sli.FailedRequestsRaw = loadAvailabilityFromConfiguredSli(ctx, prom, qFailed, rawFrom, to, rawStep)
			} else {
				sli.TotalRequests = availabilityFromInboundConnections[app.Id].total
				sli.FailedRequests = availabilityFromInboundConnections[app.Id].failed
				sli.TotalRequestsRaw = availabilityFromInboundConnectionsRaw[app.Id].total
				sli.FailedRequestsRaw = availabilityFromInboundConnectionsRaw[app.Id].failed
			}
		}
		latency, _ := w.CheckConfigs.GetLatency(app.Id, app.Category)
		for _, cfg := range latency {
			sli := &model.LatencySLI{Config: cfg}
			app.LatencySLIs = append(app.LatencySLIs, sli)
			if cfg.Custom {
				q := cfg.Histogram()
				sli.Histogram = loadLatencyFromConfiguredSli(ctx, prom, q, from, to, step)
				sli.HistogramRaw = loadLatencyFromConfiguredSli(ctx, prom, q, rawFrom, to, rawStep)
			} else {
				sli.Histogram = latencyFromInboundConnections[app.Id]
				sli.HistogramRaw = latencyFromInboundConnectionsRaw[app.Id]
			}
		}
	}
}

func loadAvailabilityFromConfiguredSli(ctx context.Context, prom prom.Client, query string, from, to timeseries.Time, step timeseries.Duration) *timeseries.TimeSeries {
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

type availabilitySlis struct {
	total  *timeseries.TimeSeries
	failed *timeseries.TimeSeries
}

func loadAvailabilityFromInboundConnections(ctx context.Context, prom prom.Client, from, to timeseries.Time, step timeseries.Duration) map[model.ApplicationId]availabilitySlis {
	values, err := prom.QueryRange(ctx, "rr_application_inbound_requests_total", from, to, step)
	if err != nil {
		klog.Warningln(err)
		return nil
	}
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

func loadLatencyFromConfiguredSli(ctx context.Context, prom prom.Client, query string, from, to timeseries.Time, step timeseries.Duration) []model.HistogramBucket {
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

func loadLatencyFromInboundConnections(ctx context.Context, prom prom.Client, from, to timeseries.Time, step timeseries.Duration) map[model.ApplicationId][]model.HistogramBucket {
	values, err := prom.QueryRange(ctx, "rr_application_inbound_requests_histogram", from, to, step)
	if err != nil {
		klog.Warningln(err)
		return nil
	}
	if len(values) == 0 {
		return nil
	}
	byApp := map[model.ApplicationId]map[float64]*timeseries.TimeSeries{}
	for _, mv := range values {
		appId, err := model.NewApplicationIdFromString(mv.Labels["application"])
		if err != nil {
			klog.Warningln(err)
			continue
		}
		le, err := strconv.ParseFloat(mv.Labels["le"], 64)
		if err != nil {
			klog.Warningln(err)
			continue
		}
		if byApp[appId] == nil {
			byApp[appId] = map[float64]*timeseries.TimeSeries{}
		}
		byApp[appId][le] = mv.Values
	}
	res := map[model.ApplicationId][]model.HistogramBucket{}
	for appId, byLe := range byApp {
		buckets := make([]model.HistogramBucket, 0, len(values))
		for le, ts := range byLe {
			buckets = append(buckets, model.HistogramBucket{Le: le, TimeSeries: ts})
		}
		sort.Slice(buckets, func(i, j int) bool {
			return buckets[i].Le < buckets[j].Le
		})
		res[appId] = buckets
	}
	return res
}
