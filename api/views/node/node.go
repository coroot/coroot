package node

import (
	utils2 "github.com/coroot/coroot-focus/api/views/utils"
	"github.com/coroot/coroot-focus/api/views/widgets"
	"github.com/coroot/coroot-focus/model"
	"github.com/coroot/coroot-focus/timeseries"
)

func Render(w *model.World, node *model.Node) *widgets.Dashboard {
	dash := widgets.NewDashboard(w.Ctx, "Node")

	cpu := dash.GetOrCreateChart("CPU usage, %").Sorted().Stacked()
	for _, s := range utils2.CpuByModeSeries(node.CpuUsageByMode) {
		cpu.Series = append(cpu.Series, s)
	}

	dash.GetOrCreateChart("CPU consumers, cores").
		Stacked().
		Sorted().
		SetThreshold("total", node.CpuCapacity, timeseries.Any).
		AddMany(timeseries.Top(utils2.CpuConsumers(node), timeseries.NanSum, 5))

	used := timeseries.Aggregate(
		timeseries.Sub,
		node.MemoryTotalBytes,
		timeseries.Aggregate(timeseries.Sum, node.MemoryCachedBytes, node.MemoryFreeBytes),
	)
	dash.
		GetOrCreateChart("Memory usage, bytes").
		Stacked().
		Sorted().
		AddSeries("free", node.MemoryFreeBytes, "light-blue").
		AddSeries("cache", node.MemoryCachedBytes, "amber").
		AddSeries("used", used, "red")

	dash.GetOrCreateChart("Memory consumers, bytes").
		Stacked().
		SetThreshold("total", node.MemoryTotalBytes, timeseries.Any).
		AddMany(timeseries.Top(utils2.MemoryConsumers(node), timeseries.Max, 5))
	netLatency(dash, w, node)

	for _, i := range node.NetInterfaces {
		dash.
			GetOrCreateChartInGroup("Network bandwidth <selector>, bits/second", i.Name).
			AddSeries("in", timeseries.Map(func(v float64) float64 { return v * 8 }, i.RxBytes), "green").
			AddSeries("out", timeseries.Map(func(v float64) float64 { return v * 8 }, i.TxBytes), "blue")
	}

	return dash
}

func netLatency(dash *widgets.Dashboard, w *model.World, n *model.Node) {
	zones := map[string]*avgTimeSeries{}
	nodes := map[string]*avgTimeSeries{}

	srcAZ := nodeAZ(n)

	update := func(m map[string]*avgTimeSeries, k string, rtt timeseries.TimeSeries) {
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
				if u.Rtt == nil || u.RemoteInstance == nil || u.RemoteInstance.Node == nil {
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

	azChart := dash.GetOrCreateChartInGroup("Network RTT between <selector>, seconds", "availability zones")
	for name, avg := range zones {
		azChart.AddSeries(name, avg.get())
	}
	nodesChart := dash.GetOrCreateChartInGroup("Network RTT between <selector>, seconds", "nodes")
	for name, avg := range nodes {
		nodesChart.AddSeries(name, avg.get())
	}
}

type avgTimeSeries struct {
	sum   *timeseries.AggregatedTimeseries
	count *timeseries.AggregatedTimeseries
}

func newAvgTimeSeries() *avgTimeSeries {
	return &avgTimeSeries{
		sum:   timeseries.Aggregate(timeseries.NanSum),
		count: timeseries.Aggregate(timeseries.NanSum),
	}
}

func (a *avgTimeSeries) add(x timeseries.TimeSeries) {
	a.sum.AddInput(x)
	a.count.AddInput(timeseries.Map(timeseries.Defined, x))
}

func (a *avgTimeSeries) get() timeseries.TimeSeries {
	return timeseries.Aggregate(timeseries.Div, a.sum, a.count)
}

func nodeAZ(n *model.Node) string {
	az := n.AvailabilityZone.Value()
	if az == "" {
		az = "unspecified"
	}
	return az
}
