package timeseries

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestIncrease(t *testing.T) {
	x := NewWithData(0, 1, []float32{NaN, 1, 1, 1, 2, 2, 2, NaN, NaN, 10, NaN, 11, 12})
	status := NewWithData(0, 1, []float32{1, 1, 1, 1, 1, 1, 1, NaN, 1, 1, 0, 1, 1})
	assert.Equal(t, "TimeSeries(0, 13, 1, [. 1 0 0 1 0 0 . . 10 . . 1])", Increase(x, status).String())
}
