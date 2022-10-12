package timeseries

import (
	"math"
	"strings"
)

type increaseIterator struct {
	input  Iterator
	status Iterator
	prev   float64
}

func (i *increaseIterator) Next() bool {
	return i.status.Next() && i.input.Next()
}

func (i *increaseIterator) Value() (Time, float64) {
	ts, v := i.input.Value()
	if !math.IsNaN(v) {
		if !math.IsNaN(i.prev) {
			d := v - i.prev
			i.prev = v
			if d >= 0 {
				return ts, d
			}
		} else {
			i.prev = v
		}
	}
	_, s := i.status.Value()
	if math.IsNaN(i.prev) && s == 1 {
		i.prev = 0
	}

	return ts, NaN
}

type IncreaseTimeseries struct {
	input  TimeSeries
	status TimeSeries
}

func (ts *IncreaseTimeseries) len() int {
	return ts.input.len()
}

func (ts *IncreaseTimeseries) last() float64 {
	return Reduce(func(t Time, accumulator, v float64) float64 {
		return v
	}, ts)
}

func (ts *IncreaseTimeseries) isEmpty() bool {
	return ts.input == nil || ts.input.isEmpty()
}

func (ts *IncreaseTimeseries) String() string {
	values := make([]string, 0)
	iter := ts.iter()
	for iter.Next() {
		_, v := iter.Value()
		values = append(values, Value(v).String())
	}
	return "IncreaseTimeseries(" + strings.Join(values, " ") + ")"
}

func (ts *IncreaseTimeseries) iter() Iterator {
	return &increaseIterator{input: ts.input.iter(), status: ts.status.iter(), prev: NaN}
}

func (ts *IncreaseTimeseries) MarshalJSON() ([]byte, error) {
	return MarshalJSON(ts)
}
