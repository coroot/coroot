package collector

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"strings"

	v1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	common "go.opentelemetry.io/proto/otlp/common/v1"
	resource "go.opentelemetry.io/proto/otlp/resource/v1"

	"github.com/coroot/coroot/db"
)

// enrichRumGeo adds coarse geo.country from X-Forwarded-For when project opt-in is enabled.
// Raw IP is never stored — only a hashed form and country/ASN placeholders for local MMDB later.
func enrichRumGeo(req *v1.ExportTraceServiceRequest, r *http.Request, project *db.Project) {
	if req == nil || project == nil || project.Settings.Rum == nil || !project.Settings.Rum.GeoEnabled {
		return
	}
	ip := clientIPFromRequest(r)
	if ip == "" {
		return
	}
	sum := sha256.Sum256([]byte(ip))
	ipHash := hex.EncodeToString(sum[:8])
	country, asn := lookupGeoCoarse(ip) // stub returns empty without MMDB
	attrs := []*common.KeyValue{
		{Key: "geo.ip_hash", Value: &common.AnyValue{Value: &common.AnyValue_StringValue{StringValue: ipHash}}},
	}
	if country != "" {
		attrs = append(attrs, &common.KeyValue{Key: "geo.country", Value: &common.AnyValue{Value: &common.AnyValue_StringValue{StringValue: country}}})
	}
	if asn != "" {
		attrs = append(attrs, &common.KeyValue{Key: "geo.asn", Value: &common.AnyValue{Value: &common.AnyValue_StringValue{StringValue: asn}}})
	}
	for _, rs := range req.GetResourceSpans() {
		if rs.Resource == nil {
			rs.Resource = &resource.Resource{}
		}
		rs.Resource.Attributes = append(rs.Resource.Attributes, attrs...)
		for _, ss := range rs.GetScopeSpans() {
			for _, s := range ss.GetSpans() {
				s.Attributes = append(s.Attributes, attrs...)
			}
		}
	}
}

func clientIPFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// lookupGeoCoarse is a hook for MaxMind/MMDB.
// Without a database, private/local IPs are labeled "local" so lab/dev geo breakdowns are usable.
func lookupGeoCoarse(ip string) (country, asn string) {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return "", ""
	}
	if parsed.IsLoopback() || parsed.IsPrivate() || parsed.IsLinkLocalUnicast() {
		return "local", ""
	}
	return "", ""
}
