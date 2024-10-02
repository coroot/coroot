package model

import (
	"fmt"

	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/dustin/go-humanize"
)

type AvailabilitySLI struct {
	Config CheckConfigSLOAvailability

	TotalRequests  *timeseries.TimeSeries
	FailedRequests *timeseries.TimeSeries

	TotalRequestsRaw  *timeseries.TimeSeries
	FailedRequestsRaw *timeseries.TimeSeries
}

type HistogramBucket struct {
	Le         float32
	TimeSeries *timeseries.TimeSeries
}

type LatencySLI struct {
	Config CheckConfigSLOLatency

	Histogram    []HistogramBucket
	HistogramRaw []HistogramBucket
}

func (sli *LatencySLI) GetTotalAndFast(raw bool) (*timeseries.TimeSeries, *timeseries.TimeSeries) {
	var total, fast *timeseries.TimeSeries
	histogram := sli.Histogram
	if raw {
		histogram = sli.HistogramRaw
	}
	for _, b := range histogram {
		if b.Le <= sli.Config.ObjectiveBucket {
			fast = b.TimeSeries
		}
		if timeseries.IsInf(b.Le, 1) {
			total = b.TimeSeries
		}
	}
	return total, fast
}

func HistogramSeries(buckets []HistogramBucket, objectiveBucket, objectivePercentage float32) []Series {
	res := make([]Series, 0, len(buckets))
	var from, to float32
	thresholdIdx := -1
	for i, b := range buckets {
		var h Series
		to = b.Le
		if i == 0 {
			from = 0
			h.Data = b.TimeSeries
		} else {
			from = buckets[i-1].Le
			h.Data = timeseries.Sub(b.TimeSeries, buckets[i-1].TimeSeries).MapInPlace(func(t timeseries.Time, v float32) float32 {
				if v < 0 {
					v = 0.
				}
				return v
			})
		}
		h.Color = "green"
		if objectiveBucket > 0 && objectivePercentage > 0 {
			if to > objectiveBucket {
				h.Color = "red"
			} else {
				thresholdIdx = i
			}
		}
		h.Value = fmt.Sprint(to)
		switch {
		case timeseries.IsInf(to, 0):
			h.Name = fmt.Sprintf(">%ss", humanize.Ftoa(float64(from)))
			h.Title = fmt.Sprintf(">%s s", humanize.Ftoa(float64(from)))
			h.Value = "inf"
		case from < 0.1:
			h.Name = fmt.Sprintf("%.0fms", to*1000)
			h.Title = fmt.Sprintf("%.0f-%.0f ms", from*1000, to*1000)
		default:
			h.Name = fmt.Sprintf("%ss", humanize.Ftoa(float64(to)))
			h.Title = fmt.Sprintf("%s-%s s", humanize.Ftoa(float64(from)), humanize.Ftoa(float64(to)))
		}
		res = append(res, h)
	}
	if thresholdIdx > -1 {
		res[thresholdIdx].Threshold = fmt.Sprintf(
			"<b>Latency objective</b><br> %s of requests should be served faster than %s",
			utils.FormatPercentage(objectivePercentage), utils.FormatLatency(objectiveBucket))
	}
	return res
}

func Quantile(histogram []HistogramBucket, q float32) *timeseries.TimeSeries {
	if len(histogram) == 0 {
		return nil
	}
	total := histogram[len(histogram)-1]
	type bucket struct {
		iter *timeseries.Iterator
		le   float32
	}
	var buckets []bucket
	for _, b := range histogram {
		buckets = append(buckets, bucket{
			iter: b.TimeSeries.Iter(),
			le:   b.Le,
		})
	}
	res := make([]float32, total.TimeSeries.Len())
	idx := 0
	totalIter := total.TimeSeries.Iter()
	var t, c, rank float32
	var i int
	var b bucket
	for totalIter.Next() {
		_, t = totalIter.Value()
		rank = t * q
		for _, b = range buckets {
			b.iter.Next()
		}
		var prev, lower, upper, bc float32
		for i, b = range buckets {
			upper = b.le
			if i > 0 {
				_, prev = buckets[i-1].iter.Value()
				lower = buckets[i-1].le
			}
			_, c = b.iter.Value()
			if timeseries.IsNaN(c) {
				c = 0.
			}
			if c < rank {
				continue
			}
			bc = c - prev
			if timeseries.IsInf(upper, 1) {
				res[idx] = lower
			} else {
				res[idx] = lower + (upper-lower)*((rank-prev)/bc)
			}
			break
		}
		idx++
	}

	return total.TimeSeries.NewWithData(res)
}
