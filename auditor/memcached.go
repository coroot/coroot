package auditor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

func (a *appAuditor) memcached() {
	appTypes := a.app.ApplicationTypes()
	isMemcached := appTypes[model.ApplicationTypeMemcached]

	if !isMemcached && !a.app.IsMemcached() {
		return
	}

	report := a.addReport(model.AuditReportMemcached)

	report.Instrumentation = model.ApplicationTypeMemcached

	if !a.app.IsMemcached() {
		report.Status = model.UNKNOWN
		return
	}
	availabilityCheck := report.CreateCheck(model.Checks.MemcachedAvailability)

	table := report.GetOrCreateTable("Instance", "Status", "Limit", "Version")
	queriesChart := report.GetOrCreateChartGroup("Memcached commands on <selector>, per seconds", nil)
	hitRateChart := report.GetOrCreateChart("Hit rate, %", nil)
	evictionsChart := report.GetOrCreateChart("Items evicted, per seconds", nil)

	for _, i := range a.app.Instances {
		if i.Memcached == nil {
			continue
		}

		obsolete := i.IsObsolete()

		if !obsolete && !i.Memcached.IsUp() {
			availabilityCheck.AddItem(i.Name)
		}

		if !obsolete && table != nil {
			name := model.NewTableCell(i.Name)
			status := model.NewTableCell().SetStatus(model.OK, "up")
			if !i.Memcached.IsUp() {
				status.SetStatus(model.WARNING, "down (no metrics)")
			}
			limit := model.NewTableCell()
			if v := i.Memcached.LimitBytes.Last(); v > 0 {
				limit.Value, limit.Unit = utils.FormatBytes(v)
			}
			table.AddRow(name, status, limit, model.NewTableCell(i.Memcached.Version.Value()))
		}

		if queriesChart != nil {
			byCmd := map[string]model.SeriesData{}
			for cmd, ts := range i.Memcached.Calls {
				byCmd[cmd] = ts
			}
			queriesChart.GetOrCreateChart(i.Name).Stacked().Sorted().
				AddMany(byCmd, 5, timeseries.NanSum)
		}
		if evictionsChart != nil {
			evictionsChart.AddSeries(i.Name, i.Memcached.EvictedItems)
		}
		if hitRateChart != nil {
			hitRateChart.AddSeries(
				i.Name,
				timeseries.Aggregate2(i.Memcached.Hits, i.Memcached.Misses, func(x, y float32) float32 { return x / (x + y) * 100 }),
			)
		}

	}
}
