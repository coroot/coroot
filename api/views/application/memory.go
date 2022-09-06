package application

import (
	"github.com/coroot/coroot/api/views/utils"
	"github.com/coroot/coroot/api/views/widgets"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func memory(ctx timeseries.Context, app *model.Application) *widgets.Dashboard {
	dash := widgets.NewDashboard(ctx, "Memory")
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
				relevantNodes[nodeName] = node
				dash.GetOrCreateChart("Node memory usage (unreclaimable), bytes").
					AddSeries(
						nodeName,
						timeseries.Aggregate(
							func(avail, total float64) float64 { return (total - avail) / total * 100 },
							node.MemoryAvailableBytes, node.MemoryTotalBytes,
						),
					)
				dash.GetOrCreateChartInGroup("Memory consumers <selector>, bytes", nodeName).
					Stacked().
					SetThreshold("total", node.MemoryTotalBytes, timeseries.Any).
					AddMany(timeseries.Top(utils.MemoryConsumers(node), timeseries.Max, 5))
			}
		}
	}
	return dash
}
