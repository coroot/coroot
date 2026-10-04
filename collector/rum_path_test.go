package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizePagePath(t *testing.T) {
	assert.Equal(t, "", normalizePagePath(""))
	assert.Equal(t, "/product/:id", normalizePagePath("/product/123"))
	assert.Equal(t, "/users/:id/orders/:id", normalizePagePath("/users/42/orders/99"))
	assert.Equal(t, "/item/:id", normalizePagePath("/item/550e8400-e29b-41d4-a716-446655440000"))
	assert.Equal(t, "/trace/:id", normalizePagePath("/trace/abcdef0123456789abcdef01"))
	assert.Equal(t, "/shop", normalizePagePath("/shop?utm=1"))
	assert.Equal(t, "/static/app.js", normalizePagePath("/static/app.js"))
	assert.Equal(t, "/v1/api", normalizePagePath("v1/api"))
}

func TestIsCwvLeafSpanName(t *testing.T) {
	assert.True(t, isCwvLeafSpanName("lcp"))
	assert.True(t, isCwvLeafSpanName("TTFB"))
	assert.True(t, isCwvLeafSpanName("webvital.inp"))
	assert.False(t, isCwvLeafSpanName("documentLoad"))
	assert.False(t, isCwvLeafSpanName("HTTP GET"))
}

func TestFilterRumEventAttributes(t *testing.T) {
	in := map[string]string{
		"exception.message": "boom",
		"exception.type":    "TypeError",
		"geo.country":       "US",
		"service.version":   "1.2.3",
		"page.path":         "/x",
		"http.url":          "https://example.com/x",
		"vital.value":       "1200",
		"vital.rating":      "good",
		"noise.key":         "drop-me",
		"user.email":        "drop-me-too",
	}
	out := filterRumEventAttributes(in)
	assert.Equal(t, "boom", out["exception.message"])
	assert.Equal(t, "US", out["geo.country"])
	assert.Equal(t, "1200", out["vital.value"])
	assert.NotContains(t, out, "noise.key")
	assert.NotContains(t, out, "user.email")
}
