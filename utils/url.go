package utils

import (
	"github.com/xhit/go-str2duration/v2"
	"k8s.io/klog"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func ParseTimeFromUrl(now time.Time, query url.Values, key string, def time.Time) time.Time {
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
		return now.Add(d)
	}
	ms, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		klog.Warningf("invalid %s=%s: %s", key, s, err)
		return def
	}
	return time.Unix(ms/1000, 0)
}
