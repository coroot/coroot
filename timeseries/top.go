package timeseries

import (
	"math"
	"sort"
)

type weighted struct {
	Named
	weight float64
}

func Top(input map[string]TimeSeries, by F, n int) []Named {
	sortable := make([]weighted, 0, len(input))
	for name, series := range input {
		if w := Reduce(by, series); !math.IsNaN(w) {
			sortable = append(sortable, weighted{Named: WithName(name, series), weight: w})
		}
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
