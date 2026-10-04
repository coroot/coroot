(function () {
  'use strict';

  var cfg = window.__RUM_DEMO__ || {};
  var API_BASE = (cfg.apiBase || 'http://localhost:4000').replace(/\/$/, '');
  var CHECKOUT_BASE = (cfg.checkoutBase || API_BASE).replace(/\/$/, '');
  var logEl = document.getElementById('log');
  var statusEl = document.getElementById('rum-status');

  function log(msg) {
    var line = new Date().toISOString().slice(11, 19) + '  ' + msg;
    if (logEl) {
      logEl.textContent = (logEl.textContent ? logEl.textContent + '\n' : '') + line;
    }
    console.log('[demo]', msg);
  }

  function setStatus(ok, text) {
    if (!statusEl) return;
    statusEl.textContent = text;
    statusEl.classList.toggle('ok', !!ok);
    statusEl.classList.toggle('err', !ok);
  }

  function initRum() {
    if (typeof CorootRum === 'undefined') {
      setStatus(false, 'RUM: SDK missing');
      log('coroot-rum.js failed to load from ' + cfg.endpoint);
      return;
    }
    try {
      window.__corootRum = CorootRum.init(cfg);
      setStatus(true, 'RUM: on · ' + cfg.serviceName);
      log('CorootRum.init ok → ' + cfg.endpoint + ' (sampleRate=' + cfg.sampleRate + ')');
      log('API base → ' + API_BASE + ' (traceparent propagation enabled)');
    } catch (e) {
      setStatus(false, 'RUM: init failed');
      log('init error: ' + e);
    }
  }

  function renderProducts(items) {
    var box = document.getElementById('products');
    if (!box) return;
    if (!items || !items.length) {
      box.textContent = 'No products';
      return;
    }
    box.innerHTML = items
      .map(function (p) {
        return (
          '<article class="card">' +
          '<h3>' +
          p.name +
          '</h3>' +
          '<div class="price">$' +
          p.price +
          '</div>' +
          '<div class="meta">In stock: ' +
          p.stock +
          '</div>' +
          '</article>'
        );
      })
      .join('');
  }

  function loadProductsFetch() {
    var url = API_BASE + '/api/products';
    log('fetch ' + url);
    return fetch(url, { headers: { Accept: 'application/json' } })
      .then(function (r) {
        if (!r.ok) throw new Error('HTTP ' + r.status);
        var tr = r.headers.get('traceresponse');
        if (tr) log('traceresponse: ' + tr);
        return r.json();
      })
      .then(function (data) {
        renderProducts(data.items || []);
        log('products loaded: ' + ((data.items && data.items.length) || 0) + ' via ' + (data.source || 'api'));
      })
      .catch(function (e) {
        log('fetch failed: ' + e);
      });
  }

  function loadProductsXHR() {
    // Note: current SDK propagates traceparent only on fetch(), not XHR.
    var url = API_BASE + '/api/products';
    log('XHR ' + url + ' (no traceparent — use fetch for full chain)');
    return new Promise(function (resolve) {
      var xhr = new XMLHttpRequest();
      xhr.open('GET', url);
      xhr.onload = function () {
        try {
          var data = JSON.parse(xhr.responseText);
          renderProducts(data.items || []);
          log('XHR ok');
        } catch (e) {
          log('XHR parse error: ' + e);
        }
        resolve();
      };
      xhr.onerror = function () {
        log('XHR network error');
        resolve();
      };
      xhr.send();
    });
  }

  function wireNav() {
    document.querySelectorAll('[data-nav]').forEach(function (a) {
      a.addEventListener('click', function () {
        log('navigate → ' + a.getAttribute('href'));
      });
    });
  }

  function wireHomeActions() {
    var loadBtn = document.getElementById('btn-load');
    if (loadBtn) {
      loadBtn.addEventListener('click', function () {
        if (location.pathname.indexOf('shop') >= 0) loadProductsXHR();
        else loadProductsFetch();
      });
    }

    var errBtn = document.getElementById('btn-error');
    if (errBtn) {
      errBtn.addEventListener('click', function () {
        log('throwing intentional error');
        setTimeout(function () {
          throw new Error('intentional demo error from Nord Outfitters');
        }, 0);
      });
    }

    var slowBtn = document.getElementById('btn-slow');
    if (slowBtn) {
      slowBtn.addEventListener('click', function () {
        log('blocking main thread ~400ms');
        var end = performance.now() + 400;
        while (performance.now() < end) {}
      });
    }

    var clsBtn = document.getElementById('btn-cls');
    if (clsBtn) {
      clsBtn.addEventListener('click', function () {
        var el = document.getElementById('cls-spacer');
        if (!el) return;
        log('injecting late content (CLS)');
        el.classList.add('boom');
        el.textContent = 'Late-injected banner — layout shift for CLS demo';
      });
    }
  }

  function wireCheckout() {
    var form = document.getElementById('checkout-form');
    if (!form) return;

    form.addEventListener('submit', function (ev) {
      ev.preventDefault();
      var url = CHECKOUT_BASE + '/api/checkout';
      log('POST ' + url);
      var email = (form.email && form.email.value) || 'demo@example.com';
      fetch(url, {
        method: 'POST',
        headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: email, total: 218 }),
      })
        .then(function (r) {
          var tr = r.headers.get('traceresponse');
          if (tr) log('traceresponse: ' + tr);
          return r.json().then(function (data) {
            return { ok: r.ok, status: r.status, data: data };
          });
        })
        .then(function (res) {
          var box = document.getElementById('order-result');
          box.hidden = false;
          if (!res.ok) {
            box.textContent = 'Payment failed (' + res.status + '): ' + (res.data.error || 'error');
            log('order failed: ' + JSON.stringify(res.data));
            return;
          }
          box.textContent = 'Order ' + res.data.orderId + ' placed · $' + res.data.total;
          log('order ok: ' + res.data.orderId);
        })
        .catch(function (e) {
          log('checkout failed: ' + e);
        });
    });

    var failBtn = document.getElementById('btn-fail');
    if (failBtn) {
      failBtn.addEventListener('click', function () {
        var url = CHECKOUT_BASE + '/api/checkout';
        log('POST ' + url + ' (force payment decline)');
        fetch(url, {
          method: 'POST',
          headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
          body: JSON.stringify({ email: 'demo@example.com', total: 218, fail: true }),
        })
          .then(function (r) {
            return r.json().then(function (data) {
              log('decline response ' + r.status + ': ' + JSON.stringify(data));
            });
          })
          .catch(function (e) {
            log('decline request failed: ' + e);
          });
      });
    }
  }

  initRum();
  wireNav();
  wireHomeActions();
  wireCheckout();

  if (document.getElementById('products')) {
    loadProductsFetch();
  }
})();
