package auditor

import (
	"fmt"
	"strings"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

type sumFromFunc func(from timeseries.Time) float32

func (a *appAuditor) slo() {
	report := a.addReport(model.AuditReportSLO)
	sloRequestsChart(a.app, report, a.clickHouseEnabled)
	availability(a.w.Ctx, a.app, report)
	latency(a.w.Ctx, a.app, report)
	clientRequests(a.app, report)
}

func availability(ctx timeseries.Context, app *model.Application, report *model.AuditReport) {
	check := report.CreateCheck(model.Checks.SLOAvailability)
	if len(app.AvailabilitySLIs) == 0 {
		check.SetStatus(model.UNKNOWN, "not configured")
		return
	}
	sli := app.AvailabilitySLIs[0]
	if sli.TotalRequestsRaw.TailIsEmpty() {
		check.SetStatus(model.UNKNOWN, "no data")
		return
	}

	if ch := report.GetOrCreateChart("Errors, per second", nil); ch != nil {
		ch.AddSeries("errors", sli.FailedRequests.Map(timeseries.NanToZero), "black").Stacked()
	}

	totalF := totalSum(sli.TotalRequestsRaw)
	failedF := func(from timeseries.Time) float32 {
		return 0
	}
	if !sli.FailedRequestsRaw.IsEmpty() {
		failedF = func(from timeseries.Time) float32 {
			iter := sli.FailedRequestsRaw.IterFrom(from)
			var sum float32
			for iter.Next() {
				_, v := iter.Value()
				if timeseries.IsNaN(v) {
					continue
				}
				sum += v
			}
			return sum
		}
	}
	if br := calcBurnRates(ctx.To, failedF, totalF, sli.Config.ObjectivePercentage); br.Severity > model.UNKNOWN {
		check.SetStatus(br.Severity, br.FormatSLOStatus())
	}
}

func latency(ctx timeseries.Context, app *model.Application, report *model.AuditReport) {
	check := report.CreateCheck(model.Checks.SLOLatency)
	if len(app.LatencySLIs) == 0 {
		check.SetStatus(model.UNKNOWN, "not configured")
		return
	}
	sli := app.LatencySLIs[0]

	if ch := report.GetOrCreateChart("Latency, seconds", nil); ch != nil {
		ch.PercentilesFrom(sli.Histogram, 0.25, 0.5, 0.75, 0.95, 0.99)
	}

	totalRaw, fastRaw := sli.GetTotalAndFast(true)
	if totalRaw.TailIsEmpty() {
		check.SetStatus(model.UNKNOWN, "no data")
		return
	}

	totalF := totalSum(totalRaw)
	slowF := totalF
	if !fastRaw.IsEmpty() {
		slowF = func(from timeseries.Time) float32 {
			totalIter := totalRaw.IterFrom(from)
			fastIter := fastRaw.IterFrom(from)
			var sum float32
			for totalIter.Next() && fastIter.Next() {
				_, total := totalIter.Value()
				if timeseries.IsNaN(total) {
					continue
				}
				_, fast := fastIter.Value()
				if timeseries.IsNaN(fast) {
					sum += total
				} else {
					sum += total - fast
				}
			}
			return sum
		}
	}
	if br := calcBurnRates(ctx.To, slowF, totalF, sli.Config.ObjectivePercentage); br.Severity > model.UNKNOWN {
		check.SetStatus(br.Severity, br.FormatSLOStatus())
	}
}

func totalSum(ts *timeseries.TimeSeries) sumFromFunc {
	return func(from timeseries.Time) float32 {
		iter := ts.IterFrom(from)
		var sum float32
		var count, countDefined int
		for iter.Next() {
			_, v := iter.Value()
			count++
			if timeseries.IsNaN(v) {
				continue
			}
			sum += v
			countDefined++
		}
		if float32(countDefined)/float32(count) < 0.5 {
			return timeseries.NaN
		}
		return sum
	}
}

func calcBurnRates(now timeseries.Time, badSum, totalSum sumFromFunc, objectivePercentage float32) model.BurnRate {
	objective := 1 - objectivePercentage/100
	first := model.BurnRate{}
	for _, r := range model.AlertRules {
		from := now.Add(-r.LongWindow)
		br := badSum(from) / totalSum(from) / objective
		if timeseries.IsNaN(br) {
			br = 0
		}
		if first.Window == 0 {
			first.Window = r.LongWindow
			first.Value = br
		}
		if br < r.BurnRateThreshold {
			continue
		}
		from = now.Add(-r.ShortWindow)
		br = badSum(from) / totalSum(from) / objective
		if timeseries.IsNaN(br) {
			br = 0
		}
		if br < r.BurnRateThreshold {
			continue
		}
		return model.BurnRate{Value: br, Window: r.LongWindow, Severity: r.Severity}
	}
	first.Severity = model.OK
	return first
}

func sloRequestsChart(app *model.Application, report *model.AuditReport, clickHouseEnabled bool) {
	hm := report.GetOrCreateHeatmap("Latency & Errors heatmap, requests per second")
	if hm == nil {
		return
	}
	ch := report.
		GetOrCreateChart(fmt.Sprintf("Requests to the <var>%s</var> app, per second", app.Id.Name), nil).
		Sorted().
		Stacked()
	if len(app.LatencySLIs) > 0 {
		sli := app.LatencySLIs[0]
		if len(sli.Histogram) > 0 {
			for _, h := range model.HistogramSeries(sli.Histogram, sli.Config.ObjectiveBucket, sli.Config.ObjectivePercentage) {
				ch.AddSeries(h.Title, h.Data, h.Color)
				hm.AddSeries(h.Name, h.Title, h.Data, h.Threshold, h.Value)
			}
		}
	}
	if len(app.AvailabilitySLIs) > 0 {
		sli := app.AvailabilitySLIs[0]
		ch.SetThreshold("total", sli.TotalRequests)
		if ch.Threshold != nil {
			ch.Threshold.Fill = true
			ch.Threshold.Color = "grey-lighten1"
		}

		ch.AddSeries("errors", sli.FailedRequests, "black")
		if !hm.IsEmpty() {
			failed := sli.FailedRequests
			if failed.IsEmpty() {
				failed = sli.TotalRequests.WithNewValue(0)
			}
			hm.AddSeries("errors", "errors", failed, "", "err")
		}
	}
	if clickHouseEnabled {
		hm.DrillDownLink = model.NewRouterLink("tracing", "overview").
			SetParam("view", "applications").
			SetParam("id", app.Id).
			SetParam("report", model.AuditReportTracing)
	}
}

type clientRequestsSummary struct {
	protocols   *utils.StringSet
	connections []*model.Connection
	rps         *timeseries.TimeSeries
}

func clientRequests(app *model.Application, report *model.AuditReport) {
	table := report.GetOrCreateTable("Client", "", "Requests", "Latency", "Errors")
	if table == nil {
		return
	}
	clients := map[model.ApplicationId]*clientRequestsSummary{}
	clientsByName := map[string]int{}
	for id, connections := range app.GetClientsConnections() {
		clients[id] = &clientRequestsSummary{connections: connections, protocols: utils.NewStringSet()}
		clientsByName[id.Name]++
		for _, c := range connections {
			for protocol := range c.RequestsCount {
				clients[id].protocols.Add(string(protocol))
			}
		}
	}
	if len(clients) == 0 {
		return
	}
	var rpsTotal float32
	for _, s := range clients {
		s.rps = model.GetConnectionsRequestsSum(s.connections, nil)
		if last := s.rps.Last(); !timeseries.IsNaN(last) {
			rpsTotal += last
		}
	}
	for id, s := range clients {
		client := model.NewTableCell(id.Name)
		if clientsByName[id.Name] > 1 {
			client.Value += " (ns: " + id.Namespace + ")"
		}
		client.Link = model.NewRouterLink(id.Name, "overview").
			SetParam("view", "applications").
			SetParam("id", id)
		client.SetUnit(strings.Join(s.protocols.Items(), ", "))

		chart := model.NewTableCell().SetChart(s.rps)

		requests := model.NewTableCell().SetUnit("/s")
		if last := s.rps.Last(); last > 0 {
			requests.SetValue(utils.FormatFloat(last))
		}

		latency := model.NewTableCell().SetUnit("ms")
		if last := model.GetConnectionsRequestsLatency(s.connections, nil).Last(); last > 0 {
			latency.SetValue(utils.FormatFloat(last * 1000))
		}

		errors := model.NewTableCell().SetUnit("/s")
		if last := model.GetConnectionsErrorsSum(s.connections, nil).Last(); last > 0 {
			errors.SetValue(utils.FormatFloat(last))
		}

		table.AddRow(client, chart, requests, latency, errors)
	}
}
