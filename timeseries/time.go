package timeseries

import (
	"encoding/json"
	"time"
)

const (
	Hour   = Duration(3600)
	Minute = Duration(60)
)

type Context struct {
	From Time     `json:"from"`
	To   Time     `json:"to"`
	Step Duration `json:"step"`
}

type Duration int64

func (d Duration) Truncate(m Duration) Duration {
	if m <= 0 {
		return d
	}
	return d - d%m
}

func (d Duration) ToStandard() time.Duration {
	return time.Duration(d) * time.Second
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(int64(d) * 1000)
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var i int64
	if err := json.Unmarshal(b, &i); err != nil {
		return err
	}
	*d = Duration(i / 1000)
	return nil
}

type Time int64

func Now() Time {
	return Time(time.Now().Unix())
}

func (t Time) IsZero() bool {
	return t == 0
}

func (t Time) Truncate(d Duration) Time {
	return t / Time(d) * Time(d)
}

func (t Time) Sub(other Time) Duration {
	return Duration(t - other)
}

func (t Time) Add(d Duration) Time {
	return t + Time(d)
}

func (t Time) ToStandard() time.Time {
	return time.Unix(int64(t), 0)
}

func (t Time) MarshalJSON() ([]byte, error) {
	if t == 0 {
		return json.Marshal(nil)
	}
	return json.Marshal(int64(t) * 1000)
}

func (t *Time) UnmarshalJSON(b []byte) error {
	var i int64
	if err := json.Unmarshal(b, &i); err != nil {
		return err
	}
	*t = Time(i / 1000)
	return nil
}
