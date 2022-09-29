package auditor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func memoryConsumers(node *model.Node) map[string]timeseries.TimeSeries {
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

func cpuByModeSeries(modes map[string]timeseries.TimeSeries) []*model.Series {
	var res []*model.Series
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
		res = append(res, &model.Series{Name: mode, Color: color, Data: v})
	}
	return res
}

func cpuConsumers(node *model.Node) map[string]timeseries.TimeSeries {
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
	return usageByApp
}
