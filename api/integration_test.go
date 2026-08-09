package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coroot/coroot/config"
	"github.com/coroot/coroot/model"
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

func TestIntegrationProjectIncidentsRequiresSecret(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/integration/incidents?project=p", nil)
	r.Header.Set("X-Handoff-Secret", "nope")
	w := httptest.NewRecorder()
	integrationTestApi("s3cret").IntegrationProjectIncidents(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestIntegrationProjectIncidentsRequiresProject(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/integration/incidents", nil)
	r.Header.Set("X-Handoff-Secret", "s3cret")
	w := httptest.NewRecorder()
	// A nil db would panic, so reaching 400 also proves we reject before any lookup.
	integrationTestApi("s3cret").IntegrationProjectIncidents(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestNewIntegrationIncidentPrefersRCASummary(t *testing.T) {
	incident := &model.ApplicationIncident{Key: "abc", Severity: model.CRITICAL}
	incident.Details.AvailabilityImpact.AffectedRequestPercentage = 12

	got := newIntegrationIncident(incident, "/", "proj")
	if got.ShortSummary != "Elevated error rate" {
		t.Errorf("expected the generated description before RCA, got %q", got.ShortSummary)
	}
	if got.RCAStatus != "" {
		t.Errorf("expected no rca status, got %q", got.RCAStatus)
	}

	incident.RCA = &model.RCA{Status: "OK", ShortSummary: "Postgres ran out of connections"}
	got = newIntegrationIncident(incident, "/", "proj")
	if got.ShortSummary != "Postgres ran out of connections" {
		t.Errorf("expected the RCA summary, got %q", got.ShortSummary)
	}
	if got.RCAStatus != "OK" {
		t.Errorf("expected rca status OK, got %q", got.RCAStatus)
	}
	if got.Severity != "critical" {
		t.Errorf("unexpected severity %q", got.Severity)
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

func TestParseNamespacesQuery(t *testing.T) {
	if parseNamespacesQuery("") != nil {
		t.Fatal("empty should be nil")
	}
	got := parseNamespacesQuery(" Foo-Bar , baz ")
	if !got["foo-bar"] || !got["baz"] || len(got) != 2 {
		t.Fatalf("unexpected set: %#v", got)
	}
}

func TestWorldWithNamespaces(t *testing.T) {
	w := &model.World{
		Applications: map[model.ApplicationId]*model.Application{
			model.NewApplicationId("", "ns-a", model.ApplicationKindDeployment, "a"): {Id: model.NewApplicationId("", "ns-a", model.ApplicationKindDeployment, "a")},
			model.NewApplicationId("", "ns-b", model.ApplicationKindDeployment, "b"): {Id: model.NewApplicationId("", "ns-b", model.ApplicationKindDeployment, "b")},
		},
	}
	filtered := worldWithNamespaces(w, map[string]bool{"ns-a": true})
	if len(filtered.Applications) != 1 {
		t.Fatalf("expected 1 app, got %d", len(filtered.Applications))
	}
	if worldWithNamespaces(w, nil) != w {
		t.Fatal("nil allowlist should return original world")
	}
}

func TestIntegrationOverviewApplicationsRequiresSecret(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/integration/overview/applications?project=p", nil)
	w := httptest.NewRecorder()
	integrationTestApi("s3cret").IntegrationOverviewApplications(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestIntegrationOverviewApplicationsRequiresProject(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/integration/overview/applications", nil)
	r.Header.Set("X-Handoff-Secret", "s3cret")
	w := httptest.NewRecorder()
	integrationTestApi("s3cret").IntegrationOverviewApplications(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestIntegrationOverviewLogsRequiresSecret(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/integration/overview/logs?project=p", nil)
	w := httptest.NewRecorder()
	integrationTestApi("s3cret").IntegrationOverviewLogs(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestIntegrationOverviewLogsRequiresProject(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/integration/overview/logs", nil)
	r.Header.Set("X-Handoff-Secret", "s3cret")
	w := httptest.NewRecorder()
	integrationTestApi("s3cret").IntegrationOverviewLogs(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestIntegrationStatusRequiresSecret(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/integration/status?project=p", nil)
	w := httptest.NewRecorder()
	integrationTestApi("s3cret").IntegrationStatus(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestIntegrationStatusRequiresProject(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/integration/status", nil)
	r.Header.Set("X-Handoff-Secret", "s3cret")
	w := httptest.NewRecorder()
	integrationTestApi("s3cret").IntegrationStatus(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
