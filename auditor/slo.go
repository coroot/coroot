package auditor

import (
	"fmt"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/dustin/go-humanize/english"
	"math"
)

func (a *appAuditor) slo() {
	report := a.addReport(model.AuditReportSLO)
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
	if timeseries.IsEmpty(sli.TotalRequests) {
		check.SetStatus(model.WARNING, "no data")
		return
	}

	failed := sli.FailedRequests
	if timeseries.IsEmpty(failed) {
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

	if dataIsMissing(sli.TotalRequestsRaw) {
		check.SetStatus(model.WARNING, "no data")
		return
	}

	failedRaw := sli.FailedRequestsRaw
	if timeseries.IsEmpty(failed) {
		failedRaw = timeseries.Replace(sli.TotalRequestsRaw, 0)
	} else {
		failedRaw = timeseries.Map(timeseries.NanToZero, failedRaw)
	}
	if br := model.CheckBurnRates(ctx.To, failedRaw, sli.TotalRequestsRaw, sli.Config.ObjectivePercentage); br.Severity > model.UNKNOWN {
		check.SetStatus(br.Severity, formatSLOStatus(br))
	}
}

func latency(ctx timeseries.Context, app *model.Application, report *model.AuditReport) {
	check := report.CreateCheck(model.Checks.SLOLatency)
	if len(app.LatencySLIs) == 0 {
		check.SetStatus(model.UNKNOWN, "not configured")
		return
	}
	sli := app.LatencySLIs[0]
	total, fast := sli.GetTotalAndFast(false)
	if timeseries.IsEmpty(total) || timeseries.IsEmpty(fast) {
		check.SetStatus(model.WARNING, "no data")
		return
	}
	fastPercentage := timeseries.Aggregate(
		func(t timeseries.Time, total, fast float64) float64 {
			return fast / total * 100
		},
		total, fast,
	)
	chart := report.
		GetOrCreateChart("Latency").
		AddSeries("requests served faster than "+utils.FormatLatency(sli.Config.ObjectiveBucket), fastPercentage)
	chart.Threshold = &model.Series{
		Name:  "target",
		Color: "red",
		Fill:  true,
		Data:  timeseries.Replace(total, sli.Config.ObjectivePercentage),
	}

	totalRaw, fastRaw := sli.GetTotalAndFast(true)
	if dataIsMissing(totalRaw) {
		check.SetStatus(model.WARNING, "no data")
		return
	}

	if timeseries.IsEmpty(fastRaw) {
		fastRaw = timeseries.Replace(totalRaw, 0)
	} else {
		fastRaw = timeseries.Map(timeseries.NanToZero, fastRaw)
	}
	slowRaw := timeseries.Aggregate(timeseries.Sub, totalRaw, fastRaw)
	if br := model.CheckBurnRates(ctx.To, slowRaw, totalRaw, sli.Config.ObjectivePercentage); br.Severity > model.UNKNOWN {
		check.SetStatus(br.Severity, formatSLOStatus(br))
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

func dataIsMissing(ts timeseries.TimeSeries) bool {
	for _, v := range timeseries.LastN(ts, 3) {
		if !math.IsNaN(v) {
			return false
		}
	}
	return true
}

func formatSLOStatus(br model.BurnRate) string {
	hours := int(br.Window / timeseries.Hour)
	return fmt.Sprintf("error budget burn rate is %.1fx within %s", br.Value, english.Plural(hours, "hour", ""))
}
