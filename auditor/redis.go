package auditor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/dustin/go-humanize/english"
)

func (a *appAuditor) redis() {
	report := model.NewAuditReport(a.w.Ctx, "Redis")

	unavailableInstances := utils.NewStringSet()
	slowInstances := utils.NewStringSet()

	for _, i := range a.app.Instances {
		if i.Redis == nil {
			continue
		}
		status := model.NewTableCell().SetStatus(model.OK, "up")
		if !(i.Redis.Up != nil && i.Redis.Up.Last() > 0) {
			unavailableInstances.Add(i.Name)
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
		avg := timeseries.Aggregate(timeseries.Div, total, calls)
		report.
			GetOrCreateChart("Redis latency, seconds").
			AddSeries(i.Name, avg)
		report.
			GetOrCreateChartInGroup("Redis queries on <selector>, per seconds", i.Name).
			Stacked().
			Sorted().
			AddMany(timeseries.Top(i.Redis.Calls, timeseries.NanSum, 5))

		if l := avg.Last(); l > a.getSimpleConfig(model.CheckIdRedisLatency, 0.005).Threshold {
			slowInstances.Add(i.Name)
		}
	}

	redisStatus := report.AddCheck(model.CheckIdRedisStatus)
	if unavailableInstances.Len() > 0 {
		redisStatus.SetStatus(
			model.WARNING,
			"%s %s unavailable",
			english.Plural(unavailableInstances.Len(), "instance", "instances"),
			utils.IsOrAre(unavailableInstances.Len()),
		)
	}
	redisLatency := report.AddCheck(model.CheckIdRedisLatency)
	if slowInstances.Len() > 0 {
		redisLatency.SetStatus(
			model.WARNING,
			"%s %s performing slowly",
			english.Plural(slowInstances.Len(), "instance", "instances"),
			utils.IsOrAre(slowInstances.Len()),
		)
	}
	a.addReport(report)
}
