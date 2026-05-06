package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/coroot/coroot/api/views/overview"
	"github.com/coroot/coroot/auditor"
	"github.com/coroot/coroot/clickhouse"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/prom"
	"github.com/coroot/coroot/rbac"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"k8s.io/klog"
)

const MCPInstructions = `Coroot is a production observability platform. Reach for it when the user asks about live behavior of a running system: why a service is slow or erroring, what changed, what alerts are firing, what depends on what, recent incidents, capacity, deploys. It is NOT for source-code questions, generic ML/ops advice, or anything that doesn't map to a running cluster.

Multiple projects (clusters) may be available. Always start with list_projects + select_project; the selection persists for the session. Application ids are 4-part 'cluster_id:namespace:Kind:name' — pass them through as returned (don't strip the cluster_id even if the project looks single-cluster).

Pick a tool by intent, cheapest first:

- "What's currently broken / where should I look?" → list_alerts (firing alerts), list_incidents (open SLO incidents), list_applications (per-app inspection issues + SLO status).
- "What's wrong with <app>?" → get_application_status: overall status, per-inspection (CPU, memory, SLO, postgres, ...) issues, log-pattern samples, upstream dependencies with connectivity/RTT/latency, and downstream clients.
- "How are the hosts doing?" → list_nodes for an overview (CPU%/mem%/network/status); get_node_details for a single host (audit report + cpu/memory/network sparklines).
- Distributed traces — three drill-down levels:
  • triage / "what's slow / failing?" → traces_summary (per-endpoint rps + error rate + p50/p95/p99). Pass service+span to focus on one endpoint.
  • errors → traces_errors (top reasons grouped by endpoint, with sample_trace_id + sample_error).
  • slow tail → traces_outliers (flamegraph diff: traces in [dur_from..dur_to] vs the rest; default dur_from=1s).
  • full trace → get_trace trace_id=… (full span tree with attributes/events; use trace_ids from traces_errors / traces_summary samples).
- "Show me logs" → query_logs: app-scoped or project-wide, with severity / search / time range. Sorted newest-first.
- "What does this metric look like?" / "Why is Coroot saying X?" → query_metrics for raw PromQL with labels and sparklines; list_metric_names to discover metric names.
- Incident detail → get_incident_details.
- Acting on alerts → resolve_alerts (only after the underlying cause is fixed; alerts whose conditions still hold will re-fire).

Time arguments accept epoch ms or relative strings like 'now-1h', 'now-15m'. Default windows are short (~1h) — widen explicitly when looking at historical patterns.`

type mcpUserCtxKey struct{}

type mcpSessionState struct {
	mu        sync.Mutex
	projectId db.ProjectId
}

type MCPHandler struct {
	Api      *Api
	Server   *mcpserver.MCPServer
	sessions sync.Map // sessionID -> *mcpSessionState
}

func (api *Api) SetupMCP(instructions string) *MCPHandler {
	h := &MCPHandler{
		Api: api,
		Server: mcpserver.NewMCPServer(
			"coroot",
			"1.0.0",
			mcpserver.WithToolCapabilities(false),
			mcpserver.WithInstructions(instructions),
		),
	}
	h.registerTools()
	return h
}

func (h *MCPHandler) HTTPHandler() http.Handler {
	httpSrv := mcpserver.NewStreamableHTTPServer(
		h.Server,
		mcpserver.WithStateful(true),
		mcpserver.WithHTTPContextFunc(func(ctx context.Context, r *http.Request) context.Context {
			if user := h.Api.MCPUserFromBearer(r); user != nil {
				ctx = context.WithValue(ctx, mcpUserCtxKey{}, user)
			}
			return ctx
		}),
	)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.Api.MCPUserFromBearer(r) == nil {
			resourceMeta := h.Api.GetAbsoluteUrl(r, "/.well-known/oauth-protected-resource").String()
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm="coroot", resource_metadata="%s"`, resourceMeta))
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		httpSrv.ServeHTTP(w, r)
	})
}

func mcpUserFromContext(ctx context.Context) *db.User {
	u, _ := ctx.Value(mcpUserCtxKey{}).(*db.User)
	return u
}

func (h *MCPHandler) sessionState(ctx context.Context) *mcpSessionState {
	cs := mcpserver.ClientSessionFromContext(ctx)
	if cs == nil {
		return nil
	}
	id := cs.SessionID()
	if id == "" {
		return nil
	}
	if v, ok := h.sessions.Load(id); ok {
		return v.(*mcpSessionState)
	}
	st := &mcpSessionState{}
	actual, _ := h.sessions.LoadOrStore(id, st)
	return actual.(*mcpSessionState)
}

func (h *MCPHandler) currentProject(ctx context.Context) (*db.Project, error) {
	st := h.sessionState(ctx)
	if st == nil {
		return nil, nil
	}
	st.mu.Lock()
	id := st.projectId
	st.mu.Unlock()
	if id == "" {
		return nil, nil
	}
	return h.Api.db.GetProject(id)
}

func (h *MCPHandler) AddTool(tool mcp.Tool, handler mcpserver.ToolHandlerFunc) {
	name := tool.Name
	h.Server.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		h.Api.stats.RegisterMCPCall(name)
		return handler(ctx, req)
	})
}

func (h *MCPHandler) registerTools() {
	h.AddTool(
		mcp.NewTool("list_projects",
			mcp.WithDescription("List Coroot projects (clusters) the current user can access. Returns a {name: id} map. Call before select_project (which takes the id)."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
		),
		h.toolListProjects,
	)
	h.AddTool(
		mcp.NewTool("select_project",
			mcp.WithDescription("Set the active project (cluster) for subsequent tool calls in this session."),
			mcp.WithString("project_id",
				mcp.Required(),
				mcp.Description("Project id from list_projects."),
			),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
		),
		h.toolSelectProject,
	)
	h.AddTool(
		mcp.NewTool("list_applications",
			mcp.WithDescription("List applications in the selected project. Returns id, namespace, application category (e.g. application, monitoring, control-plane), detected types (postgres, java, nginx, ...), overall status (ok|warning|critical), and a list of failing inspections (CPU, Memory, SLO, Postgres, Logs, ...) with their statuses. Use this to triage which apps to drill into via get_application_status."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
		),
		h.toolListApplications,
	)
	h.AddTool(
		mcp.NewTool("list_alerts",
			mcp.WithDescription("List alerts for the selected project. Use state to choose: 'firing' (active triage, default), 'resolved' (most-recent resolved history), or 'any' (mixed, sorted by opened time)."),
			mcp.WithString("state", mcp.Description("'firing' | 'resolved' | 'any'. Default: 'firing'.")),
			mcp.WithString("app_id", mcp.Description("Filter to one application (id from list_applications).")),
			mcp.WithNumber("limit", mcp.Description("Max alerts to return. Default: 100, max: 1000.")),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
		),
		h.toolListAlerts,
	)
	h.AddTool(
		mcp.NewTool("list_incidents",
			mcp.WithDescription("List SLO incident summaries (severity, opened/resolved time, burn rates, impact). Use get_incident_details for full RCA + propagation map."),
			mcp.WithString("state", mcp.Description("'open' | 'resolved' | 'any'. Default: 'any' (open first, then most-recent resolved).")),
			mcp.WithString("app_id", mcp.Description("Filter to one application (id from list_applications).")),
			mcp.WithNumber("hours", mcp.Description("Look-back window in hours. Default: 0 (no time filter — most recent overall).")),
			mcp.WithNumber("limit", mcp.Description("Max incidents to return. Default: 50, max: 500.")),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
		),
		h.toolListIncidents,
	)
	h.AddTool(
		mcp.NewTool("resolve_alerts",
			mcp.WithDescription("Manually resolve one or more alerts. Triggers configured downstream notifications (Slack, PagerDuty, ...). Use only after confirming the underlying issue is fixed — alerts whose conditions still hold will re-fire on the next evaluation cycle."),
			mcp.WithArray("ids",
				mcp.Required(),
				mcp.Description("Alert ids from list_alerts."),
				mcp.WithStringItems(),
			),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		),
		h.toolResolveAlerts,
	)
	h.AddTool(
		mcp.NewTool("get_incident_details",
			mcp.WithDescription("Get full incident: summary + RCA (root cause, fixes) + propagation map showing how the failure spread across applications."),
			mcp.WithString("key", mcp.Required(), mcp.Description("Incident key from list_incidents.")),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
		),
		h.toolGetIncidentDetails,
	)
	h.AddTool(
		mcp.NewTool("get_application_status",
			mcp.WithDescription("Get health status of an application: overall status, per-inspection (CPU, Memory, SLO, Postgres, ...) status with failing checks, upstream dependencies with status / connectivity / RTT / request latency, and downstream clients (apps that call this one) with their status only."),
			mcp.WithString("app_id",
				mcp.Required(),
				mcp.Description("Application id from list_applications (4-part 'cluster_id:namespace:Kind:name', e.g. 'hwvop6p7:default:Deployment:checkout')."),
			),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
		),
		h.toolGetApplicationStatus,
	)
	h.AddTool(
		mcp.NewTool("list_nodes",
			mcp.WithDescription("List nodes (hosts/VMs) in the selected project: name, cluster, status (up/down/no-agent), OS/kernel, instance type, CPU/memory utilization %, network throughput, GPUs. Mirrors the UI's Nodes view."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
		),
		h.toolListNodes,
	)
	h.AddTool(
		mcp.NewTool("get_node_details",
			mcp.WithDescription("Per-node audit report (CPU/memory/disk/network inspections + their checks). Use after list_nodes to drill into a specific host."),
			mcp.WithString("name", mcp.Required(), mcp.Description("Node name from list_nodes.")),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
		),
		h.toolGetNodeDetails,
	)
	h.AddTool(
		mcp.NewTool("traces_summary",
			mcp.WithDescription("Per-endpoint distributed-trace summary: requests/sec, error rate, p50/p95/p99 latency. The 'full picture' for triage. Pass service+span to focus on one endpoint (the UI's drill-down)."),
			mcp.WithString("service", mcp.Description("Filter to one service.name (e.g. 'checkout').")),
			mcp.WithString("span", mcp.Description("Filter to one SpanName (e.g. 'GET /cart').")),
			mcp.WithString("from", mcp.Description("Start time. Epoch ms or relative like 'now-1h'. Default: 'now-1h'.")),
			mcp.WithString("to", mcp.Description("End time, same format. Default: 'now'.")),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
		),
		h.toolTracesSummary,
	)
	h.AddTool(
		mcp.NewTool("traces_errors",
			mcp.WithDescription("Top error reasons across distributed traces — grouped by endpoint, with a sample trace_id and the error message for each. Use after traces_summary identifies a high-error endpoint."),
			mcp.WithString("service", mcp.Description("Filter to one service.name.")),
			mcp.WithString("span", mcp.Description("Filter to one SpanName.")),
			mcp.WithString("from", mcp.Description("Start time. Epoch ms or relative like 'now-1h'. Default: 'now-1h'.")),
			mcp.WithString("to", mcp.Description("End time, same format. Default: 'now'.")),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
		),
		h.toolTracesErrors,
	)
	h.AddTool(
		mcp.NewTool("traces_outliers",
			mcp.WithDescription("Latency explorer: returns a flamegraph of where time is spent in the SLOW tail vs the rest. Selection = traces with duration in [dur_from..dur_to] (or just dur_from for 'slower than'); baseline = the rest. Use to explain why p99 is high."),
			mcp.WithString("service", mcp.Description("Filter to one service.name.")),
			mcp.WithString("span", mcp.Description("Filter to one SpanName.")),
			mcp.WithString("dur_from", mcp.Description("Min duration of slow band, e.g. '1s', '500ms'. Default: '1s'.")),
			mcp.WithString("dur_to", mcp.Description("Max duration, e.g. '5s', 'inf'. Default: 'inf'.")),
			mcp.WithString("from", mcp.Description("Start time. Epoch ms or relative like 'now-1h'. Default: 'now-1h'.")),
			mcp.WithString("to", mcp.Description("End time, same format. Default: 'now'.")),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
		),
		h.toolTracesOutliers,
	)
	h.AddTool(
		mcp.NewTool("get_trace",
			mcp.WithDescription("Fetch a single distributed trace by id. Returns the full span tree with attributes and events. Use trace_ids returned by traces_errors / traces_summary samples."),
			mcp.WithString("trace_id", mcp.Required(), mcp.Description("Trace id (e.g. from traces_errors.sample_trace_id).")),
			mcp.WithString("from", mcp.Description("Start time. Default: 'now-1h'.")),
			mcp.WithString("to", mcp.Description("End time. Default: 'now'.")),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
		),
		h.toolGetTrace,
	)
	h.AddTool(
		mcp.NewTool("list_metric_names",
			mcp.WithDescription("List metric names available in the project's metrics backend (Prometheus or ClickHouse-as-Prometheus). Use this to discover what metrics exist before querying with query_metrics. Returns at most `limit` distinct names. Cheap — runs `group by (__name__) ({__name__=~match})` over a recent 5-minute window."),
			mcp.WithString("match", mcp.Description("RE2 regex applied to the metric name (must match a non-empty value). Default: '.+' (all metrics). Examples: 'redis.*', 'container_net_.*', 'kube_pod_.*'.")),
			mcp.WithNumber("limit", mcp.Description("Max names to return. Default: 500, max: 5000.")),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
		),
		h.toolListMetricNames,
	)
	h.AddTool(
		mcp.NewTool("query_metrics",
			mcp.WithDescription("Run a PromQL range query against the project's metrics backend. Returns time series with their labels and a value summary (last/min/max/avg + sparkline). Use this to inspect raw metric values, label distributions, or to verify Coroot's detection/aggregation logic. Discover metric names with list_metric_names first."),
			mcp.WithString("query", mcp.Required(), mcp.Description("PromQL expression. Examples: 'up', 'rate(container_net_tcp_active_connections[1m])', 'group by (instance) ({__name__=\"redis_up\"})'.")),
			mcp.WithString("from", mcp.Description("Start time. Either epoch milliseconds, or a relative string like 'now-1h', 'now-15m'. Default: 'now-1h'.")),
			mcp.WithString("to", mcp.Description("End time, same format as `from`. Default: 'now'.")),
			mcp.WithNumber("step_seconds", mcp.Description("Query step in seconds. Default: project refresh interval (typically 30s).")),
			mcp.WithNumber("limit", mcp.Description("Max series to return. Default: 100, max: 1000. If the query returned more, the response sets `truncated: true`.")),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
		),
		h.toolQueryMetrics,
	)
	h.AddTool(
		mcp.NewTool("query_logs",
			mcp.WithDescription("Query log entries from ClickHouse, scoped to one application or across the whole project. Supports time range, severity filter, full-text search, and log-pattern filter. Returned entries are sorted newest-first and each includes the originating service so cluster-wide queries remain attributable."),
			mcp.WithString("app_id", mcp.Description("Application id from list_applications (4-part 'cluster_id:namespace:Kind:name', e.g. 'hwvop6p7:default:Deployment:checkout'). Omit to search across all applications in the project.")),
			mcp.WithString("from", mcp.Description("Start time. Epoch ms or relative like 'now-1h'. Default: 'now-1h'.")),
			mcp.WithString("to", mcp.Description("End time, same format as `from`. Default: 'now'.")),
			mcp.WithNumber("limit", mcp.Description("Max entries. Default: 100, max: 1000.")),
			mcp.WithArray("severity",
				mcp.Description("Filter to one or more severities (OR). Allowed: 'unknown','trace','debug','info','warning','error','fatal'. Default: all."),
				mcp.WithStringItems(),
			),
			mcp.WithString("search", mcp.Description("Full-text search over the log body. Tokenized on whitespace/punctuation; tokens are AND'd, case-insensitive variants are OR'd within each token. Backed by ClickHouse `hasToken`.")),
			mcp.WithString("log_pattern", mcp.Description("Pattern hash from get_application_status's log_patterns. When app_id is set, the hash is expanded to its similar-pattern equivalence class so all variants of the pattern match. Source is forced to 'agent' since pattern hashes are only emitted by the node-agent.")),
			mcp.WithString("source", mcp.Description("'auto' (default) | 'agent' (container stdout/stderr collected by coroot-node-agent) | 'otel' (OpenTelemetry-shipped logs). When `app_id` is omitted and source is 'auto', defaults to 'agent'.")),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
		),
		h.toolQueryLogs,
	)
}

func (h *MCPHandler) toolListProjects(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user := mcpUserFromContext(ctx)
	if user == nil {
		return mcp.NewToolResultError("unauthorized"), nil
	}
	names, err := h.Api.db.GetProjectNames()
	if err != nil {
		klog.Errorln("mcp: list_projects:", err)
		return mcp.NewToolResultError("failed to load projects"), nil
	}
	out := map[string]string{}
	for id, name := range names {
		if !h.Api.IsAllowed(user, rbac.Actions.Project(string(id)).List()...) {
			continue
		}
		out[name] = string(id)
	}
	return MCPJSON(out)
}

func (h *MCPHandler) toolSelectProject(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user := mcpUserFromContext(ctx)
	if user == nil {
		return mcp.NewToolResultError("unauthorized"), nil
	}
	projectId, err := req.RequireString("project_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if !h.Api.IsAllowed(user, rbac.Actions.Project(projectId).List()...) {
		return mcp.NewToolResultError("forbidden: no access to this project"), nil
	}
	p, err := h.Api.db.GetProject(db.ProjectId(projectId))
	if err != nil || p == nil {
		return mcp.NewToolResultError("project not found"), nil
	}
	st := h.sessionState(ctx)
	if st == nil {
		return mcp.NewToolResultError("no session"), nil
	}
	st.mu.Lock()
	st.projectId = p.Id
	st.mu.Unlock()
	return MCPJSON(map[string]string{"selected_project_id": string(p.Id), "name": p.Name})
}

type mcpAppInfo struct {
	Id        string             `json:"id"`
	Namespace string             `json:"namespace,omitempty"`
	Category  string             `json:"category,omitempty"`
	Types     []string           `json:"types,omitempty"`
	Status    string             `json:"status,omitempty"`
	Issues    []mcpInspectionRef `json:"issues,omitempty"`
}

type mcpInspectionRef struct {
	Inspection string `json:"inspection"`
	Status     string `json:"status"`
}

func (h *MCPHandler) RequireUserAndProject(ctx context.Context) (*db.User, *db.Project, *mcp.CallToolResult) {
	user := mcpUserFromContext(ctx)
	if user == nil {
		return nil, nil, mcp.NewToolResultError("unauthorized")
	}
	project, err := h.currentProject(ctx)
	if err != nil {
		return nil, nil, mcp.NewToolResultError("failed to load project: " + err.Error())
	}
	if project == nil {
		return nil, nil, mcp.NewToolResultError("no project selected — call select_project first")
	}
	if !h.Api.IsAllowed(user, rbac.Actions.Project(string(project.Id)).List()...) {
		return nil, nil, mcp.NewToolResultError("forbidden: no access to this project")
	}
	return user, project, nil
}

func (h *MCPHandler) toolListApplications(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user, project, errResult := h.RequireUserAndProject(ctx)
	if errResult != nil {
		return errResult, nil
	}
	now := timeseries.Now()
	world, _, err := h.Api.LoadWorld(ctx, project, now.Add(-timeseries.Hour), now)
	if err != nil {
		klog.Errorln("mcp: list_applications:", err)
		return mcp.NewToolResultError("failed to load world"), nil
	}
	if world == nil {
		return MCPJSON([]mcpAppInfo{})
	}
	auditor.Audit(world, project, nil, nil)
	apps := make([]mcpAppInfo, 0, len(world.Applications))
	for _, app := range world.Applications {
		if !h.Api.IsAllowed(user, rbac.Actions.Project(string(project.Id)).Application(app.Category, app.Id.Namespace, app.Id.Kind, app.Id.Name).View()) {
			continue
		}
		info := mcpAppInfo{Id: app.Id.String(), Category: string(app.Category)}
		if app.Id.Namespace != "_" {
			info.Namespace = app.Id.Namespace
		}
		for t := range app.ApplicationTypes() {
			info.Types = append(info.Types, string(t))
		}
		sort.Strings(info.Types)
		if app.Status != model.UNKNOWN {
			info.Status = app.Status.String()
		}
		for _, r := range app.Reports {
			if r.Status >= model.WARNING {
				info.Issues = append(info.Issues, mcpInspectionRef{
					Inspection: string(r.Name),
					Status:     r.Status.String(),
				})
			}
		}
		sort.Slice(info.Issues, func(i, j int) bool { return info.Issues[i].Inspection < info.Issues[j].Inspection })
		apps = append(apps, info)
	}
	sort.Slice(apps, func(i, j int) bool { return apps[i].Id < apps[j].Id })
	return MCPJSON(apps)
}

type mcpIssue struct {
	Id      string `json:"id"`
	Title   string `json:"title"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type MCPSeriesValue struct {
	Name      string            `json:"name,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	Last      *float32          `json:"last,omitempty"`
	Min       *float32          `json:"min,omitempty"`
	Max       *float32          `json:"max,omitempty"`
	Avg       *float32          `json:"avg,omitempty"`
	Sparkline []*float32        `json:"sparkline,omitempty"`
}

type mcpChart struct {
	Title  string           `json:"title"`
	Series []MCPSeriesValue `json:"series,omitempty"`
}

type mcpLogPattern struct {
	Hash     string `json:"hash"`
	Severity string `json:"severity"`
	Sample   string `json:"sample"`
	Messages int    `json:"messages"`
}

type mcpReportStatus struct {
	Name        string          `json:"name"`
	Status      string          `json:"status"`
	Issues      []mcpIssue      `json:"issues,omitempty"`
	Charts      []mcpChart      `json:"charts,omitempty"`
	LogPatterns []mcpLogPattern `json:"log_patterns,omitempty"`
}

type mcpLatency struct {
	Avg *float32 `json:"avg,omitempty"`
	P50 *float32 `json:"p50,omitempty"`
	P95 *float32 `json:"p95,omitempty"`
	P99 *float32 `json:"p99,omitempty"`
}

type mcpDependency struct {
	Id                  string      `json:"id"`
	Status              string      `json:"status,omitempty"`
	Connectivity        string      `json:"connectivity"`
	ConnectivityMessage string      `json:"connectivity_message,omitempty"`
	Protocols           []string    `json:"protocols,omitempty"`
	RttSeconds          *float32    `json:"rtt_seconds,omitempty"`
	Rps                 *float32    `json:"rps,omitempty"`
	ErrorsPerSec        *float32    `json:"errors_per_sec,omitempty"`
	LatencySeconds      *mcpLatency `json:"latency_seconds,omitempty"`
}

type mcpAppStatus struct {
	Id           string            `json:"id"`
	Namespace    string            `json:"namespace,omitempty"`
	Vitals       []MCPSeriesValue  `json:"vitals,omitempty"`
	Status       string            `json:"status"`
	Reports      []mcpReportStatus `json:"reports"`
	Dependencies []mcpDependency   `json:"dependencies,omitempty"`
	Clients      []mcpClient       `json:"clients,omitempty"`
}

type mcpClient struct {
	Id     string `json:"id"`
	Status string `json:"status,omitempty"`
}

func (h *MCPHandler) toolGetApplicationStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user, project, errResult := h.RequireUserAndProject(ctx)
	if errResult != nil {
		return errResult, nil
	}
	appIdStr, err := req.RequireString("app_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	now := timeseries.Now()
	world, _, err := h.Api.LoadWorld(ctx, project, now.Add(-timeseries.Hour), now)
	if err != nil {
		klog.Errorln("mcp: get_application_status:", err)
		return mcp.NewToolResultError("failed to load world"), nil
	}
	if world == nil {
		return mcp.NewToolResultError("no data available"), nil
	}
	app, errResult := h.ResolveApp(user, project, world, appIdStr)
	if errResult != nil {
		return errResult, nil
	}

	auditor.Audit(world, project, app, nil)

	out := mcpAppStatus{
		Id:     app.Id.String(),
		Status: app.Status.String(),
		Vitals: mcpComputeVitals(app),
	}
	if app.Id.Namespace != "_" {
		out.Namespace = app.Id.Namespace
	}
	for _, r := range app.Reports {
		rs := mcpReportStatus{Name: string(r.Name), Status: r.Status.String()}
		for _, c := range r.Checks {
			if c.Status < model.WARNING {
				continue
			}
			rs.Issues = append(rs.Issues, mcpIssue{
				Id:      string(c.Id),
				Title:   c.Title,
				Status:  c.Status.String(),
				Message: c.Message,
			})
		}
		if r.Status >= model.WARNING {
			rs.Charts = mcpExtractCharts(r.Widgets)
		}
		if r.Name == model.AuditReportLogs {
			rs.LogPatterns = mcpExtractLogPatterns(app, mcpLogPatternsTopN)
		}
		out.Reports = append(out.Reports, rs)
	}
	sort.Slice(out.Reports, func(i, j int) bool { return out.Reports[i].Name < out.Reports[j].Name })

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

		for p := range conn.RequestsCount {
			dep.Protocols = append(dep.Protocols, string(p))
		}
		sort.Strings(dep.Protocols)
		out.Dependencies = append(out.Dependencies, dep)
	}
	sort.Slice(out.Dependencies, func(i, j int) bool { return out.Dependencies[i].Id < out.Dependencies[j].Id })

	for _, conn := range app.Downstreams {
		if conn == nil || conn.RemoteApplication == nil || !conn.IsActual() {
			continue
		}
		rem := conn.RemoteApplication
		c := mcpClient{Id: rem.Id.String()}
		if rem.Status != model.UNKNOWN {
			c.Status = rem.Status.String()
		}
		out.Clients = append(out.Clients, c)
	}
	sort.Slice(out.Clients, func(i, j int) bool { return out.Clients[i].Id < out.Clients[j].Id })

	return MCPJSON(out)
}

const (
	mcpLogPatternsTopN    = 10
	mcpLogSampleMaxLength = 200
	MCPSparklineBuckets   = 12
)

func mcpExtractCharts(widgets []*model.Widget) []mcpChart {
	var out []mcpChart
	for _, w := range widgets {
		if w.Chart != nil {
			if c := mcpSummarizeChart(w.Chart); c != nil {
				out = append(out, *c)
			}
		}
		if w.ChartGroup != nil {
			for _, ch := range w.ChartGroup.Charts {
				if c := mcpSummarizeChart(ch); c != nil {
					out = append(out, *c)
				}
			}
		}
	}
	return out
}

func mcpSummarizeChart(ch *model.Chart) *mcpChart {
	if ch == nil || ch.IsEmpty() {
		return nil
	}
	data, err := json.Marshal(ch.Series)
	if err != nil {
		return nil
	}
	var series []*model.Series
	if err := json.Unmarshal(data, &series); err != nil {
		return nil
	}
	out := mcpChart{Title: ch.Title}
	for _, s := range series {
		ts, ok := s.Data.(*timeseries.TimeSeries)
		if !ok {
			continue
		}
		sv := MCPSummarize(s.Name, nil, ts, MCPSparklineBuckets)
		if sv == nil {
			continue
		}
		out.Series = append(out.Series, *sv)
	}
	if len(out.Series) == 0 {
		return nil
	}
	return &out
}

func MCPSummarize(name string, labels map[string]string, ts *timeseries.TimeSeries, sparkBuckets int) *MCPSeriesValue {
	out := &MCPSeriesValue{Name: name, Labels: labels}
	if ts.IsEmpty() {
		if labels == nil {
			return nil
		}
		return out
	}
	values := make([]float32, 0, ts.Len())
	var sum, last, minV, maxV float32
	var count int
	hasAny := false
	iter := ts.Iter()
	for iter.Next() {
		_, v := iter.Value()
		values = append(values, v)
		if timeseries.IsNaN(v) {
			continue
		}
		if !hasAny {
			minV, maxV = v, v
			hasAny = true
		} else {
			if v < minV {
				minV = v
			}
			if v > maxV {
				maxV = v
			}
		}
		sum += v
		last = v
		count++
	}
	if !hasAny {
		if labels == nil {
			return nil
		}
		return out
	}
	avg := sum / float32(count)
	out.Last, out.Min, out.Max, out.Avg = &last, &minV, &maxV, &avg
	if sparkBuckets > 0 && len(values) > 0 {
		out.Sparkline = mcpDownsample(values, sparkBuckets)
	}
	return out
}

func mcpComputeVitals(app *model.Application) []MCPSeriesValue {
	var v []MCPSeriesValue
	add := func(name string, ts *timeseries.TimeSeries) {
		if sv := MCPSummarize(name, nil, ts, MCPSparklineBuckets); sv != nil {
			v = append(v, *sv)
		}
	}
	if len(app.AvailabilitySLIs) > 0 {
		sli := app.AvailabilitySLIs[0]
		add("rps", sli.TotalRequests)
		add("errors_per_sec", sli.FailedRequests)
	}
	if len(app.LatencySLIs) > 0 && len(app.LatencySLIs[0].Histogram) > 0 {
		h := app.LatencySLIs[0].Histogram
		add("latency_p50_seconds", model.Quantile(h, 0.5))
		add("latency_p95_seconds", model.Quantile(h, 0.95))
		add("latency_p99_seconds", model.Quantile(h, 0.99))
	}
	cpu := timeseries.NewAggregate(timeseries.NanSum)
	mem := timeseries.NewAggregate(timeseries.NanSum)
	for _, inst := range app.Instances {
		for _, c := range inst.Containers {
			if c == nil || c.InitContainer {
				continue
			}
			cpu.Add(c.CpuUsage)
			mem.Add(c.MemoryRss)
		}
	}
	add("cpu_cores", cpu.Get())
	add("memory_bytes", mem.Get())
	return v
}

func mcpDownsample(values []float32, n int) []*float32 {
	total := len(values)
	if total == 0 || n <= 0 {
		return nil
	}
	out := make([]*float32, n)
	for i := 0; i < n; i++ {
		start := i * total / n
		end := (i + 1) * total / n
		if end > total {
			end = total
		}
		var s float32
		var c int
		for j := start; j < end; j++ {
			if !timeseries.IsNaN(values[j]) {
				s += values[j]
				c++
			}
		}
		if c > 0 {
			v := s / float32(c)
			out[i] = &v
		}
	}
	return out
}

func mcpExtractLogPatterns(app *model.Application, n int) []mcpLogPattern {
	type ranked struct {
		hash     string
		severity model.Severity
		pattern  *model.LogPattern
		total    float32
	}
	var all []ranked
	for sev, lm := range app.LogMessages {
		if lm == nil {
			continue
		}
		for hash, p := range lm.Patterns {
			if p == nil || p.Messages.IsEmpty() {
				continue
			}
			if total := p.Messages.Reduce(timeseries.NanSum); total > 0 {
				all = append(all, ranked{hash: hash, severity: sev, pattern: p, total: total})
			}
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].total > all[j].total })
	if len(all) > n {
		all = all[:n]
	}
	out := make([]mcpLogPattern, 0, len(all))
	for _, r := range all {
		out = append(out, mcpLogPattern{
			Hash:     r.hash,
			Severity: r.severity.String(),
			Sample:   utils.Truncate(r.pattern.Sample, mcpLogSampleMaxLength),
			Messages: int(r.total),
		})
	}
	return out
}

func MCPFormatTime(t timeseries.Time) string {
	if t == 0 {
		return ""
	}
	return t.ToStandard().Format(time.RFC3339)
}

func (h *MCPHandler) toolListAlerts(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user, project, errResult := h.RequireUserAndProject(ctx)
	if errResult != nil {
		return errResult, nil
	}
	if !h.Api.IsAllowed(user, rbac.Actions.Project(string(project.Id)).Alerts().View()) {
		return mcp.NewToolResultError("forbidden: no permission to view alerts in this project"), nil
	}
	state := req.GetString("state", "firing")
	appIdFilter := req.GetString("app_id", "")
	limit := int(req.GetFloat("limit", 100))
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	q := db.AlertsQuery{SortBy: "opened_at", SortDesc: true, Limit: limit}
	switch state {
	case "firing":
		q.IncludeResolved = false
	case "resolved":
		q.IncludeResolved = true
		q.SortBy = "resolved_at" // most-recent resolved first; firing alerts have resolved_at=0 and sort last
	case "any":
		q.IncludeResolved = true
	default:
		return mcp.NewToolResultError("state must be 'firing', 'resolved', or 'any'"), nil
	}

	res, err := h.Api.db.QueryAlerts(project.Id, q)
	if err != nil {
		klog.Errorln("mcp: list_alerts:", err)
		return mcp.NewToolResultError("failed to query alerts"), nil
	}

	var filterId model.ApplicationId
	if appIdFilter != "" {
		filterId, err = model.NewApplicationIdFromString(appIdFilter, string(project.Id))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid app_id: %s", err)), nil
		}
	}

	out := make([]*model.Alert, 0, len(res.Alerts))
	for _, a := range res.Alerts {
		if state == "resolved" && a.ResolvedAt == 0 && a.ManuallyResolvedAt == 0 && !a.Suppressed {
			continue
		}
		if appIdFilter != "" && a.ApplicationId != filterId {
			continue
		}
		if !h.Api.IsAllowed(user, rbac.Actions.Project(string(project.Id)).Application(a.ApplicationCategory, a.ApplicationId.Namespace, a.ApplicationId.Kind, a.ApplicationId.Name).View()) {
			continue
		}
		out = append(out, a)
	}
	return MCPJSON(out)
}

func (h *MCPHandler) toolListIncidents(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user, project, errResult := h.RequireUserAndProject(ctx)
	if errResult != nil {
		return errResult, nil
	}
	state := req.GetString("state", "any")
	switch state {
	case "open", "resolved", "any":
	default:
		return mcp.NewToolResultError("state must be 'open', 'resolved', or 'any'"), nil
	}
	hours := int(req.GetFloat("hours", 0))
	limit := int(req.GetFloat("limit", 50))
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	var filterId model.ApplicationId
	if s := req.GetString("app_id", ""); s != "" {
		id, err := model.NewApplicationIdFromString(s, string(project.Id))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid app_id: %s", err)), nil
		}
		filterId = id
	}

	raw, err := h.fetchIncidents(project.Id, hours, state, limit)
	if err != nil {
		klog.Errorln("mcp: list_incidents:", err)
		return mcp.NewToolResultError("failed to query incidents"), nil
	}

	now := timeseries.Now()
	world, _, _ := h.Api.LoadWorld(ctx, project, now.Add(-timeseries.Hour), now)

	filtered := make([]*model.ApplicationIncident, 0, len(raw))
	for _, i := range raw {
		if !filterId.IsZero() && i.ApplicationId != filterId {
			continue
		}
		if state == "open" && i.Resolved() {
			continue
		}
		if state == "resolved" && !i.Resolved() {
			continue
		}
		var category model.ApplicationCategory
		if world != nil {
			if a := world.GetApplication(i.ApplicationId); a != nil {
				category = a.Category
			}
		}
		if !h.Api.IsAllowed(user, rbac.Actions.Project(string(project.Id)).Application(category, i.ApplicationId.Namespace, i.ApplicationId.Kind, i.ApplicationId.Name).View()) {
			continue
		}
		filtered = append(filtered, i)
	}
	sort.Slice(filtered, func(a, b int) bool {
		ai, aj := filtered[a].Resolved(), filtered[b].Resolved()
		if ai != aj {
			return !ai
		}
		if ai {
			return filtered[a].ResolvedAt > filtered[b].ResolvedAt
		}
		return filtered[a].OpenedAt > filtered[b].OpenedAt
	})
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return MCPJSON(filtered)
}

func (h *MCPHandler) fetchIncidents(projectId db.ProjectId, hours int, state string, limit int) ([]*model.ApplicationIncident, error) {
	if hours > 0 {
		now := timeseries.Now()
		from := now.Add(-timeseries.Duration(hours) * timeseries.Hour)
		byApp, err := h.Api.db.GetApplicationIncidents(projectId, from, now)
		if err != nil {
			return nil, err
		}
		var out []*model.ApplicationIncident
		for _, incs := range byApp {
			out = append(out, incs...)
		}
		return out, nil
	}
	fetch := limit
	if state != "any" {
		fetch = limit * 4
	}
	return h.Api.db.GetLatestIncidents(projectId, fetch)
}

func (h *MCPHandler) toolResolveAlerts(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user, project, errResult := h.RequireUserAndProject(ctx)
	if errResult != nil {
		return errResult, nil
	}
	if !h.Api.IsAllowed(user, rbac.Actions.Project(string(project.Id)).Alerts().Edit()) {
		return mcp.NewToolResultError("forbidden: no permission to edit alerts in this project"), nil
	}
	ids, err := req.RequireStringSlice("ids")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if len(ids) == 0 {
		return mcp.NewToolResultError("ids must be a non-empty array"), nil
	}
	resolvedBy := user.Name
	if resolvedBy == "" {
		resolvedBy = user.Email
	}
	resolvedBy += " (via MCP)"
	notified, err := h.Api.resolveAlerts(project, ids, resolvedBy)
	if err != nil {
		klog.Errorln("mcp: resolve_alerts:", err)
		return mcp.NewToolResultError("failed to resolve alerts"), nil
	}
	return MCPJSON(map[string]any{"resolved": len(ids), "notified": notified})
}

func (h *MCPHandler) toolGetIncidentDetails(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user, project, errResult := h.RequireUserAndProject(ctx)
	if errResult != nil {
		return errResult, nil
	}
	key, err := req.RequireString("key")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	i, err := h.Api.db.GetIncidentByKey(project.Id, key)
	if err != nil {
		klog.Errorln("mcp: get_incident_details:", err)
		return mcp.NewToolResultError("incident not found"), nil
	}
	now := timeseries.Now()
	world, _, _ := h.Api.LoadWorld(ctx, project, now.Add(-timeseries.Hour), now)
	var category model.ApplicationCategory
	if world != nil {
		if a := world.GetApplication(i.ApplicationId); a != nil {
			category = a.Category
		}
	}
	if !h.Api.IsAllowed(user, rbac.Actions.Project(string(project.Id)).Application(category, i.ApplicationId.Namespace, i.ApplicationId.Kind, i.ApplicationId.Name).View()) {
		return mcp.NewToolResultError("forbidden: no access to this application"), nil
	}
	if r := i.RCA; r != nil && len(r.Widgets) > 0 {
		rca := *r
		rca.Widgets = nil
		ic := *i
		ic.RCA = &rca
		i = &ic
	}
	return MCPJSON(i)
}

const (
	mcpMetricsDefaultLimit  = 100
	mcpMetricsMaxLimit      = 1000
	mcpMetricsNamesLimit    = 500
	mcpMetricsNamesMaxLimit = 5000
	mcpMetricsMinStep       = 15 * timeseries.Second
	mcpMetricsMaxRange      = 24 * timeseries.Hour
)

type mcpQuerySeries struct {
	Labels map[string]string `json:"labels,omitempty"`
	Values []*float32        `json:"values"`
}

type mcpQueryMetricsResult struct {
	Query        string           `json:"query"`
	From         string           `json:"from"`
	To           string           `json:"to"`
	StepSeconds  int64            `json:"step_seconds"`
	SeriesTotal  int              `json:"series_total"`
	SeriesReturn int              `json:"series_returned"`
	Truncated    bool             `json:"truncated,omitempty"`
	Series       []mcpQuerySeries `json:"series"`
}

type mcpListMetricNamesResult struct {
	Match     string   `json:"match"`
	Total     int      `json:"total"`
	Returned  int      `json:"returned"`
	Truncated bool     `json:"truncated,omitempty"`
	Names     []string `json:"names"`
}

func (h *MCPHandler) toolQueryMetrics(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user, project, errResult := h.RequireUserAndProject(ctx)
	if errResult != nil {
		return errResult, nil
	}
	if !h.Api.IsAllowed(user, rbac.Actions.Project(string(project.Id)).Metrics().View()) {
		return mcp.NewToolResultError("forbidden: no permission to view metrics in this project"), nil
	}
	query, err := req.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	limit := int(req.GetFloat("limit", mcpMetricsDefaultLimit))
	if limit <= 0 || limit > mcpMetricsMaxLimit {
		limit = mcpMetricsDefaultLimit
	}

	client, err := prom.NewClient(project.PrometheusConfig(h.Api.globalPrometheus), project.ClickHouseConfig(h.Api.globalClickHouse))

	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer client.Close()

	from, to, _, _ := h.Api.getTimeContext(project.Id, req.GetString("from", ""), req.GetString("to", ""), "", "")
	if to.Sub(from) > mcpMetricsMaxRange {
		return mcp.NewToolResultError(fmt.Sprintf("range too large (max %d hours)", int(mcpMetricsMaxRange/timeseries.Hour))), nil
	}

	step, err := client.GetStep(0, 0)
	if s := req.GetFloat("step_seconds", 0); s > 0 {
		step = timeseries.Duration(s)
	}
	if step < mcpMetricsMinStep {
		step = mcpMetricsMinStep
	}

	series, err := client.QueryRange(ctx, query, prom.FilterLabelsKeepAll, from, to, step)
	if err != nil {
		return mcp.NewToolResultError("query failed: " + err.Error()), nil
	}

	out := mcpQueryMetricsResult{
		Query:        query,
		From:         MCPFormatTime(from),
		To:           MCPFormatTime(to),
		StepSeconds:  int64(step),
		SeriesTotal:  len(series),
		SeriesReturn: len(series),
	}
	if len(series) > limit {
		series = series[:limit]
		out.SeriesReturn = limit
		out.Truncated = true
	}
	out.Series = make([]mcpQuerySeries, 0, len(series))
	for _, mv := range series {
		s := mcpQuerySeries{Labels: mv.Labels}
		if mv.Values != nil {
			s.Values = make([]*float32, 0, mv.Values.Len())
			iter := mv.Values.Iter()
			for iter.Next() {
				_, v := iter.Value()
				if timeseries.IsNaN(v) {
					s.Values = append(s.Values, nil)
					continue
				}
				vv := v
				s.Values = append(s.Values, &vv)
			}
		}
		out.Series = append(out.Series, s)
	}
	sort.Slice(out.Series, func(i, j int) bool {
		return labelsString(out.Series[i].Labels) < labelsString(out.Series[j].Labels)
	})
	return MCPJSON(out)
}

func (h *MCPHandler) toolListNodes(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user, project, errResult := h.RequireUserAndProject(ctx)
	if errResult != nil {
		return errResult, nil
	}
	now := timeseries.Now()
	world, _, err := h.Api.LoadWorld(ctx, project, now.Add(-timeseries.Hour), now)
	if err != nil {
		klog.Errorln("mcp: list_nodes:", err)
		return mcp.NewToolResultError("failed to load world"), nil
	}
	if world == nil {
		return MCPJSON([]overview.Node{})
	}
	all := overview.RenderNodes(world, project)
	out := make([]overview.Node, 0, len(all))
	for _, n := range all {
		if !h.Api.IsAllowed(user, rbac.Actions.Project(string(project.Id)).Node(n.Name).View()) {
			continue
		}
		out = append(out, n)
	}
	return MCPJSON(out)
}

func (h *MCPHandler) toolGetNodeDetails(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user, project, errResult := h.RequireUserAndProject(ctx)
	if errResult != nil {
		return errResult, nil
	}
	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if !h.Api.IsAllowed(user, rbac.Actions.Project(string(project.Id)).Node(name).View()) {
		return mcp.NewToolResultError("forbidden: no access to this node"), nil
	}
	now := timeseries.Now()
	world, _, err := h.Api.LoadWorld(ctx, project, now.Add(-timeseries.Hour), now)
	if err != nil {
		klog.Errorln("mcp: get_node_details:", err)
		return mcp.NewToolResultError("failed to load world"), nil
	}
	if world == nil {
		return mcp.NewToolResultError("no data available"), nil
	}
	node := world.GetNode(name)
	if node == nil {
		return mcp.NewToolResultError("node not found"), nil
	}
	auditor.Audit(world, project, nil, nil)
	report := auditor.AuditNode(world, node)
	if report != nil {
		r := *report
		r.Widgets = nil
		report = &r
	}
	var sparklines []MCPSeriesValue
	add := func(name string, ts *timeseries.TimeSeries) {
		if sv := MCPSummarize(name, nil, ts, MCPSparklineBuckets); sv != nil {
			sparklines = append(sparklines, *sv)
		}
	}
	add("cpu_percent", node.CpuUsagePercent)
	if !node.MemoryTotalBytes.IsEmpty() && !node.MemoryAvailableBytes.IsEmpty() {
		used := timeseries.Sub(node.MemoryTotalBytes, node.MemoryAvailableBytes)
		add("memory_percent", timeseries.Aggregate2(used, node.MemoryTotalBytes, func(u, t float32) float32 {
			if t == 0 {
				return timeseries.NaN
			}
			return 100 * u / t
		}))
	}
	rx := timeseries.NewAggregate(timeseries.NanSum)
	tx := timeseries.NewAggregate(timeseries.NanSum)
	for _, iface := range node.NetInterfaces {
		rx.Add(iface.RxBytes)
		tx.Add(iface.TxBytes)
	}
	add("net_rx_bytes_per_sec", rx.Get())
	add("net_tx_bytes_per_sec", tx.Get())
	return MCPJSON(map[string]any{"report": report, "sparklines": sparklines})
}

func (h *MCPHandler) runTracesQuery(ctx context.Context, req mcp.CallToolRequest, q overview.Query) (*overview.Traces, *mcp.CallToolResult) {
	user, project, errResult := h.RequireUserAndProject(ctx)
	if errResult != nil {
		return nil, errResult
	}
	if !h.Api.IsAllowed(user, rbac.Actions.Project(string(project.Id)).Traces().View()) {
		return nil, mcp.NewToolResultError("forbidden: no permission to view traces in this project")
	}
	from, to, _, _ := h.Api.getTimeContext(project.Id, req.GetString("from", ""), req.GetString("to", ""), "", "")
	world, _, err := h.Api.LoadWorld(ctx, project, from, to)
	if err != nil || world == nil {
		klog.Errorln("mcp: traces:", err)
		return nil, mcp.NewToolResultError("failed to load world")
	}
	chs := h.Api.GetClickhouseClients(project)
	defer chs.Close()
	if s := req.GetString("service", ""); s != "" {
		q.Filters = append(q.Filters, overview.Filter{Field: "ServiceName", Op: "=", Value: s})
	}
	if s := req.GetString("span", ""); s != "" {
		q.Filters = append(q.Filters, overview.Filter{Field: "SpanName", Op: "=", Value: s})
	}
	queryJSON, _ := json.Marshal(q)
	res := overview.RenderTraces(ctx, chs, world, string(queryJSON))
	if res.Error != "" {
		return nil, mcp.NewToolResultError(res.Error)
	}
	return res, nil
}

func (h *MCPHandler) toolTracesSummary(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	res, errResult := h.runTracesQuery(ctx, req, overview.Query{})
	if errResult != nil {
		return errResult, nil
	}
	return MCPJSON(res.Summary)
}

func (h *MCPHandler) toolTracesErrors(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	res, errResult := h.runTracesQuery(ctx, req, overview.Query{View: "errors"})
	if errResult != nil {
		return errResult, nil
	}
	return MCPJSON(res.Errors)
}

func (h *MCPHandler) toolTracesOutliers(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	q := overview.Query{
		View:    "latency",
		DurFrom: req.GetString("dur_from", "1s"),
		DurTo:   req.GetString("dur_to", "inf"),
	}
	res, errResult := h.runTracesQuery(ctx, req, q)
	if errResult != nil {
		return errResult, nil
	}
	return MCPJSON(res.Latency)
}

func (h *MCPHandler) toolGetTrace(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	traceId, err := req.RequireString("trace_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	res, errResult := h.runTracesQuery(ctx, req, overview.Query{TraceId: traceId})
	if errResult != nil {
		return errResult, nil
	}
	return MCPJSON(res.Trace)
}

func (h *MCPHandler) toolListMetricNames(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user, project, errResult := h.RequireUserAndProject(ctx)
	if errResult != nil {
		return errResult, nil
	}
	if !h.Api.IsAllowed(user, rbac.Actions.Project(string(project.Id)).Metrics().View()) {
		return mcp.NewToolResultError("forbidden: no permission to view metrics in this project"), nil
	}
	match := req.GetString("match", ".+")
	if match == "" {
		match = ".+"
	}
	limit := int(req.GetFloat("limit", mcpMetricsNamesLimit))
	if limit <= 0 || limit > mcpMetricsNamesMaxLimit {
		limit = mcpMetricsNamesLimit
	}

	client, err := prom.NewClient(project.PrometheusConfig(h.Api.globalPrometheus), project.ClickHouseConfig(h.Api.globalClickHouse))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer client.Close()

	now := timeseries.Now()
	from := now.Add(-5 * timeseries.Minute)
	step := timeseries.Minute

	query := fmt.Sprintf("group by (__name__) ({__name__=~%q})", match)
	series, err := client.QueryRange(ctx, query, prom.FilterLabelsKeepAll, from, now, step)
	if err != nil {
		return mcp.NewToolResultError("query failed: " + err.Error()), nil
	}
	seen := map[string]struct{}{}
	for _, mv := range series {
		name := mv.Labels["__name__"]
		if name == "" {
			continue
		}
		seen[name] = struct{}{}
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	out := mcpListMetricNamesResult{
		Match:    match,
		Total:    len(names),
		Returned: len(names),
	}
	if len(names) > limit {
		names = names[:limit]
		out.Returned = limit
		out.Truncated = true
	}
	out.Names = names
	return MCPJSON(out)
}

func labelsString(ls map[string]string) string {
	keys := make([]string, 0, len(ls))
	for k := range ls {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b []byte
	for _, k := range keys {
		b = append(b, k...)
		b = append(b, '=')
		b = append(b, ls[k]...)
		b = append(b, ',')
	}
	return string(b)
}

const (
	mcpLogsDefaultLimit = 100
	mcpLogsMaxLimit     = 1000
)

type mcpLogsResult struct {
	AppId    string            `json:"app_id,omitempty"`
	Source   string            `json:"source"`
	From     string            `json:"from"`
	To       string            `json:"to"`
	Limit    int               `json:"limit"`
	Returned int               `json:"returned"`
	Entries  []*model.LogEntry `json:"entries"`
}

func (h *MCPHandler) toolQueryLogs(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user, project, errResult := h.RequireUserAndProject(ctx)
	if errResult != nil {
		return errResult, nil
	}
	if !h.Api.IsAllowed(user, rbac.Actions.Project(string(project.Id)).Logs().View()) {
		return mcp.NewToolResultError("forbidden: no permission to view logs in this project"), nil
	}

	from, to, _, _ := h.Api.getTimeContext(project.Id, req.GetString("from", ""), req.GetString("to", ""), "", "")
	limit := int(req.GetFloat("limit", mcpLogsDefaultLimit))
	if limit <= 0 || limit > mcpLogsMaxLimit {
		limit = mcpLogsDefaultLimit
	}
	search := req.GetString("search", "")
	logPattern := strings.TrimSpace(req.GetString("log_pattern", ""))
	source := model.LogSource(req.GetString("source", ""))
	if logPattern != "" {
		source = model.LogSourceAgent
	}

	world, _, err := h.Api.LoadWorld(ctx, project, from, to)
	if err != nil {
		klog.Errorln("mcp: query_logs:", err)
		return mcp.NewToolResultError("failed to load world"), nil
	}
	if world == nil {
		return mcp.NewToolResultError("no data available"), nil
	}

	app, errResult := h.ResolveApp(user, project, world, req.GetString("app_id", ""))
	if errResult != nil {
		return errResult, nil
	}
	clusterId := ""
	if app != nil {
		clusterId = app.Id.ClusterId
	}
	ch, err := h.Api.GetClickhouseClient(project, clusterId)
	if err != nil || ch == nil {
		return mcp.NewToolResultError("ClickHouse is not configured for this project"), nil
	}

	source, services, errResult := h.resolveLogSource(ctx, ch, world, app, source)
	if errResult != nil {
		return errResult, nil
	}

	lq := clickhouse.LogQuery{
		Ctx:      timeseries.NewContext(from, to, timeseries.Minute),
		Source:   source,
		Services: services,
		Limit:    limit,
	}
	for _, sev := range req.GetStringSlice("severity", nil) {
		if s := strings.ToLower(strings.TrimSpace(sev)); s != "" {
			lq.Filters = append(lq.Filters, clickhouse.LogFilter{Name: "Severity", Op: "=", Value: s})
		}
	}
	if search != "" {
		lq.Filters = append(lq.Filters, clickhouse.LogFilter{Name: "Message", Op: "=", Value: search})
	}
	if logPattern != "" {
		hashes := []string{logPattern}
		if app != nil {
			hashes = app.SimilarLogPatternHashes(logPattern)
		}
		for _, h := range hashes {
			lq.Filters = append(lq.Filters, clickhouse.LogFilter{Name: "pattern.hash", Op: "=", Value: h})
		}
	}

	entries, err := ch.GetLogs(ctx, lq)
	if err != nil {
		klog.Errorln("mcp: query_logs:", err)
		return mcp.NewToolResultError("failed to query logs: " + err.Error()), nil
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Timestamp.After(entries[j].Timestamp) })
	out := mcpLogsResult{
		Source:   string(source),
		From:     MCPFormatTime(from),
		To:       MCPFormatTime(to),
		Limit:    limit,
		Returned: len(entries),
		Entries:  entries,
	}
	if app != nil {
		out.AppId = app.Id.String()
	}
	return MCPJSON(out)
}

func (h *MCPHandler) ResolveApp(user *db.User, project *db.Project, world *model.World, appIdStr string) (*model.Application, *mcp.CallToolResult) {
	if appIdStr == "" {
		return nil, nil
	}
	appId, err := model.NewApplicationIdFromString(appIdStr, string(project.Id))
	if err != nil {
		return nil, mcp.NewToolResultError(fmt.Sprintf("invalid app_id: %s", err))
	}
	app := world.GetApplication(appId)
	if app == nil {
		return nil, mcp.NewToolResultError("application not found")
	}
	if !h.Api.IsAllowed(user, rbac.Actions.Project(string(project.Id)).Application(app.Category, app.Id.Namespace, app.Id.Kind, app.Id.Name).View()) {
		return nil, mcp.NewToolResultError("forbidden: no access to this application")
	}
	return app, nil
}

func (h *MCPHandler) resolveLogSource(ctx context.Context, ch *clickhouse.Client, world *model.World, app *model.Application, source model.LogSource) (model.LogSource, []string, *mcp.CallToolResult) {
	if app == nil {
		if source == "" {
			source = model.LogSourceAgent
		}
		return source, nil, nil
	}

	otelService := ""
	if source != model.LogSourceAgent {
		otelServices, _, err := ch.GetLogSources(ctx, world.Ctx.From)
		if err != nil {
			klog.Errorln("mcp: query_logs: get services:", err)
			return "", nil, mcp.NewToolResultError("failed to load services from logs: " + err.Error())
		}
		otelService = app.OtelLogService(otelServices, world)
	}

	if source == "" {
		if otelService != "" {
			source = model.LogSourceOtel
		} else {
			source = model.LogSourceAgent
		}
	}

	services := app.LogQueryServices(source, otelService)
	if source == model.LogSourceOtel && otelService == "" {
		return "", nil, mcp.NewToolResultError("no OpenTelemetry service is associated with this application")
	}
	if source == model.LogSourceAgent && len(services) == 0 {
		return "", nil, mcp.NewToolResultError("no container services found for this application — try source='otel'")
	}
	return source, services, nil
}

func mcpLastNotNull(ts *timeseries.TimeSeries) *float32 {
	_, v := ts.LastNotNull()
	if timeseries.IsNaN(v) {
		return nil
	}
	return &v
}

func mcpAggregateUpstreamHistogram(from, to *model.Application) []model.HistogramBucket {
	sums := map[float32]*timeseries.Aggregate{}
	for _, inst := range from.Instances {
		for _, u := range inst.Upstreams {
			if u == nil || u.RemoteApplication() != to {
				continue
			}
			for _, byLe := range u.RequestsHistogram {
				for le, ts := range byLe {
					agg, ok := sums[le]
					if !ok {
						agg = timeseries.NewAggregate(timeseries.NanSum)
						sums[le] = agg
					}
					agg.Add(ts)
				}
			}
		}
	}
	if len(sums) == 0 {
		return nil
	}
	out := make([]model.HistogramBucket, 0, len(sums))
	for le, agg := range sums {
		out = append(out, model.HistogramBucket{Le: le, TimeSeries: agg.Get()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Le < out[j].Le })
	return out
}

func MCPJSON(v any) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return mcp.NewToolResultText(string(data)), nil
}
