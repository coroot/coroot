package model

import (
	"encoding/json"

	"github.com/coroot/coroot/timeseries"
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
	if t < lv.t || timeseries.IsNaN(v) {
		return
	}
	lv.v = value
	lv.t = t
}

func (lv LabelLastValue) MarshalJSON() ([]byte, error) {
	return json.Marshal(lv.v)
}
