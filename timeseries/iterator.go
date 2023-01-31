package timeseries

type Iterator struct {
	from Time
	step Duration
	data []float64
	idx  int

	t Time
	v float64
}

func (i *Iterator) Next() bool {
	if len(i.data) == 0 {
		return false
	}
	i.idx++
	i.t = i.t.Add(i.step)
	if i.idx >= len(i.data) {
		return false
	}
	if i.data != nil {
		i.v = i.data[i.idx]
	}
	return true
}

func (i *Iterator) Value() (Time, float64) {
	return i.t, i.v
}
