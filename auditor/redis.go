package auditor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func (a *appAuditor) redis() {
	report := model.NewAuditReport(a.w.Ctx, "Redis")

	for _, i := range a.app.Instances {
		if i.Redis == nil {
			continue
		}
		status := model.NewTableCell().SetStatus(model.OK, "up")
		if !(i.Redis.Up != nil && i.Redis.Up.Last() > 0) {
			status.SetStatus(model.WARNING, "down (no metrics)")
		}
		roleCell := model.NewTableCell(i.Redis.Role.Value())
		switch i.Redis.Role.Value() {
		case "master":
			roleCell.SetIcon("mdi-database-edit-outline", "rgba(0,0,0,0.87)")
		case "slave":
			roleCell.SetIcon("mdi-database-import-outline", "grey")
		}

		report.GetOrCreateTable("Instance", "Role", "Status").AddRow(
			model.NewTableCell(i.Name).AddTag("version: %s", i.Redis.Version.Value()),
			roleCell,
			status,
		)

		total := timeseries.Aggregate(timeseries.NanSum)
		calls := timeseries.Aggregate(timeseries.NanSum)
		for cmd, t := range i.Redis.CallsTime {
			if c, ok := i.Redis.Calls[cmd]; ok {
				total.AddInput(t)
				calls.AddInput(c)
			}
		}
		report.
			GetOrCreateChart("Redis latency, seconds").
			AddSeries(i.Name, timeseries.Aggregate(timeseries.Div, total, calls))
		report.
			GetOrCreateChartInGroup("Redis queries on <selector>, per seconds", i.Name).
			Stacked().
			Sorted().
			AddMany(timeseries.Top(i.Redis.Calls, timeseries.NanSum, 5))

	}
	a.addReport(report)
}
