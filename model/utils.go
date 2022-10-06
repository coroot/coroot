package model

import (
	"encoding/json"
	"fmt"
	"github.com/coroot/coroot/timeseries"
	"math"
	"strconv"
)

type LabelLastValue struct {
	v string
	t timeseries.Time
}

func (lv LabelLastValue) Value() string {
	return lv.v
}

func (lv *LabelLastValue) Update(ts timeseries.TimeSeries, value string) {
	t, v := timeseries.LastNotNull(ts)
	if t < lv.t || math.IsNaN(v) {
		return
	}
	lv.v = value
	lv.t = t
}

func (lv LabelLastValue) MarshalJSON() ([]byte, error) {
	return json.Marshal(lv.v)
}

func FormatLatencyBucket(bucket string) string {
	b := fmt.Sprintf(`%ss`, bucket)
	if v, err := strconv.ParseFloat(bucket, 64); err == nil && v < 1 {
		b = fmt.Sprintf(`%.fms`, v*1000)
	}
	return b
}
