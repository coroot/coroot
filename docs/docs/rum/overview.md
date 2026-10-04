---
sidebar_position: 1
---

# Real User Monitoring (RUM)

Coroot RUM collects **real browser telemetry** — Core Web Vitals, page loads, fetch/XHR, and JavaScript errors — and correlates it with backend metrics and traces already collected by Coroot (eBPF / OpenTelemetry).

## Architecture

1. The lightweight [`coroot-rum`](https://github.com/coroot/coroot/tree/main/rum-js) script runs in the browser.
2. It exports spans over **OTLP/HTTP** (`POST /v1/traces`) using a **RUM API key** (origin allowlist).
3. Coroot stores data in dedicated ClickHouse tables: `rum_spans`, `rum_events`, histogram MVs, and `rum_service_edges`.
4. Backend OTLP/eBPF data stays in `otel_traces`. Shared **`TraceId` / `traceparent`** joins browser and backend in one trace tree.
5. **Service Map (Topology)** shows `RumClient` nodes (category **frontend**) and edges derived from browser → API calls.
6. Auditor **RUM checks** (LCP/INP/CLS/errors) feed the same alerting pipeline as backend SLO checks. Incidents can include **RUM signals** in the blast radius.

## Setup

### 1. Create a RUM API key

In **Project settings → API keys**, click **Generate RUM key** and set **Allowed origins** to your site origins (exact match), for example:

```
https://shop.example.com
https://www.example.com
```

Optional key fields for server-side sampling: `keep_slow_ms` (default 2500), `keep_error`, `server_sample_rate`.

RUM keys only accept browser traffic (`telemetry.sdk.language=webjs` or `X-Coroot-Signal: rum`). Regular API keys cannot ingest RUM.

### 2. Add the snippet

```html
<script src="https://coroot.example.com/static/rum/coroot-rum.js"></script>
<!-- or CDN: https://cdn.coroot.com/rum/coroot-rum@0.2.js -->
<script>
  CorootRum.init({
    endpoint: "https://coroot.example.com",
    apiKey: "<rum-public-key>",
    serviceName: "web-shop",
    serviceVersion: "1.2.3",
    sampleRate: 0.1,
    allowedTraceUrls: [/https:\/\/api\.example\.com/],
    consent: true,
  });
</script>
```

| Option | Meaning |
|--------|---------|
| `endpoint` | Public Coroot URL reachable from browsers |
| `apiKey` | RUM public key |
| `serviceName` | Appears as a RumClient app on the Service Map |
| `serviceVersion` | Release dimension for before/after CWV compare |
| `sampleRate` | Head sampling (default `0.1`) |
| `allowedTraceUrls` | Origins that receive W3C `traceparent` |
| `consent` | Set `false` until CMP grants; call `setConsent(true)` |

Ensure Coroot is reachable cross-origin; CORS is enabled for RUM keys' allowed origins on `/v1/traces` and `/v1/rum/replay`.

CSP: allow `connect-src` to your Coroot ingest host.

### 3. Open the UI

- Side nav **RUM** — list of frontend services
- Application tab **RUM** — CWV p75, latency heatmap, errors, top pages, versions, linked backends, session timeline
- **Service Map** — RumClient nodes; filter category **frontend**; edges may show `source: rum|both`
- Inspection **RUM** — objectives / checks (LCP ≤ 2500 ms, INP ≤ 200 ms, CLS ≤ 0.1, …)

## Host → backend auto-link

Coroot resolves browser `http.host` / `server.address` to Kubernetes Services (name, FQDN, ClusterIP, LB IP), app listens, Tracing.service, and:

- Per-app `ApplicationSettingsRum.host_patterns`
- Project `settings.rum.host_mappings` (`pattern` → `app_id`)

## Privacy & geo (opt-in)

- Prefer path-only URLs (SDK default); no user ids (random `session.id` only).
- Project `rum.geo_enabled`: enrich `geo.country` / `geo.asn` from `X-Forwarded-For` via local MMDB; store **IP hash**, never raw IP.
- Server keep-slow/error sampling retains slow/error spans even when head sample rate is low (`X-Coroot-Rum-Hint`).

## Session Replay (Enterprise)

Opt-in add-on — not part of the OSS default bundle.

1. Enable `project.settings.rum.replay_enabled`.
2. Load `coroot-rum-replay.js` after consent.
3. Upload goes to `POST /v1/rum/replay` (requires `consent: true`).
4. Storage: `rum_replay_segments` (configurable TTL, default 7d). RBAC scope: `project.rum.replay`.
5. Inputs masked by default (`password`, `email`, `[data-coroot-mask]`).

## What is collected

| Signal | Storage |
|--------|---------|
| Document / SPA soft navigation | `rum_spans` + `rum_page_stats_1m` |
| LCP, INP, CLS, TTFB, FID | `rum_events` + `rum_events_1m` (not duplicated as leaf spans) |
| `fetch` / XHR | `rum_spans` + `rum_service_edges` |
| JS errors | `rum_events` / error spans |
| Replay segments (EE) | `rum_replay_segments` |

## Alerting

Builtin rules: `rum-lcp-p75`, `rum-inp-p75`, `rum-cls-p75`, `rum-js-errors`, `rum-fetch-errors`. Notifications reuse Slack/Teams/PagerDuty/Webhook channels. Backend incidents may attach `rum_signals` when a RumClient upstream is degraded.

## ClickHouse tables

| Table | Role |
|-------|------|
| `rum_spans` | Raw browser spans (detailed drill-down) |
| `rum_events` | Raw CWV / JS errors |
| `rum_page_stats_1m` | Per-minute AggregatingMergeTree rollups for page views |
| `rum_events_1m` | Per-minute rollups for CWV / errors / ratings |
| `rum_spans_histogram` | Latency heatmap MV |
| `rum_web_vitals_hist` | CWV histogram MV |
| `rum_service_name` | Service discovery |
| `rum_service_edges` | Topology edges ClientService → ServerService |
| `rum_replay_segments` | EE session replay blobs |

## Storage and retention

RUM data is split into three retention tiers so dashboards can outlive expensive raw detail:

| Tier | Tables | Default TTL |
|------|--------|-------------|
| Raw | `rum_spans`, `rum_events` | 7d (`--rum-ttl` / `RUM_TTL`) |
| Replay | `rum_replay_segments` | 7d (`--rum-replay-ttl` / `RUM_REPLAY_TTL`) |
| Aggregates | `rum_*_1m`, histograms, edges, service name | 30d (`--rum-aggregates-ttl` / `RUM_AGGREGATES_TTL`) |

**Global defaults** live in Coroot config / flags / env (same style as `--traces-ttl`).

**Per-project overrides** are under Project settings → **RUM data retention** (`settings.rum.retention`). Empty fields mean “use global default”. Aggregates TTL must be ≥ raw TTL; minimum is 1 day.

Coroot applies TTL changes hourly (and immediately after saving project RUM settings):

1. `ALTER TABLE … MODIFY TTL … SETTINGS materialize_ttl_after_modify=0` when the configured TTL differs from the table definition.
2. `DROP PARTITION` for day partitions whose newest data is older than the effective TTL (so shortening retention frees space without waiting for merges).

The ClickHouse **space manager** also includes `rum_%` tables. When disk usage exceeds the threshold it drops oldest partitions preferring **replay → raw → aggregates**.

**Volume tips**

- Prefer `sampleRate` / server `server_sample_rate` and keep-slow/error sampling.
- Page paths are normalized (`/product/123` → `/product/:id`) before rollups to limit cardinality.
- Session replay is the heaviest table — keep its TTL short and use a low `replay_sample_rate`.
- Dashboards and RUM checks read rollups; explorer / sessions / tracing still need raw data within the raw TTL.
