package auditor

import (
	"fmt"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/dustin/go-humanize"
	"k8s.io/klog"
	"sort"
	"strconv"
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

type latencyBucket struct {
	Le         float64
	TimeSeries timeseries.TimeSeries
}

func histogramBuckets(histogram map[string]timeseries.TimeSeries) []latencyBucket {
	buckets := make([]latencyBucket, 0, len(histogram))
	var err error
	for le, ts := range histogram {
		b := latencyBucket{TimeSeries: ts}
		b.Le, err = strconv.ParseFloat(le, 64)
		if err != nil {
			klog.Warningln(err)
			continue
		}
		buckets = append(buckets, b)
	}
	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].Le < buckets[j].Le
	})
	return buckets
}

func histogramSeries(histogram map[string]timeseries.TimeSeries, objectiveBucket string) []*model.Series {
	if len(histogram) < 1 {
		return nil
	}
	buckets := histogramBuckets(histogram)
	obj, _ := strconv.ParseFloat(objectiveBucket, 64)
	var res []*model.Series

	for i := len(buckets) - 1; i > 0; i-- {
		buckets[i].TimeSeries = timeseries.Aggregate(timeseries.Sub, buckets[i].TimeSeries, buckets[i-1].TimeSeries)
	}
	for i, b := range buckets {
		color := "green"
		if obj > 0 && b.Le > obj {
			color = "red"
		}
		legend := ""
		if i == 0 {
			legend = fmt.Sprintf("0-%.0f ms", b.Le*1000)
		} else {
			prev := buckets[i-1]
			if prev.Le >= 0.1 {
				legend = fmt.Sprintf("%s-%s s", humanize.Ftoa(prev.Le), humanize.Ftoa(b.Le))
			} else {
				legend = fmt.Sprintf("%.0f-%.0f ms", prev.Le*1000, b.Le*1000)
			}
		}
		res = append(res, &model.Series{Name: legend, Data: b.TimeSeries, Color: color})
	}
	return res
}
