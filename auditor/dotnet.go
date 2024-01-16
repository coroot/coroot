package auditor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func (a *appAuditor) dotnet() {
	if !a.app.IsDotNet() {
		return
	}

	report := a.addReport(model.AuditReportDotNet)

	availabilityCheck := report.CreateCheck(model.Checks.DotNetAvailability)

	table := report.GetOrCreateTable("Instance", "Status", "Runtime version")
	heapChart := report.GetOrCreateChartGroup("Heap size <selector>, bytes")
	gcChart := report.GetOrCreateChartGroup("GC <selector>, collections/second")
	allocationChart := report.GetOrCreateChart("Memory allocation rate, bytes/second")
	exceptionsChart := report.GetOrCreateChart("Exceptions, per second")
	heapFragmentationChart := report.GetOrCreateChart("Heap fragmentation, %")
	threadPoolQueueChart := report.GetOrCreateChart("Thread pool queue size, items")
	threadPoolSizeChart := report.GetOrCreateChart("Thread pool size, threads")
	threadPoolCompletedItemsChart := report.GetOrCreateChart("Thread pool completed work items, per second")
	monitorLockContentions := report.GetOrCreateChart("Monitor's lock contentions, per second")

	for _, i := range a.app.Instances {
		obsolete := i.IsObsolete()
		for name, runtime := range i.DotNet {
			fullName := name + "@" + i.Name

			if !obsolete && !runtime.IsUp() {
				availabilityCheck.AddItem(fullName)
			}
			if heapChart != nil {
				chart := heapChart.GetOrCreateChart(fullName).Stacked()
				for gen, ts := range runtime.HeapSize {
					chart.AddSeries(gen, ts)
				}
			}
			if gcChart != nil {
				total := timeseries.NewAggregate(timeseries.NanSum)
				for gc, ts := range runtime.GcCount {
					gcChart.GetOrCreateChart(gc).Stacked().AddSeries(fullName, ts)
					total.Add(ts)
				}
				gcChart.GetOrCreateChart("overview").Feature().AddSeries(fullName, total)
			}
			if allocationChart != nil {
				allocationChart.AddSeries(fullName, runtime.MemoryAllocationRate)
			}
			if exceptionsChart != nil {
				exceptionsChart.AddSeries(fullName, runtime.Exceptions)
			}
			if heapFragmentationChart != nil {
				heapFragmentationChart.AddSeries(fullName, runtime.HeapFragmentationPercent)
			}
			if threadPoolQueueChart != nil {
				threadPoolQueueChart.AddSeries(fullName, runtime.ThreadPoolQueueSize)
			}
			if threadPoolSizeChart != nil {
				threadPoolSizeChart.AddSeries(fullName, runtime.ThreadPoolSize)
			}
			if threadPoolCompletedItemsChart != nil {
				threadPoolCompletedItemsChart.AddSeries(fullName, runtime.ThreadPoolCompletedItems)
			}
			if monitorLockContentions != nil {
				monitorLockContentions.AddSeries(fullName, runtime.MonitorLockContentions)
			}
			if !obsolete && table != nil {
				name := model.NewTableCell(fullName)
				status := model.NewTableCell().SetStatus(model.OK, "up")
				if !runtime.IsUp() {
					status.SetStatus(model.WARNING, "down (no metrics)")
				}
				version := model.NewTableCell(runtime.RuntimeVersion.Value())
				table.AddRow(name, status, version)
			}
		}
	}
}
