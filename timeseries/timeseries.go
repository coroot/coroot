package timeseries

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

var NaN = float32(math.NaN())

func IsNaN(v float32) bool {
	return v != v
}

func IsInf(f float32, sign int) bool {
	return sign >= 0 && f > math.MaxFloat32 || sign <= 0 && f < -math.MaxFloat32
}

type TimeSeries struct {
	from Time
	step Duration
	data []float32
	last float32
}

func New(from Time, pointsCount int, step Duration) *TimeSeries {
	data := make([]float32, pointsCount)
	for i := range data {
		data[i] = NaN
	}
	return NewWithData(from, step, data)
}

func NewWithData(from Time, step Duration, data []float32) *TimeSeries {
	ts := &TimeSeries{
		from: from,
		step: step,
		data: data,
		last: data[len(data)-1],
	}
	return ts
}

func (ts *TimeSeries) Len() int {
	if ts.IsEmpty() {
		return 0
	}
	return len(ts.data)
}

func (ts *TimeSeries) MarshalJSON() ([]byte, error) {
	if ts.IsEmpty() {
		return json.Marshal(nil)
	}
	vs := make([]Value, 0, ts.Len())
	iter := ts.Iter()
	for iter.Next() {
		_, v := iter.Value()
		vs = append(vs, Value(v))
	}
	if len(vs) == 0 {
		return json.Marshal(nil)
	}
	d, err := json.Marshal(vs)
	return d, err
}

func (ts *TimeSeries) String() string {
	if ts.IsEmpty() {
		return "TimeSeries(nil)"
	}
	values := make([]string, 0, ts.Len())
	iter := ts.Iter()
	for iter.Next() {
		_, v := iter.Value()
		values = append(values, Value(v).String())
	}
	return fmt.Sprintf("TimeSeries(%d, %d, %d, [%s])", ts.from, ts.Len(), ts.step, strings.Join(values, " "))
}

func (ts *TimeSeries) Get() *TimeSeries {
	return ts
}

func (ts *TimeSeries) Set(t Time, v float32) {
	t = t.Truncate(ts.step)
	if t < ts.from {
		return
	}
	idx := int((t - ts.from) / Time(ts.step))
	l := len(ts.data) - 1
	if idx <= l {
		ts.data[idx] = v
		if idx == l {
			ts.last = v
		}
	}
}

func (ts *TimeSeries) Fill(from Time, step Duration, data []float32) bool {
	changed := false
	to := ts.from.Add(Duration(ts.Len()-1) * ts.step)

	tNext := Time(0)
	iNext := -1
	var v float32
	t := from.Add(-step)
	for i := range data {
		t = t.Add(step)
		if t > to {
			break
		}
		if t < ts.from {
			continue
		}
		if t < tNext {
			continue
		}
		if iNext == -1 {
			iNext = int((t - ts.from) / Time(ts.step))
			tNext = t.Truncate(ts.step)
		}
		l := len(ts.data) - 1
		if iNext <= l {
			v = data[i]
			if !IsNaN(v) {
				ts.data[iNext] = v
				changed = true
				if iNext == l {
					ts.last = v
				}
			}
			tNext = tNext.Add(ts.step)
			iNext++
		}
	}
	return changed
}

func (ts *TimeSeries) Iter() *Iterator {
	if ts.IsEmpty() {
		return &Iterator{data: nil}
	}
	return &Iterator{
		step: ts.step,
		data: ts.data,
		idx:  -1,
		t:    ts.from.Add(-ts.step),
	}
}

func (ts *TimeSeries) IterFrom(from Time) *Iterator {
	if ts.IsEmpty() || from.Before(ts.from) {
		return &Iterator{data: nil}
	}
	to := ts.from.Add(ts.step * Duration(len(ts.data)-1))
	if from.After(to) {
		return &Iterator{data: nil}
	}
	from = from.Truncate(ts.step)
	return &Iterator{
		step: ts.step,
		data: ts.data,
		idx:  int(from.Sub(ts.from)/ts.step) - 1,
		t:    from.Add(-ts.step),
	}
}

func (ts *TimeSeries) IsEmpty() bool {
	return ts == nil
}

func (ts *TimeSeries) Last() float32 {
	if ts.IsEmpty() {
		return NaN
	}
	return ts.last
}

func (ts *TimeSeries) TailIsEmpty() bool {
	if ts.IsEmpty() {
		return true
	}
	if !IsNaN(ts.last) {
		return false
	}
	l := len(ts.data)
	if l >= 2 && !IsNaN(ts.data[l-2]) {
		return false
	}
	if l >= 3 && !IsNaN(ts.data[l-3]) {
		return false
	}
	return true
}

func (ts *TimeSeries) Reduce(f F) float32 {
	if ts.IsEmpty() {
		return NaN
	}
	accumulator := NaN
	iter := ts.Iter()
	for iter.Next() {
		t, v := iter.Value()
		accumulator = f(t, accumulator, v)
	}
	return accumulator
}

func (ts *TimeSeries) Map(f func(t Time, v float32) float32) *TimeSeries {
	if ts.IsEmpty() {
		return nil
	}

	data := make([]float32, ts.Len())
	iter := ts.Iter()
	i := 0
	for iter.Next() {
		t, v := iter.Value()
		data[i] = f(t, v)
		i++
	}
	return NewWithData(ts.from, ts.step, data)
}

func (ts *TimeSeries) MapInPlace(f func(t Time, v float32) float32) *TimeSeries {
	if ts.IsEmpty() {
		return nil
	}

	t := ts.from
	for i, v := range ts.data {
		ts.data[i] = f(t, v)
		t = t.Add(ts.step)
	}
	return ts
}

func (ts *TimeSeries) WithNewValue(newValue float32) *TimeSeries {
	if ts.IsEmpty() {
		return nil
	}

	data := make([]float32, ts.Len())
	for i := range data {
		data[i] = newValue
	}
	return NewWithData(ts.from, ts.step, data)
}

func (ts *TimeSeries) NewWithData(data []float32) *TimeSeries {
	if ts.IsEmpty() {
		return nil
	}
	return NewWithData(ts.from, ts.step, data)
}

func (ts *TimeSeries) LastNotNull() (Time, float32) {
	if ts.IsEmpty() {
		return 0, NaN
	}
	var vv float32
	var tt Time
	iter := ts.Iter()
	for iter.Next() {
		t, v := iter.Value()
		if !IsNaN(v) {
			vv = v
			tt = t
		}
	}
	return tt, vv
}

func Increase(x, status *TimeSeries) *TimeSeries {
	if x.IsEmpty() || status.IsEmpty() {
		return nil
	}
	data := make([]float32, 0, x.Len())
	prev, prevStatus := NaN, NaN
	iter := x.Iter()
	statusIter := status.Iter()
	var d float32
	for iter.Next() && statusIter.Next() {
		_, v := iter.Value()
		d = NaN
		switch {
		case !IsNaN(v) && !IsNaN(prev):
			if v-prev >= 0 {
				d = v - prev
			} else {
				d = v
			}
		case IsNaN(prev) && prevStatus == 1:
			d = v
		}
		prev = v
		_, prevStatus = statusIter.Value()
		data = append(data, d)
	}
	return NewWithData(x.from, x.step, data)
}

func Aggregate2(x, y *TimeSeries, f func(x, y float32) float32) *TimeSeries {
	if x.IsEmpty() || y.IsEmpty() {
		return nil
	}
	data := make([]float32, x.Len())
	xIter := x.Iter()
	yIter := y.Iter()
	i := 0
	for xIter.Next() && yIter.Next() {
		_, xv := xIter.Value()
		_, yv := yIter.Value()
		data[i] = f(xv, yv)
		i++
	}
	return NewWithData(x.from, x.step, data)
}

func Mul(x, y *TimeSeries) *TimeSeries {
	return Aggregate2(x, y, func(x, y float32) float32 { return x * y })
}

func Div(x, y *TimeSeries) *TimeSeries {
	return Aggregate2(x, y, func(x, y float32) float32 { return x / y })
}

func Sub(x, y *TimeSeries) *TimeSeries {
	return Aggregate2(x, y, func(x, y float32) float32 { return x - y })
}

func Sum(x, y *TimeSeries) *TimeSeries {
	return Aggregate2(x, y, func(x, y float32) float32 { return x + y })
}
