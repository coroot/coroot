package auditor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/profiling"
	"github.com/coroot/coroot/timeseries"
)

func (a *appAuditor) memory(ncs nodeConsumersByNode) {
	report := a.addReport(model.AuditReportMemory)
	relevantNodes := map[string]*model.Node{}

	oomCheck := report.CreateCheck(model.Checks.MemoryOOM)
	leakCheck := report.CreateCheck(model.Checks.MemoryLeakPercent)

	usageChart := report.GetOrCreateChartGroup("Memory usage (RSS) <selector>, bytes")
	oomChart := report.GetOrCreateChart("Out of memory events").Column()
	nodesChart := report.GetOrCreateChart("Node memory usage (unreclaimable), %")
	consumersChart := report.GetOrCreateChartGroup("Memory consumers <selector>, bytes")

	seenContainers := false
	limitByContainer := map[string]*timeseries.Aggregate{}
	totalRss := timeseries.NewAggregate(timeseries.NanSum)
	for _, i := range a.app.Instances {
		oom := timeseries.NewAggregate(timeseries.NanSum)
		instanceRss := timeseries.NewAggregate(timeseries.NanSum)
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
			totalRss.Add(c.MemoryRssForTrend)
			instanceRss.Add(c.MemoryRss)
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

	if a.p.Settings.Integrations.Pyroscope != nil && usageChart != nil {
		for _, ch := range usageChart.Charts {
			ch.DrillDownLink = model.NewRouterLink("profile").SetParam("report", model.AuditReportProfiling).SetArg("profile", profiling.TypeMemory)
		}
	}

	if !seenContainers {
		oomCheck.SetStatus(model.UNKNOWN, "no data")
		leakCheck.SetStatus(model.UNKNOWN, "no data")
		return
	}
	switch a.app.Id.Kind {
	case model.ApplicationKindCronJob, model.ApplicationKindJob:
		leakCheck.SetStatus(model.UNKNOWN, "not checked for Jobs and CronJobs")
	default:
		v := totalRss.Get().MapInPlace(timeseries.ZeroToNan)
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
}
