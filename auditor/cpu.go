package auditor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func (a *appAuditor) cpu(ncs nodeConsumersByNode) {
	report := a.addReport(model.AuditReportCPU)

	nodeCpuCheck := report.CreateCheck(model.Checks.CPUNode)
	containerCpuCheck := report.CreateCheck(model.Checks.CPUContainer)

	usageChart := report.GetOrCreateChartGroup(
		"CPU usage <selector>, cores",
		model.NewDocLink("inspections", "cpu", "cpu-usage"),
	)
	delayChart := report.GetOrCreateChartGroup(
		"CPU delay <selector>, seconds/second",
		model.NewDocLink("inspections", "cpu", "cpu-delay"),
	)
	throttlingChart := report.GetOrCreateChartGroup(
		"Throttled time <selector>, seconds/second",
		model.NewDocLink("inspections", "cpu", "throttled-time"),
	)
	nodesChart := report.GetOrCreateChartGroup(
		"Node CPU usage <selector>, %",
		model.NewDocLink("inspections", "cpu", "node-cpu-usage"),
	)
	consumersChart := report.GetOrCreateChartGroup(
		"CPU consumers on <selector>, cores",
		model.NewDocLink("inspections", "cpu", "cpu-consumers"),
	)

	seenContainers, seenRelatedNodes := false, false
	relevantNodes := map[string]*model.Node{}
	limitByContainer := map[string]*timeseries.Aggregate{}
	for _, i := range a.app.Instances {
		instanceDelay := timeseries.NewAggregate(timeseries.NanSum)
		instanceThrottledTime := timeseries.NewAggregate(timeseries.NanSum)
		instanceUsage := timeseries.NewAggregate(timeseries.NanSum)
		for _, c := range i.Containers {
			seenContainers = true
			if limitByContainer[c.Name] == nil {
				limitByContainer[c.Name] = timeseries.NewAggregate(timeseries.Max)
			}
			limitByContainer[c.Name].Add(c.CpuLimit)
			instanceDelay.Add(c.CpuDelay)
			instanceThrottledTime.Add(c.ThrottledTime)
			instanceUsage.Add(c.CpuUsage)
			title := "container: " + c.Name
			if usageChart != nil {
				usageChart.GetOrCreateChart(title).AddSeries(i.Name, c.CpuUsage)
			}
			if delayChart != nil {
				delayChart.GetOrCreateChart(title).AddSeries(i.Name, c.CpuDelay)
			}
			if throttlingChart != nil {
				throttlingChart.GetOrCreateChart(title).AddSeries(i.Name, c.ThrottledTime)
			}
			usage := c.CpuUsage.Last() / c.CpuLimit.Last() * 100
			if usage > containerCpuCheck.Threshold {
				usageChart.GetOrCreateChart(title).Feature()
				containerCpuCheck.AddItem("%s@%s", c.Name, i.Name)
			}
		}
		if usageChart != nil && len(usageChart.Charts) > 1 {
			usageChart.GetOrCreateChart("total").AddSeries(i.Name, instanceUsage).Feature()
		}
		if delayChart != nil && len(delayChart.Charts) > 1 {
			delayChart.GetOrCreateChart("total").AddSeries(i.Name, instanceDelay).Feature()
		}
		if throttlingChart != nil && len(throttlingChart.Charts) > 1 {
			throttlingChart.GetOrCreateChart("total").AddSeries(i.Name, instanceThrottledTime).Feature()
		}

		if node := i.Node; i.Node != nil {
			seenRelatedNodes = true
			nodeName := node.GetName()
			if relevantNodes[nodeName] != nil {
				continue
			}
			relevantNodes[nodeName] = i.Node
			if nodesChart != nil {
				nodesChart.GetOrCreateChart("overview").
					AddSeries(nodeName, i.Node.CpuUsagePercent).
					Feature()
				cpuByModeChart(nodesChart.GetOrCreateChart(nodeName), node.CpuUsageByMode)
			}
			if consumersChart != nil {
				consumersChart.GetOrCreateChart(nodeName).
					Stacked().
					Sorted().
					SetThreshold("total", node.CpuCapacity).
					AddMany(ncs.get(node).cpu, 5, timeseries.Max)
			}

			if i.Node.CpuUsagePercent.Last() > nodeCpuCheck.Threshold {
				consumersChart.GetOrCreateChart(nodeName).Feature()
				nodeCpuCheck.AddItem(i.NodeName())
			}
		}
	}

	if usageChart != nil {
		for container, limit := range limitByContainer {
			usageChart.GetOrCreateChart("container: "+container).SetThreshold("limit", limit.Get())
		}
	}

	if a.clickHouseEnabled && usageChart != nil {
		for _, ch := range usageChart.Charts {
			ch.DrillDownLink = model.NewRouterLink("profile", "overview").
				SetParam("view", "applications").
				SetParam("id", a.app.Id).
				SetParam("report", model.AuditReportProfiling).
				SetArg("query", model.ProfileCategoryCPU)
		}
	}

	if !seenContainers {
		containerCpuCheck.SetStatus(model.UNKNOWN, "no data")
	}
	if !seenRelatedNodes {
		nodeCpuCheck.SetStatus(model.UNKNOWN, "no data")
	}
}
