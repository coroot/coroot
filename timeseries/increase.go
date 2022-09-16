package timeseries

import (
	"math"
	"strings"
)

func Increase(x, status TimeSeries) TimeSeries {
	if x == nil || x.IsEmpty() || status == nil || status.IsEmpty() {
		return nil
	}
	return &IncreaseTimeseries{input: x, status: status}
}

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

func (ts *IncreaseTimeseries) Len() int {
	return ts.input.Len()
}

func (ts *IncreaseTimeseries) Last() float64 {
	return Reduce(Last, ts)
}

func (ts *IncreaseTimeseries) IsEmpty() bool {
	return ts.input == nil || ts.input.IsEmpty()
}

func (ts *IncreaseTimeseries) String() string {
	values := make([]string, 0)
	iter := ts.Iter()
	for iter.Next() {
		_, v := iter.Value()
		values = append(values, Value(v).String())
	}
	return "IncreaseTimeseries(" + strings.Join(values, " ") + ")"
}

func (ts *IncreaseTimeseries) Iter() Iterator {
	return &increaseIterator{input: ts.input.Iter(), status: ts.status.Iter(), prev: NaN}
}

func (ts *IncreaseTimeseries) MarshalJSON() ([]byte, error) {
	return MarshalJSON(ts)
}
