package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidRedirectURI(t *testing.T) {
	// Valid URIs
	assert.True(t, validRedirectURI("https://example.com/oauth/callback"))
	assert.True(t, validRedirectURI("https://sub.domain.org:8443/cb?foo=bar"))
	assert.True(t, validRedirectURI("http://localhost:8080/callback"))
	assert.True(t, validRedirectURI("http://127.0.0.1:3000/auth"))
	assert.True(t, validRedirectURI("http://[::1]:9090/oauth/callback"))
	assert.True(t, validRedirectURI("vscode://extension/callback"))
	assert.True(t, validRedirectURI("cursor://login"))

	// Disallowed: Relative path or empty
	assert.False(t, validRedirectURI(""))
	assert.False(t, validRedirectURI("/relative/path"))

	// Disallowed: Insecure remote HTTP (RFC 8252 requires loopback for HTTP)
	assert.False(t, validRedirectURI("http://attacker.example.com/steal"))
	assert.False(t, validRedirectURI("http://192.168.1.1/cb"))

	// Disallowed: Fragment component (RFC 6749 Section 3.1.2)
	assert.False(t, validRedirectURI("https://example.com/callback#token=123"))

	// Disallowed: Dangerous or invalid schemes
	assert.False(t, validRedirectURI("javascript:alert(1)"))
	assert.False(t, validRedirectURI("data:text/html,<script>alert(1)</script>"))
	assert.False(t, validRedirectURI("file:///etc/passwd"))
	assert.False(t, validRedirectURI("vbscript:msgbox(1)"))
	assert.False(t, validRedirectURI("123invalid://auth"))
}
