(function () {
  'use strict';

  var cfg = window.__RUM_DEMO__ || {};
  var API_BASE = (cfg.apiBase || 'http://localhost:4002').replace(/\/$/, '');
  var logEl = document.getElementById('log');
  var statusEl = document.getElementById('rum-status');

  function log(msg) {
    var line = new Date().toISOString().slice(11, 19) + '  ' + msg;
    if (logEl) {
      logEl.textContent = (logEl.textContent ? logEl.textContent + '\n' : '') + line;
    }
    console.log('[portal]', msg);
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
      log('CorootRum.init ok → ' + cfg.endpoint);
      log('API base → ' + API_BASE);
    } catch (e) {
      setStatus(false, 'RUM: init failed');
      log('init error: ' + e);
    }
  }

  function renderTickets(items) {
    var box = document.getElementById('tickets');
    if (!box) return;
    if (!items || !items.length) {
      box.textContent = 'No tickets';
      return;
    }
    box.innerHTML = items
      .map(function (t) {
        return (
          '<article class="card">' +
          '<h3>#' +
          t.id +
          ' · ' +
          t.subject +
          '</h3>' +
          '<div class="meta">' +
          t.email +
          ' · <span class="tag">' +
          t.status +
          '</span></div>' +
          '</article>'
        );
      })
      .join('');
  }

  function loadTickets() {
    var url = API_BASE + '/api/tickets';
    log('fetch ' + url);
    return fetch(url, { headers: { Accept: 'application/json' } })
      .then(function (r) {
        if (!r.ok) throw new Error('HTTP ' + r.status);
        return r.json();
      })
      .then(function (data) {
        renderTickets(data.items || []);
        log('tickets loaded: ' + ((data.items && data.items.length) || 0));
      })
      .catch(function (e) {
        log('tickets error: ' + e);
      });
  }

  function loadStatus() {
    var box = document.getElementById('status-panel');
    if (!box) return;
    var url = API_BASE + '/api/status';
    log('fetch ' + url);
    return fetch(url, { headers: { Accept: 'application/json' } })
      .then(function (r) {
        if (!r.ok) throw new Error('HTTP ' + r.status);
        return r.json();
      })
      .then(function (data) {
        box.innerHTML =
          '<div class="stat"><strong>' +
          data.open +
          '</strong><span>open</span></div>' +
          '<div class="stat"><strong>' +
          data.pending +
          '</strong><span>pending</span></div>' +
          '<div class="stat"><strong>' +
          data.resolved +
          '</strong><span>resolved</span></div>';
        log('status ok hits=' + data.hits);
      })
      .catch(function (e) {
        box.textContent = 'Failed to load status';
        log('status error: ' + e);
      });
  }

  function createTicket() {
    var subjectEl = document.getElementById('ticket-subject');
    var emailEl = document.getElementById('ticket-email');
    var subject = (subjectEl && subjectEl.value) || 'Help request';
    var email = (emailEl && emailEl.value) || 'user@example.com';
    var url = API_BASE + '/api/tickets';
    log('POST ' + url);
    return fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      body: JSON.stringify({ subject: subject, email: email }),
    })
      .then(function (r) {
        if (!r.ok) throw new Error('HTTP ' + r.status);
        return r.json();
      })
      .then(function (data) {
        log('created ticket #' + data.id);
        return loadTickets();
      })
      .catch(function (e) {
        log('create error: ' + e);
      });
  }

  function throwError() {
    log('throwing intentional error');
    setTimeout(function () {
      throw new Error('intentional portal demo error');
    }, 0);
  }

  document.addEventListener('DOMContentLoaded', function () {
    initRum();
    var btnLoad = document.getElementById('btn-load');
    if (btnLoad) btnLoad.addEventListener('click', loadTickets);
    var btnCreate = document.getElementById('btn-create');
    if (btnCreate) btnCreate.addEventListener('click', createTicket);
    var btnErr = document.getElementById('btn-error');
    if (btnErr) btnErr.addEventListener('click', throwError);
    var btnStatus = document.getElementById('btn-status');
    if (btnStatus) btnStatus.addEventListener('click', loadStatus);

    if (document.getElementById('tickets')) loadTickets();
    if (document.getElementById('status-panel')) loadStatus();

    document.querySelectorAll('a[data-nav]').forEach(function (a) {
      if (a.getAttribute('href') === location.pathname.split('/').pop() || (location.pathname.endsWith('/') && a.getAttribute('href') === '/')) {
        a.classList.add('active');
      }
    });
  });
})();
