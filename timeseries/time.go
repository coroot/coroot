package timeseries

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

const (
	Second = Duration(1)
	Minute = Second * 60
	Hour   = Minute * 60
	Day    = Hour * 24
	Month  = Day * 30
)

type Context struct {
	From    Time     `json:"from"`
	To      Time     `json:"to"`
	Step    Duration `json:"step"`
	RawStep Duration `json:"raw_step"`
}

type Duration int64

func DurationFromStandard(d time.Duration) Duration {
	return Duration(d / time.Second)
}

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
	if err := json.Unmarshal(b, &i); err == nil {
		*d = Duration(i / 1000)
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		if td, err := time.ParseDuration(s); err == nil {
			*d = Duration(td.Seconds())
			return nil
		}
	}
	return fmt.Errorf("invalid duration: %s", string(b))
}

type Time int64

func TimeFromStandard(t time.Time) Time {
	return Time(t.Unix())
}

func Now() Time {
	return Time(time.Now().Unix())
}

func Since(t Time) Duration {
	return Now().Sub(t)
}

func (t Time) IsZero() bool {
	return t == 0
}

func (t Time) Truncate(d Duration) Time {
	return t - t%Time(d)
}

func (t Time) Sub(other Time) Duration {
	return Duration(t - other)
}

func (t Time) Add(d Duration) Time {
	return t + Time(d)
}

func (t Time) Before(other Time) bool {
	return t < other
}

func (t Time) After(other Time) bool {
	return t > other
}

func (t Time) ToStandard() time.Time {
	return time.Unix(int64(t), 0).UTC()
}

func (t Time) String() string {
	return strconv.FormatInt(int64(t), 10)
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
