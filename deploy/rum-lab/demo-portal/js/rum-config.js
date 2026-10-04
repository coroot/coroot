window.__RUM_DEMO__ = Object.assign(
  {
    endpoint: 'http://localhost:8080',
    apiKey: 'rum-local-key-000000000000000001',
    serviceName: 'demo-portal',
    serviceVersion: '1.0.0',
    sampleRate: 1,
    apiBase: 'http://localhost:4002',
    allowedTraceUrls: [
      /^http:\/\/localhost:4002/,
      /^http:\/\/127\.0\.0\.1:4002/,
    ],
    backendServices: {
      'localhost:4002': 'demo-portal-api',
      '127.0.0.1:4002': 'demo-portal-api',
      'http://localhost:4002': 'demo-portal-api',
      'http://127.0.0.1:4002': 'demo-portal-api',
    },
    consent: true,
  },
  window.__RUM_DEMO__ || {}
);
