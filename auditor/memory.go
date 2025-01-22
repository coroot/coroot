package auditor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func (a *appAuditor) memory(ncs nodeConsumersByNode) {
	report := a.addReport(model.AuditReportMemory)
	relevantNodes := map[string]*model.Node{}

	oomCheck := report.CreateCheck(model.Checks.MemoryOOM)
	leakCheck := report.CreateCheck(model.Checks.MemoryLeakPercent)

	usageChart := report.GetOrCreateChartGroup(
		"Memory usage (RSS) <selector>, bytes",
		model.NewDocLink("inspections", "memory", "memory-usage"),
	)
	oomChart := report.GetOrCreateChart(
		"Out of memory events",
		model.NewDocLink("inspections", "memory", "out-of-memory-events"),
	).Column()
	nodesChart := report.GetOrCreateChart(
		"Node memory usage (unreclaimable), %",
		model.NewDocLink("inspections", "memory", "node-memory-usage-unreclaimable"),
	)
	consumersChart := report.GetOrCreateChartGroup(
		"Memory consumers <selector>, bytes",
		model.NewDocLink("inspections", "memory", "memory-consumers"),
	)

	seenContainers := false
	limitByContainer := map[string]*timeseries.Aggregate{}
	for _, i := range a.app.Instances {
		oom := timeseries.NewAggregate(timeseries.NanSum)
		instanceRss := timeseries.NewAggregate(timeseries.NanSum)
		instanceRssForTrend := timeseries.NewAggregate(timeseries.NanSum)
		for _, c := range i.Containers {
			seenContainers = true
			if limitByContainer[c.Name] == nil {
				limitByContainer[c.Name] = timeseries.NewAggregate(timeseries.Max)
			}
			limitByContainer[c.Name].Add(c.MemoryLimit)
			if usageChart != nil {
				usageChart.GetOrCreateChart("container: "+c.Name).AddSeries(i.Name, c.MemoryRss)
			}
			oom.Add(c.OOMKills)
			instanceRssForTrend.Add(c.MemoryRssForTrend)
			instanceRss.Add(c.MemoryRss)
		}
		if a.app.PeriodicJob() {
			leakCheck.SetStatus(model.UNKNOWN, "not checked for periodic jobs")
		} else {
			v := instanceRssForTrend.Get().MapInPlace(timeseries.ZeroToNan)
			if v.Reduce(timeseries.NanCount) > float32(v.Len())*0.8 { // we require 80% of the data to be present
				if lr := timeseries.NewLinearRegression(v); lr != nil {
					s := lr.Calc(a.w.Ctx.To.Add(-timeseries.Hour))
					e := lr.Calc(a.w.Ctx.To)
					if s > 0 && e > 0 {
						leakCheck.SetValue((e - s) / s * 100)
					}
				}
			}
		}

		if usageChart != nil && len(usageChart.Charts) > 1 {
			usageChart.GetOrCreateChart("total").AddSeries(i.Name, instanceRss).Feature()
		}

		oomTs := oom.Get()
		if oomChart != nil {
			oomChart.AddSeries(i.Name, oomTs)
		}
		if ooms := oomTs.Reduce(timeseries.NanSum); ooms > 0 {
			oomCheck.Inc(int64(ooms))
		}

		if node := i.Node; node != nil {
			nodeName := node.GetName()
			if relevantNodes[nodeName] != nil {
				continue
			}
			relevantNodes[nodeName] = node
			if nodesChart != nil {
				nodesChart.AddSeries(
					nodeName,
					timeseries.Aggregate2(
						node.MemoryAvailableBytes, node.MemoryTotalBytes,
						func(avail, total float32) float32 { return (total - avail) / total * 100 },
					),
				)
			}
			if consumersChart != nil {
				consumersChart.GetOrCreateChart(nodeName).
					Stacked().
					SetThreshold("total", node.MemoryTotalBytes).
					AddMany(ncs.get(node).memory, 5, timeseries.Max)
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
				SetArg("query", model.ProfileCategoryMemory)
		}
	}
	if !seenContainers {
		oomCheck.SetStatus(model.UNKNOWN, "no data")
		leakCheck.SetStatus(model.UNKNOWN, "no data")
		return
	}
}
