package utils

import (
	"github.com/coroot/coroot/timeseries"
	"github.com/xhit/go-str2duration/v2"
	"k8s.io/klog"
	"strconv"
	"strings"
)

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
