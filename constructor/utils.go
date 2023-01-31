package constructor

import "github.com/coroot/coroot/timeseries"

func merge(dest, ts *timeseries.TimeSeries, f timeseries.F) *timeseries.TimeSeries {
	if dest == nil && ts == nil {
		return nil
	}
	if dest == nil {
		return ts
	}
	return timeseries.NewAggregate(f).Add(dest, ts).Get()
}
