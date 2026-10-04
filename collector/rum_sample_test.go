package collector

import (
	"testing"

	"github.com/coroot/coroot/db"
	v1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	common "go.opentelemetry.io/proto/otlp/common/v1"
	tracev1 "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func rumSpan(name string, startNs, endNs uint64, status tracev1.Status_StatusCode, attrs map[string]string) *tracev1.Span {
	var kv []*common.KeyValue
	for k, v := range attrs {
		kv = append(kv, strKV(k, v))
	}
	return &tracev1.Span{
		Name:              name,
		StartTimeUnixNano: startNs,
		EndTimeUnixNano:   endNs,
		Status:            &tracev1.Status{Code: status},
		Attributes:        kv,
	}
}

func TestShouldKeepRumSpan(t *testing.T) {
	fast := rumSpan("page", 0, 100_000_000, tracev1.Status_STATUS_CODE_OK, nil)   // 100ms
	slow := rumSpan("page", 0, 3_000_000_000, tracev1.Status_STATUS_CODE_OK, nil) // 3000ms
	errSpan := rumSpan("page", 0, 100_000_000, tracev1.Status_STATUS_CODE_ERROR, nil)
	lcpSlow := rumSpan("lcp", 0, 0, tracev1.Status_STATUS_CODE_OK, map[string]string{"vital.value": "3000"})

	tests := []struct {
		name       string
		span       *tracev1.Span
		keepSlowMs int
		keepError  bool
		sampleRate float64
		hint       string
		want       bool
	}{
		{name: "slow span", span: slow, keepSlowMs: 2500, keepError: true, sampleRate: 0, want: true},
		{name: "error span", span: errSpan, keepSlowMs: 2500, keepError: true, sampleRate: 0, want: true},
		{name: "error disabled", span: errSpan, keepSlowMs: 2500, keepError: false, sampleRate: 0, want: false},
		{name: "lcp vital slow", span: lcpSlow, keepSlowMs: 2500, keepError: true, sampleRate: 0, want: true},
		{name: "sample rate one keeps fast", span: fast, keepSlowMs: 2500, keepError: true, sampleRate: 1, want: true},
		{name: "sample rate zero drops fast", span: fast, keepSlowMs: 2500, keepError: true, sampleRate: 0, want: false},
		{name: "slow hint keeps fast", span: fast, keepSlowMs: 2500, keepError: true, sampleRate: 0, hint: "slow", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldKeepRumSpan(tt.span, tt.keepSlowMs, tt.keepError, tt.sampleRate, tt.hint)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFilterRumRequest(t *testing.T) {
	fast := rumSpan("fast", 0, 50_000_000, tracev1.Status_STATUS_CODE_OK, nil)
	slow := rumSpan("slow", 0, 3_000_000_000, tracev1.Status_STATUS_CODE_OK, nil)

	makeReq := func(spans ...*tracev1.Span) *v1.ExportTraceServiceRequest {
		return &v1.ExportTraceServiceRequest{
			ResourceSpans: []*tracev1.ResourceSpans{
				{
					ScopeSpans: []*tracev1.ScopeSpans{{Spans: spans}},
				},
			},
		}
	}

	t.Run("keeps slow spans", func(t *testing.T) {
		key := &db.ApiKey{
			Type:           db.ApiKeyTypeRum,
			Key:            "k",
			AllowedOrigins: []string{"*"},
			KeepSlowMs:     2500,
		}
		out := filterRumRequest(makeReq(slow), key, "")
		require.NotNil(t, out)
		assert.Equal(t, "slow", out.ResourceSpans[0].ScopeSpans[0].Spans[0].GetName())
	})

	t.Run("zero server sample rate defaults to full sample", func(t *testing.T) {
		key := &db.ApiKey{
			Type:             db.ApiKeyTypeRum,
			Key:              "k",
			AllowedOrigins:   []string{"*"},
			ServerSampleRate: 0,
		}
		out := filterRumRequest(makeReq(fast, slow), key, "")
		require.NotNil(t, out)
		assert.Len(t, out.ResourceSpans[0].ScopeSpans[0].Spans, 2)
	})

	t.Run("sample rate one keeps all", func(t *testing.T) {
		key := &db.ApiKey{
			Type:             db.ApiKeyTypeRum,
			Key:              "k",
			AllowedOrigins:   []string{"*"},
			ServerSampleRate: 1,
		}
		out := filterRumRequest(makeReq(fast, slow), key, "")
		require.NotNil(t, out)
		assert.Len(t, out.ResourceSpans[0].ScopeSpans[0].Spans, 2)
	})

	t.Run("errors kept when keep error enabled", func(t *testing.T) {
		keepErr := true
		key := &db.ApiKey{
			Type:           db.ApiKeyTypeRum,
			Key:            "k",
			AllowedOrigins: []string{"*"},
			KeepError:      &keepErr,
			KeepSlowMs:     60_000,
		}
		errOnly := rumSpan("err", 0, 10_000_000, tracev1.Status_STATUS_CODE_ERROR, nil)
		out := filterRumRequest(makeReq(errOnly), key, "")
		require.NotNil(t, out)
		assert.Equal(t, "err", out.ResourceSpans[0].ScopeSpans[0].Spans[0].GetName())
	})

	t.Run("nil request", func(t *testing.T) {
		assert.Nil(t, filterRumRequest(nil, nil, ""))
	})
}
