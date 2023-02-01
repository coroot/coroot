package timeseries

import (
	"encoding/json"
	"fmt"
	promModel "github.com/prometheus/common/model"
	"math"
	"strings"
)

var NaN = math.NaN()

type TimeSeries struct {
	from Time
	step Duration
	data []float64
}

func New(from Time, pointsCount int, step Duration) *TimeSeries {
	data := make([]float64, pointsCount)
	for i := range data {
		data[i] = NaN
	}
	return NewWithData(from, step, data)
}

func NewWithData(from Time, step Duration, data []float64) *TimeSeries {
	ts := &TimeSeries{
		from: from,
		step: step,
		data: data,
	}
	return ts
}

func (ts *TimeSeries) len() int {
	if ts.IsEmpty() {
		return 0
	}
	return len(ts.data)
}

func (ts *TimeSeries) MarshalJSON() ([]byte, error) {
	if ts.IsEmpty() {
		return json.Marshal(nil)
	}
	vs := make([]Value, 0, ts.len())
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
	values := make([]string, 0, ts.len())
	iter := ts.Iter()
	for iter.Next() {
		_, v := iter.Value()
		values = append(values, Value(v).String())
	}
	return fmt.Sprintf("TimeSeries(%d, %d, %d, [%s])", ts.from, ts.len(), ts.step, strings.Join(values, " "))
}

func (ts *TimeSeries) FillFromSamplePairs(pairs []promModel.SamplePair) {
	to := ts.from.Add(Duration(ts.len()-1) * ts.step)
	var (
		t Time
		v float64
	)
	for _, p := range pairs {
		t = Time(p.Timestamp.Time().Unix()).Truncate(ts.step)
		v = float64(p.Value)
		if t > to {
			break
		}
		if t < ts.from {
			continue
		}
		idx := int((t - ts.from) / Time(ts.step))
		if idx < len(ts.data) {
			ts.data[idx] = v
		}
	}
}

func (ts *TimeSeries) Fill(from Time, step Duration, data []float64) bool {
	changed := false
	to := ts.from.Add(Duration(ts.len()-1) * ts.step)

	tNext := Time(0)
	iNext := -1
	for i, v := range data {
		t := from.Add(Duration(i) * step)
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
		}
		if iNext < len(ts.data) {
			ts.data[iNext] = v
			tNext = tNext.Add(ts.step)
			iNext++
			if !changed && !math.IsNaN(v) {
				changed = true
			}
		}
	}
	return changed
}

func (ts *TimeSeries) Iter() *Iterator {
	if ts.IsEmpty() {
		return &Iterator{data: nil}
	}
	return &Iterator{
		from: ts.from,
		step: ts.step,
		data: ts.data,
		idx:  -1,
		t:    ts.from.Add(-ts.step),
	}
}

func (ts *TimeSeries) IsEmpty() bool {
	return ts == nil
}

func (ts *TimeSeries) Last() float64 {
	if ts.IsEmpty() {
		return NaN
	}
	return ts.data[len(ts.data)-1]
}

func (ts *TimeSeries) LastN(n int) []float64 {
	res := make([]float64, n)
	for i := range res {
		res[i] = NaN
	}
	if ts.IsEmpty() || n == 0 {
		return res
	}
	iter := ts.Iter()
	l := len(iter.data)
	if i := l - n; i < 0 {
		copy(res[-i:], iter.data)
	} else {
		copy(res, iter.data[i:])
	}
	return res
}

func (ts *TimeSeries) Reduce(f F) float64 {
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

func (ts *TimeSeries) Map(f func(t Time, v float64) float64) *TimeSeries {
	if ts.IsEmpty() {
		return nil
	}
	data := make([]float64, ts.len())
	iter := ts.Iter()
	i := 0
	for iter.Next() {
		t, v := iter.Value()
		data[i] = f(t, v)
		i++
	}
	return NewWithData(ts.from, ts.step, data)
}

func (ts *TimeSeries) WithNewValue(newValue float64) *TimeSeries {
	if ts.IsEmpty() {
		return nil
	}
	data := make([]float64, ts.len())
	for i := range data {
		data[i] = newValue
	}
	return NewWithData(ts.from, ts.step, data)
}

func (ts *TimeSeries) LastNotNull() (Time, float64) {
	if ts.IsEmpty() {
		return 0, NaN
	}
	var vv float64
	var tt Time
	iter := ts.Iter()
	for iter.Next() {
		t, v := iter.Value()
		if !math.IsNaN(v) {
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
	data := make([]float64, 0, x.len())
	prev := NaN
	iter := x.Iter()
	statusIter := status.Iter()
	var d float64
	for iter.Next() && statusIter.Next() {
		_, v := iter.Value()
		d = func() float64 {
			if !math.IsNaN(v) {
				if !math.IsNaN(prev) {
					d := v - prev
					prev = v
					if d >= 0 {
						return d
					}
				}
			}
			prev = v
			_, s := statusIter.Value()
			if math.IsNaN(prev) && s == 1 {
				prev = 0
			}
			return NaN
		}()
		data = append(data, d)
	}
	return NewWithData(x.from, x.step, data)
}

func Aggregate2(x, y *TimeSeries, f func(x, y float64) float64) *TimeSeries {
	if x.IsEmpty() || y.IsEmpty() {
		return nil
	}
	data := make([]float64, x.len())
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
	return Aggregate2(x, y, func(x, y float64) float64 { return x * y })
}

func Div(x, y *TimeSeries) *TimeSeries {
	return Aggregate2(x, y, func(x, y float64) float64 { return x / y })
}

func Sub(x, y *TimeSeries) *TimeSeries {
	return Aggregate2(x, y, func(x, y float64) float64 { return x - y })
}

func Sum(x, y *TimeSeries) *TimeSeries {
	return Aggregate2(x, y, func(x, y float64) float64 { return x + y })
}
