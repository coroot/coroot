package timeseries

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIncrease(t *testing.T) {
	x := NewWithData(0, 1, []float32{NaN, 1, 1, 1, 2, 2, 2, NaN, NaN, 10, NaN, 11, 12})
	status := NewWithData(0, 1, []float32{1, 1, 1, 1, 1, 1, 1, NaN, 1, 1, 0, 1, 1})
	assert.Equal(t, "TimeSeries(0, 13, 1, [. 1 0 0 1 0 0 . . 10 . . 1])", Increase(x, status).String())
}

func TestIterFrom(t *testing.T) {
	ts := NewWithData(60, 15, []float32{1, 2, 3})
	iter := ts.IterFrom(30)
	assert.False(t, iter.Next())

	iter = ts.IterFrom(60)
	assert.True(t, iter.Next())
	tt, v := iter.Value()
	assert.Equal(t, Time(60), tt)
	assert.Equal(t, float32(1), v)

	iter = ts.IterFrom(70)
	assert.True(t, iter.Next())
	tt, v = iter.Value()
	assert.Equal(t, Time(60), tt)
	assert.Equal(t, float32(1), v)

	iter = ts.IterFrom(75)
	assert.True(t, iter.Next())
	tt, v = iter.Value()
	assert.Equal(t, Time(75), tt)
	assert.Equal(t, float32(2), v)

	iter = ts.IterFrom(90)
	assert.True(t, iter.Next())
	tt, v = iter.Value()
	assert.Equal(t, Time(90), tt)
	assert.Equal(t, float32(3), v)

	iter = ts.IterFrom(100)
	assert.False(t, iter.Next())
}
