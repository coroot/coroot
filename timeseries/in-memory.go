package timeseries

import (
	"fmt"
	"strings"
)

type InMemoryTimeSeries struct {
	from Time
	step Duration
	data []float64
}

func (ts *InMemoryTimeSeries) len() int {
	return len(ts.data)
}

func (ts *InMemoryTimeSeries) last() float64 {
	if len(ts.data) == 0 {
		return NaN
	}
	return ts.data[len(ts.data)-1]
}

func (ts *InMemoryTimeSeries) iter() Iterator {
	return &timeseriesIterator{
		from: ts.from,
		step: ts.step,
		data: ts.data,
		idx:  -1,
	}
}

func (ts *InMemoryTimeSeries) isEmpty() bool {
	return len(ts.data) == 0
}

func (ts *InMemoryTimeSeries) MarshalJSON() ([]byte, error) {
	return MarshalJSON(ts)
}

func (ts *InMemoryTimeSeries) String() string {
	values := make([]string, 0)
	iter := ts.iter()
	for iter.Next() {
		_, v := iter.Value()
		values = append(values, Value(v).String())
	}
	return fmt.Sprintf("InMemoryTimeSeries(%d, %d, %d, [%s])", ts.from, ts.len(), ts.step, strings.Join(values, " "))
}

func (ts *InMemoryTimeSeries) Data() []float64 {
	return ts.data
}

func (ts *InMemoryTimeSeries) Set(t Time, v float64) Time {
	t = t.Truncate(ts.step)
	if t < ts.from {
		return t
	}
	idx := int((t - ts.from) / Time(ts.step))
	if idx < len(ts.data) {
		ts.data[idx] = v
	}
	return t
}

func (ts *InMemoryTimeSeries) CopyFrom(other *InMemoryTimeSeries) {
	copy(ts.data, other.data)
}

func New(from Time, pointsCount int, step Duration) *InMemoryTimeSeries {
	data := make([]float64, pointsCount)
	for i := range data {
		data[i] = NaN
	}
	return NewWithData(from, step, data)
}

func NewWithData(from Time, step Duration, data []float64) *InMemoryTimeSeries {
	return &InMemoryTimeSeries{
		from: from,
		step: step,
		data: data,
	}
}

type timeseriesIterator struct {
	from Time
	step Duration
	data []float64
	idx  int

	t Time
	v float64
}

func (i *timeseriesIterator) Next() bool {
	i.idx++
	if i.idx >= len(i.data) {
		return false
	}
	i.t = i.from.Add(Duration(i.idx) * i.step)
	if i.data != nil {
		i.v = i.data[i.idx]
	}
	return true
}

func (i *timeseriesIterator) Value() (Time, float64) {
	return i.t, i.v
}
