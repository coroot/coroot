package timeseries

import (
	"encoding/json"
	"fmt"
	"math"
)

type Value float64

func (v Value) String() string {
	if math.IsNaN(float64(v)) {
		return "."
	}
	if v == 0 {
		return "0"
	}
	if v == Value(int(v)) {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%f", v)
}

func (v Value) MarshalJSON() ([]byte, error) {
	f := float64(v)
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return json.Marshal(nil)
	}
	return json.Marshal(float32(f))
}
