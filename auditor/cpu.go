package auditor

import (
	"github.com/coroot/coroot/api/views/utils"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func (a *appAuditor) cpu() {
	report := model.NewAuditReport(a.w.Ctx, "CPU")
	relevantNodes := map[string]*model.Node{}

	for _, i := range a.app.Instances {
		for _, c := range i.Containers {
			report.GetOrCreateChartInGroup("CPU usage of container <selector>, cores", c.Name).
				AddSeries(i.Name, c.CpuUsage).
				SetThreshold("limit", c.CpuLimit, timeseries.Max)
			report.GetOrCreateChartInGroup("CPU delay of container <selector>, seconds/second", c.Name).AddSeries(i.Name, c.CpuDelay)
			report.GetOrCreateChartInGroup("Throttled time of container <selector>, seconds/second", c.Name).AddSeries(i.Name, c.ThrottledTime)
		}
		if node := i.Node; i.Node != nil {
			nodeName := node.Name.Value()
			if relevantNodes[nodeName] == nil {
				relevantNodes[nodeName] = i.Node
				report.GetOrCreateChartInGroup("Node CPU usage <selector>, %", "overview").
					AddSeries(nodeName, i.Node.CpuUsagePercent).
					Feature()

				byMode := report.GetOrCreateChartInGroup("Node CPU usage <selector>, %", nodeName).Sorted().Stacked()
				for _, s := range utils.CpuByModeSeries(node.CpuUsageByMode) {
					byMode.Series = append(byMode.Series, s)
				}

				report.GetOrCreateChartInGroup("CPU consumers on <selector>, cores", nodeName).
					Stacked().
					Sorted().
					SetThreshold("total", node.CpuCapacity, timeseries.Any).
					AddMany(timeseries.Top(utils.CpuConsumers(node), timeseries.NanSum, 5))
			}
		}
	}
	a.addReport(report)
}
