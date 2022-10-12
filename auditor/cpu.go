package auditor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func (a *appAuditor) cpu() {
	report := a.addReport("CPU")
	relevantNodes := map[string]*model.Node{}
	nodeCpuCheck := report.CreateCheck(model.Checks.CPUNode)
	containerCpuCheck := report.CreateCheck(model.Checks.CPUContainer)
	seenContainers, seenRelatedNodes := false, false
	for _, i := range a.app.Instances {
		for _, c := range i.Containers {
			seenContainers = true
			report.GetOrCreateChartInGroup("CPU usage of container <selector>, cores", c.Name).
				AddSeries(i.Name, c.CpuUsage).
				SetThreshold("limit", c.CpuLimit, timeseries.Max)
			report.GetOrCreateChartInGroup("CPU delay of container <selector>, seconds/second", c.Name).AddSeries(i.Name, c.CpuDelay)
			report.GetOrCreateChartInGroup("Throttled time of container <selector>, seconds/second", c.Name).AddSeries(i.Name, c.ThrottledTime)

			usage := timeseries.Last(c.CpuUsage) / timeseries.Last(c.CpuLimit)
			if usage > containerCpuCheck.Threshold {
				containerCpuCheck.AddItem("%s@%s", c.Name, i.Name)
			}
		}
		if node := i.Node; i.Node != nil {
			seenRelatedNodes = true
			nodeName := node.Name.Value()
			if relevantNodes[nodeName] == nil {
				relevantNodes[nodeName] = i.Node
				report.GetOrCreateChartInGroup("Node CPU usage <selector>, %", "overview").
					AddSeries(nodeName, i.Node.CpuUsagePercent).
					Feature()

				if timeseries.Last(i.Node.CpuUsagePercent) > nodeCpuCheck.Threshold {
					nodeCpuCheck.AddItem(i.Node.Name.Value())
				}

				byMode := report.GetOrCreateChartInGroup("Node CPU usage <selector>, %", nodeName).Sorted().Stacked()
				for _, s := range cpuByModeSeries(node.CpuUsageByMode) {
					byMode.Series = append(byMode.Series, s)
				}

				report.GetOrCreateChartInGroup("CPU consumers on <selector>, cores", nodeName).
					Stacked().
					Sorted().
					SetThreshold("total", node.CpuCapacity, timeseries.Any).
					AddMany(timeseries.Top(cpuConsumers(node), timeseries.NanSum, 5))
			}
		}
	}
	if !seenContainers {
		containerCpuCheck.SetStatus(model.UNKNOWN, "no data")
	}
	if !seenRelatedNodes {
		nodeCpuCheck.SetStatus(model.UNKNOWN, "no data")
	}
}
