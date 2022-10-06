package auditor

import (
	"fmt"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
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
	totalRequests := timeseries.Reduce(timeseries.NanSum, timeseries.Map(last3points, sli.TotalRequests)) * 60
	totalFailedRequests := timeseries.Reduce(timeseries.NanSum, timeseries.Map(last3points, sli.FailedRequests)) * 60
	if math.IsNaN(totalFailedRequests) {
		totalFailedRequests = 0
	}
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

	fastPercentage := timeseries.Aggregate(
		func(t timeseries.Time, total, fast float64) float64 {
			return fast / total * 100
		},
		sli.TotalRequests, sli.FastRequests,
	)
	b := fmt.Sprintf(`%ss`, sli.Config.ObjectiveBucket)
	if v, err := strconv.ParseFloat(sli.Config.ObjectiveBucket, 64); err == nil && v < 1 {
		b = fmt.Sprintf(`%.fms`, v*1000)
	}
	chart := report.
		GetOrCreateChart("Latency").
		AddSeries("requests served in < "+b, fastPercentage)
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
	totalRequests := timeseries.Reduce(timeseries.NanSum, timeseries.Map(last3points, sli.TotalRequests)) * 60
	totalFastRequests := timeseries.Reduce(timeseries.NanSum, timeseries.Map(last3points, sli.FastRequests)) * 60

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
