package auditor

import (
	"github.com/coroot/coroot/model"
)

func (a *appAuditor) jvm() {
	if !a.app.IsJvm() {
		return
	}

	report := a.addReport(model.AuditReportJvm)

	availabilityCheck := report.CreateCheck(model.Checks.JvmAvailability)
	safepointCheck := report.CreateCheck(model.Checks.JvmSafepointTime)

	table := report.GetOrCreateTable("Instance", "Status", "Java version")
	heapChart := report.GetOrCreateChartGroup("Heap size <selector>, bytes")
	gcChart := report.GetOrCreateChartGroup("GC time <selector>, seconds/second")
	safepointChart := report.GetOrCreateChart("Safepoint time, seconds/second")

	for _, i := range a.app.Instances {
		obsolete := i.IsObsolete()
		for name, j := range i.Jvms {
			fullName := name + "@" + i.Name

			if !obsolete && !j.IsUp() {
				availabilityCheck.AddItem(fullName)
			}
			if !obsolete && j.SafepointTime.Last() > safepointCheck.Threshold {
				safepointCheck.AddItem(i.Name)
			}

			if heapChart != nil {
				heapChart.GetOrCreateChart(fullName).Stacked().
					AddSeries("used", j.HeapUsed, "blue").
					SetThreshold("total", j.HeapSize)
			}
			if gcChart != nil {
				for gc, ts := range j.GcTime {
					gcChart.GetOrCreateChart(gc).AddSeries(fullName, ts)
				}
			}
			if safepointChart != nil {
				safepointChart.AddSeries(fullName, j.SafepointTime)
			}

			if !obsolete && table != nil {
				name := model.NewTableCell(fullName)
				status := model.NewTableCell().SetStatus(model.OK, "up")
				if !j.IsUp() {
					status.SetStatus(model.WARNING, "down (no metrics)")
				}
				version := model.NewTableCell(j.JavaVersion.Value())
				table.AddRow(name, status, version)
			}
		}
	}
}
