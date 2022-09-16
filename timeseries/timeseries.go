package timeseries

import (
	"encoding/json"
	"math"
)

var NaN = math.NaN()

type TimeSeries interface {
	Last() float64
	String() string
	Len() int
	Iter() Iterator

	MarshalJSON() ([]byte, error)

	IsEmpty() bool
}

func Reduce(f F, ts TimeSeries) float64 {
	if ts == nil {
		return NaN
	}
	accumulator := NaN
	iter := ts.Iter()
	for iter.Next() {
		_, v := iter.Value()
		accumulator = f(accumulator, v)
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

func Map(f func(v float64) float64, x TimeSeries) TimeSeries {
	return Aggregate(
		func(agg, v float64) float64 { return f(v) },
		x,
	)
}

//func Replace(x TimeSeries, newValue float64) TimeSeries {
//	return Map(func(v float64) float64 {
//		return newValue
//	}, x)
//}

func LastNotNull(ts TimeSeries) (Time, float64) {
	if ts == nil {
		return 0, NaN
	}
	iter := ts.Iter()
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
