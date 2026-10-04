package collector

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coroot/coroot/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	common "go.opentelemetry.io/proto/otlp/common/v1"
	v1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	resource "go.opentelemetry.io/proto/otlp/resource/v1"
	trace "go.opentelemetry.io/proto/otlp/trace/v1"
)

func TestMatchOrigin(t *testing.T) {
	tests := []struct {
		name    string
		origin  string
		allowed []string
		want    bool
	}{
		{name: "empty origin", origin: "", allowed: []string{"*"}, want: false},
		{name: "empty allowed", origin: "https://a.com", allowed: nil, want: false},
		{name: "wildcard star", origin: "https://any.example", allowed: []string{"*"}, want: true},
		{name: "exact host", origin: "https://app.example.com", allowed: []string{"app.example.com"}, want: true},
		{name: "url compat", origin: "https://app.example.com", allowed: []string{"https://app.example.com"}, want: true},
		{name: "case insensitive host", origin: "HTTPS://APP.EXAMPLE.COM", allowed: []string{"app.example.com"}, want: true},
		{name: "scheme ignored", origin: "http://app.example.com", allowed: []string{"https://app.example.com"}, want: true},
		{name: "port mismatch", origin: "https://app.example.com:8443", allowed: []string{"app.example.com"}, want: false},
		{name: "path-scoped host preflight", origin: "https://app.example.com", allowed: []string{"app.example.com/shop"}, want: true},
		{name: "wildcard subdomain", origin: "https://app.example.com", allowed: []string{"*.example.com"}, want: true},
		{name: "wildcard subdomain bare deny", origin: "https://example.com", allowed: []string{"*.example.com"}, want: false},
		{name: "skips blank entries", origin: "https://a.com", allowed: []string{"", "  ", "a.com"}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, matchOrigin(tt.origin, tt.allowed))
		})
	}
}

func rumKey(origins ...string) *db.ApiKey {
	return &db.ApiKey{
		Type:           db.ApiKeyTypeRum,
		Key:            "rum-key",
		AllowedOrigins: origins,
	}
}

func TestAllowRumRequestPath(t *testing.T) {
	key := rumKey("example.com/shop", "example.com/portal")

	req := httptest.NewRequest(http.MethodPost, "/v1/traces", nil)
	req.Header.Set("Origin", "https://example.com")

	origin, ok := allowRumRequest(req, key, "/shop/cart")
	assert.True(t, ok)
	assert.Equal(t, "https://example.com", origin)

	_, ok = allowRumRequest(req, key, "/admin")
	assert.False(t, ok)

	_, ok = allowRumRequest(req, key, "")
	assert.False(t, ok) // path required when only path-scoped patterns match

	hostWide := rumKey("example.com")
	_, ok = allowRumRequest(req, hostWide, "")
	assert.True(t, ok)
	_, ok = allowRumRequest(req, hostWide, "/anything")
	assert.True(t, ok)
}

func TestResolveCORSOrigin(t *testing.T) {
	key := rumKey("app.example.com")
	req := httptest.NewRequest(http.MethodPost, "/v1/traces", nil)
	req.Header.Set("Origin", "https://app.example.com")

	origin, ok := resolveCORSOrigin(req, key)
	assert.True(t, ok)
	assert.Equal(t, "https://app.example.com", origin)

	req.Header.Set("Origin", "https://other.example.com")
	origin, ok = resolveCORSOrigin(req, key)
	assert.False(t, ok)
	assert.Empty(t, origin)

	origin, ok = resolveCORSOrigin(req, nil)
	assert.False(t, ok)
	assert.Empty(t, origin)

	defaultKey := &db.ApiKey{Type: db.ApiKeyTypeDefault, Key: "default"}
	origin, ok = resolveCORSOrigin(req, defaultKey)
	assert.False(t, ok)
	assert.Empty(t, origin)
}

func TestHandleCORSPreflight(t *testing.T) {
	key := rumKey("app.example.com/shop")

	t.Run("success path-scoped host", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/v1/traces", nil)
		req.Header.Set("Origin", "https://app.example.com")
		rec := httptest.NewRecorder()

		handled := handleCORSPreflight(rec, req, key)
		require.True(t, handled)
		assert.Equal(t, http.StatusNoContent, rec.Code)
		assert.Equal(t, "https://app.example.com", rec.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "POST, OPTIONS", rec.Header().Get("Access-Control-Allow-Methods"))
	})

	t.Run("forbidden origin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/v1/traces", nil)
		req.Header.Set("Origin", "https://evil.example.com")
		rec := httptest.NewRecorder()

		handled := handleCORSPreflight(rec, req, key)
		require.True(t, handled)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("non options passthrough", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/traces", nil)
		req.Header.Set("Origin", "https://app.example.com")
		rec := httptest.NewRecorder()

		handled := handleCORSPreflight(rec, req, key)
		assert.False(t, handled)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestRumPreflight(t *testing.T) {
	c := &Collector{
		projects: map[db.ProjectId]*db.Project{
			"proj": {
				Settings: db.ProjectSettings{
					ApiKeys: []db.ApiKey{
						{
							Type:           db.ApiKeyTypeRum,
							Key:            "k1",
							AllowedOrigins: []string{"shop.example.com"},
						},
					},
				},
			},
		},
	}

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/v1/traces", nil)
		req.Header.Set("Origin", "https://shop.example.com")
		rec := httptest.NewRecorder()
		c.rumPreflight(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
		assert.Equal(t, "https://shop.example.com", rec.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("forbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/v1/traces", nil)
		req.Header.Set("Origin", "https://unknown.example.com")
		rec := httptest.NewRecorder()
		c.rumPreflight(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

func TestRumRequestPath(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/traces", nil)
	req.Header.Set("Referer", "https://example.com/shop/cart?x=1")
	assert.Equal(t, "/shop/cart", rumRequestPath(req, nil))

	otlp := &v1.ExportTraceServiceRequest{
		ResourceSpans: []*trace.ResourceSpans{
			{
				Resource: &resource.Resource{},
				ScopeSpans: []*trace.ScopeSpans{{
					Spans: []*trace.Span{{
						Attributes: []*common.KeyValue{
							{Key: "page.path", Value: &common.AnyValue{Value: &common.AnyValue_StringValue{StringValue: "/portal"}}},
						},
					}},
				}},
			},
		},
	}
	req2 := httptest.NewRequest(http.MethodPost, "/v1/traces", nil)
	assert.Equal(t, "/portal", rumRequestPath(req2, otlp))
}
