/**
 * Coroot RUM browser SDK
 * Lightweight OpenTelemetry-compatible exporter using OTLP/HTTP JSON.
 * Supports fetch + XHR, SPA soft navigations, service.version, consent gate.
 */
(function (global) {
  'use strict';

  var DEFAULT_SAMPLE_RATE = 0.1;
  var SESSION_KEY = 'coroot_rum_session';
  var SDK_VERSION = '0.2.0';

  function uuid() {
    if (global.crypto && crypto.randomUUID) return crypto.randomUUID().replace(/-/g, '');
    var s = '';
    for (var i = 0; i < 32; i++) s += Math.floor(Math.random() * 16).toString(16);
    return s;
  }

  function getSessionId() {
    try {
      var id = global.sessionStorage && sessionStorage.getItem(SESSION_KEY);
      if (!id) {
        id = uuid();
        sessionStorage.setItem(SESSION_KEY, id);
      }
      return id;
    } catch (e) {
      return uuid();
    }
  }

  function parseUA() {
    var ua = navigator.userAgent || '';
    var browser = 'unknown',
      os = 'unknown',
      device = /Mobi|Android/i.test(ua) ? 'mobile' : 'desktop';
    if (/Edg\//.test(ua)) browser = 'Edge';
    else if (/Chrome\//.test(ua)) browser = 'Chrome';
    else if (/Firefox\//.test(ua)) browser = 'Firefox';
    else if (/Safari\//.test(ua)) browser = 'Safari';
    if (/Windows/.test(ua)) os = 'Windows';
    else if (/Mac OS X/.test(ua)) os = 'macOS';
    else if (/Android/.test(ua)) os = 'Android';
    else if (/iPhone|iPad/.test(ua)) os = 'iOS';
    else if (/Linux/.test(ua)) os = 'Linux';
    return { browser: browser, os: os, device: device };
  }

  function pagePath() {
    try {
      return location.pathname || '/';
    } catch (e) {
      return '/';
    }
  }

  function rateVital(name, value) {
    if (name === 'lcp' || name === 'fcp') return value <= 2500 ? 'good' : value <= 4000 ? 'needs-improvement' : 'poor';
    if (name === 'inp' || name === 'fid') return value <= 200 ? 'good' : value <= 500 ? 'needs-improvement' : 'poor';
    if (name === 'cls') return value <= 0.1 ? 'good' : value <= 0.25 ? 'needs-improvement' : 'poor';
    if (name === 'ttfb') return value <= 800 ? 'good' : value <= 1800 ? 'needs-improvement' : 'poor';
    return '';
  }

  function toNano(ms) {
    return String(Math.round(ms * 1e6));
  }

  function attr(key, value) {
    return { key: key, value: { stringValue: String(value == null ? '' : value) } };
  }

  function CorootRum(opts) {
    this.opts = opts || {};
    this.endpoint = (this.opts.endpoint || '').replace(/\/$/, '');
    this.apiKey = this.opts.apiKey || '';
    this.serviceName = this.opts.serviceName || 'web-app';
    this.serviceVersion = this.opts.serviceVersion || this.opts.version || '';
    this.deploymentEnvironment = this.opts.deploymentEnvironment || '';
    this.sampleRate = this.opts.sampleRate == null ? DEFAULT_SAMPLE_RATE : Number(this.opts.sampleRate);
    this.allowedTraceUrls = this.opts.allowedTraceUrls || [];
    // Optional map of host[:port] or URL prefix → peer.service (links browser calls to a backend app).
    this.backendServices = this.opts.backendServices || {};
    this.consent = this.opts.consent !== false; // default true; set consent:false until CMP grants
    this.sessionId = getSessionId();
    this.ua = parseUA();
    this.queue = [];
    this.traceId = uuid();
    this.rootSpanId = uuid().slice(0, 16);
    this.sampled = Math.random() < this.sampleRate;
    this._timer = null;
    this._lastPath = pagePath();
  }

  CorootRum.prototype.setConsent = function (granted) {
    this.consent = !!granted;
    if (this.consent && this.sampled && !this._started) this.start();
    return this;
  };

  CorootRum.prototype.resourceAttrs = function () {
    var attrs = [
      attr('service.name', this.serviceName),
      attr('telemetry.sdk.language', 'webjs'),
      attr('telemetry.sdk.name', 'coroot-rum'),
      attr('telemetry.sdk.version', SDK_VERSION),
      attr('browser.name', this.ua.browser),
      attr('os.name', this.ua.os),
      attr('device.type', this.ua.device),
      attr('session.id', this.sessionId),
    ];
    if (this.serviceVersion) attrs.push(attr('service.version', this.serviceVersion));
    if (this.deploymentEnvironment) attrs.push(attr('deployment.environment', this.deploymentEnvironment));
    return attrs;
  };

  CorootRum.prototype.shouldPropagate = function (url) {
    var list = this.allowedTraceUrls || [];
    for (var i = 0; i < list.length; i++) {
      var p = list[i];
      if (p instanceof RegExp) {
        if (p.test(url)) return true;
      } else if (typeof p === 'string' && url.indexOf(p) === 0) return true;
    }
    return false;
  };

  CorootRum.prototype.resolvePeerService = function (url, host) {
    var map = this.backendServices || {};
    if (!map || typeof map !== 'object') return '';
    if (host && map[host]) return String(map[host]);
    try {
      var abs = String(url || '');
      if (map[abs]) return String(map[abs]);
      for (var key in map) {
        if (!Object.prototype.hasOwnProperty.call(map, key)) continue;
        if (key && abs.indexOf(key) === 0) return String(map[key]);
      }
    } catch (e) {}
    return '';
  };

  CorootRum.prototype.enqueueSpan = function (span, hint) {
    if (!this.consent || !this.sampled) return;
    span._hint = hint || '';
    this.queue.push(span);
    if (this.queue.length >= 20) this.flush();
    else if (!this._timer) {
      var self = this;
      this._timer = setTimeout(function () {
        self._timer = null;
        self.flush();
      }, 5000);
    }
  };

  CorootRum.prototype.makeSpan = function (name, kind, startMs, endMs, attributes, parentId, statusCode) {
    var attrs = (attributes || []).concat([attr('session.id', this.sessionId), attr('page.url.path', pagePath())]);
    if (this.serviceVersion) attrs.push(attr('service.version', this.serviceVersion));
    return {
      traceId: this.traceId,
      spanId: uuid().slice(0, 16),
      parentSpanId: parentId || this.rootSpanId,
      name: name,
      kind: kind || 1,
      startTimeUnixNano: toNano(startMs),
      endTimeUnixNano: toNano(endMs),
      attributes: attrs,
      status: { code: statusCode || 0 },
    };
  };

  CorootRum.prototype.recordVital = function (name, value) {
    var now = performance.now() + (performance.timing ? performance.timing.navigationStart : Date.now());
    var start = now - (name === 'cls' ? 0 : value);
    var hint = '';
    if ((name === 'lcp' || name === 'fcp') && value >= 2500) hint = 'slow';
    if (name === 'inp' && value >= 200) hint = 'slow';
    this.enqueueSpan(
      this.makeSpan(
        name,
        1,
        start,
        now,
        [attr('vital.name', name), attr('vital.value', value), attr('vital.rating', rateVital(name, value)), attr('value', value)],
        this.rootSpanId,
        0,
      ),
      hint,
    );
  };

  CorootRum.prototype.recordError = function (message, stack) {
    var now = Date.now();
    this.enqueueSpan(
      this.makeSpan(
        'window.error',
        1,
        now,
        now,
        [
          attr('exception.message', message || ''),
          attr('exception.stacktrace', stack || ''),
          attr('exception.type', 'Error'),
        ],
        this.rootSpanId,
        2,
      ),
      'error',
    );
  };

  CorootRum.prototype.flush = function () {
    if (!this.consent || !this.sampled || !this.queue.length || !this.endpoint) return;
    var hint = '';
    for (var i = 0; i < this.queue.length; i++) {
      if (this.queue[i]._hint === 'error') {
        hint = 'error';
        break;
      }
      if (this.queue[i]._hint === 'slow') hint = 'slow';
    }
    var spans = this.queue.splice(0, this.queue.length).map(function (s) {
      delete s._hint;
      return s;
    });
    var body = {
      resourceSpans: [
        {
          resource: { attributes: this.resourceAttrs() },
          scopeSpans: [
            {
              scope: { name: 'coroot-rum', version: SDK_VERSION },
              spans: spans,
            },
          ],
        },
      ],
    };
    var url = this.endpoint + '/v1/traces';
    var headers = {
      'Content-Type': 'application/json',
      'X-API-Key': this.apiKey,
      'X-Coroot-Signal': 'rum',
    };
    if (hint) headers['X-Coroot-Rum-Hint'] = hint;
    var payload = JSON.stringify(body);
    try {
      fetch(url, { method: 'POST', headers: headers, body: payload, keepalive: true, mode: 'cors' }).catch(function () {});
    } catch (e) {}
  };

  CorootRum.prototype.recordNavPhases = function (nav, navStart) {
    if (!nav) return;
    var self = this;
    var phases = [
      ['dns', nav.domainLookupStart, nav.domainLookupEnd],
      ['tcp', nav.connectStart, nav.connectEnd],
      ['tls', nav.secureConnectionStart > 0 ? nav.secureConnectionStart : 0, nav.connectEnd],
      ['request', nav.requestStart, nav.responseStart],
      ['response', nav.responseStart, nav.responseEnd],
      ['dom', nav.domInteractive, nav.domComplete],
      ['load', nav.loadEventStart, nav.loadEventEnd],
    ];
    phases.forEach(function (p) {
      var name = p[0],
        a = p[1],
        b = p[2];
      if (a == null || b == null || b <= a || a < 0) return;
      var start = navStart + a;
      var end = navStart + b;
      self.enqueueSpan(
        self.makeSpan(
          'navigation.' + name,
          1,
          start,
          end,
          [attr('navigation.phase', name), attr('vital.value', b - a)],
          self.rootSpanId,
          0,
        ),
      );
    });
  };

  CorootRum.prototype.instrumentResources = function () {
    var self = this;
    if (!performance.getEntriesByType) return;
    try {
      var entries = performance.getEntriesByType('resource') || [];
      var max = Math.min(entries.length, 40);
      var navStart = performance.timing ? performance.timing.navigationStart : Date.now() - performance.now();
      for (var i = 0; i < max; i++) {
        var e = entries[i];
        if (!e || e.duration < 50) continue;
        var name = String(e.name || '').split('?')[0];
        if (name.indexOf('/v1/traces') >= 0 || name.indexOf('coroot-rum') >= 0) continue;
        var start = navStart + e.startTime;
        var end = start + e.duration;
        self.enqueueSpan(
          self.makeSpan(
            'resource ' + (e.initiatorType || 'other'),
            3,
            start,
            end,
            [
              attr('http.url', name),
              attr('resource.initiator', e.initiatorType || ''),
              attr('resource.transfer_size', e.transferSize || 0),
              attr('resource.encoded_body_size', e.encodedBodySize || 0),
            ],
            self.rootSpanId,
            0,
          ),
        );
      }
    } catch (err) {}
  };

  CorootRum.prototype.instrumentNavigation = function () {
    var self = this;
    var navStart = performance.timing ? performance.timing.navigationStart : Date.now() - performance.now();
    var start = navStart;
    var end = navStart + (performance.timing ? performance.timing.loadEventEnd || performance.now() : performance.now());
    if (performance.getEntriesByType) {
      var nav = performance.getEntriesByType('navigation')[0];
      if (nav) {
        var s = navStart + nav.startTime;
        var e = navStart + (nav.duration || nav.loadEventEnd || 0);
        this.enqueueSpan(
          this.makeSpan(
            'documentLoad',
            1,
            s,
            e > s ? e : s + 1,
            [
              attr('navigation.type', nav.type || ''),
              attr('navigation.redirect_count', nav.redirectCount || 0),
              attr('navigation.transfer_size', nav.transferSize || 0),
            ],
            '',
            0,
          ),
        );
        this.rootSpanId = this.queue.length ? this.queue[this.queue.length - 1].spanId : this.rootSpanId;
        this.recordVital('ttfb', nav.responseStart);
        this.recordNavPhases(nav, navStart);
        var selfRef = this;
        setTimeout(function () {
          selfRef.instrumentResources();
        }, 3000);
      } else if (performance.timing && performance.timing.loadEventEnd) {
        this.enqueueSpan(this.makeSpan('documentLoad', 1, start, end, [], '', 0));
        this.rootSpanId = this.queue.length ? this.queue[this.queue.length - 1].spanId : this.rootSpanId;
        if (performance.timing.responseStart) {
          this.enqueueSpan(
            this.makeSpan('ttfb', 1, start, performance.timing.responseStart, [attr('vital.name', 'ttfb'), attr('vital.value', performance.timing.responseStart - start)], this.rootSpanId, 0),
          );
        }
      }
    } else if (performance.timing && performance.timing.loadEventEnd) {
      this.enqueueSpan(this.makeSpan('documentLoad', 1, start, end, [], '', 0));
      this.rootSpanId = this.queue.length ? this.queue[this.queue.length - 1].spanId : this.rootSpanId;
      if (performance.timing.responseStart) {
        this.enqueueSpan(
          this.makeSpan('ttfb', 1, start, performance.timing.responseStart, [attr('vital.name', 'ttfb'), attr('vital.value', performance.timing.responseStart - start)], this.rootSpanId, 0),
        );
      }
    }

    function onVital(metric) {
      var n = (metric.name || '').toLowerCase();
      self.recordVital(n, metric.value);
    }
    try {
      if (PerformanceObserver) {
        var po = new PerformanceObserver(function (list) {
          list.getEntries().forEach(function (entry) {
            if (entry.entryType === 'largest-contentful-paint') onVital({ name: 'lcp', value: entry.startTime });
            if (entry.entryType === 'layout-shift' && !entry.hadRecentInput) onVital({ name: 'cls', value: entry.value });
            if (entry.entryType === 'event' && entry.name === 'event' && entry.duration) onVital({ name: 'inp', value: entry.duration });
            if (entry.entryType === 'first-input') onVital({ name: 'fid', value: entry.processingStart - entry.startTime });
          });
        });
        try {
          po.observe({ type: 'largest-contentful-paint', buffered: true });
        } catch (e) {}
        try {
          po.observe({ type: 'layout-shift', buffered: true });
        } catch (e) {}
        try {
          po.observe({ type: 'first-input', buffered: true });
        } catch (e) {}
        try {
          po.observe({ type: 'event', buffered: true, durationThreshold: 16 });
        } catch (e) {}
      }
    } catch (e) {}
  };

  CorootRum.prototype.recordHttp = function (method, url, start, end, status, errMsg) {
    var host = '';
    try {
      host = new URL(url, location.href).host;
    } catch (e) {}
    var ok = !errMsg && status >= 200 && status < 400;
    var hint = ok ? (end - start >= 2500 ? 'slow' : '') : 'error';
    var attrs = [
      attr('http.method', method),
      attr('http.url', String(url).split('?')[0]),
      attr('http.status_code', status || 0),
      attr('server.address', host),
      attr('exception.message', errMsg || ''),
    ];
    var peer = this.resolvePeerService(url, host);
    if (peer) attrs.push(attr('peer.service', peer));
    this.enqueueSpan(
      this.makeSpan(
        method + ' ' + String(url).split('?')[0],
        3,
        start,
        end,
        attrs,
        this.rootSpanId,
        ok ? 0 : 2,
      ),
      hint,
    );
  };

  CorootRum.prototype.instrumentFetch = function () {
    if (!global.fetch) return;
    var self = this;
    var orig = global.fetch;
    global.fetch = function (input, init) {
      var url = typeof input === 'string' ? input : (input && input.url) || '';
      var method = (init && init.method) || (input && input.method) || 'GET';
      var start = Date.now();
      var spanId = uuid().slice(0, 16);
      init = init ? Object.assign({}, init) : {};
      init.headers = new Headers(init.headers || (input && input.headers) || {});
      if (self.sampled && self.consent && self.shouldPropagate(url)) {
        var flags = self.sampled ? '01' : '00';
        init.headers.set('traceparent', '00-' + self.traceId + '-' + spanId + '-' + flags);
      }
      return orig.call(this, input, init).then(
        function (res) {
          self.recordHttp(method, url, start, Date.now(), res.status, '');
          return res;
        },
        function (err) {
          self.recordHttp(method, url, start, Date.now(), 0, String(err));
          throw err;
        },
      );
    };
  };

  CorootRum.prototype.instrumentXHR = function () {
    if (!global.XMLHttpRequest) return;
    var self = this;
    var open = XMLHttpRequest.prototype.open;
    var send = XMLHttpRequest.prototype.send;
    XMLHttpRequest.prototype.open = function (method, url) {
      this._coroot = { method: method || 'GET', url: url || '', start: 0 };
      return open.apply(this, arguments);
    };
    XMLHttpRequest.prototype.send = function () {
      var xhr = this;
      if (xhr._coroot) xhr._coroot.start = Date.now();
      xhr.addEventListener('loadend', function () {
        if (!xhr._coroot) return;
        self.recordHttp(xhr._coroot.method, xhr._coroot.url, xhr._coroot.start, Date.now(), xhr.status, xhr.status === 0 ? 'network_error' : '');
      });
      return send.apply(this, arguments);
    };
  };

  CorootRum.prototype.instrumentSPA = function () {
    var self = this;
    function softNav(reason) {
      var path = pagePath();
      if (path === self._lastPath) return;
      var now = Date.now();
      self._lastPath = path;
      self.traceId = uuid();
      self.rootSpanId = uuid().slice(0, 16);
      self.enqueueSpan(self.makeSpan('softNavigation', 1, now, now, [attr('navigation.type', reason || 'spa')], '', 0));
    }
    var push = history.pushState;
    var replace = history.replaceState;
    if (push) {
      history.pushState = function () {
        var r = push.apply(this, arguments);
        softNav('pushState');
        return r;
      };
    }
    if (replace) {
      history.replaceState = function () {
        var r = replace.apply(this, arguments);
        softNav('replaceState');
        return r;
      };
    }
    global.addEventListener('popstate', function () {
      softNav('popstate');
    });
  };

  CorootRum.prototype.instrumentErrors = function () {
    var self = this;
    global.addEventListener('error', function (ev) {
      self.recordError(ev.message, ev.error && ev.error.stack);
    });
    global.addEventListener('unhandledrejection', function (ev) {
      var r = ev.reason;
      self.recordError(r && r.message ? r.message : String(r), r && r.stack);
    });
  };

  CorootRum.prototype.start = function () {
    if (!this.endpoint || !this.apiKey) {
      console.warn('[coroot-rum] endpoint and apiKey are required');
      return this;
    }
    if (!this.consent) {
      console.info('[coroot-rum] waiting for consent');
      return this;
    }
    if (!this.sampled) return this;
    if (this._started) return this;
    this._started = true;
    this.instrumentNavigation();
    this.instrumentFetch();
    this.instrumentXHR();
    this.instrumentSPA();
    this.instrumentErrors();
    var self = this;
    global.addEventListener('visibilitychange', function () {
      if (document.visibilityState === 'hidden') self.flush();
    });
    global.addEventListener('pagehide', function () {
      self.flush();
    });
    return this;
  };

  function init(opts) {
    var rum = new CorootRum(opts);
    rum.start();
    global.__corootRum = rum;
    return rum;
  }

  global.CorootRum = { init: init, CorootRum: CorootRum, version: SDK_VERSION };

  // ESM interop for bundlers that wrap UMD
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = global.CorootRum;
  }
})(typeof window !== 'undefined' ? window : this);
