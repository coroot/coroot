package collector

import (
	"math/rand"
	"strings"

	"github.com/coroot/coroot/db"
	v1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracev1 "go.opentelemetry.io/proto/otlp/trace/v1"
)

const defaultKeepSlowMs = 2500

// filterRumRequest applies server-side keep-slow / keep-error / probabilistic sampling.
func filterRumRequest(req *v1.ExportTraceServiceRequest, key *db.ApiKey, hint string) *v1.ExportTraceServiceRequest {
	if req == nil {
		return nil
	}
	keepSlowMs := defaultKeepSlowMs
	keepError := true
	sampleRate := 1.0
	if key != nil {
		if key.KeepSlowMs > 0 {
			keepSlowMs = key.KeepSlowMs
		}
		if key.KeepError != nil {
			keepError = *key.KeepError
		}
		if key.ServerSampleRate > 0 && key.ServerSampleRate <= 1 {
			sampleRate = key.ServerSampleRate
		}
	}
	if strings.EqualFold(hint, "slow") || strings.EqualFold(hint, "error") {
		// client hint — still validate below, but prefer keeping
		sampleRate = 1.0
	}

	out := &v1.ExportTraceServiceRequest{}
	for _, rs := range req.GetResourceSpans() {
		ors := &tracev1.ResourceSpans{Resource: rs.GetResource(), SchemaUrl: rs.GetSchemaUrl()}
		for _, ss := range rs.GetScopeSpans() {
			oss := &tracev1.ScopeSpans{Scope: ss.GetScope(), SchemaUrl: ss.GetSchemaUrl()}
			for _, s := range ss.GetSpans() {
				if shouldKeepRumSpan(s, keepSlowMs, keepError, sampleRate, hint) {
					oss.Spans = append(oss.Spans, s)
				}
			}
			if len(oss.Spans) > 0 {
				ors.ScopeSpans = append(ors.ScopeSpans, oss)
			}
		}
		if len(ors.ScopeSpans) > 0 {
			out.ResourceSpans = append(out.ResourceSpans, ors)
		}
	}
	if len(out.ResourceSpans) == 0 {
		return nil
	}
	return out
}

func shouldKeepRumSpan(s *tracev1.Span, keepSlowMs int, keepError bool, sampleRate float64, hint string) bool {
	if keepError && s.GetStatus().GetCode() == tracev1.Status_STATUS_CODE_ERROR {
		return true
	}
	durMs := float64(s.GetEndTimeUnixNano()-s.GetStartTimeUnixNano()) / 1e6
	name := strings.ToLower(s.GetName())
	attrs := attributesToMap(s.GetAttributes())
	vitalVal := parseFloat(firstNonEmpty(attrs["vital.value"], attrs["value"]))
	interesting := durMs >= float64(keepSlowMs) ||
		vitalVal >= float64(keepSlowMs) ||
		strings.Contains(name, "lcp") && vitalVal >= 2500 ||
		strings.EqualFold(hint, "slow") ||
		strings.EqualFold(hint, "error")
	if interesting {
		return true
	}
	if sampleRate >= 1 {
		return true
	}
	if sampleRate <= 0 {
		return false
	}
	return rand.Float64() < sampleRate
}
