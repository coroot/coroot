package auditor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func (a *appAuditor) redis() {
	appTypes := a.app.ApplicationTypes()
	isRedis := appTypes[model.ApplicationTypeRedis] ||
		appTypes[model.ApplicationTypeKeyDB] ||
		appTypes[model.ApplicationTypeValkey] ||
		appTypes[model.ApplicationTypeDragonfly]

	if !isRedis && !a.app.IsRedis() {
		return
	}

	report := a.addReport(model.AuditReportRedis)

	report.Instrumentation = model.ApplicationTypeRedis

	if !a.app.IsRedis() {
		report.Status = model.UNKNOWN
		return
	}
	availabilityCheck := report.CreateCheck(model.Checks.RedisAvailability)
	latencyCheck := report.CreateCheck(model.Checks.RedisLatency)

	table := report.GetOrCreateTable("Instance", "Role", "Status", "Version")
	latencyChart := report.GetOrCreateChart("Redis average latency, seconds", nil)
	queriesChart := report.GetOrCreateChartGroup("Redis queries on <selector>, per seconds", nil)

	for _, i := range a.app.Instances {
		if i.Redis == nil {
			continue
		}

		obsolete := i.IsObsolete()

		if !obsolete && !i.Redis.IsUp() {
			availabilityCheck.AddItem(i.Name)
		}

		var total float32
		var calls float32
		for cmd, t := range i.Redis.CallsTime {
			if c, ok := i.Redis.Calls[cmd]; ok {
				total = timeseries.NanSum(0, total, t.Last())
				calls = timeseries.NanSum(0, calls, c.Last())
			}
		}
		if !obsolete && total > 0 && calls > 0 && total/calls > latencyCheck.Threshold {
			latencyCheck.AddItem(i.Name)
		}

		if !obsolete && table != nil {
			name := model.NewTableCell(i.Name)
			role := model.NewTableCell(i.Redis.Role.Value())
			switch i.Redis.Role.Value() {
			case "primary":
				role.SetIcon("mdi-database-edit-outline", "rgba(0,0,0,0.87)")
			case "replica":
				role.SetIcon("mdi-database-import-outline", "grey")
			}
			status := model.NewTableCell().SetStatus(model.OK, "up")
			if !i.Redis.IsUp() {
				if v := i.Redis.Error.Value(); v != "" {
					status.SetStatus(model.WARNING, v)
				} else {
					status.SetStatus(model.WARNING, "down (no metrics)")
				}
			}
			table.AddRow(name, role, status, model.NewTableCell(i.Redis.Version.Value()))
		}

		if latencyChart != nil {
			total := timeseries.NewAggregate(timeseries.NanSum)
			calls := timeseries.NewAggregate(timeseries.NanSum)
			for cmd, t := range i.Redis.CallsTime {
				if c, ok := i.Redis.Calls[cmd]; ok {
					total.Add(t)
					calls.Add(c)
				}
			}
			avg := timeseries.Div(total.Get(), calls.Get())
			latencyChart.AddSeries(i.Name, avg)
		}

		if queriesChart != nil {
			byCmd := map[string]model.SeriesData{}
			for cmd, ts := range i.Redis.Calls {
				byCmd[cmd] = ts
			}
			queriesChart.GetOrCreateChart(i.Name).Stacked().Sorted().
				AddMany(byCmd, 5, timeseries.NanSum)
		}
	}
}
