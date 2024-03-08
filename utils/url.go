package utils

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/coroot/coroot/timeseries"
	"github.com/xhit/go-str2duration/v2"
	"golang.org/x/net/http/httpguts"
	"k8s.io/klog"
)

type Header struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (h Header) Valid() bool {
	return httpguts.ValidHeaderFieldName(h.Key) && httpguts.ValidHeaderFieldValue(h.Value)
}

type BasicAuth struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

func (ba *BasicAuth) AddTo(address string) (string, error) {
	if ba == nil {
		return address, nil
	}
	u, err := url.Parse(address)
	if err != nil {
		return "", err
	}
	u.User = url.UserPassword(ba.User, ba.Password)
	return u.String(), nil
}

func ParseTime(now timeseries.Time, val string, def timeseries.Time) timeseries.Time {
	if val == "" {
		return def
	}
	if strings.HasPrefix(val, "now") {
		if val == "now" {
			return now
		}
		d, err := str2duration.ParseDuration(val[3:])
		if err != nil {
			klog.Warningf("invalid %s: %s", val, err)
			return def
		}
		return now.Add(timeseries.Duration(d.Seconds()))
	}
	ms, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		klog.Warningf("invalid %s: %s", val, err)
		return def
	}
	if ms == 0 {
		return def
	}
	return timeseries.Time(ms / 1000)
}

func ParseHeatmapDuration(s string) time.Duration {
	if s == "" || s == "inf" || s == "err" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		klog.Warningln(err)
		return 0
	}
	return time.Duration(v * float64(time.Second))
}
