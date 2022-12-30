package auditor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func (a *appAuditor) jvm() {
	if !a.app.IsJvm() {
		return
	}
	report := a.addReport(model.AuditReportJvm)

	availability := report.CreateCheck(model.Checks.JvmAvailability)
	safepointTime := report.CreateCheck(model.Checks.JvmSafepointTime)

	for _, i := range a.app.Instances {
		if i.Jvm == nil {
			continue
		}
		for gc, ts := range i.Jvm.GcTime {
			report.GetOrCreateChartInGroup("GC time <selector>, seconds/second", gc).AddSeries(i.Name, ts)
		}
		report.GetOrCreateChart("Safepoint time, seconds/second").AddSeries(i.Name, i.Jvm.SafepointTime)
		report.
			GetOrCreateChartInGroup("Heap size <selector>, bytes", i.Name).
			Stacked().
			AddSeries("used", i.Jvm.HeapUsed, "blue").
			SetThreshold("total", i.Jvm.HeapSize, timeseries.Max)

		if i.IsObsolete() {
			continue
		}
		status := model.NewTableCell().SetStatus(model.OK, "up")
		if !i.Jvm.IsUp() {
			availability.AddItem(i.Name)
			status.SetStatus(model.WARNING, "down (no metrics)")
		}
		report.GetOrCreateTable("Instance", "Status").AddRow(
			model.NewTableCell(i.Name).AddTag("java: %s", i.Jvm.JavaVersion.Value()),
			status,
		)
		if timeseries.Last(i.Jvm.SafepointTime) > safepointTime.Threshold {
			safepointTime.AddItem(i.Name)
		}
	}
}
