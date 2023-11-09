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
	seenContainers := false
	limitByContainer := map[string]*timeseries.Aggregate{}
	memoryUsageChartTitle := "Memory usage (RSS) <selector>, bytes"
	totalRss := timeseries.NewAggregate(timeseries.NanSum)
	for _, i := range a.app.Instances {
		oom := timeseries.NewAggregate(timeseries.NanSum)
		instanceRss := timeseries.NewAggregate(timeseries.NanSum)
		for _, c := range i.Containers {
			seenContainers = true
			l := limitByContainer[c.Name]
			if l == nil {
				l = timeseries.NewAggregate(timeseries.Max)
				limitByContainer[c.Name] = l
			}
			l.Add(c.MemoryLimit)
			report.GetOrCreateChartInGroup(memoryUsageChartTitle, "container: "+c.Name).AddSeries(i.Name, c.MemoryRss)
			oom.Add(c.OOMKills)
			totalRss.Add(c.MemoryRssForTrend)
			instanceRss.Add(c.MemoryRss)
		}
		if cg := report.GetChartGroup(memoryUsageChartTitle); cg != nil && len(cg.Charts) > 1 {
			cg.GetOrCreateChart(a.w.Ctx, "total").AddSeries(i.Name, instanceRss).Feature()
		}
		oomTs := oom.Get()
		report.GetOrCreateChart("Out of memory events").Column().AddSeries(i.Name, oomTs)

		if ooms := oomTs.Reduce(timeseries.NanSum); ooms > 0 {
			oomCheck.Inc(int64(ooms))
		}
		if node := i.Node; node != nil {
			nodeName := node.GetName()
			if relevantNodes[nodeName] == nil {
				relevantNodes[nodeName] = node
				report.GetOrCreateChart("Node memory usage (unreclaimable), %").
					AddSeries(
						nodeName,
						timeseries.Aggregate2(
							node.MemoryAvailableBytes, node.MemoryTotalBytes,
							func(avail, total float32) float32 { return (total - avail) / total * 100 },
						),
					)
				report.GetOrCreateChartInGroup("Memory consumers <selector>, bytes", nodeName).
					Stacked().
					SetThreshold("total", node.MemoryTotalBytes).
					AddMany(ncs.get(node).memory, 5, timeseries.Max)
			}
		}
	}

	for container, limit := range limitByContainer {
		report.GetOrCreateChartInGroup(memoryUsageChartTitle, "container: "+container).SetThreshold("limit", limit.Get())
	}

	if a.p.Settings.Integrations.Pyroscope != nil {
		for _, ch := range report.GetOrCreateChartGroup(memoryUsageChartTitle).Charts {
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
		v := totalRss.Get().Map(timeseries.ZeroToNan)
		if v.Map(timeseries.Defined).Reduce(timeseries.NanSum) > float32(v.Len())*0.8 { // we require 80% of the data to be present
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
