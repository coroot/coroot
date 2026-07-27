package api

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func rcaTestApp() *model.Application {
	id := model.NewApplicationId("test", "checkout-prod", model.ApplicationKindDeployment, "checkout")
	app := model.NewApplication(id)
	app.Status = model.CRITICAL
	app.Instances = []*model.Instance{{Name: "checkout-abc-1"}, {Name: "checkout-abc-2"}}
	app.Reports = []*model.AuditReport{
		{
			Name:   model.AuditReportSLO,
			Status: model.CRITICAL,
			Checks: []*model.Check{
				{Id: "SLOAvailability", Title: "Availability", Status: model.CRITICAL, Message: "5% of requests failed"},
				{Id: "SLOLatency", Title: "Latency", Status: model.OK, Message: "all good"},
			},
		},
		{
			Name:   model.AuditReportInstances,
			Status: model.OK,
			Checks: []*model.Check{
				{Id: "Instances", Title: "Instances", Status: model.OK},
			},
		},
	}
	return app
}

func rcaTestIncident(app *model.Application, openedAt timeseries.Time) *model.ApplicationIncident {
	return &model.ApplicationIncident{
		ApplicationId: app.Id,
		Key:           "abc123",
		OpenedAt:      openedAt,
		Severity:      model.CRITICAL,
		Details: model.IncidentDetails{
			AvailabilityImpact: model.Impact{AffectedRequestPercentage: 5.5},
			AvailabilityBurnRates: []model.BurnRate{
				{LongWindow: timeseries.Hour, ShortWindow: 5 * timeseries.Minute, LongWindowBurnRate: 14.4, Threshold: 6, Severity: model.CRITICAL},
				{LongWindow: 6 * timeseries.Hour, LongWindowBurnRate: 1, Threshold: 3, Severity: model.OK},
			},
		},
	}
}

func TestRCAEvidenceReportsFailingChecksOnly(t *testing.T) {
	app := rcaTestApp()
	now := timeseries.Now()

	e := buildRCAEvidence(rcaEvidenceInput{
		app:      app,
		from:     now.Add(-timeseries.Hour),
		to:       now,
		incident: rcaTestIncident(app, now.Add(-30*timeseries.Minute)),
	})

	if e.Application.Namespace != "checkout-prod" {
		t.Errorf("unexpected namespace: %q", e.Application.Namespace)
	}
	if e.Application.Status != "critical" {
		t.Errorf("unexpected status: %q", e.Application.Status)
	}
	if e.Application.Instances != 2 {
		t.Errorf("unexpected instance count: %d", e.Application.Instances)
	}

	if len(e.FailingChecks) != 1 {
		t.Fatalf("expected only the failing report, got %d", len(e.FailingChecks))
	}
	if len(e.FailingChecks[0].Issues) != 1 {
		t.Fatalf("expected only the failing check, got %d", len(e.FailingChecks[0].Issues))
	}
	if e.FailingChecks[0].Issues[0].Id != "SLOAvailability" {
		t.Errorf("unexpected check: %q", e.FailingChecks[0].Issues[0].Id)
	}

	if e.Incident == nil {
		t.Fatal("expected incident details")
	}
	if e.Incident.AffectedRequestsAvailabilty != 5.5 {
		t.Errorf("unexpected impact: %v", e.Incident.AffectedRequestsAvailabilty)
	}
	if len(e.Incident.ViolatedObjectives) != 1 {
		t.Fatalf("expected only violated objectives, got %d", len(e.Incident.ViolatedObjectives))
	}
	if e.Incident.ViolatedObjectives[0].BurnRate != 14.4 {
		t.Errorf("unexpected burn rate: %v", e.Incident.ViolatedObjectives[0].BurnRate)
	}
}

func TestRCAEvidenceOmitsRawTimeSeries(t *testing.T) {
	app := rcaTestApp()
	now := timeseries.Now()
	ts := timeseries.New(now.Add(-timeseries.Hour), 12, timeseries.Minute)
	for i := 0; i < 12; i++ {
		ts.Set(now.Add(timeseries.Duration(-60+i*5)*timeseries.Minute), float32(i))
	}
	app.AvailabilitySLIs = []*model.AvailabilitySLI{{TotalRequests: ts, FailedRequests: ts}}

	e := buildRCAEvidence(rcaEvidenceInput{app: app, from: now.Add(-timeseries.Hour), to: now})

	if len(e.Vitals) == 0 {
		t.Fatal("expected vitals to be summarized")
	}
	var rps *MCPSeriesValue
	for i := range e.Vitals {
		if e.Vitals[i].Name == "rps" {
			rps = &e.Vitals[i]
		}
	}
	if rps == nil {
		t.Fatal("expected an rps vital")
	}
	if rps.Last == nil || rps.Avg == nil || rps.Max == nil {
		t.Error("expected summary statistics on the vital")
	}
	if len(rps.Sparkline) != MCPSparklineBuckets {
		t.Errorf("expected a %d bucket sparkline, got %d", MCPSparklineBuckets, len(rps.Sparkline))
	}

	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("evidence must be serializable: %s", err)
	}
	// The cloud path ships full time series; the local payload must stay compact.
	if len(raw) > 16<<10 {
		t.Errorf("evidence unexpectedly large: %d bytes", len(raw))
	}
	if strings.Contains(string(raw), "timeseries.TimeSeries") {
		t.Error("evidence leaked a raw time series")
	}
}

func TestRCAEvidenceDeploymentWindow(t *testing.T) {
	app := rcaTestApp()
	now := timeseries.Now()
	from, to := now.Add(-timeseries.Hour), now
	other := model.NewApplicationId("test", "other", model.ApplicationKindDeployment, "other")

	deployments := []*model.ApplicationDeployment{
		{ApplicationId: app.Id, Name: "checkout-v2", StartedAt: now.Add(-20 * timeseries.Minute)},
		{ApplicationId: app.Id, Name: "checkout-v1", StartedAt: now.Add(-90 * timeseries.Minute)},
		{ApplicationId: app.Id, Name: "checkout-old", StartedAt: now.Add(-40 * timeseries.Hour)},
		{ApplicationId: other, Name: "other-v9", StartedAt: now.Add(-10 * timeseries.Minute)},
	}

	out := rcaSummarizeDeployments(deployments, app.Id, from, to)

	if len(out) != 2 {
		t.Fatalf("expected the in-window and just-before deployments, got %d: %+v", len(out), out)
	}
	if !strings.Contains(out[0].Version, "v2") {
		t.Errorf("expected the newest deployment first, got %q", out[0].Version)
	}
}

func TestRCAEvidenceGroupsKubernetesEvents(t *testing.T) {
	app := rcaTestApp()
	base := time.Now().Add(-30 * time.Minute)
	event := func(ns, name, reason string, at time.Time) *model.LogEntry {
		return &model.LogEntry{
			Timestamp: at,
			Severity:  model.SeverityWarning,
			Body:      reason + " for " + name,
			LogAttributes: map[string]string{
				"object.namespace": ns,
				"object.name":      name,
				"object.kind":      "Pod",
				"event.reason":     reason,
			},
		}
	}

	events := []*model.LogEntry{
		event("checkout-prod", "checkout-abc-1", "OOMKilling", base),
		event("checkout-prod", "checkout-abc-1", "OOMKilling", base.Add(time.Minute)),
		event("checkout-prod", "checkout-abc-1", "OOMKilling", base.Add(2*time.Minute)),
		event("checkout-prod", "checkout-abc-1", "OOMKilling", base.Add(3*time.Minute)),
		event("unrelated-ns", "other-pod", "Scheduled", base),
	}

	out := rcaSummarizeKubernetesEvents(events, nil, app)

	if len(out) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(out))
	}
	first := out[0]
	if !first.RelatedToApp {
		t.Error("expected the app's own events to sort first")
	}
	if first.Count != 4 {
		t.Errorf("expected 4 grouped events, got %d", first.Count)
	}
	if len(first.Samples) > rcaMaxEventsPerGroup {
		t.Errorf("expected samples to be capped at %d, got %d", rcaMaxEventsPerGroup, len(first.Samples))
	}
	if first.FirstSeen == "" || first.LastSeen == "" {
		t.Error("expected first and last seen timestamps")
	}
	if out[1].RelatedToApp {
		t.Error("the unrelated namespace event should not be marked as related")
	}
}

func TestRCAEvidenceTraceKeepsSlowestSpans(t *testing.T) {
	spans := []*model.TraceSpan{
		{Name: "GET /fast", TraceId: "t1", ServiceName: "checkout", Duration: 2 * time.Millisecond, StatusCode: "OK"},
		{Name: "SELECT orders", TraceId: "t1", ServiceName: "postgres", Duration: 900 * time.Millisecond, StatusCode: "ERROR", StatusMessage: "timeout"},
	}
	for i := 0; i < rcaMaxTraceSpans; i++ {
		spans = append(spans, &model.TraceSpan{Name: "filler", TraceId: "t1", Duration: time.Millisecond})
	}

	out := rcaSummarizeTrace(&model.Trace{Spans: spans})

	if out == nil {
		t.Fatal("expected a trace summary")
	}
	if out.TraceId != "t1" {
		t.Errorf("unexpected trace id: %q", out.TraceId)
	}
	if len(out.Spans) != rcaMaxTraceSpans {
		t.Fatalf("expected spans capped at %d, got %d", rcaMaxTraceSpans, len(out.Spans))
	}
	if out.Spans[0].Name != "SELECT orders" {
		t.Errorf("expected the slowest span first, got %q", out.Spans[0].Name)
	}
	if out.Spans[0].DurationMs != 900 {
		t.Errorf("unexpected duration: %v", out.Spans[0].DurationMs)
	}
	if out.Spans[0].StatusMessage != "timeout" {
		t.Errorf("unexpected status message: %q", out.Spans[0].StatusMessage)
	}
}

func TestRCAEvidenceHandlesEmptyInputs(t *testing.T) {
	app := model.NewApplication(model.NewApplicationId("test", "_", model.ApplicationKindDeployment, "bare"))
	now := timeseries.Now()

	e := buildRCAEvidence(rcaEvidenceInput{app: app, from: now.Add(-timeseries.Hour), to: now})

	if e == nil {
		t.Fatal("expected evidence")
	}
	if e.Application.Namespace != "" {
		t.Errorf("the placeholder namespace should be omitted, got %q", e.Application.Namespace)
	}
	if e.Incident != nil || e.ErrorTrace != nil || e.SlowTrace != nil {
		t.Error("expected optional sections to be omitted")
	}
	if _, err := json.Marshal(e); err != nil {
		t.Fatalf("evidence must be serializable: %s", err)
	}
}
