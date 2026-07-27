package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coroot/coroot/config"
)

func integrationTestApi(secret string) *Api {
	return &Api{cfg: &config.Config{
		UrlBasePath: "/",
		Auth:        config.Auth{HandoffSecret: secret},
	}}
}

func TestIncidentUrl(t *testing.T) {
	cases := []struct {
		basePath, project, key, want string
	}{
		{"/", "j8v4hjot", "abc123", "/p/j8v4hjot/incidents?incident=abc123"},
		{"", "j8v4hjot", "abc123", "/p/j8v4hjot/incidents?incident=abc123"},
		{"/coroot/", "j8v4hjot", "abc123", "/coroot/p/j8v4hjot/incidents?incident=abc123"},
	}
	for _, c := range cases {
		if got := incidentUrl(c.basePath, c.project, c.key); got != c.want {
			t.Errorf("incidentUrl(%q, %q, %q) = %q, want %q", c.basePath, c.project, c.key, got, c.want)
		}
	}
}

// The endpoint exposes incident data without a user session, so the secret check is the
// only thing standing in front of it.
func TestIntegrationAppIncidentRequiresSecret(t *testing.T) {
	cases := map[string]struct {
		configured string
		header     string
		value      string
	}{
		"no secret configured": {"", "X-Handoff-Secret", "anything"},
		"no credentials":       {"s3cret", "", ""},
		"wrong secret":         {"s3cret", "X-Handoff-Secret", "nope"},
		"wrong bearer":         {"s3cret", "Authorization", "Bearer nope"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/integration/incident?project=p&application=ns:Deployment:app", nil)
			if c.header != "" {
				r.Header.Set(c.header, c.value)
			}
			w := httptest.NewRecorder()
			integrationTestApi(c.configured).IntegrationAppIncident(w, r)
			if w.Code != http.StatusUnauthorized {
				t.Errorf("expected 401, got %d", w.Code)
			}
		})
	}
}

func TestIntegrationAppIncidentValidatesParams(t *testing.T) {
	cases := map[string]string{
		"missing both":           "/api/integration/incident",
		"missing application":    "/api/integration/incident?project=p",
		"missing project":        "/api/integration/incident?application=ns:Deployment:app",
		"invalid application id": "/api/integration/incident?project=p&application=garbage",
	}
	for name, target := range cases {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, target, nil)
			r.Header.Set("X-Handoff-Secret", "s3cret")
			w := httptest.NewRecorder()
			// A nil db would panic, so reaching 400 also proves we reject before any lookup.
			integrationTestApi("s3cret").IntegrationAppIncident(w, r)
			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d", w.Code)
			}
		})
	}
}

// Bearer is the alternate credential form; it must pass the secret check and fall through
// to parameter validation rather than being rejected as unauthorized.
func TestIntegrationAppIncidentAcceptsBearer(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/integration/incident", nil)
	r.Header.Set("Authorization", "Bearer s3cret")
	w := httptest.NewRecorder()
	integrationTestApi("s3cret").IntegrationAppIncident(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected the bearer token to be accepted (400 for missing params), got %d", w.Code)
	}
}
