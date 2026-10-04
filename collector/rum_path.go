package collector

import (
	"regexp"
	"strings"
)

var (
	uuidSegmentRe = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	hexSegmentRe  = regexp.MustCompile(`(?i)^[0-9a-f]{16,}$`)
	numSegmentRe  = regexp.MustCompile(`^\d+$`)
)

// normalizePagePath replaces high-cardinality path segments (numeric IDs, UUIDs, long hex) with :id.
func normalizePagePath(path string) string {
	if path == "" {
		return ""
	}
	if q := strings.IndexAny(path, "?#"); q >= 0 {
		path = path[:q]
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if p == "" {
			continue
		}
		if numSegmentRe.MatchString(p) || uuidSegmentRe.MatchString(p) || hexSegmentRe.MatchString(p) {
			parts[i] = ":id"
		}
	}
	out := strings.Join(parts, "/")
	if out == "" {
		return "/"
	}
	return out
}

func isCwvLeafSpanName(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	switch n {
	case "lcp", "ttfb", "cls", "inp", "fcp", "fid":
		return true
	}
	return strings.HasPrefix(n, "webvital.") || strings.HasPrefix(n, "web_vital_")
}

// filterRumEventAttributes keeps only attributes needed by RUM queries.
func filterRumEventAttributes(attrs map[string]string) map[string]string {
	if len(attrs) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, 8)
	for k, v := range attrs {
		if v == "" {
			continue
		}
		switch k {
		case "exception.message", "exception.type", "exception.stacktrace",
			"geo.country", "service.version", "page.path", "http.url",
			"vital.value", "vital.rating", "vital.name", "value", "rating":
			out[k] = v
		default:
			if strings.HasPrefix(k, "vital.") {
				out[k] = v
			}
		}
	}
	return out
}
