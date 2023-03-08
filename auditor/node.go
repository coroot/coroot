package auditor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func AuditNode(w *model.World, node *model.Node) *model.AuditReport {
	id := model.NewApplicationId("", model.ApplicationKindNode, node.Name.Value())
	report := model.NewAuditReport(model.NewApplication(id), w.Ctx, nil, model.AuditReportNode)

	cpuByModeChart(report.GetOrCreateChart("CPU usage, %"), node.CpuUsageByMode)

	report.GetOrCreateChart("CPU consumers, cores").
		Stacked().
		Sorted().
		SetThreshold("total", node.CpuCapacity).
		AddMany(cpuConsumers(node), 5, timeseries.Max)

	used := timeseries.Sub(
		node.MemoryTotalBytes,
		timeseries.Sum(node.MemoryCachedBytes, node.MemoryFreeBytes),
	)
	report.
		GetOrCreateChart("Memory usage, bytes").
		Stacked().
		Sorted().
		AddSeries("free", node.MemoryFreeBytes, "light-blue").
		AddSeries("cache", node.MemoryCachedBytes, "amber").
		AddSeries("used", used, "red")

	report.GetOrCreateChart("Memory consumers, bytes").
		Stacked().
		SetThreshold("total", node.MemoryTotalBytes).
		AddMany(memoryConsumers(node), 5, timeseries.Max)
	netLatency(report, w, node)

	for _, i := range node.NetInterfaces {
		report.
			GetOrCreateChartInGroup("Network bandwidth <selector>, bits/second", i.Name).
			AddSeries("in", i.RxBytes.Map(func(t timeseries.Time, v float64) float64 { return v * 8 }), "green").
			AddSeries("out", i.TxBytes.Map(func(t timeseries.Time, v float64) float64 { return v * 8 }), "blue")
	}

	return report
}

func netLatency(report *model.AuditReport, w *model.World, n *model.Node) {
	zones := map[string]*avgTimeSeries{}
	nodes := map[string]*avgTimeSeries{}

	srcAZ := nodeAZ(n)

	update := func(m map[string]*avgTimeSeries, k string, rtt *timeseries.TimeSeries) {
		avg := m[k]
		if avg == nil {
			avg = newAvgTimeSeries()
			m[k] = avg
		}
		avg.add(rtt)
	}

	for _, app := range w.Applications {
		for _, i := range app.Instances {
			if i.Node == nil {
				continue
			}
			for _, u := range i.Upstreams {
				if u.Rtt.IsEmpty() || u.RemoteInstance == nil || u.RemoteInstance.Node == nil {
					continue
				}
				var src, dst *model.Node
				if i.NodeName() == n.Name.Value() {
					src = i.Node
				} else {
					dst = i.Node
				}
				if u.RemoteInstance.NodeName() == n.Name.Value() {
					src = u.RemoteInstance.Node
				} else {
					dst = u.RemoteInstance.Node
				}
				if src == nil || dst == nil || src.Name == dst.Name {
					continue
				}
				update(zones, srcAZ+" - "+nodeAZ(dst), u.Rtt)
				update(nodes, n.Name.Value()+" - "+dst.Name.Value(), u.Rtt)
			}
		}
	}
	if len(nodes) == 0 && len(zones) == 0 {
		return
	}

	azChart := report.GetOrCreateChartInGroup("Network RTT between <selector>, seconds", "availability zones")
	for name, avg := range zones {
		azChart.AddSeries(name, avg.get())
	}
	nodesChart := report.GetOrCreateChartInGroup("Network RTT between <selector>, seconds", "nodes")
	for name, avg := range nodes {
		nodesChart.AddSeries(name, avg.get())
	}
}

type avgTimeSeries struct {
	sum   *timeseries.Aggregate
	count *timeseries.Aggregate
}

func newAvgTimeSeries() *avgTimeSeries {
	return &avgTimeSeries{
		sum:   timeseries.NewAggregate(timeseries.NanSum),
		count: timeseries.NewAggregate(timeseries.NanSum),
	}
}

func (a *avgTimeSeries) add(x *timeseries.TimeSeries) {
	a.sum.Add(x)
	a.count.Add(x.Map(timeseries.Defined))
}

func (a *avgTimeSeries) get() model.SeriesData {
	return timeseries.Div(a.sum.Get(), a.count.Get())
}

func nodeAZ(n *model.Node) string {
	az := n.AvailabilityZone.Value()
	if az == "" {
		az = "unspecified"
	}
	return az
}
