package collector

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coroot/coroot/db"

	semconv "go.opentelemetry.io/collector/semconv/v1.18.0"
	v1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	common "go.opentelemetry.io/proto/otlp/common/v1"
	resource "go.opentelemetry.io/proto/otlp/resource/v1"
	tracev1 "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/stretchr/testify/assert"
)

func strKV(key, val string) *common.KeyValue {
	return &common.KeyValue{
		Key:   key,
		Value: &common.AnyValue{Value: &common.AnyValue_StringValue{StringValue: val}},
	}
}

func traceReqWithResourceAttrs(attrs ...*common.KeyValue) *v1.ExportTraceServiceRequest {
	return &v1.ExportTraceServiceRequest{
		ResourceSpans: []*tracev1.ResourceSpans{
			{Resource: &resource.Resource{Attributes: attrs}},
		},
	}
}

func TestIsRumRequest(t *testing.T) {
	tests := []struct {
		name   string
		header map[string]string
		req    *v1.ExportTraceServiceRequest
		want   bool
	}{
		{
			name:   "coroot signal header rum",
			header: map[string]string{CorootSignalHeader: "rum"},
			req:    &v1.ExportTraceServiceRequest{},
			want:   true,
		},
		{
			name:   "coroot signal case insensitive",
			header: map[string]string{CorootSignalHeader: "RUM"},
			req:    &v1.ExportTraceServiceRequest{},
			want:   true,
		},
		{
			name: "telemetry sdk language webjs",
			req: traceReqWithResourceAttrs(
				strKV(semconv.AttributeTelemetrySDKLanguage, "webjs"),
			),
			want: true,
		},
		{
			name: "telemetry sdk language javascript",
			req: traceReqWithResourceAttrs(
				strKV(semconv.AttributeTelemetrySDKLanguage, "javascript"),
			),
			want: true,
		},
		{
			name: "literal telemetry.sdk.language webjs",
			req: traceReqWithResourceAttrs(
				strKV("telemetry.sdk.language", "webjs"),
			),
			want: true,
		},
		{
			name:   "negative empty",
			header: map[string]string{CorootSignalHeader: "trace"},
			req: traceReqWithResourceAttrs(
				strKV(semconv.AttributeTelemetrySDKLanguage, "go"),
			),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/", nil)
			for k, v := range tt.header {
				r.Header.Set(k, v)
			}
			assert.Equal(t, tt.want, isRumRequest(r, tt.req))
		})
	}
}

func TestPathFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{url: "", want: ""},
		{url: "https://example.com/shop?id=1", want: "/shop"},
		{url: "https://example.com", want: "/"},
		{url: "/already/path", want: "/already/path"},
		{url: "https://example.com/a#frag", want: "/a"},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			assert.Equal(t, tt.want, pathFromURL(tt.url))
		})
	}
}

func TestFirstNonEmpty(t *testing.T) {
	assert.Equal(t, "", firstNonEmpty("", "", ""))
	assert.Equal(t, "b", firstNonEmpty("", "b", "c"))
	assert.Equal(t, "a", firstNonEmpty("a", "b"))
}

func TestVitalValueFromAttrs(t *testing.T) {
	assert.Equal(t, float64(1200), vitalValueFromAttrs(map[string]string{"vital.value": "1200"}, "lcp"))
	assert.Equal(t, float64(99), vitalValueFromAttrs(map[string]string{"value": "99"}, "inp"))
	assert.Equal(t, float64(0.15), vitalValueFromAttrs(map[string]string{"cls.value": "0.15"}, "cls"))
	assert.Equal(t, float64(800), vitalValueFromAttrs(map[string]string{"ttfb": "800"}, "ttfb"))
	assert.Equal(t, float64(0), vitalValueFromAttrs(map[string]string{}, "lcp"))
}

func TestRateVital(t *testing.T) {
	tests := []struct {
		vital string
		value float64
		want  string
	}{
		{vital: "lcp", value: 2000, want: "good"},
		{vital: "lcp", value: 3000, want: "needs-improvement"},
		{vital: "lcp", value: 5000, want: "poor"},
		{vital: "fcp", value: 2500, want: "good"},
		{vital: "inp", value: 100, want: "good"},
		{vital: "inp", value: 300, want: "needs-improvement"},
		{vital: "fid", value: 600, want: "poor"},
		{vital: "cls", value: 0.05, want: "good"},
		{vital: "cls", value: 0.2, want: "needs-improvement"},
		{vital: "cls", value: 0.3, want: "poor"},
		{vital: "ttfb", value: 500, want: "good"},
		{vital: "ttfb", value: 1000, want: "needs-improvement"},
		{vital: "ttfb", value: 2000, want: "poor"},
		{vital: "unknown", value: 1, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.vital, func(t *testing.T) {
			assert.Equal(t, tt.want, rateVital(tt.vital, tt.value))
		})
	}
}

func TestGetAgentProjectRejectsRumKey(t *testing.T) {
	c := &Collector{
		projects: map[db.ProjectId]*db.Project{
			"p1": {
				Id: "p1",
				Settings: db.ProjectSettings{
					ApiKeys: []db.ApiKey{
						{Key: "agent-key", Description: "agent"},
						{Key: "rum-key", Type: db.ApiKeyTypeRum, AllowedOrigins: []string{"http://localhost:3000"}},
					},
				},
			},
		},
	}
	p, err := c.getAgentProject("agent-key")
	assert.NoError(t, err)
	assert.Equal(t, db.ProjectId("p1"), p.Id)

	_, err = c.getAgentProject("rum-key")
	assert.ErrorIs(t, err, ErrProjectNotFound)

	_, err = c.getProject("rum-key")
	assert.NoError(t, err)
}
