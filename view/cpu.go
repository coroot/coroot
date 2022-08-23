package view

import (
	"github.com/coroot/coroot-focus/model"
	"github.com/coroot/coroot-focus/timeseries"
)

func cpu(app *model.Application) *Dashboard {
	dash := &Dashboard{Name: "CPU"}
	relevantNodes := map[string]*model.Node{}

	for _, i := range app.Instances {
		for _, c := range i.Containers {
			dash.GetOrCreateChartInGroup("CPU usage <selector>, cores", c.Name).
				AddSeries(c.Name, c.CpuUsage).
				SetThreshold("limit", c.CpuLimit, timeseries.Max)
			dash.GetOrCreateChartInGroup("CPU delay <selector>, seconds/second", c.Name).AddSeries(c.Name, c.CpuDelay)
			dash.GetOrCreateChartInGroup("Throttled time <selector>, seconds/second", c.Name).AddSeries(c.Name, c.ThrottledTime)
		}
		if node := i.Node; i.Node != nil {
			nodeName := node.Name.Value()
			if relevantNodes[nodeName] == nil {
				relevantNodes[nodeName] = i.Node
				dash.GetOrCreateChartInGroup("Node CPU usage <selector>, %", "overview").
					AddSeries(nodeName, i.Node.CpuUsagePercent).
					Feature()

				byMode := dash.GetOrCreateChartInGroup("Node CPU usage <selector>, %", nodeName).Sorted().Stacked()
				for _, s := range cpuByMode(node.CpuUsageByMode) {
					byMode.Series = append(byMode.Series, s)
				}

				usageByApp := map[string]timeseries.TimeSeries{}
				for _, instance := range node.Instances {
					appUsage := usageByApp[instance.OwnerId.Name]
					if appUsage == nil {
						appUsage = timeseries.Aggregate(timeseries.NanSum)
						usageByApp[instance.OwnerId.Name] = appUsage
					}
					for _, c := range instance.Containers {
						appUsage.(*timeseries.AggregatedTimeseries).AddInput(c.CpuUsage)
					}
				}
				dash.GetOrCreateChartInGroup("CPU consumers on <selector>, cores", nodeName).
					Stacked().
					SetThreshold("total", node.MemoryTotalBytes, timeseries.Any).
					AddMany(timeseries.TopByCumSum(usageByApp, 5, 1))

			}
		}
	}
	return dash
}

func cpuByMode(modes map[string]timeseries.TimeSeries) []*Series {
	var res []*Series
	for _, mode := range []string{"user", "nice", "system", "wait", "iowait", "steal", "irq", "softirq"} {
		v, ok := modes[mode]
		if !ok {
			continue
		}
		var color string
		switch mode {
		case "user":
			color = "blue"
		case "system":
			color = "red"
		case "wait", "iowait":
			color = "orange"
		case "steal":
			color = "black"
		case "irq":
			color = "grey"
		case "softirq":
			color = "yellow"
		case "nice":
			color = "lightGreen"
		}
		res = append(res, &Series{Name: mode, Color: color, Data: v})
	}
	return res
}
