/**
 * Coroot RUM Session Replay (Enterprise add-on)
 * Minimal masked DOM recorder. Requires consent:true and project.rum.replay_enabled.
 * Load separately: <script src=".../coroot-rum-replay.js"></script>
 */
(function (global) {
  'use strict';

  var MASK_SEL = 'input, textarea, [data-coroot-mask], [type=password], [type=email]';

  function CorootRumReplay(opts) {
    this.opts = opts || {};
    this.rum = this.opts.rum || global.__corootRum;
    this.sampleRate = this.opts.sampleRate == null ? 0.01 : Number(this.opts.sampleRate);
    this.consent = !!(this.opts.consent || (this.rum && this.rum.consent));
    this.enabled = this.consent && Math.random() < this.sampleRate;
    this.seq = 0;
    this.buffer = [];
    this._obs = null;
  }

  CorootRumReplay.prototype.maskNode = function (node) {
    if (!node || node.nodeType !== 1) return;
    try {
      if (node.matches && node.matches(MASK_SEL)) {
        if (node.value != null) node.setAttribute('data-coroot-masked', '1');
      }
      if (node.querySelectorAll) {
        node.querySelectorAll(MASK_SEL).forEach(function (el) {
          el.setAttribute('data-coroot-masked', '1');
        });
      }
    } catch (e) {}
  };

  CorootRumReplay.prototype.capture = function (type, data) {
    if (!this.enabled || !this.consent) return;
    this.buffer.push({ t: Date.now(), type: type, data: data });
    if (this.buffer.length >= 20) this.flush();
  };

  CorootRumReplay.prototype.flush = function () {
    if (!this.enabled || !this.consent || !this.buffer.length || !this.rum) return;
    var endpoint = this.rum.endpoint;
    var payload = JSON.stringify({
      session_id: this.rum.sessionId,
      trace_id: this.rum.traceId,
      service_name: this.rum.serviceName,
      seq: this.seq++,
      consent: true,
      payload: btoa(unescape(encodeURIComponent(JSON.stringify(this.buffer.splice(0, this.buffer.length))))),
    });
    try {
      fetch(endpoint + '/v1/rum/replay', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-API-Key': this.rum.apiKey,
        },
        body: payload,
        keepalive: true,
        mode: 'cors',
      }).catch(function () {});
    } catch (e) {}
  };

  CorootRumReplay.prototype.start = function () {
    if (!this.enabled || !this.rum) return this;
    var self = this;
    this.capture('viewport', { w: innerWidth, h: innerHeight, path: location.pathname });
    document.addEventListener(
      'click',
      function (ev) {
        self.capture('click', { x: ev.clientX, y: ev.clientY, tag: ev.target && ev.target.tagName });
      },
      true,
    );
    document.addEventListener(
      'scroll',
      function () {
        self.capture('scroll', { x: scrollX, y: scrollY });
      },
      { passive: true },
    );
    if (global.MutationObserver) {
      this._obs = new MutationObserver(function (mutations) {
        mutations.forEach(function (m) {
          if (m.type === 'childList') {
            m.addedNodes.forEach(function (n) {
              self.maskNode(n);
            });
          }
          self.capture('mutation', { type: m.type, target: m.target && m.target.nodeName });
        });
      });
      this._obs.observe(document.documentElement, { childList: true, subtree: true, attributes: true, characterData: false });
    }
    global.addEventListener('visibilitychange', function () {
      if (document.visibilityState === 'hidden') self.flush();
    });
    return this;
  };

  CorootRumReplay.prototype.setConsent = function (granted) {
    this.consent = !!granted;
    if (this.consent && !this.enabled && Math.random() < this.sampleRate) {
      this.enabled = true;
      this.start();
    }
    return this;
  };

  function init(opts) {
    var replay = new CorootRumReplay(opts);
    replay.start();
    global.__corootRumReplay = replay;
    return replay;
  }

  global.CorootRumReplay = { init: init, CorootRumReplay: CorootRumReplay };
})(typeof window !== 'undefined' ? window : this);
