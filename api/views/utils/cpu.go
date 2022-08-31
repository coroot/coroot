package utils

import (
	"github.com/coroot/coroot-focus/api/views/widgets"
	"github.com/coroot/coroot-focus/model"
	"github.com/coroot/coroot-focus/timeseries"
)

func CpuByModeSeries(modes map[string]timeseries.TimeSeries) []*widgets.Series {
	var res []*widgets.Series
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
		res = append(res, &widgets.Series{Name: mode, Color: color, Data: v})
	}
	return res
}

func CpuConsumers(node *model.Node) map[string]timeseries.TimeSeries {
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
