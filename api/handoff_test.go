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
