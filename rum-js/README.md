# Coroot RUM SDK

Browser Real User Monitoring via OTLP/HTTP JSON.

## Install

```html
<script src="/static/rum/coroot-rum.js"></script>
<script>
  CorootRum.init({
    endpoint: 'https://coroot.example.com',
    apiKey: 'YOUR_RUM_API_KEY',
    serviceName: 'web-shop',
    serviceVersion: '1.2.3',
    sampleRate: 0.1,
    allowedTraceUrls: ['https://api.example.com'],
    consent: true, // set false until CMP grants; then rum.setConsent(true)
  });
</script>
```

CDN (versioned): `https://cdn.coroot.com/rum/coroot-rum@0.2.js` (also available under `/static/rum/` on your Coroot instance).

npm: `npm install coroot-rum`

## Features

- Core Web Vitals (LCP, INP, CLS, TTFB)
- `fetch` + `XMLHttpRequest` instrumentation
- SPA soft navigations (`pushState` / `popstate`)
- `service.version` / `deployment.environment` for release comparison
- Trace context propagation (`traceparent`) to allowlisted backends
- Server-side keep-slow / keep-error sampling (`X-Coroot-Rum-Hint`)
- Consent gate (no data without consent)

## Session Replay (Enterprise)

Load separately after base SDK + consent:

```html
<script src="/static/rum/coroot-rum-replay.js"></script>
<script>
  CorootRumReplay.init({ rum: window.__corootRum, consent: true, sampleRate: 0.01 });
</script>
```

Requires project setting `rum.replay_enabled` and RBAC `project.rum.replay` view permission.

Inputs are masked by default (`password`, `email`, `[data-coroot-mask]`).

## Privacy

- No user ids; random `session.id` only
- Opt-in geo: project `rum.geo_enabled` enriches country/ASN from reverse-proxy IP; raw IP is hashed, not stored
- CSP: allow `connect-src` to your Coroot ingest host
