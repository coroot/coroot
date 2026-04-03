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
	heapChart := report.GetOrCreateChartGroup("Heap size <selector>, bytes", nil)
	gcChart := report.GetOrCreateChartGroup("GC time <selector>, seconds/second", nil)
	safepointChart := report.GetOrCreateChart("Safepoint time, seconds/second", nil)
	allocChart := report.GetOrCreateChartGroup("Allocation rate <selector>", nil)
	lockChart := report.GetOrCreateChartGroup("Lock contention <selector>", nil)

	availabilityCheck.AddWidget(table.Widget())

	safepointCheck.AddWidget(safepointChart.Widget())
	safepointCheck.AddWidget(gcChart.Widget())

	profileLink := func(category model.ProfileCategory) *model.RouterLink {
		return model.NewRouterLink("profile", "overview").
			SetParam("view", "applications").
			SetParam("id", a.app.Id).
			SetParam("report", model.AuditReportProfiling).
			SetArg("query", string(category))
	}

	for _, i := range a.app.Instances {
		obsolete := i.IsObsolete()
		succeeded := false
		if i.Pod != nil {
			succeeded = i.Pod.IsSucceeded()
		}
		for name, j := range i.Jvms {
			fullName := name + "@" + i.Name

			if !obsolete && !succeeded && !j.IsUp() {
				availabilityCheck.AddItem(fullName)
			}
			if !obsolete && j.SafepointTime.Last() > safepointCheck.Threshold {
				safepointCheck.AddItem(i.Name)
			}

			if heapChart != nil {
				heapChart.GetOrCreateChart("overview").Feature().AddSeries(fullName, j.HeapUsed)
				heapChart.GetOrCreateChart(fullName).Stacked().
					AddSeries("used", j.HeapUsed, "blue").
					SetThreshold("total", j.HeapMaxSize)
			}
			if gcChart != nil {
				for gc, ts := range j.GcTime {
					gcChart.GetOrCreateChart(gc).AddSeries(fullName, ts)
				}
			}
			if safepointChart != nil {
				safepointChart.AddSeries(fullName, j.SafepointTime)
			}
			if allocChart != nil {
				allocChart.GetOrCreateChart("bytes/second").AddSeries(fullName, j.AllocBytes).Feature()
				allocChart.GetOrCreateChart("objects/second").AddSeries(fullName, j.AllocObjects)
			}
			if lockChart != nil {
				lockChart.GetOrCreateChart("contentions/second").AddSeries(fullName, j.LockContentions)
				lockChart.GetOrCreateChart("delay, seconds/second").AddSeries(fullName, j.LockTime).Feature()
			}

			if !obsolete && !succeeded && table != nil {
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

	if allocChart != nil {
		for _, ch := range allocChart.Charts {
			ch.DrillDownLink = profileLink(model.ProfileCategoryMemory)
		}
	}
	if lockChart != nil {
		for _, ch := range lockChart.Charts {
			ch.DrillDownLink = profileLink(model.ProfileCategoryLock)
		}
	}

	profilingEnabled := false
	for _, i := range a.app.Instances {
		for _, j := range i.Jvms {
			if j.ProfilingEnabled {
				profilingEnabled = true
				break
			}
		}
		if profilingEnabled {
			break
		}
	}
	if !profilingEnabled {
		report.ConfigurationHint = &model.ConfigurationHint{
			Message:      "Enable async-profiler to get Java CPU, memory allocation, and lock contention profiles and metrics.",
			ReadMoreLink: "https://docs.coroot.com/profiling/java-profiling",
		}
	}
}
