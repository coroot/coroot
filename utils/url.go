package utils

import (
	"github.com/coroot/coroot/timeseries"
	"github.com/xhit/go-str2duration/v2"
	"k8s.io/klog"
	"net/url"
	"strconv"
	"strings"
)

func ParseTimeFromUrl(now timeseries.Time, query url.Values, key string, def timeseries.Time) timeseries.Time {
	s := query.Get(key)
	if s == "" {
		return def
	}
	if strings.HasPrefix(s, "now") {
		if s == "now" {
			return now
		}
		d, err := str2duration.ParseDuration(s[3:])
		if err != nil {
			klog.Warningf("invalid %s=%s: %s", key, s, err)
			return def
		}
		return now.Add(timeseries.Duration(d.Seconds()))
	}
	ms, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		klog.Warningf("invalid %s=%s: %s", key, s, err)
		return def
	}
	return timeseries.Time(ms / 1000)
}
