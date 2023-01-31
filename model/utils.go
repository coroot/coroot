package model

import (
	"encoding/json"
	"github.com/coroot/coroot/timeseries"
	"math"
)

type LabelLastValue struct {
	v string
	t timeseries.Time
}

func (lv LabelLastValue) Value() string {
	return lv.v
}

func (lv *LabelLastValue) Update(ts *timeseries.TimeSeries, value string) {
	t, v := ts.LastNotNull()
	if t < lv.t || math.IsNaN(v) {
		return
	}
	lv.v = value
	lv.t = t
}

func (lv LabelLastValue) MarshalJSON() ([]byte, error) {
	return json.Marshal(lv.v)
}

func DataIsMissing(ts *timeseries.TimeSeries) bool {
	for _, v := range ts.LastN(3) {
		if !math.IsNaN(v) {
			return false
		}
	}
	return true
}
