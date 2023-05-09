package auditor

import (
	"fmt"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"strings"
)

func (a *appAuditor) slo() {
	report := a.addReport(model.AuditReportSLO)
	requestsChart(a.app, report, a.p)
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

	if !sli.TotalRequests.IsEmpty() {
		failed := sli.FailedRequests
		if failed.IsEmpty() {
			failed = sli.TotalRequests.WithNewValue(0)
		}
		successfulPercentage := timeseries.Aggregate2(
			sli.TotalRequests, failed.Map(timeseries.NanToZero),
			func(total, failed float32) float32 {
				return (total - failed) / total * 100
			},
		)
		chart := report.
			GetOrCreateChart("Availability").
			AddSeries("successful requests", successfulPercentage)
		chart.Threshold = &model.Series{
			Name:  "target",
			Color: "red",
			Fill:  true,
			Data:  sli.TotalRequests.WithNewValue(sli.Config.ObjectivePercentage),
		}
	}

	if sli.TotalRequestsRaw.IsEmpty() {
		check.SetStatus(model.UNKNOWN, "no data")
		return
	}
	if model.DataIsMissing(sli.TotalRequestsRaw) {
		check.SetStatus(model.WARNING, "no data")
		return
	}

	failedRaw := sli.FailedRequestsRaw
	if failedRaw.IsEmpty() {
		failedRaw = sli.TotalRequestsRaw.WithNewValue(0)
	} else {
		failedRaw = failedRaw.Map(timeseries.NanToZero)
	}
	if br := model.CheckBurnRates(ctx.To, failedRaw, sli.TotalRequestsRaw, sli.Config.ObjectivePercentage); br.Severity > model.UNKNOWN {
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

	total, fast := sli.GetTotalAndFast(false)
	if !total.IsEmpty() {
		fastPercentage := timeseries.Aggregate2(
			total, fast,
			func(total, fast float32) float32 {
				return fast / total * 100
			},
		)
		chart := report.
			GetOrCreateChart("Latency").
			AddSeries("requests served faster than "+utils.FormatLatency(sli.Config.ObjectiveBucket), fastPercentage)
		chart.Threshold = &model.Series{
			Name:  "target",
			Color: "red",
			Fill:  true,
			Data:  total.WithNewValue(sli.Config.ObjectivePercentage),
		}
	}

	totalRaw, fastRaw := sli.GetTotalAndFast(true)
	if totalRaw.IsEmpty() {
		check.SetStatus(model.UNKNOWN, "no data")
		return
	}
	if model.DataIsMissing(totalRaw) {
		check.SetStatus(model.WARNING, "no data")
		return
	}

	if fastRaw.IsEmpty() {
		fastRaw = totalRaw.WithNewValue(0)
	} else {
		fastRaw = fastRaw.Map(timeseries.NanToZero)
	}
	slowRaw := timeseries.Sub(totalRaw, fastRaw)
	if br := model.CheckBurnRates(ctx.To, slowRaw, totalRaw, sli.Config.ObjectivePercentage); br.Severity > model.UNKNOWN {
		check.SetStatus(br.Severity, br.FormatSLOStatus())
	}
}

func requestsChart(app *model.Application, report *model.AuditReport, p *db.Project) {
	title := fmt.Sprintf("Requests to the <var>%s</var> app, per second", app.Id.Name)
	var ch *model.Chart
	var hm *model.Heatmap
	if len(app.LatencySLIs) > 0 {
		sli := app.LatencySLIs[0]
		if len(sli.Histogram) > 0 {
			hm = report.GetOrCreateHeatmap("Latency & Errors heatmap, requests per second")
			ch = report.GetOrCreateChart(title).Sorted().Stacked()
			for _, h := range model.HistogramSeries(sli.Histogram, sli.Config.ObjectiveBucket, sli.Config.ObjectivePercentage) {
				ch.AddSeries(h.Title, h.Data, h.Color)
				hm.AddSeries(h.Name, h.Title, h.Data, h.Threshold, h.Value)
			}
		}
	}
	if len(app.AvailabilitySLIs) > 0 {
		sli := app.AvailabilitySLIs[0]
		if ch == nil {
			ch = report.GetOrCreateChart(title).Sorted().Stacked()
			ch.AddSeries("total", sli.TotalRequests, "grey-lighten1")
		}
		ch.AddSeries("errors", sli.FailedRequests, "black")
		if hm != nil {
			failed := sli.FailedRequests
			if failed.IsEmpty() {
				failed = sli.TotalRequests.WithNewValue(0)
			}
			hm.AddSeries("errors", "errors", failed, "", "err")
		}
	}
	if hm != nil && p.Settings.Integrations.Clickhouse != nil {
		hm.DrillDownLink = model.NewRouterLink("tracing").SetParam("report", model.AuditReportTracing)
	}
}

type clientRequestsSummary struct {
	protocols   *utils.StringSet
	connections []*model.Connection
	rps         *timeseries.TimeSeries
}

func clientRequests(app *model.Application, report *model.AuditReport) {
	clients := map[model.ApplicationId]*clientRequestsSummary{}
	for id, connections := range app.GetClientsConnections() {
		clients[id] = &clientRequestsSummary{connections: connections, protocols: utils.NewStringSet()}
		for _, c := range connections {
			for protocol := range c.RequestsCount {
				clients[id].protocols.Add(string(protocol))
			}
		}
	}
	if len(clients) == 0 {
		return
	}
	t := report.GetOrCreateTable("Client", "", "Requests", "Latency", "Errors")
	var rpsTotal float32
	for _, s := range clients {
		s.rps = model.GetConnectionsRequestsSum(s.connections)
		if last := s.rps.Last(); !timeseries.IsNaN(last) {
			rpsTotal += last
		}
	}
	for id, s := range clients {
		client := model.NewTableCell(id.Name)
		client.Link = model.NewRouterLink(id.Name).SetRoute("application").SetParam("id", id)
		client.SetUnit(strings.Join(s.protocols.Items(), " "))

		chart := model.NewTableCell().SetChart(s.rps)

		requests := model.NewTableCell().SetUnit("/s")
		if last := s.rps.Last(); last > 0 {
			requests.SetValue(utils.FormatFloat(last))
		}

		latency := model.NewTableCell().SetUnit("ms")
		if last := model.GetConnectionsRequestsLatency(s.connections).Last(); last > 0 {
			latency.SetValue(utils.FormatFloat(last * 1000))
		}

		errors := model.NewTableCell().SetUnit("/s")
		if last := model.GetConnectionsErrorsSum(s.connections).Last(); last > 0 {
			errors.SetValue(utils.FormatFloat(last))
		}

		t.AddRow(client, chart, requests, latency, errors)
	}
}
