package auditor

import (
	"fmt"
	"strings"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

func (a *appAuditor) slo() {
	report := a.addReport(model.AuditReportSLO)
	sloRequestsChart(a.app, report, a.clickHouseEnabled)
	availability(a.w, a.app, report)
	latency(a.w, a.app, report)
	clientRequests(a.app, report)
}

func availability(w *model.World, app *model.Application, report *model.AuditReport) {
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
	check.SetStatus(model.OK, "OK")
	if last := lastIncident(app); last != nil && (!last.Resolved() || last.ResolvedAt.After(w.Ctx.To)) {
		for _, br := range last.Details.AvailabilityBurnRates {
			if br.Severity > model.OK {
				check.SetStatus(br.Severity, br.FormatSLOStatus())
				break
			}
		}
	}
}

func latency(w *model.World, app *model.Application, report *model.AuditReport) {
	check := report.CreateCheck(model.Checks.SLOLatency)
	if len(app.LatencySLIs) == 0 {
		check.SetStatus(model.UNKNOWN, "not configured")
		return
	}
	sli := app.LatencySLIs[0]

	totalRaw, _ := sli.GetTotalAndFast(false)
	if totalRaw.TailIsEmpty() {
		check.SetStatus(model.UNKNOWN, "no data")
		return
	}
	check.SetStatus(model.OK, "OK")
	if last := lastIncident(app); last != nil && (!last.Resolved() || last.ResolvedAt.After(w.Ctx.To)) {
		for _, br := range last.Details.LatencyBurnRates {
			if br.Severity > model.OK {
				check.SetStatus(br.Severity, br.FormatSLOStatus())
				break
			}
		}
	}
}

func lastIncident(app *model.Application) *model.ApplicationIncident {
	if len(app.Incidents) == 0 {
		return nil
	}
	return app.Incidents[len(app.Incidents)-1]
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

func clientRequests(app *model.Application, report *model.AuditReport) {
	table := report.GetOrCreateTable("Client", "", "Requests", "Latency", "Errors")
	if table == nil {
		return
	}

	clientsByName := map[string]int{}
	for id := range app.Downstreams {
		clientsByName[id.Name]++
	}

	for id, connection := range app.Downstreams {
		protocols := utils.NewStringSet()
		for proto := range connection.RequestsCount {
			protocols.Add(string(proto))
		}
		rps := connection.GetConnectionsRequestsSum(nil)
		client := model.NewTableCell(id.Name)
		if clientsByName[id.Name] > 1 {
			client.Value += " (ns: " + id.Namespace + ")"
		}
		client.Link = model.NewRouterLink(id.Name, "overview").
			SetParam("view", "applications").
			SetParam("id", id)
		client.SetUnit(strings.Join(protocols.Items(), ", "))

		chart := model.NewTableCell().SetChart(rps)

		requests := model.NewTableCell().SetUnit("/s")
		if last := rps.Last(); last > 0 {
			requests.SetValue(utils.FormatFloat(last))
		}

		latency := model.NewTableCell().SetUnit("ms")
		if last := connection.GetConnectionsRequestsLatency(nil).Last(); last > 0 {
			latency.SetValue(utils.FormatFloat(last * 1000))
		}
		errors := model.NewTableCell().SetUnit("/s")
		if last := connection.GetConnectionsErrorsSum(nil).Last(); last > 0 {
			errors.SetValue(utils.FormatFloat(last))
		}
		table.AddRow(client, chart, requests, latency, errors)
	}
}
