package timeseries

import (
	"math"
	"sort"
)

type weighted struct {
	Named
	weight float64
}

func Top[T *TimeSeries | *Aggregate](input map[string]T, by F, n int) []Named {
	sortable := make([]weighted, 0, len(input))
	for name, series := range input {
		var ts *TimeSeries
		switch s := any(series).(type) {
		case *TimeSeries:
			ts = s
		case *Aggregate:
			ts = s.Get()
		}
		w := ts.Reduce(by)
		if !math.IsNaN(w) {
			sortable = append(sortable, weighted{Named: WithName(name, ts), weight: w})
		}
	}
	sort.Slice(sortable, func(i, j int) bool {
		return sortable[i].weight > sortable[j].weight
	})
	res := make([]Named, 0, n+1)
	other := NewAggregate(NanSum)
	for i, s := range sortable {
		if (i + 1) < n {
			res = append(res, WithName(s.Name, s.Series))
		} else {
			other.Add(s.Series)
		}
	}
	if otherTs := other.Get(); !otherTs.IsEmpty() {
		res = append(res, WithName("other", otherTs))
	}
	return res
}
