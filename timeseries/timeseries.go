package timeseries

import (
	"encoding/json"
	"fmt"
	"math"
)

var NaN = math.NaN()

type TimeSeries interface {
	len() int
	iter() Iterator
	isEmpty() bool
	last() float64

	fmt.Stringer
	json.Marshaler
}

func Iter(ts TimeSeries) Iterator {
	if ts == nil {
		return &NilIterator{}
	}
	return ts.iter()
}

func IsEmpty(ts TimeSeries) bool {
	if ts == nil {
		return true
	}
	return ts.isEmpty()
}

func Last(ts TimeSeries) float64 {
	if ts == nil {
		return NaN
	}
	return ts.last()
}

func LastN(ts TimeSeries, n int) []float64 {
	if ts == nil || n == 0 {
		return nil
	}
	if n == 1 {
		return []float64{Last(ts)}
	}
	res := make([]float64, n, n)
	iter := ts.iter()
	for iter.Next() {
		_, v := iter.Value()
		for i := 0; i < n-1; i++ {
			res[i] = res[i+1]
		}
		res[n-1] = v
	}
	return res
}

func Merge(src, ts TimeSeries, f F) *AggregatedTimeseries {
	var res *AggregatedTimeseries
	if src == nil {
		res = Aggregate(f)
	} else {
		ats, ok := src.(*AggregatedTimeseries)
		if !ok || ats == nil {
			res = Aggregate(f, src)
		} else {
			res = ats
		}
	}
	res.AddInput(ts)
	return res
}

func Reduce(f F, ts TimeSeries) float64 {
	if ts == nil {
		return NaN
	}
	accumulator := NaN
	iter := ts.iter()
	for iter.Next() {
		t, v := iter.Value()
		accumulator = f(t, accumulator, v)
	}
	return accumulator
}

func Aggregate(f F, tss ...TimeSeries) *AggregatedTimeseries {
	a := &AggregatedTimeseries{
		aggFunc: f,
	}
	a.AddInput(tss...)
	return a
}

func Increase(x, status TimeSeries) TimeSeries {
	if IsEmpty(x) || IsEmpty(status) {
		return nil
	}
	return &IncreaseTimeseries{input: x, status: status}
}

func Map(f func(t Time, v float64) float64, ts TimeSeries) TimeSeries {
	return Aggregate(
		func(t Time, agg, v float64) float64 { return f(t, v) },
		ts,
	)
}

func Replace(ts TimeSeries, newValue float64) TimeSeries {
	return Map(func(t Time, v float64) float64 {
		return newValue
	}, ts)
}

func LastNotNull(ts TimeSeries) (Time, float64) {
	if ts == nil {
		return 0, NaN
	}
	iter := ts.iter()
	var vv float64
	var tt Time
	for iter.Next() {
		t, v := iter.Value()
		if !math.IsNaN(v) {
			vv = v
			tt = t
		}
	}
	return tt, vv
}

func MarshalJSON(ts TimeSeries) ([]byte, error) {
	if ts == nil {
		return json.Marshal(nil)
	}
	vs := make([]Value, 0, ts.len())
	iter := ts.iter()
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
