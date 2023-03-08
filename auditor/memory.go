package auditor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/profiling"
	"github.com/coroot/coroot/timeseries"
	"math"
)

func (a *appAuditor) memory() {
	report := a.addReport(model.AuditReportMemory)
	relevantNodes := map[string]*model.Node{}

	oomCheck := report.CreateCheck(model.Checks.MemoryOOM)
	leakCheck := report.CreateCheck(model.Checks.MemoryLeak)
	var leak float64
	now := timeseries.Now()
	seenContainers := false
	limitByContainer := map[string]*timeseries.Aggregate{}
	memoryUsageChartTitle := "Memory usage (RSS) <selector>, bytes"
	for _, i := range a.app.Instances {
		oom := timeseries.NewAggregate(timeseries.NanSum)
		for _, c := range i.Containers {
			seenContainers = true
			l := limitByContainer[c.Name]
			if l == nil {
				l = timeseries.NewAggregate(timeseries.Max)
				limitByContainer[c.Name] = l
			}
			l.Add(c.MemoryLimit)
			report.GetOrCreateChartInGroup(memoryUsageChartTitle, c.Name).AddSeries(i.Name, c.MemoryRss)
			oom.Add(c.OOMKills)
			if lr := timeseries.NewLinearRegression(c.MemoryRss); lr != nil {
				if v := (lr.Calc(now) - lr.Calc(now.Add(-timeseries.Hour))) / 1024 / 1024; !math.IsNaN(v) {
					leak += v
				}
			}
		}
		oomTs := oom.Get()
		report.GetOrCreateChart("Out of memory events").Column().AddSeries(i.Name, oomTs)

		if ooms := oomTs.Reduce(timeseries.NanSum); ooms > 0 {
			oomCheck.Inc(int64(ooms))
		}
		if node := i.Node; node != nil {
			nodeName := node.Name.Value()
			if relevantNodes[nodeName] == nil {
				relevantNodes[nodeName] = node
				report.GetOrCreateChart("Node memory usage (unreclaimable), %").
					AddSeries(
						nodeName,
						timeseries.Aggregate2(
							node.MemoryAvailableBytes, node.MemoryTotalBytes,
							func(avail, total float64) float64 { return (total - avail) / total * 100 },
						),
					)
				report.GetOrCreateChartInGroup("Memory consumers <selector>, bytes", nodeName).
					Stacked().
					SetThreshold("total", node.MemoryTotalBytes).
					AddMany(timeseries.Top(memoryConsumers(node), timeseries.Max, 5))
			}
		}
	}

	for container, limit := range limitByContainer {
		report.GetOrCreateChartInGroup(memoryUsageChartTitle, container).SetThreshold("limit", limit.Get())
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
		leakCheck.SetValue(leak)
	}
}
