# Coroot RUM lab (local Docker)

Test stand for verifying RUM with your local Coroot build — **two frontends**, separate backends, shared Postgres/Valkey.

## Start

```bash
# 1) Build UI on the host (required — npm inside Docker often hangs on macOS)
cd front && npm ci && npm run build-prod && cd ..
mkdir -p static/rum && cp -f static/static/rum/*.js static/rum/

# 2) Start the lab
cd deploy/rum-lab
docker compose up -d --build
```

First Go image build downloads modules and can take a few minutes; subsequent rebuilds are faster.

Optional eBPF agent (Linux hosts only):

```bash
docker compose --profile ebpf up -d
```

## URLs

| Service | URL |
|---------|-----|
| Coroot UI | http://localhost:8080 |
| Demo shop (Nord Outfitters) | http://localhost:3000 |
| Demo portal (Harbor Help) | http://localhost:3001 |
| Demo API (catalog, 2× behind LB) | http://localhost:4000 |
| Demo checkout | http://localhost:4001 |
| Demo portal API | http://localhost:4002 |
| Demo Postgres | localhost:5432 (shop/shop) |
| Demo Valkey | localhost:6379 |
| Prometheus | http://127.0.0.1:9090 |
| ClickHouse HTTP | http://127.0.0.1:8123 |

Auth is disabled (`anonymous_role: Admin`). Project **rum-lab** is created automatically.

## End-to-end chain

```
demo-web :3000 (serviceName=demo-web)
  ├─ RUM → Coroot /v1/traces
  ├─ demo-api :4000 /api/products → Postgres + Valkey
  └─ demo-checkout :4001 /api/checkout → Postgres + Valkey

demo-portal :3001 (serviceName=demo-portal)
  ├─ RUM → Coroot /v1/traces
  └─ demo-portal-api :4002 /api/tickets|/api/status → same Postgres + Valkey

demo-traffic (~every 5s, ~60% shop / ~40% portal)
  └─ synthetic RUM + real HTTP to the matching backends
```

`demo-traffic` generates browse / SPA / checkout or tickets / payment & ticket failures / JS errors / **slow API (2–8s)** / **backend 5xx**, with realistic Core Web Vitals distributions (`vital.value` attrs matching the SDK).

Apps after ~1–2 minutes (Docker Desktop uses metric shims instead of eBPF node-agent):

| App | Notes |
|-----|-------|
| demo-web / demo-portal | RumClient apps — open **RUM** report (RED: rate / errors / duration) |
| demo-api | Catalog, 2 instances behind LB |
| demo-checkout | Orders / payments |
| demo-portal-api | Support tickets / status |
| demo-postgres / demo-valkey | Shared data plane |

Postgres credentials for cluster-agent: user `coroot` / password `coroot`. Valkey: no AUTH. App DB user: `shop`/`shop`.

Open Coroot → **RUM** to compare both services side-by-side (page views, errors, p75 LCP). Service Map should show:

- `demo-web` → `demo-api` / `demo-checkout`
- `demo-portal` → `demo-portal-api`

Stop the generator:

```bash
docker compose stop demo-traffic
```

Tune rate via env in `docker-compose.yaml`: `INTERVAL_SEC`, `ERROR_RATE`, `SLOW_RATE`, `SHOP_WEIGHT`, `PORTAL_WEIGHT`.

Manual API knobs (also used by traffic):

- `?delay_ms=3000` or header `X-Demo-Delay: 3000` — slow response
- `?force_error=500` or header `X-Demo-Error: 503` — forced 5xx
- checkout / tickets `fail: true` / `X-Demo-Fail: 1` — forced failure

## RUM key (preconfigured)

```
rum-local-key-000000000000000001
```

Allowed domains: `localhost:3000`, `localhost:3001` (+ 127.0.0.1 / demo-web / demo-portal). Path prefixes (e.g. `example.com/shop`) can scope a key to one app under a shared host.

Agent key: `agent-local-key-0000000000000001`.

## Verify

1. Open http://localhost:3000 and http://localhost:3001 — badges should show `RUM: on`.
2. Click around both sites (products / tickets / status).
3. In Coroot → **RUM** — both `demo-web` and `demo-portal` with RED summary columns.
4. Open each app’s **RUM** tab: page views/s, errors/s, CWV p75 (ms), CLS p75 (separate chart).

## Stop / reset

```bash
docker compose down
# wipe volumes (re-runs Postgres init including tickets table):
docker compose down -v
```
