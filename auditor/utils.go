package auditor

import (
	"fmt"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/dustin/go-humanize"
)

func memoryConsumers(node *model.Node) map[string]timeseries.TimeSeries {
	usageByApp := map[string]timeseries.TimeSeries{}
	for _, instance := range node.Instances {
		for _, c := range instance.Containers {
			usageByApp[instance.OwnerId.Name] = timeseries.Merge(usageByApp[instance.OwnerId.Name], c.MemoryRss, timeseries.NanSum)
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
		for _, c := range instance.Containers {
			usageByApp[instance.OwnerId.Name] = timeseries.Merge(usageByApp[instance.OwnerId.Name], c.CpuUsage, timeseries.NanSum)
		}
	}
	return usageByApp
}

func histogramSeries(histogram []model.HistogramBucket, objectiveBucket float64) []*model.Series {
	if len(histogram) < 1 {
		return nil
	}
	var res []*model.Series
	for i, b := range histogram {
		color := "green"
		if objectiveBucket > 0 && b.Le > objectiveBucket {
			color = "red"
		}
		data := b.TimeSeries
		legend := ""
		if i == 0 {
			legend = fmt.Sprintf("0-%.0f ms", b.Le*1000)
		} else {
			prev := histogram[i-1]
			data = timeseries.Aggregate(timeseries.Sub, data, prev.TimeSeries)
			if prev.Le >= 0.1 {
				legend = fmt.Sprintf("%s-%s s", humanize.Ftoa(prev.Le), humanize.Ftoa(b.Le))
			} else {
				legend = fmt.Sprintf("%.0f-%.0f ms", prev.Le*1000, b.Le*1000)
			}
		}
		res = append(res, &model.Series{Name: legend, Data: data, Color: color})
	}
	return res
}
