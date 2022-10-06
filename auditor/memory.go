package auditor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func (a *appAuditor) memory() {
	report := a.addReport("Memory")
	relevantNodes := map[string]*model.Node{}

	oomCheck := report.CreateCheck(model.Checks.MemoryOOM)
	seenContainers := false
	for _, i := range a.app.Instances {
		oom := timeseries.Aggregate(timeseries.NanSum)
		for _, c := range i.Containers {
			seenContainers = true
			report.GetOrCreateChartInGroup("Memory usage (RSS) <selector>, bytes", c.Name).
				AddSeries(i.Name, c.MemoryRss).
				SetThreshold("limit", c.MemoryLimit, timeseries.Max)
			oom.AddInput(c.OOMKills)
		}
		report.GetOrCreateChart("Out of memory events").AddSeries(i.Name, oom)

		if ooms := timeseries.Reduce(timeseries.NanSum, oom); ooms > 0 {
			oomCheck.Inc(int64(ooms))
		}
		if node := i.Node; node != nil {
			nodeName := node.Name.Value()
			if relevantNodes[nodeName] == nil {
				relevantNodes[nodeName] = node
				report.GetOrCreateChart("Node memory usage (unreclaimable), bytes").
					AddSeries(
						nodeName,
						timeseries.Aggregate(
							func(t timeseries.Time, avail, total float64) float64 { return (total - avail) / total * 100 },
							node.MemoryAvailableBytes, node.MemoryTotalBytes,
						),
					)
				report.GetOrCreateChartInGroup("Memory consumers <selector>, bytes", nodeName).
					Stacked().
					SetThreshold("total", node.MemoryTotalBytes, timeseries.Any).
					AddMany(timeseries.Top(memoryConsumers(node), timeseries.Max, 5))
			}
		}
	}
	if !seenContainers {
		oomCheck.SetStatus(model.UNKNOWN, "no data")
	}

}
