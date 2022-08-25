package application

import (
	"github.com/coroot/coroot-focus/model"
	"github.com/coroot/coroot-focus/timeseries"
	"github.com/coroot/coroot-focus/views/widgets"
)

func memory(app *model.Application) *widgets.Dashboard {
	dash := &widgets.Dashboard{Name: "Memory"}
	relevantNodes := map[string]*model.Node{}

	for _, i := range app.Instances {
		oom := timeseries.Aggregate(timeseries.NanSum)
		for _, c := range i.Containers {
			dash.GetOrCreateChartInGroup("Memory usage (RSS) <selector>, bytes", c.Name).
				AddSeries(i.Name, c.MemoryRss).
				SetThreshold("limit", c.MemoryLimit, timeseries.Max)
			oom.AddInput(c.OOMKills)
		}
		dash.GetOrCreateChart("Out of memory events").AddSeries(i.Name, oom)
		if node := i.Node; node != nil {
			nodeName := node.Name.Value()
			if relevantNodes[nodeName] == nil {
				usageByApp := map[string]timeseries.TimeSeries{}
				relevantNodes[nodeName] = node
				dash.GetOrCreateChart("Node memory usage (unreclaimable), bytes").
					AddSeries(
						nodeName,
						timeseries.Aggregate(
							func(avail, total float64) float64 { return (total - avail) / total * 100 },
							node.MemoryAvailableBytes, node.MemoryTotalBytes,
						),
					)
				for _, instance := range node.Instances {
					for _, c := range instance.Containers {
						byApp := usageByApp[instance.OwnerId.Name]
						if byApp == nil {
							byApp = timeseries.Aggregate(timeseries.NanSum)
							usageByApp[instance.OwnerId.Name] = byApp
						}
						byApp.(*timeseries.AggregatedTimeseries).AddInput(c.MemoryRss)
					}
				}
				dash.GetOrCreateChartInGroup("Memory consumers <selector>, bytes", nodeName).
					Stacked().
					SetThreshold("total", node.MemoryTotalBytes, timeseries.Any).
					AddMany(timeseries.Top(usageByApp, timeseries.Max, 5))
			}
		}
	}
	return dash
}
