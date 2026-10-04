# RUM lab testing checklist

## Automated (local)

```bash
# Unit + race
go test -race ./...

# ClickHouse integration (compose clickhouse exposed on 9000)
CLICKHOUSE_ADDR=localhost:9000 go test -tags=integration ./clickhouse/ -run RumIntegration -count=1

# API e2e against running stack
./deploy/rum-lab/e2e.sh

# Ingest load
RUM_KEY=... ORIGIN=http://localhost:8081 k6 run deploy/rum-lab/k6-rum-ingest.js
```

## Browser SDK (manual / Playwright)

On `demo-web` / `demo-portal`:

- [ ] `/v1/traces` requests with `telemetry.sdk.language=webjs`
- [ ] `session.id`, `page.path`, LCP/INP/CLS/TTFB/FCP
- [ ] JS errors and failing fetch
- [ ] `traceparent` on XHR correlates with backend
- [ ] SPA route change
- [ ] Replay chunks when enabled
- [ ] Tab close delivers via sendBeacon
- [ ] Strict CSP `connect-src` still works

## UI checklist

- [ ] Overview → RUM list, KPIs, map
- [ ] AppRum / RumExplorer / RumSessionPanel (incl. replay RBAC)
- [ ] ProjectRumSettings save/validate/readonly
- [ ] ProjectApiKeys RUM key generate + origins
- [ ] RumClient redirects to `overview/rum`; compact check chips
- [ ] Service Map shows RumClient + `source` rum/both
- [ ] Incident shows `rum_signals` when backend is linked
- [ ] Empty state without ClickHouse

## Security

- [ ] RUM key rejected on `/v1/logs`, `/v1/profiles`, `/v1/metrics`, `/v1/config`
- [ ] CORS: foreign Origin → 403; `null` origin rejected
- [ ] `GeoEnabled=false` stores no IP
