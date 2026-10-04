import http from 'k6/http';
import { check, sleep } from 'k6';

// Load test RUM OTLP JSON ingest.
//   RUM_KEY=... ORIGIN=http://localhost:8081 k6 run deploy/rum-lab/k6-rum-ingest.js
export const options = {
  stages: [
    { duration: '30s', target: 50 },
    { duration: '1m', target: 200 },
    { duration: '30s', target: 0 },
  ],
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<500'],
  },
};

const BASE = __ENV.COROOT_URL || 'http://localhost:8080';
const KEY = __ENV.RUM_KEY || '';
const ORIGIN = __ENV.ORIGIN || 'http://localhost:8081';

function payload(i) {
  const traceId = ('a'.repeat(32)).slice(0, 30) + (i % 100).toString().padStart(2, '0');
  const spanId = ('b'.repeat(16)).slice(0, 14) + (i % 100).toString().padStart(2, '0');
  return JSON.stringify({
    resourceSpans: [
      {
        resource: {
          attributes: [
            { key: 'service.name', value: { stringValue: 'k6-web' } },
            { key: 'telemetry.sdk.language', value: { stringValue: 'webjs' } },
          ],
        },
        scopeSpans: [
          {
            spans: [
              {
                traceId,
                spanId,
                name: 'documentLoad',
                kind: 1,
                startTimeUnixNano: `${Date.now()}000000`,
                endTimeUnixNano: `${Date.now() + 100}000000`,
                attributes: [
                  { key: 'session.id', value: { stringValue: `sess-${i}` } },
                  { key: 'http.url', value: { stringValue: `${ORIGIN}/` } },
                ],
              },
            ],
          },
        ],
      },
    ],
  });
}

export default function () {
  const res = http.post(`${BASE}/v1/traces`, payload(__ITER), {
    headers: {
      'Content-Type': 'application/json',
      'X-API-Key': KEY,
      Origin: ORIGIN,
    },
  });
  check(res, { 'status 200': (r) => r.status === 200 });
  sleep(0.01);
}
