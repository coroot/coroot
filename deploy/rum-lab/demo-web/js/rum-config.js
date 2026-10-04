/*! Shared RUM demo config fragment — kept in sync across pages via app.js defaults. */
window.__RUM_DEMO__ = Object.assign(
  {
    endpoint: 'http://localhost:8080',
    apiKey: 'rum-local-key-000000000000000001',
    serviceName: 'demo-web',
    serviceVersion: '1.0.0',
    sampleRate: 1,
    apiBase: 'http://localhost:4000',
    checkoutBase: 'http://localhost:4001',
    allowedTraceUrls: [
      /^http:\/\/localhost:4000/,
      /^http:\/\/127\.0\.0\.1:4000/,
      /^http:\/\/localhost:4001/,
      /^http:\/\/127\.0\.0\.1:4001/,
    ],
    backendServices: {
      'localhost:4000': 'demo-api',
      '127.0.0.1:4000': 'demo-api',
      'http://localhost:4000': 'demo-api',
      'http://127.0.0.1:4000': 'demo-api',
      'localhost:4001': 'demo-checkout',
      '127.0.0.1:4001': 'demo-checkout',
      'http://localhost:4001': 'demo-checkout',
      'http://127.0.0.1:4001': 'demo-checkout',
    },
    consent: true,
  },
  window.__RUM_DEMO__ || {}
);
