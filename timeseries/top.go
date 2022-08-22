package timeseries

import (
	"sort"
)

type weighted struct {
	Named
	weight float64
}

func TopByCumSum(input map[string]TimeSeries, n int, minPercent float64) []Named {
	sortable := make([]weighted, 0, len(input))
	var total float64
	for name, series := range input {
		sum := Reduce(NanSum, series)
		total += sum
		sortable = append(sortable, weighted{Named: WithName(name, series), weight: sum})
	}
	sort.Slice(sortable, func(i, j int) bool {
		return sortable[i].weight > sortable[j].weight
	})
	res := make([]Named, 0, n+1)
	var other *AggregatedTimeseries
	for i, s := range sortable {
		if s.weight/total*100 > minPercent && (i+1) < n {
			res = append(res, WithName(s.Name, s.Series))
		} else {
			if other == nil {
				other = Aggregate(NanSum)
			}
			other.AddInput(s.Series)
		}
	}
	if other != nil {
		res = append(res, WithName("other", other))
	}
	return res
}

func TopByMax(input map[string]TimeSeries, n int) []Named {
	sortable := make([]weighted, 0, len(input))
	for name, series := range input {
		max := Reduce(Max, series)
		sortable = append(sortable, weighted{Named: WithName(name, series), weight: max})
	}
	sort.Slice(sortable, func(i, j int) bool {
		return sortable[i].weight > sortable[j].weight
	})
	res := make([]Named, 0, n+1)
	var other *AggregatedTimeseries
	for i, s := range sortable {
		if (i + 1) < n {
			res = append(res, WithName(s.Name, s.Series))
		} else {
			if other == nil {
				other = Aggregate(NanSum)
			}
			other.AddInput(s.Series)
		}
	}
	if other != nil {
		res = append(res, WithName("other", other))
	}
	return res
}
