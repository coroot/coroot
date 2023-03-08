package auditor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func memoryConsumers(node *model.Node) map[string]model.SeriesData {
	usageByApp := map[string]model.SeriesData{}
	for _, instance := range node.Instances {
		agg := timeseries.NewAggregate(timeseries.NanSum)
		for _, c := range instance.Containers {
			agg.Add(c.MemoryRss)
		}
		usageByApp[instance.OwnerId.Name] = agg
	}
	return usageByApp
}

func cpuConsumers(node *model.Node) map[string]model.SeriesData {
	usageByApp := map[string]model.SeriesData{}
	for _, instance := range node.Instances {
		agg := timeseries.NewAggregate(timeseries.NanSum)
		for _, c := range instance.Containers {
			agg.Add(c.CpuUsage)
		}
		usageByApp[instance.OwnerId.Name] = agg
	}
	return usageByApp
}

func cpuByModeChart(ch *model.Chart, modes map[string]*timeseries.TimeSeries) {
	ch.Sorted()
	ch.Stacked()
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
		ch.AddSeries(mode, v, color)
	}
}
