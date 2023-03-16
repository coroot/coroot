package auditor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/profiling"
	"github.com/coroot/coroot/timeseries"
)

func (a *appAuditor) cpu(ncs nodeConsumersByNode) {
	report := a.addReport(model.AuditReportCPU)
	relevantNodes := map[string]*model.Node{}
	nodeCpuCheck := report.CreateCheck(model.Checks.CPUNode)
	containerCpuCheck := report.CreateCheck(model.Checks.CPUContainer)
	seenContainers, seenRelatedNodes := false, false
	limitByContainer := map[string]*timeseries.Aggregate{}
	cpuChartTitle := "CPU usage of container <selector>, cores"

	for _, i := range a.app.Instances {
		for _, c := range i.Containers {
			seenContainers = true
			l := limitByContainer[c.Name]
			if l == nil {
				l = timeseries.NewAggregate(timeseries.Max)
				limitByContainer[c.Name] = l
			}
			l.Add(c.CpuLimit)
			usageChart := report.GetOrCreateChartInGroup(cpuChartTitle, c.Name).AddSeries(i.Name, c.CpuUsage)
			report.GetOrCreateChartInGroup("CPU delay of container <selector>, seconds/second", c.Name).AddSeries(i.Name, c.CpuDelay)
			report.GetOrCreateChartInGroup("Throttled time of container <selector>, seconds/second", c.Name).AddSeries(i.Name, c.ThrottledTime)

			usage := c.CpuUsage.Last() / c.CpuLimit.Last()
			if usage > containerCpuCheck.Threshold {
				usageChart.Feature()
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

				cpuByModeChart(report.GetOrCreateChartInGroup("Node CPU usage <selector>, %", nodeName), node.CpuUsageByMode)

				consumersChart := report.GetOrCreateChartInGroup("CPU consumers on <selector>, cores", nodeName).
					Stacked().
					Sorted().
					SetThreshold("total", node.CpuCapacity).
					AddMany(ncs.get(node).cpu, 5, timeseries.Max)

				if i.Node.CpuUsagePercent.Last() > nodeCpuCheck.Threshold {
					consumersChart.Feature()
					nodeCpuCheck.AddItem(i.Node.Name.Value())
				}
			}
		}
	}
	for container, limit := range limitByContainer {
		report.GetOrCreateChartInGroup(cpuChartTitle, container).SetThreshold("limit", limit)
	}

	if a.p.Settings.Integrations.Pyroscope != nil {
		for _, ch := range report.GetOrCreateChartGroup(cpuChartTitle).Charts {
			ch.DrillDownLink = model.NewRouterLink("profile").SetParam("report", model.AuditReportProfiling).SetArg("profile", profiling.TypeCPU)
		}
	}

	if !seenContainers {
		containerCpuCheck.SetStatus(model.UNKNOWN, "no data")
	}
	if !seenRelatedNodes {
		nodeCpuCheck.SetStatus(model.UNKNOWN, "no data")
	}
}
