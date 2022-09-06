package utils

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func MemoryConsumers(node *model.Node) map[string]timeseries.TimeSeries {
	usageByApp := map[string]timeseries.TimeSeries{}
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
	return usageByApp
}
