---
sidebar_position: 1
hide_table_of_contents: true
---

# Overview

## Agent-ready observability

Coroot exposes its observability data to AI agents through the [Model Context Protocol](https://modelcontextprotocol.io/). Point any MCP-compatible agent (Claude Code, Cursor, Codex) at the Coroot endpoint and it can investigate live production systems the way an SRE does, from the big picture down to the raw telemetry.

The agent gets:

* **Application topology**. Which services exist, what each one talks to, what depends on what.
* **Health signals**. Current alerts, open SLO incidents, per-app inspection results (CPU, memory, SLO, postgres, logs), node-level health.
* **Distributed traces**. Per-endpoint rps, error rate, latency percentiles, error reasons grouped by endpoint, latency-tail flamegraphs, full traces by id.
* **Raw telemetry**. PromQL queries, log search, metric discovery, distributed traces.

This is the same data the Coroot UI uses, exposed as structured tools so an LLM can drive triage and investigation without screen-scraping dashboards.

## Tools

The MCP endpoint is served at `/mcp` on your Coroot instance. Tools marked **EE** are available in Coroot Enterprise Edition. The rest ship with the Community Edition.

| Tool | Purpose | Returns |
| --- | --- | --- |
| `list_projects` | Discover projects (clusters) the user can access. | A `{name: id}` map. |
| `select_project` | Set the active project for the session. | Acknowledgement of the selected project. |
| `list_applications` | Triage which apps to look at. Filter with `namespace`, `search`, `min_status`. | One row per application (unhealthy first) with id, namespace, category, detected types (postgres, java), overall status, list of failing inspections. |
| `list_alerts` | See currently firing or recently resolved alerts. | List of alerts with id, application, severity, summary, opened and resolved timestamps, full alert details. |
| `list_incidents` | Browse the SLO incident timeline (open and resolved). | Incidents with id, application, severity, opened and resolved timestamps, burn rates, impact. |
| `list_nodes` | Get a fleet-wide host overview. Filter with `search`. | One row per node (down first) with id, name, cluster, status, OS, kernel, instance type, current CPU%, memory%, GPUs, network throughput, IPs. |
| `get_application_status` | Drill into one application's health. | Overall status, per-inspection issues with the failing checks, top log-pattern samples, upstream dependencies (connectivity, RTT, request latency), downstream clients. |
| `get_incident_details` | Pull full context on one SLO incident. | One incident with full burn rates, impact percentages, and any persisted RCA (root cause, immediate fixes, propagation map). |
| `get_node_details` | Drill into one host. | Per-node audit report (CPU, Memory, Disk, Network, GPU inspections plus their checks) and sparklines for CPU%, memory%, network rx and tx. |
| `traces_summary` | Triage which endpoints are slow or failing. | Per-endpoint stats. Requests per second, error rate, p50, p95, p99 latency. Optionally focused on one `service` and `span`. |
| `traces_errors` | Find out why requests fail. | Top error reasons grouped by endpoint with count, sample error message, sample `trace_id`. |
| `traces_outliers` | Explain why p95 or p99 is high. | Latency flamegraph that diffs slow traces (`dur_from..dur_to`) against the rest, showing where time is spent in the slow tail. |
| `get_trace` | Inspect one specific request end to end. | Full span tree for one trace. Each span has id, parent, service, name, timestamp, duration, status, plus attributes and events. |
| `query_metrics` | Run a custom PromQL query. | Time series for the expression. Per-series labels and raw values aligned to a step (about 120 points per series at most, the step is widened for long ranges). |
| `list_metric_names` | Discover what metrics exist. | Distinct metric names, filterable by regex. |
| `query_logs` | Search application or project-wide logs. | Log entries (newest first) with timestamp, severity, body (cut to `max_body_length`, 1000 characters by default), trace id, log and resource attributes. |
| `resolve_alerts` | Manually resolve alerts after the underlying issue is fixed. | Number of alerts resolved and notifications sent. |
| **`list_anomalies`** *(EE)* | Surface SLO violations and sub-SLO error or latency spikes across the fleet. | Apps with active anomalies, each with status, sample issue messages, and the related open incident. |
| **`investigate_anomaly`** *(EE)* | Find the root cause of a problem in one app. Coroot follows the dependency graph from the affected service the way an engineer would, checking each candidate cause (saturation, deploys, downstream errors, slow databases, log spikes, profile shifts) against the anomaly window. The findings are then handed to an LLM that writes the human-readable explanation. | Root cause, immediate fixes, a detailed explanation, and a propagation map showing how the failure spread across services. Persisted onto the incident when an `incident_key` is provided, so subsequent `get_incident_details` calls return the same RCA without rerunning it. |

:::info
The tools marked **EE** are available in Coroot Enterprise Edition (from $1 per CPU core/month). [Start](https://coroot.com/account) your free trial today.
:::

The Enterprise Edition tools turn Coroot into a production-aware investigator. The agent does not have to feed raw metrics, logs, and traces back into an LLM and hope it spots the pattern. Coroot already knows the topology of each system, the SLIs of each service, the deploy history, and how the components depend on each other. `investigate_anomaly` uses that model directly. It correlates the affected SLI against every candidate signal (CPU, memory, GC, network, log patterns, downstream latency, deploys) using deterministic ML, ranks the most likely causes, and only then asks an LLM to put the result into words. What the agent receives is a focused diagnosis with evidence, not a wall of telemetry to interpret.

## Connecting an agent

The endpoint is `https://<your-coroot>/mcp` with HTTP streamable transport and OAuth 2.0 (see [Authentication](#authentication) below).

### Claude Code

```bash
claude mcp add --transport http coroot https://<your-coroot>/mcp
```

Then start `claude`, run the `/mcp` slash command, pick **coroot**, and choose **Authenticate**. A browser opens to complete the OAuth flow.

### Cursor

Edit `~/.cursor/mcp.json`.

```json
{
  "mcpServers": {
    "coroot": {
      "url": "https://<your-coroot>/mcp"
    }
  }
}
```

Open **Cursor Settings → MCP**, find the **coroot** server, and click **Connect** to start the OAuth flow.

### Codex

```bash
codex mcp add coroot --url https://<your-coroot>/mcp
```

Then start `codex`, run the `/mcp` slash command, pick **coroot**, and choose **Authenticate**. A browser opens to complete the OAuth flow.

## Authentication

The MCP endpoint supports two ways to authenticate. In both cases every tool call is authorized server side against Coroot's RBAC, so an agent can only see and act on what its identity is allowed to.

### Interactive clients: OAuth 2.0

Each user signs in with their own Coroot account on first connect, and the agent runs with that user's RBAC permissions. This is what Claude Code, Cursor, and Codex do when you follow the steps above.

### Autonomous agents: service accounts and API keys

A headless agent (a scheduled investigation job, a remote agent runtime, a CI step) cannot complete a browser-based OAuth flow. For such agents, create a **[service account](/configuration/authentication#service-accounts-and-api-keys)**: a Coroot user without a password that authenticates with an API key. The service account has a regular role (Viewer, Editor, or a custom role in the Enterprise Edition), so the same RBAC rules apply. A Viewer can investigate but cannot resolve alerts, an Editor can do both, and an EE custom role can be scoped to specific projects.

The agent sends the key as a bearer token:

```
Authorization: Bearer <api key>
```

The key works for the `/mcp` endpoint and for the regular HTTP API.

**Via the UI**. Go to **Project settings → Organization**, click **Add user**, check **Service account**, and pick a role. Then click the key icon next to the account to create an API key. The key is shown once.

<img alt="API keys of a service account" src="/img/docs/service_account_api_keys.png" class="card w-600"/>

**Via the config file** (recommended for infrastructure-as-code setups). Define the account and its keys in `config.yaml`. Coroot creates the account on startup, and its keys are exactly the ones listed here, so adding, removing, or rotating a key is a config change and a restart. Accounts defined this way are locked in the UI. Only the SHA-256 hash of a key is stored in the database.

```yaml
auth:
  serviceAccounts:
    - name: claude-agent
      role: Viewer
      apiKeys:
        - key: ${CLAUDE_AGENT_API_KEY}   # environment variables are expanded
          description: production investigation agent
```

**Connecting a client with a key**. Any MCP client that supports custom headers can use a key instead of OAuth, for example Claude Code:

```bash
claude mcp add --transport http coroot https://<your-coroot>/mcp \
  --header "Authorization: Bearer <api key>"
```

or a generic `mcp.json`:

```json
{
  "mcpServers": {
    "coroot": {
      "url": "https://<your-coroot>/mcp",
      "headers": { "Authorization": "Bearer <api key>" }
    }
  }
}
```

Service accounts are independent of SSO. They never log in, so they keep working when password login is disabled in favor of SSO.

## Switching between projects

A Coroot project usually corresponds to one cluster, and a single MCP session can move between as many projects as the user has access to.

* The agent calls `list_projects` to see what is available. The result is a `{name: id}` map.
* It calls `select_project` with one id. The selection sticks for the rest of the session, so subsequent tools (list_applications, query_logs, traces_summary) all run against that project.
* Switching is just another `select_project` call. No reconnect, no re-auth.
* Application ids returned by tools are 4-part `cluster_id:namespace:Kind:name` (for example `hwvop6p7:default:Deployment:checkout`). The cluster prefix keeps ids unambiguous when the user moves between projects, so pass them back unchanged. Short forms such as `namespace:Kind:name` are rejected with an error that asks for the full id.
* Node ids work the same way. They are `cluster_id:name` (for example `hwvop6p7:ip-10-0-1-15`). Names of managed databases contain a colon themselves, so their ids look like `hwvop6p7:rds:db1`. `get_node_details` takes the `id` returned by `list_nodes`.

If you also use [multi-cluster projects](../configuration/multi-cluster.md), they show up alongside regular ones in `list_projects`. Pick the multi-cluster project to query the aggregated view, or pick a member project to scope the question to one cluster.

## Response size

Agent runtimes reject tool results that are too large for the model's context (Claude Code, for example, caps MCP tool output at 25,000 tokens), and some of them report such a call as failed even though the data was fetched. Coroot sizes every tool response to stay within such limits.

* Responses are summaries built for an LLM, not the payloads the UI uses. Charts are reduced to last/min/max/avg plus a 12-point sparkline, and only failing inspections carry charts (top 10 series per chart).
* List results (`list_applications`, `list_alerts`, `list_incidents`, `list_nodes`, `traces_summary`, `traces_errors`, `get_trace`) come as `{total, returned, items}` and are cut at about 50 KB. `query_logs` entries and `query_metrics` series are cut the same way.
* When a result is cut, the response sets `truncated: true` and a `hint` that tells the agent how to narrow the request (filters, a smaller `limit`, a shorter time range). The most relevant items come first: unhealthy applications, down nodes, newest log entries, busiest endpoints, most frequent errors.
* Any other response is capped at 80 KB.
