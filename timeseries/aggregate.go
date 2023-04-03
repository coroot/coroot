package timeseries

type Aggregate struct {
	f     F
	input []*TimeSeries
}

func NewAggregate(f F) *Aggregate {
	return &Aggregate{f: f}
}

func (a *Aggregate) Add(tss ...*TimeSeries) *Aggregate {
	for _, ts := range tss {
		if !ts.IsEmpty() {
			a.input = append(a.input, ts)
		}
	}
	return a
}

func (a *Aggregate) Get() *TimeSeries {
	if a == nil || len(a.input) == 0 {
		return nil
	}
	if len(a.input) == 1 {
		return a.input[0]
	}

	data := make([]float32, a.input[0].Len())
	for i := range data {
		data[i] = NaN
	}
	for _, src := range a.input {
		iter := src.Iter()
		i := 0
		for iter.Next() {
			t, v := iter.Value()
			data[i] = a.f(t, data[i], v)
			i++
		}
	}
	return NewWithData(a.input[0].from, a.input[0].step, data)
}

func (a *Aggregate) IsEmpty() bool {
	return len(a.input) == 0
}

func (a *Aggregate) Reduce(f F) float32 {
	return a.Get().Reduce(f)
}

func (a *Aggregate) MarshalJSON() ([]byte, error) {
	return a.Get().MarshalJSON()
}
