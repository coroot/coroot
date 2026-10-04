package collector

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coroot/coroot/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		{name: "exact match", origin: "https://app.example.com", allowed: []string{"https://app.example.com"}, want: true},
		{name: "case insensitive", origin: "HTTPS://APP.EXAMPLE.COM", allowed: []string{"https://app.example.com"}, want: true},
		{name: "scheme mismatch", origin: "http://app.example.com", allowed: []string{"https://app.example.com"}, want: false},
		{name: "port mismatch", origin: "https://app.example.com:8443", allowed: []string{"https://app.example.com"}, want: false},
		{name: "prefix wildcard", origin: "https://app.example.com/shop", allowed: []string{"https://app.example.com/*"}, want: true},
		{name: "prefix no match", origin: "https://evil.example.com", allowed: []string{"https://app.example.com/*"}, want: false},
		{name: "skips blank entries", origin: "https://a.com", allowed: []string{"", "  ", "https://a.com"}, want: true},
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

func TestResolveCORSOrigin(t *testing.T) {
	key := rumKey("https://app.example.com")
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
	key := rumKey("https://app.example.com")

	t.Run("success", func(t *testing.T) {
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
							AllowedOrigins: []string{"https://shop.example.com"},
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
