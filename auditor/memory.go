package auditor

import (
	"github.com/coroot/coroot/api/views/utils"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func (a *appAuditor) memory() {
	report := model.NewAuditReport(a.w.Ctx, "Memory")
	relevantNodes := map[string]*model.Node{}

	var totalOOMEvents float64
	for _, i := range a.app.Instances {
		oom := timeseries.Aggregate(timeseries.NanSum)
		for _, c := range i.Containers {
			report.GetOrCreateChartInGroup("Memory usage (RSS) <selector>, bytes", c.Name).
				AddSeries(i.Name, c.MemoryRss).
				SetThreshold("limit", c.MemoryLimit, timeseries.Max)
			oom.AddInput(c.OOMKills)
		}
		report.GetOrCreateChart("Out of memory events").AddSeries(i.Name, oom)

		if events := timeseries.Reduce(timeseries.NanSum, oom); events > 0 {
			totalOOMEvents += events
		}
		if node := i.Node; node != nil {
			nodeName := node.Name.Value()
			if relevantNodes[nodeName] == nil {
				relevantNodes[nodeName] = node
				report.GetOrCreateChart("Node memory usage (unreclaimable), bytes").
					AddSeries(
						nodeName,
						timeseries.Aggregate(
							func(avail, total float64) float64 { return (total - avail) / total * 100 },
							node.MemoryAvailableBytes, node.MemoryTotalBytes,
						),
					)
				report.GetOrCreateChartInGroup("Memory consumers <selector>, bytes", nodeName).
					Stacked().
					SetThreshold("total", node.MemoryTotalBytes, timeseries.Any).
					AddMany(timeseries.Top(utils.MemoryConsumers(node), timeseries.Max, 5))
			}
		}
	}

	check := report.AddCheck(model.CheckIdOOM)
	if totalOOMEvents > a.getSimpleConfig(model.CheckIdOOM, 0).Threshold {
		check.SetStatus(model.WARNING, "app containers have been restarted %.0f times by the OOM killer", totalOOMEvents)
	}

	a.addReport(report)
}
