package collector

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"

	v1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
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

// rumRequestPath extracts the browser page path for allowlist checks:
// Referer first, then page.path / page.url.path / url.full / http.url from OTLP spans.
func rumRequestPath(r *http.Request, req *v1.ExportTraceServiceRequest) string {
	if r != nil {
		if ref := strings.TrimSpace(r.Header.Get("Referer")); ref != "" {
			if u, err := url.Parse(ref); err == nil && u.Path != "" {
				return u.Path
			}
			if p := pathFromURL(ref); p != "" {
				return p
			}
		}
	}
	if req == nil {
		return ""
	}
	for _, rs := range req.GetResourceSpans() {
		for _, ss := range rs.GetScopeSpans() {
			for _, span := range ss.GetSpans() {
				attrs := attributesToMap(span.GetAttributes())
				if p := firstNonEmpty(
					attrs["page.path"],
					attrs["page.url.path"],
					pathFromURL(attrs["url.full"]),
					pathFromURL(attrs["http.url"]),
				); p != "" {
					return p
				}
			}
		}
	}
	return ""
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
