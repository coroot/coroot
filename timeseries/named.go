package timeseries

type Named struct {
	Name   string
	Series *TimeSeries
}

func WithName(name string, series *TimeSeries) Named {
	return Named{
		Name:   name,
		Series: series,
	}
}
