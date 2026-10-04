package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAllowedOrigin(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "*", want: "*"},
		{in: "Shop.Example.COM", want: "shop.example.com"},
		{in: "https://app.example.com", want: "app.example.com"},
		{in: "https://app.example.com/", want: "app.example.com"},
		{in: "https://app.example.com/shop", want: "app.example.com/shop"},
		{in: "https://app.example.com/shop/", want: "app.example.com/shop"},
		{in: "https://app.example.com/shop?x=1", want: "app.example.com/shop"},
		{in: "localhost:3000", want: "localhost:3000"},
		{in: "http://localhost:3000", want: "localhost:3000"},
		{in: "example.com/shop", want: "example.com/shop"},
		{in: "example.com/Portal", want: "example.com/portal"},
		{in: "*.example.com", want: "*.example.com"},
		{in: "", wantErr: true},
		{in: "https://", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := NormalizeAllowedOrigin(tt.in)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchAllowedOrigin(t *testing.T) {
	origin := "https://example.com"

	assert.False(t, MatchAllowedOrigin("", "/shop", []string{"example.com"}))
	assert.False(t, MatchAllowedOrigin(origin, "", nil))

	// host-only / preflight
	assert.True(t, MatchAllowedOrigin(origin, "", []string{"example.com"}))
	assert.True(t, MatchAllowedOrigin(origin, "", []string{"example.com/shop"}))
	assert.True(t, MatchAllowedOrigin("https://app.example.com", "", []string{"*.example.com"}))
	assert.False(t, MatchAllowedOrigin("https://example.com", "", []string{"*.example.com"}))
	assert.True(t, MatchAllowedOrigin("https://evil.com", "", []string{"*"}))

	// path-scoped POST
	assert.True(t, MatchAllowedOrigin(origin, "/shop", []string{"example.com/shop"}))
	assert.True(t, MatchAllowedOrigin(origin, "/shop/cart", []string{"example.com/shop"}))
	assert.False(t, MatchAllowedOrigin(origin, "/portal", []string{"example.com/shop"}))
	assert.False(t, MatchAllowedOrigin(origin, "/shopper", []string{"example.com/shop"}))

	// host-wide allows any path
	assert.True(t, MatchAllowedOrigin(origin, "/anything", []string{"example.com"}))

	// multi-service behind one domain
	allowed := []string{"example.com/shop", "example.com/portal"}
	assert.True(t, MatchAllowedOrigin(origin, "/portal/home", allowed))
	assert.False(t, MatchAllowedOrigin(origin, "/admin", allowed))

	// port
	assert.True(t, MatchAllowedOrigin("http://localhost:3000", "", []string{"localhost:3000"}))
	assert.False(t, MatchAllowedOrigin("http://localhost:3000", "", []string{"localhost"}))

	// compat with stored full URL
	assert.True(t, MatchAllowedOrigin("https://app.example.com", "/x", []string{"https://app.example.com"}))
}

func TestPathRequiredForOrigin(t *testing.T) {
	assert.True(t, PathRequiredForOrigin("https://example.com", []string{"example.com/shop"}))
	assert.False(t, PathRequiredForOrigin("https://example.com", []string{"example.com"}))
	assert.False(t, PathRequiredForOrigin("https://example.com", []string{"example.com", "example.com/shop"}))
	assert.False(t, PathRequiredForOrigin("https://example.com", []string{"*"}))
	assert.False(t, PathRequiredForOrigin("https://other.com", []string{"example.com/shop"}))
}

func TestApiKeyValidateNormalizesOrigins(t *testing.T) {
	k := &ApiKey{
		Key:            "rum-key",
		Type:           ApiKeyTypeRum,
		AllowedOrigins: []string{"https://App.Example.com/Shop/"},
	}
	require.NoError(t, k.Validate())
	assert.Equal(t, []string{"app.example.com/shop"}, k.AllowedOrigins)

	k2 := &ApiKey{Key: "rum-key", Type: ApiKeyTypeRum, AllowedOrigins: nil}
	assert.Error(t, k2.Validate())

	def := &ApiKey{Key: "k", Type: ApiKeyTypeDefault, AllowedOrigins: []string{"example.com"}}
	require.NoError(t, def.Validate())
	assert.Nil(t, def.AllowedOrigins)
}
