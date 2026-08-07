package api

import "testing"

func TestSanitizeHandoffRedirect(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"/p/abc/app/ns:Deployment:app", "/p/abc/app/ns:Deployment:app"},
		{"p/abc", "/p/abc"},
		{"//evil.com", ""},
		{"https://evil.com/x", ""},
		{"http://evil.com", ""},
	}
	for _, c := range cases {
		if got := sanitizeHandoffRedirect(c.in); got != c.want {
			t.Errorf("sanitizeHandoffRedirect(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestJoinBasePathPreserveQuery(t *testing.T) {
	cases := []struct {
		base, target, want string
	}{
		{"/metrics", "/p/proj/app/x?theme=dark", "/metrics/p/proj/app/x?theme=dark"},
		{"/metrics/", "/p/proj/app/foo:bar?theme=dark", "/metrics/p/proj/app/foo:bar?theme=dark"},
		{"/metrics", "/p/proj/app/x", "/metrics/p/proj/app/x"},
	}
	for _, c := range cases {
		if got := joinBasePathPreserveQuery(c.base, c.target); got != c.want {
			t.Errorf("joinBasePathPreserveQuery(%q, %q) = %q, want %q", c.base, c.target, got, c.want)
		}
	}
}

func TestAppendQueryParam(t *testing.T) {
	cases := []struct {
		raw, want string
	}{
		{"/p/x", "/p/x?theme=dark"},
		{"/p/x?foo=1", "/p/x?foo=1&theme=dark"},
		{"/p/x?theme=light", "/p/x?theme=dark"},
	}
	for _, c := range cases {
		if got := appendQueryParam(c.raw, "theme", "dark"); got != c.want {
			t.Errorf("appendQueryParam(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}
