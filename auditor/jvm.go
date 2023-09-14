package auditor

import (
	"github.com/coroot/coroot/model"
)

func (a *appAuditor) jvm() {
	if !a.app.IsJvm() {
		return
	}
	report := a.addReport(model.AuditReportJvm)

	unavailable := report.CreateCheck(model.Checks.JvmAvailability)
	safepointTime := report.CreateCheck(model.Checks.JvmSafepointTime)

	for _, i := range a.app.Instances {
		for name, j := range i.Jvms {
			fullName := name + "@" + i.Name
			report.
				GetOrCreateChartInGroup("Heap size <selector>, bytes", fullName).
				Stacked().
				AddSeries("used", j.HeapUsed, "blue").
				SetThreshold("total", j.HeapSize)
			for gc, ts := range j.GcTime {
				report.GetOrCreateChartInGroup("GC time <selector>, seconds/second", gc).AddSeries(fullName, ts)
			}
			report.GetOrCreateChart("Safepoint time, seconds/second").AddSeries(fullName, j.SafepointTime)

			if i.IsObsolete() {
				continue
			}
			status := model.NewTableCell().SetStatus(model.OK, "up")
			if !j.IsUp() {
				unavailable.AddItem(fullName)
				status.SetStatus(model.WARNING, "down (no metrics)")
			}
			if j.SafepointTime.Last() > safepointTime.Threshold {
				safepointTime.AddItem(i.Name)
			}
			report.GetOrCreateTable("Instance", "Status", "Java version").AddRow(
				model.NewTableCell(fullName),
				status,
				model.NewTableCell(j.JavaVersion.Value()),
			)
		}
	}
}
