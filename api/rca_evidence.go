package api

import (
	"sort"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

// Caps keep the evidence document small enough to fit comfortably in a model context window.
const (
	rcaMaxEventGroups       = 15
	rcaMaxEventsPerGroup    = 3
	rcaMaxDeployments       = 5
	rcaMaxTraceSpans        = 20
	rcaEventBodyMaxLength   = 300
	rcaTraceStatusMaxLength = 300
)

type rcaEvidence struct {
	Application      rcaApplication   `json:"application"`
	Window           rcaWindow        `json:"window"`
	Incident         *rcaIncident     `json:"incident,omitempty"`
	Vitals           []MCPSeriesValue `json:"vitals,omitempty"`
	FailingChecks    []rcaReport      `json:"failing_checks,omitempty"`
	LogPatterns      []mcpLogPattern  `json:"log_patterns,omitempty"`
	Dependencies     []mcpDependency  `json:"dependencies,omitempty"`
	Clients          []mcpClient      `json:"clients,omitempty"`
	Deployments      []rcaDeployment  `json:"recent_deployments,omitempty"`
	KubernetesEvents []rcaEventGroup  `json:"kubernetes_events,omitempty"`
	ErrorTrace       *rcaTrace        `json:"error_trace,omitempty"`
	SlowTrace        *rcaTrace        `json:"slow_trace,omitempty"`
}

type rcaApplication struct {
	Id        string `json:"id"`
	Namespace string `json:"namespace,omitempty"`
	Kind      string `json:"kind,omitempty"`
	Status    string `json:"status"`
	Instances int    `json:"instances"`
}

type rcaWindow struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type rcaIncident struct {
	Severity                    string         `json:"severity"`
	OpenedAt                    string         `json:"opened_at"`
	ResolvedAt                  string         `json:"resolved_at,omitempty"`
	AffectedRequestsAvailabilty float32        `json:"affected_requests_availability_percent,omitempty"`
	AffectedRequestsLatency     float32        `json:"affected_requests_latency_percent,omitempty"`
	ViolatedObjectives          []rcaObjective `json:"violated_objectives,omitempty"`
}

type rcaObjective struct {
	Kind      string  `json:"kind"`
	Window    string  `json:"window"`
	BurnRate  float32 `json:"burn_rate"`
	Threshold float32 `json:"threshold"`
}

type rcaReport struct {
	Name   string     `json:"name"`
	Status string     `json:"status"`
	Issues []mcpIssue `json:"issues"`
}

type rcaDeployment struct {
	Version    string `json:"version"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at,omitempty"`
}

type rcaEventGroup struct {
	Reason       string   `json:"reason,omitempty"`
	ObjectKind   string   `json:"object_kind,omitempty"`
	ObjectName   string   `json:"object_name,omitempty"`
	Namespace    string   `json:"namespace,omitempty"`
	Severity     string   `json:"severity,omitempty"`
	Count        int      `json:"count"`
	FirstSeen    string   `json:"first_seen,omitempty"`
	LastSeen     string   `json:"last_seen,omitempty"`
	RelatedToApp bool     `json:"related_to_app"`
	Samples      []string `json:"samples,omitempty"`
}

type rcaTrace struct {
	TraceId string         `json:"trace_id,omitempty"`
	Spans   []rcaTraceSpan `json:"spans,omitempty"`
}

type rcaTraceSpan struct {
	Name          string  `json:"name"`
	Service       string  `json:"service,omitempty"`
	DurationMs    float64 `json:"duration_ms"`
	StatusCode    string  `json:"status_code,omitempty"`
	StatusMessage string  `json:"status_message,omitempty"`
}

type rcaEvidenceInput struct {
	world            *model.World
	app              *model.Application
	incident         *model.ApplicationIncident
	from, to         timeseries.Time
	deployments      []*model.ApplicationDeployment
	kubernetesEvents []*model.LogEntry
	errorTrace       *model.Trace
	slowTrace        *model.Trace
}

// buildRCAEvidence turns the raw telemetry into a compact JSON-serializable document for an LLM.
// The caller must run auditor.Audit first, otherwise app.Reports is empty and no checks are reported.
func buildRCAEvidence(in rcaEvidenceInput) *rcaEvidence {
	app := in.app
	e := &rcaEvidence{
		Application: rcaApplication{
			Id:        app.Id.String(),
			Kind:      string(app.Id.Kind),
			Status:    app.Status.String(),
			Instances: len(app.Instances),
		},
		Window: rcaWindow{From: MCPFormatTime(in.from), To: MCPFormatTime(in.to)},
		Vitals: mcpComputeVitals(app),
	}
	if app.Id.Namespace != "_" {
		e.Application.Namespace = app.Id.Namespace
	}

	if in.incident != nil {
		e.Incident = rcaBuildIncident(in.incident)
	}

	for _, r := range app.Reports {
		var issues []mcpIssue
		for _, c := range r.Checks {
			if c.Status < model.WARNING {
				continue
			}
			issues = append(issues, mcpIssue{
				Id:      string(c.Id),
				Title:   c.Title,
				Status:  c.Status.String(),
				Message: c.Message,
			})
		}
		if len(issues) == 0 {
			continue
		}
		e.FailingChecks = append(e.FailingChecks, rcaReport{Name: string(r.Name), Status: r.Status.String(), Issues: issues})
		if r.Name == model.AuditReportLogs {
			e.LogPatterns = mcpExtractLogPatterns(app, mcpLogPatternsTopN)
		}
	}
	sort.Slice(e.FailingChecks, func(i, j int) bool { return e.FailingChecks[i].Name < e.FailingChecks[j].Name })
	if e.LogPatterns == nil {
		e.LogPatterns = mcpExtractLogPatterns(app, mcpLogPatternsTopN)
	}

	e.Dependencies, e.Clients = rcaSummarizeConnections(app)
	e.Deployments = rcaSummarizeDeployments(in.deployments, app.Id, in.from, in.to)
	e.KubernetesEvents = rcaSummarizeKubernetesEvents(in.kubernetesEvents, in.world, app)
	e.ErrorTrace = rcaSummarizeTrace(in.errorTrace)
	e.SlowTrace = rcaSummarizeTrace(in.slowTrace)

	return e
}

func rcaBuildIncident(incident *model.ApplicationIncident) *rcaIncident {
	out := &rcaIncident{
		Severity:                    incident.Severity.String(),
		OpenedAt:                    MCPFormatTime(incident.OpenedAt),
		AffectedRequestsAvailabilty: incident.Details.AvailabilityImpact.AffectedRequestPercentage,
		AffectedRequestsLatency:     incident.Details.LatencyImpact.AffectedRequestPercentage,
	}
	if incident.Resolved() {
		out.ResolvedAt = MCPFormatTime(incident.ResolvedAt)
	}
	add := func(kind string, brs []model.BurnRate) {
		for _, br := range brs {
			if br.Severity < model.WARNING {
				continue
			}
			out.ViolatedObjectives = append(out.ViolatedObjectives, rcaObjective{
				Kind:      kind,
				Window:    br.LongWindow.String(),
				BurnRate:  br.LongWindowBurnRate,
				Threshold: br.Threshold,
			})
		}
	}
	add("availability", incident.Details.AvailabilityBurnRates)
	add("latency", incident.Details.LatencyBurnRates)
	return out
}

// rcaSummarizeConnections mirrors the MCP application status view: upstream health explains whether
// the cause is downstream of this application, and clients show the blast radius.
func rcaSummarizeConnections(app *model.Application) ([]mcpDependency, []mcpClient) {
	var deps []mcpDependency
	for _, conn := range app.Upstreams {
		if conn == nil || conn.RemoteApplication == nil || !conn.IsActual() {
			continue
		}
		rem := conn.RemoteApplication
		dep := mcpDependency{Id: rem.Id.String()}
		if rem.Status != model.UNKNOWN {
			dep.Status = rem.Status.String()
		}
		cs, msg := conn.Status()
		dep.Connectivity = cs.String()
		dep.ConnectivityMessage = msg
		dep.RttSeconds = mcpLastNotNull(conn.Rtt)
		dep.Rps = mcpLastNotNull(conn.GetConnectionsRequestsSum(nil))
		dep.ErrorsPerSec = mcpLastNotNull(conn.GetConnectionsErrorsSum(nil))

		lat := mcpLatency{Avg: mcpLastNotNull(conn.GetConnectionsRequestsLatency(nil))}
		if buckets := mcpAggregateUpstreamHistogram(app, rem); len(buckets) > 0 {
			lat.P50 = mcpLastNotNull(model.Quantile(buckets, 0.5))
			lat.P95 = mcpLastNotNull(model.Quantile(buckets, 0.95))
			lat.P99 = mcpLastNotNull(model.Quantile(buckets, 0.99))
		}
		if lat.Avg != nil || lat.P50 != nil || lat.P95 != nil || lat.P99 != nil {
			dep.LatencySeconds = &lat
		}
		deps = append(deps, dep)
	}
	sort.Slice(deps, func(i, j int) bool { return deps[i].Id < deps[j].Id })

	var clients []mcpClient
	for _, conn := range app.Downstreams {
		if conn == nil || conn.RemoteApplication == nil || !conn.IsActual() {
			continue
		}
		c := mcpClient{Id: conn.RemoteApplication.Id.String()}
		if conn.RemoteApplication.Status != model.UNKNOWN {
			c.Status = conn.RemoteApplication.Status.String()
		}
		clients = append(clients, c)
	}
	sort.Slice(clients, func(i, j int) bool { return clients[i].Id < clients[j].Id })

	return deps, clients
}

func rcaSummarizeDeployments(deployments []*model.ApplicationDeployment, appId model.ApplicationId, from, to timeseries.Time) []rcaDeployment {
	var out []rcaDeployment
	for _, d := range deployments {
		if d == nil || d.ApplicationId != appId {
			continue
		}
		// A rollout shortly before the window is still a prime suspect.
		if d.StartedAt.Before(from.Add(-timeseries.Hour)) || d.StartedAt.After(to) {
			continue
		}
		out = append(out, rcaDeployment{
			Version:    d.Version(),
			StartedAt:  MCPFormatTime(d.StartedAt),
			FinishedAt: MCPFormatTime(d.FinishedAt),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt > out[j].StartedAt })
	if len(out) > rcaMaxDeployments {
		out = out[:rcaMaxDeployments]
	}
	return out
}

// rcaSummarizeKubernetesEvents groups identical events and prefers the ones tied to this application,
// since the raw query returns cluster-wide events.
func rcaSummarizeKubernetesEvents(events []*model.LogEntry, world *model.World, app *model.Application) []rcaEventGroup {
	if len(events) == 0 {
		return nil
	}

	instanceNames := map[string]bool{}
	for _, inst := range app.Instances {
		instanceNames[inst.Name] = true
	}

	type group struct {
		out   rcaEventGroup
		first timeseries.Time
		last  timeseries.Time
	}
	groups := map[string]*group{}
	var order []string

	for _, event := range events {
		if event == nil {
			continue
		}
		attrs := event.LogAttributes
		ns := attrs["object.namespace"]
		name := attrs["object.name"]
		kind := attrs["object.kind"]
		reason := attrs["event.reason"]

		related := instanceNames[name]
		if !related && app.Id.Namespace != "_" && ns == app.Id.Namespace {
			related = true
		}
		if !related && world != nil {
			if id := model.NewApplicationId(event.ClusterId, ns, model.ApplicationKind(kind), name); id == app.Id {
				related = true
			}
		}

		key := ns + "|" + kind + "|" + name + "|" + reason + "|" + event.Severity.String()
		g := groups[key]
		if g == nil {
			g = &group{out: rcaEventGroup{
				Reason:       reason,
				ObjectKind:   kind,
				ObjectName:   name,
				Namespace:    ns,
				Severity:     event.Severity.String(),
				RelatedToApp: related,
			}}
			groups[key] = g
			order = append(order, key)
		}
		g.out.Count++
		g.out.RelatedToApp = g.out.RelatedToApp || related
		ts := timeseries.Time(event.Timestamp.Unix())
		if g.first.IsZero() || ts.Before(g.first) {
			g.first = ts
		}
		if ts.After(g.last) {
			g.last = ts
		}
		if len(g.out.Samples) < rcaMaxEventsPerGroup && event.Body != "" {
			g.out.Samples = append(g.out.Samples, utils.Truncate(event.Body, rcaEventBodyMaxLength))
		}
	}

	out := make([]rcaEventGroup, 0, len(order))
	for _, key := range order {
		g := groups[key]
		g.out.FirstSeen = MCPFormatTime(g.first)
		g.out.LastSeen = MCPFormatTime(g.last)
		out = append(out, g.out)
	}
	// App-related events first, then the noisiest, so the cap keeps the most relevant groups.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].RelatedToApp != out[j].RelatedToApp {
			return out[i].RelatedToApp
		}
		return out[i].Count > out[j].Count
	})
	if len(out) > rcaMaxEventGroups {
		out = out[:rcaMaxEventGroups]
	}
	return out
}

func rcaSummarizeTrace(trace *model.Trace) *rcaTrace {
	if trace == nil || len(trace.Spans) == 0 {
		return nil
	}
	out := &rcaTrace{TraceId: trace.Spans[0].TraceId}
	spans := trace.Spans
	// Slowest spans first: those are the ones worth spending context on.
	sorted := make([]*model.TraceSpan, len(spans))
	copy(sorted, spans)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Duration > sorted[j].Duration })
	if len(sorted) > rcaMaxTraceSpans {
		sorted = sorted[:rcaMaxTraceSpans]
	}
	for _, s := range sorted {
		if s == nil {
			continue
		}
		out.Spans = append(out.Spans, rcaTraceSpan{
			Name:          s.Name,
			Service:       s.ServiceName,
			DurationMs:    float64(s.Duration.Microseconds()) / 1000,
			StatusCode:    s.StatusCode,
			StatusMessage: utils.Truncate(s.StatusMessage, rcaTraceStatusMaxLength),
		})
	}
	if len(out.Spans) == 0 {
		return nil
	}
	return out
}
