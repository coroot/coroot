package timeseries

import (
	"math"
	"strings"
)

func Increase(x, status TimeSeries) TimeSeries {
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

func (ts *IncreaseTimeseries) Range() Context {
	return ts.input.Range()
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
	var statusIter Iterator
	if ts.status != nil {
		statusIter = ts.status.Iter()
	} else {
		ctx := ts.Range()
		statusIter = &NanIterator{startTs: ctx.From, endTs: ctx.To, step: ctx.Step}
	}
	return &increaseIterator{input: ts.input.Iter(), status: statusIter, prev: NaN}
}

func (ts *IncreaseTimeseries) MarshalJSON() ([]byte, error) {
	return MarshalJSON(ts)
}
