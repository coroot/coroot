package auditor

import (
	"fmt"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
	"math"
	"strconv"
)

func (a *appAuditor) slo() {
	report := a.addReport("SLO")
	requestsChart(a.app, report)
	availability(a.w.Ctx, a.app, report)
	latency(a.w.Ctx, a.app, report)
}

func availability(ctx timeseries.Context, app *model.Application, report *model.AuditReport) {
	check := report.CreateCheck(model.Checks.SLOAvailability)
	if len(app.AvailabilitySLIs) == 0 {
		check.SetStatus(model.UNKNOWN, "not configured")
		return
	}
	sli := app.AvailabilitySLIs[0]
	if sli.TotalRequests == nil {
		check.SetStatus(model.WARNING, "no data")
	}

	failed := sli.FailedRequests
	if failed == nil {
		failed = timeseries.Replace(sli.TotalRequests, 0)
	}
	successfulPercentage := timeseries.Aggregate(
		func(t timeseries.Time, total, failed float64) float64 {
			return (total - failed) / total * 100
		},
		sli.TotalRequests, timeseries.Map(timeseries.NanToZero, failed),
	)
	chart := report.
		GetOrCreateChart("Availability").
		AddSeries("successful requests", successfulPercentage)
	chart.Threshold = &model.Series{
		Name:  "target",
		Color: "red",
		Fill:  true,
		Data:  timeseries.Replace(sli.TotalRequests, sli.Config.ObjectivePercentage),
	}
	last3points := func(t timeseries.Time, v float64) float64 {
		if t >= ctx.To.Add(-3*ctx.Step) {
			return v
		}
		return 0
	}
	last3pointsDefined := func(t timeseries.Time, v float64) float64 {
		if t >= ctx.To.Add(-3*ctx.Step) && !math.IsNaN(v) {
			return 1
		}
		return 0
	}
	if timeseries.Reduce(timeseries.NanSum, timeseries.Map(last3pointsDefined, sli.TotalRequests)) < 1 {
		check.SetStatus(model.WARNING, "no data")
		return
	}
	totalRequests := timeseries.Reduce(timeseries.NanSum, timeseries.Map(last3points, sli.TotalRequests)) * 60
	totalFailedRequests := timeseries.Reduce(timeseries.NanSum, timeseries.Map(last3points, sli.FailedRequests)) * 60

	if (totalRequests-totalFailedRequests)/totalRequests*100 < sli.Config.ObjectivePercentage {
		check.Fire()
	}
}

func latency(ctx timeseries.Context, app *model.Application, report *model.AuditReport) {
	check := report.CreateCheck(model.Checks.SLOLatency)
	if len(app.LatencySLIs) == 0 {
		check.SetStatus(model.UNKNOWN, "not configured")
		return
	}
	sli := app.LatencySLIs[0]
	objectiveBucket, err := strconv.ParseFloat(sli.Config.ObjectiveBucket, 64)
	if err != nil {
		klog.Warningf(`invalid objective bucket "%s": %s`, sli.Config.ObjectiveBucket, err)
		return
	}
	var total, fast timeseries.TimeSeries
	for _, b := range histogramBuckets(sli.Histogram) {
		if b.Le <= objectiveBucket {
			fast = b.TimeSeries
		}
		if math.IsInf(b.Le, 1) {
			total = b.TimeSeries
		}
	}
	if total == nil || fast == nil {
		check.SetStatus(model.WARNING, "no data")
	}
	fastPercentage := timeseries.Aggregate(
		func(t timeseries.Time, total, fast float64) float64 {
			return fast / total * 100
		},
		total, fast,
	)
	chart := report.
		GetOrCreateChart("Latency").
		AddSeries("requests served faster than "+model.FormatLatencyBucket(sli.Config.ObjectiveBucket), fastPercentage)
	chart.Threshold = &model.Series{
		Name:  "target",
		Color: "red",
		Fill:  true,
		Data:  timeseries.Replace(total, sli.Config.ObjectivePercentage),
	}
	last3points := func(t timeseries.Time, v float64) float64 {
		if t >= ctx.To.Add(-3*ctx.Step) {
			return v
		}
		return 0
	}
	last3pointsDefined := func(t timeseries.Time, v float64) float64 {
		if t >= ctx.To.Add(-3*ctx.Step) && !math.IsNaN(v) {
			return 1
		}
		return 0
	}

	if timeseries.Reduce(timeseries.NanSum, timeseries.Map(last3pointsDefined, total)) < 1 {
		check.SetStatus(model.WARNING, "no data")
		return
	}

	totalRequests := timeseries.Reduce(timeseries.NanSum, timeseries.Map(last3points, total)) * 60
	totalFastRequests := timeseries.Reduce(timeseries.NanSum, timeseries.Map(last3points, fast)) * 60
	if totalFastRequests/totalRequests*100 < sli.Config.ObjectivePercentage {
		check.Fire()
	}
}

func requestsChart(app *model.Application, report *model.AuditReport) {
	ch := report.GetOrCreateChart(fmt.Sprintf("Requests to the <var>%s</var> app, per second", app.Id.Name)).Sorted().Stacked()
	if len(app.LatencySLIs) > 0 {
		sli := app.LatencySLIs[0]
		if hist := sli.Histogram; len(hist) > 0 {
			for _, s := range histogramSeries(sli.Histogram, sli.Config.ObjectiveBucket) {
				ch.Series = append(ch.Series, s)
			}
		}
	}
	if len(app.AvailabilitySLIs) > 0 {
		if len(ch.Series) == 0 {
			ch.AddSeries("total", app.AvailabilitySLIs[0].TotalRequests, "grey-lighten1")
		}
		ch.AddSeries("errors", app.AvailabilitySLIs[0].FailedRequests, "black")
	}
}
