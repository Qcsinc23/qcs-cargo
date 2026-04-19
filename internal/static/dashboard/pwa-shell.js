(function () {
  'use strict';

  var THEME_KEY = 'qcs_theme';
  var LOCALE_KEY = 'qcs_locale';
  var LAST_SHORTCUT_KEY = 'qcs_shortcut_hint_seen';
  var DEFAULT_LOCALE = 'en';
  var STREAM_DEFAULT = '/api/v1/notifications/stream';
  var PUSH_SUBSCRIBE_DEFAULT = '/api/v1/notifications/push/subscribe';
  var ROUTES = {
    dashboard: '/dashboard',
    inbox: '/dashboard/inbox',
    shipments: '/dashboard/shipments',
    notifications: '/dashboard/settings/notifications'
  };

  // Single source of truth for the customer dashboard sidebar. Every page
  // renders this via QCSPWA.renderSidebar() so a change here propagates
  // everywhere — no more 13-page sweep when a tab is added or relabeled.
  var SIDEBAR_LINKS = [
    { key: 'dashboard',    href: '/dashboard',                 label: 'Dashboard',         icon: '🏠' },
    { key: 'inbox',        href: '/dashboard/inbox',           label: 'My Packages',       icon: '📦' },
    { key: 'mailbox',      href: '/dashboard/mailbox',         label: 'My Mailbox',        icon: '📬' },
    { key: 'inbound',      href: '/dashboard/inbound',         label: 'Expected Packages', icon: '🛬' },
    { key: 'ship',         href: '/dashboard/ship',            label: 'Ship a Package',    icon: '✈️',  cta: true },
    { key: 'ship-requests',href: '/dashboard/ship-requests',   label: 'Ship Requests',     icon: '📤' },
    { key: 'shipments',    href: '/dashboard/shipments',       label: 'Shipments',         icon: '🚚' },
    { key: 'bookings',     href: '/dashboard/bookings',        label: 'Bookings',          icon: '📅' },
    { key: 'recipients',   href: '/dashboard/recipients',      label: 'Recipients',        icon: '👥' },
    { key: 'templates',    href: '/dashboard/templates',       label: 'Templates',         icon: '📑' },
    { key: 'parcel-plus',  href: '/dashboard/parcel-plus',     label: 'Parcel+',           icon: '✨' },
    { key: 'invoices',     href: '/dashboard/invoices',        label: 'Invoices',          icon: '💳' },
    { key: 'profile',      href: '/dashboard/profile',         label: 'Profile',           icon: '👤' },
    { key: 'settings',     href: '/dashboard/settings',        label: 'Settings',          icon: '⚙️' }
  ];

  var dict = {
    en: {
      loading: 'Loading...',
      loading_dashboard: 'Loading dashboard...',
      loading_packages: 'Loading packages...',
      loading_shipments: 'Loading shipments...',
      loading_notifications: 'Loading notification settings...',
      empty_title: 'Nothing here yet',
      empty_desc: 'There is no data to show yet.',
      empty_action: 'Go to dashboard',
      realtime_connecting: 'Realtime updates connecting...',
      realtime_connected: 'Realtime updates connected',
      realtime_retry: 'Realtime updates reconnecting...',
      realtime_offline: 'Realtime updates paused (offline)',
      theme_light: 'Light',
      theme_dark: 'Dark',
      shortcut_help: 'Keyboard shortcuts',
      push_unsupported: 'Push notifications are not supported in this browser.',
      push_denied: 'Push permission was denied.',
      push_ready: 'Push notifications enabled on this device.',
      push_local_only: 'Push subscribed locally; server endpoint not available.',
      error_title: 'Something went wrong',
      error_desc: 'We could not load this page. Please try again.',
      error_action: 'Retry',
      session_expired: 'Your session has expired. Please sign in again.',
      sign_out: 'Sign out',
      app_brand: 'QCS Cargo',
      net_error: 'Network error. Please check your connection.',
      copy_done: 'Copied to clipboard'
    },
    es: {
      // Phase 3.4 (CRF-002): Spanish accents restored. Pass 1 review noted
      // Caribbean destinations (Guyana / Trinidad / Jamaica / Suriname /
      // Barbados) have substantial Spanish-speaking customer bases and
      // accent-stripped copy reads as broken to native speakers.
      loading: 'Cargando…',
      loading_dashboard: 'Cargando panel…',
      loading_packages: 'Cargando paquetes…',
      loading_shipments: 'Cargando envíos…',
      loading_notifications: 'Cargando configuración de notificaciones…',
      empty_title: 'Aún no hay datos',
      empty_desc: 'Todavía no hay información para mostrar.',
      empty_action: 'Ir al panel',
      realtime_connecting: 'Conectando actualizaciones en tiempo real…',
      realtime_connected: 'Actualizaciones en tiempo real conectadas',
      realtime_retry: 'Reconectando actualizaciones en tiempo real…',
      realtime_offline: 'Actualizaciones en tiempo real en pausa (sin conexión)',
      theme_light: 'Claro',
      theme_dark: 'Oscuro',
      shortcut_help: 'Atajos del teclado',
      push_unsupported: 'Las notificaciones push no son compatibles con este navegador.',
      push_denied: 'El permiso de push fue denegado.',
      push_ready: 'Notificaciones push habilitadas en este dispositivo.',
      push_local_only: 'Push suscrito localmente; endpoint del servidor no disponible.',
      error_title: 'Algo salió mal',
      error_desc: 'No pudimos cargar esta página. Inténtalo de nuevo.',
      error_action: 'Reintentar',
      session_expired: 'Tu sesión expiró. Vuelve a iniciar sesión.',
      sign_out: 'Cerrar sesión',
      app_brand: 'QCS Cargo',
      net_error: 'Error de red. Verifica tu conexión.',
      copy_done: 'Copiado al portapapeles'
    }
  };

  var keyboardState = {
    gPressedAt: 0,
    initialized: false
  };

  function nowMs() {
    return Date.now ? Date.now() : new Date().getTime();
  }

  function getLocale() {
    var stored = '';
    try {
      stored = localStorage.getItem(LOCALE_KEY) || '';
    } catch (err) {
      stored = '';
    }
    if (stored && dict[stored]) return stored;
    var browser = (navigator.language || DEFAULT_LOCALE).slice(0, 2).toLowerCase();
    return dict[browser] ? browser : DEFAULT_LOCALE;
  }

  function setLocale(locale) {
    if (!dict[locale]) return;
    try {
      localStorage.setItem(LOCALE_KEY, locale);
    } catch (err) {
      // Ignore storage errors.
    }
    document.documentElement.setAttribute('lang', locale);
    document.documentElement.setAttribute('data-locale', locale);
    updateUtilityLabels();
  }

  function t(key) {
    var locale = getLocale();
    var langPack = dict[locale] || dict[DEFAULT_LOCALE];
    return langPack[key] || dict[DEFAULT_LOCALE][key] || key;
  }

  function getTheme() {
    try {
      var stored = localStorage.getItem(THEME_KEY);
      if (stored === 'dark' || stored === 'light') return stored;
    } catch (err) {
      // Ignore storage errors.
    }
    if (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) {
      return 'dark';
    }
    return 'light';
  }

  function applyTheme(theme) {
    var root = document.documentElement;
    var body = document.body;
    root.setAttribute('data-theme', theme);
    if (body) {
      body.classList.toggle('qcs-theme-dark', theme === 'dark');
      body.classList.toggle('qcs-theme-light', theme !== 'dark');
    }
    updateUtilityLabels();
  }

  function toggleTheme() {
    var next = getTheme() === 'dark' ? 'light' : 'dark';
    try {
      localStorage.setItem(THEME_KEY, next);
    } catch (err) {
      // Ignore storage errors.
    }
    applyTheme(next);
    return next;
  }

  function ensureEscape() {
    if (typeof window.qcsEscapeHTML === 'function') return;
    window.qcsEscapeHTML = function (str) {
      var safe = String(str == null ? '' : str);
      return safe
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#039;');
    };
  }

  function renderLoadingHTML(message) {
    return '' +
      '<section class="qcs-loading" role="status" aria-live="polite">' +
      '<div class="qcs-loading-spinner" aria-hidden="true"></div>' +
      '<p class="qcs-loading-text">' + window.qcsEscapeHTML(message || t('loading')) + '</p>' +
      '</section>';
  }

  function mountLoading(target, message) {
    if (!target) return;
    // CRF-006 (backlog) fix: defer the spinner by ~150ms. On fast
    // networks the request often resolves first, and showing the
    // spinner only to immediately replace it produces a perceived
    // flicker. If the caller mounts new content within that window,
    // the deferred render is skipped.
    var token = (target.__qcsLoadingToken || 0) + 1;
    target.__qcsLoadingToken = token;
    setTimeout(function () {
      if (target.__qcsLoadingToken === token) {
        target.innerHTML = renderLoadingHTML(message);
      }
    }, 150);
  }

  function renderEmptyState(options) {
    var opts = options || {};
    var title = opts.title || t('empty_title');
    var description = opts.description || t('empty_desc');
    var actionHref = opts.actionHref || ROUTES.dashboard;
    var actionLabel = opts.actionLabel || t('empty_action');
    return '' +
      '<section class="qcs-empty" role="status" aria-live="polite">' +
      '<h2 class="qcs-empty-title">' + window.qcsEscapeHTML(title) + '</h2>' +
      '<p class="qcs-empty-desc">' + window.qcsEscapeHTML(description) + '</p>' +
      '<a class="qcs-empty-action" href="' + window.qcsEscapeHTML(actionHref) + '">' + window.qcsEscapeHTML(actionLabel) + '</a>' +
      '</section>';
  }

  function mountEmpty(target, options) {
    if (!target) return;
    target.innerHTML = renderEmptyState(options);
  }

  function renderErrorState(options) {
    var opts = options || {};
    var title = opts.title || t('error_title');
    var description = opts.description || t('error_desc');
    var actionLabel = opts.actionLabel || t('error_action');
    var actionId = opts.actionId || 'qcs-error-retry';
    return '' +
      '<section class="qcs-error-state" role="alert" aria-live="assertive">' +
      '<svg class="qcs-error-icon" aria-hidden="true" width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="8" x2="12" y2="12"></line><line x1="12" y1="16" x2="12.01" y2="16"></line></svg>' +
      '<h2 class="qcs-error-title">' + window.qcsEscapeHTML(title) + '</h2>' +
      '<p class="qcs-error-desc">' + window.qcsEscapeHTML(description) + '</p>' +
      '<button type="button" id="' + window.qcsEscapeHTML(actionId) + '" class="qcs-error-action">' + window.qcsEscapeHTML(actionLabel) + '</button>' +
      '</section>';
  }

  function mountError(target, options) {
    if (!target) return;
    var opts = options || {};
    target.innerHTML = renderErrorState(opts);
    var actionId = opts.actionId || 'qcs-error-retry';
    var btn = document.getElementById(actionId);
    if (btn && typeof opts.onRetry === 'function') {
      btn.addEventListener('click', opts.onRetry);
    } else if (btn) {
      btn.addEventListener('click', function () { window.location.reload(); });
    }
  }

  // Toast notifications. Stacks; auto-dismisses; aria-live=polite.
  function toast(message, level, options) {
    var opts = options || {};
    var lvl = level || 'info';
    var stack = document.getElementById('qcs-toast-stack');
    if (!stack) {
      stack = document.createElement('div');
      stack.id = 'qcs-toast-stack';
      stack.className = 'qcs-toast-stack';
      stack.setAttribute('role', 'status');
      stack.setAttribute('aria-live', 'polite');
      document.body.appendChild(stack);
    }
    var el = document.createElement('div');
    el.className = 'qcs-toast qcs-toast-' + lvl;
    el.textContent = message == null ? '' : String(message);
    stack.appendChild(el);
    requestAnimationFrame(function () { el.classList.add('is-shown'); });
    var ms = typeof opts.duration === 'number' ? opts.duration : (lvl === 'error' ? 6000 : 3500);
    setTimeout(function () {
      el.classList.remove('is-shown');
      el.classList.add('is-hidden');
      setTimeout(function () { if (el.parentNode) el.parentNode.removeChild(el); }, 250);
    }, ms);
  }

  // fetchJson wraps fetch with: bearer auth, JSON body/parse, 401 => attempt
  // refresh via /auth/refresh and retry once, throws QCSHttpError otherwise.
  function QCSHttpError(message, status, body) {
    this.message = message;
    this.status = status;
    this.body = body;
  }
  QCSHttpError.prototype = Object.create(Error.prototype);

  function buildHeaders(extra) {
    var token = readToken();
    var h = { 'Accept': 'application/json' };
    if (token) h['Authorization'] = 'Bearer ' + token;
    if (extra) {
      for (var k in extra) {
        if (Object.prototype.hasOwnProperty.call(extra, k)) h[k] = extra[k];
      }
    }
    return h;
  }

  function attemptRefresh() {
    return fetch('/api/v1/auth/refresh', { method: 'POST', credentials: 'include' })
      .then(function (r) { return r.ok ? r.json() : null; })
      .then(function (j) {
        if (j && j.data && j.data.access_token) {
          try { localStorage.setItem('qcs_access_token', j.data.access_token); } catch (e) {}
          return true;
        }
        return false;
      })
      .catch(function () { return false; });
  }

  function fetchJson(url, options) {
    var opts = options || {};
    var method = (opts.method || 'GET').toUpperCase();
    var hasBody = opts.body !== undefined && opts.body !== null;
    var bodyIsForm = hasBody && (opts.body instanceof FormData);
    var headers = buildHeaders(opts.headers);
    if (hasBody && !bodyIsForm && !headers['Content-Type']) headers['Content-Type'] = 'application/json';
    var init = {
      method: method,
      headers: headers,
      credentials: 'include',
      signal: opts.signal
    };
    if (hasBody) init.body = bodyIsForm ? opts.body : JSON.stringify(opts.body);
    var attempt = 0;
    function run() {
      return fetch(url, init).then(function (resp) {
        if (resp.status === 401 && attempt === 0 && opts.retryOnRefresh !== false) {
          attempt += 1;
          return attemptRefresh().then(function (ok) {
            if (!ok) {
              var loginRedirect = '/login?redirect=' + encodeURIComponent(window.location.pathname + window.location.search);
              try { localStorage.removeItem('qcs_access_token'); } catch (e) {}
              window.location.href = loginRedirect;
              return Promise.reject(new QCSHttpError(t('session_expired'), 401, null));
            }
            init.headers = buildHeaders(opts.headers);
            return run();
          });
        }
        var ct = resp.headers.get('Content-Type') || '';
        var bodyPromise = ct.indexOf('application/json') >= 0
          ? resp.json().catch(function () { return null; })
          : resp.text().catch(function () { return null; });
        return bodyPromise.then(function (body) {
          if (!resp.ok) {
            var msg = (body && body.error && body.error.message) || ('HTTP ' + resp.status);
            throw new QCSHttpError(msg, resp.status, body);
          }
          return body;
        });
      });
    }
    return run();
  }

  // statusBadge returns escaped HTML for a colored status pill.
  function statusBadge(value) {
    var v = (value == null ? '' : String(value)).toLowerCase().trim();
    var cls = 'qcs-badge qcs-badge-neutral';
    if (v === 'paid' || v === 'delivered' || v === 'completed' || v === 'received' || v === 'active' || v === 'shipped' || v === 'staged') cls = 'qcs-badge qcs-badge-success';
    else if (v === 'pending' || v === 'pending_payment' || v === 'pending_customs' || v === 'in_progress' || v === 'processing' || v === 'queued' || v === 'in_transit' || v === 'requires_action' || v === 'in_progres') cls = 'qcs-badge qcs-badge-warning';
    else if (v === 'failed' || v === 'cancelled' || v === 'canceled' || v === 'review_required' || v === 'banned' || v === 'inactive') cls = 'qcs-badge qcs-badge-danger';
    else if (v === 'draft' || v === 'unpaid') cls = 'qcs-badge qcs-badge-info';
    var label = (value == null ? '—' : String(value).replace(/_/g, ' '));
    return '<span class="' + cls + '">' + window.qcsEscapeHTML(label) + '</span>';
  }

  function formatMoney(amount, currency) {
    var n = Number(amount || 0);
    var cur = (currency || 'USD').toUpperCase();
    try {
      return new Intl.NumberFormat(getLocale() === 'es' ? 'es-US' : 'en-US', {
        style: 'currency', currency: cur, minimumFractionDigits: 2
      }).format(n);
    } catch (e) {
      return cur + ' ' + n.toFixed(2);
    }
  }

  function formatDate(value, opts) {
    if (!value) return '—';
    var d = (value instanceof Date) ? value : new Date(value);
    if (isNaN(d.getTime())) return String(value);
    var o = opts || { dateStyle: 'medium' };
    try {
      return new Intl.DateTimeFormat(getLocale() === 'es' ? 'es-US' : 'en-US', o).format(d);
    } catch (e) {
      return d.toISOString().slice(0, 10);
    }
  }

  // safeURL returns the URL if it is a same-origin path or http(s) URL on
  // the same origin, otherwise '#'. Use when interpolating user-supplied URLs.
  function safeURL(url) {
    if (url == null) return '#';
    var s = String(url).trim();
    if (!s) return '#';
    if (s.charAt(0) === '/' && (s.length === 1 || s.charAt(1) !== '/')) return s;
    var lower = s.toLowerCase();
    if (lower.indexOf('http://') === 0 || lower.indexOf('https://') === 0) {
      try {
        var parsed = new URL(s);
        if (parsed.origin === window.location.origin) return parsed.pathname + parsed.search + parsed.hash;
      } catch (e) {}
    }
    return '#';
  }

  // renderSidebar returns the canonical HTML string for the customer sidebar.
  // active = the SIDEBAR_LINKS.key of the current page (so it gets aria-current
  // and the active class).
  function renderSidebar(active, opts) {
    var options = opts || {};
    var brand = window.qcsEscapeHTML(t('app_brand'));
    var html = '<aside class="qcs-sidebar" aria-label="Primary navigation" data-qcs-sidebar="">' +
      '<a href="/dashboard" class="qcs-sidebar-brand">' +
      '<span aria-hidden="true">📦</span> <span>' + brand + '</span>' +
      '</a>' +
      '<nav class="qcs-sidebar-nav" aria-label="Sections">';
    for (var i = 0; i < SIDEBAR_LINKS.length; i += 1) {
      var item = SIDEBAR_LINKS[i];
      var isActive = item.key === active;
      var cls = 'qcs-sidebar-link';
      if (isActive) cls += ' is-active';
      if (item.cta) cls += ' is-cta';
      html += '<a href="' + window.qcsEscapeHTML(item.href) + '" class="' + cls + '"' +
        (isActive ? ' aria-current="page"' : '') + '>' +
        '<span class="qcs-sidebar-icon" aria-hidden="true">' + window.qcsEscapeHTML(item.icon) + '</span>' +
        '<span>' + window.qcsEscapeHTML(item.label) + '</span>' +
        '</a>';
    }
    html += '</nav>';
    if (options.showLogout !== false) {
      html += '<button type="button" id="qcs-sidebar-logout" class="qcs-sidebar-logout">' + window.qcsEscapeHTML(t('sign_out')) + '</button>';
    }
    html += '</aside>';
    return html;
  }

  // bindLogout attaches a logout handler to #qcs-sidebar-logout (plus any
  // legacy buttons with class .sidebar-logout or id #logout-btn).
  function bindLogout() {
    var btns = [].concat(
      [document.getElementById('qcs-sidebar-logout')].filter(Boolean),
      [document.getElementById('logout-btn')].filter(Boolean),
      [].slice.call(document.querySelectorAll('.sidebar-logout'))
    );
    btns.forEach(function (btn) {
      if (btn._qcsBound) return;
      btn._qcsBound = true;
      btn.addEventListener('click', function () {
        fetch('/api/v1/auth/logout', { method: 'POST', credentials: 'include' })
          .catch(function () {})
          .then(function () {
            try { localStorage.removeItem('qcs_access_token'); } catch (e) {}
            window.location.href = '/';
          });
      });
    });
  }

  // copyToClipboard with toast feedback. Falls back to a hidden textarea if
  // the modern Clipboard API is unavailable.
  function copyToClipboard(text) {
    var done = function () { toast(t('copy_done'), 'success'); };
    if (navigator.clipboard && navigator.clipboard.writeText) {
      return navigator.clipboard.writeText(String(text || '')).then(done, function () {
        legacyCopy(text); done();
      });
    }
    legacyCopy(text); done();
    return Promise.resolve();
  }
  function legacyCopy(text) {
    var ta = document.createElement('textarea');
    ta.value = String(text || '');
    ta.setAttribute('readonly', '');
    ta.style.position = 'absolute';
    ta.style.left = '-9999px';
    document.body.appendChild(ta);
    ta.select();
    try { document.execCommand('copy'); } catch (e) {}
    document.body.removeChild(ta);
  }

  function readToken() {
    try {
      return localStorage.getItem('qcs_access_token') || '';
    } catch (err) {
      return '';
    }
  }

  function authHeaders(token) {
    if (!token) return {};
    return { Authorization: 'Bearer ' + token };
  }

  function registerSW() {
    if (!('serviceWorker' in navigator)) return Promise.resolve(null);
    return navigator.serviceWorker.register('/sw.js', { scope: '/' }).catch(function () {
      return null;
    });
  }

  function updateStreamStatus(selector, text, stateClass) {
    if (!selector) return;
    var el = document.querySelector(selector);
    if (!el) return;
    el.textContent = text;
    el.classList.remove('qcs-stream-connected', 'qcs-stream-retry', 'qcs-stream-offline');
    if (stateClass) el.classList.add(stateClass);
  }

  function openNotificationStream(options) {
    var opts = options || {};
    var endpoint = opts.endpoint || STREAM_DEFAULT;
    var onEvent = typeof opts.onEvent === 'function' ? opts.onEvent : function () {};
    var onError = typeof opts.onError === 'function' ? opts.onError : function () {};
    var statusSelector = opts.statusSelector || '';

    if (!window.navigator.onLine) {
      updateStreamStatus(statusSelector, t('realtime_offline'), 'qcs-stream-offline');
      return { close: function () {} };
    }

    if (typeof window.EventSource !== 'function') {
      onError(new Error('EventSource not supported'));
      updateStreamStatus(statusSelector, t('realtime_retry'), 'qcs-stream-retry');
      return { close: function () {} };
    }

    var source = new EventSource(endpoint, { withCredentials: true });
    updateStreamStatus(statusSelector, t('realtime_connecting'), 'qcs-stream-retry');

    source.onopen = function () {
      updateStreamStatus(statusSelector, t('realtime_connected'), 'qcs-stream-connected');
    };

    source.onmessage = function (event) {
      var payload = event.data;
      try {
        payload = JSON.parse(event.data);
      } catch (err) {
        payload = { raw: event.data };
      }
      onEvent(payload);
    };

    source.onerror = function (err) {
      onError(err);
      updateStreamStatus(statusSelector, t('realtime_retry'), 'qcs-stream-retry');
    };

    window.addEventListener('offline', function () {
      updateStreamStatus(statusSelector, t('realtime_offline'), 'qcs-stream-offline');
    });

    return {
      close: function () {
        source.close();
      }
    };
  }

  function base64ToUint8(base64) {
    var normalized = (base64 || '').replace(/-/g, '+').replace(/_/g, '/');
    while (normalized.length % 4) normalized += '=';
    var raw = atob(normalized);
    var out = new Uint8Array(raw.length);
    for (var i = 0; i < raw.length; i += 1) out[i] = raw.charCodeAt(i);
    return out;
  }

  function subscribePush(options) {
    var opts = options || {};
    var token = opts.token || '';
    var endpoint = opts.endpoint || PUSH_SUBSCRIBE_DEFAULT;
    var vapidPublicKey = opts.vapidPublicKey || '';

    if (!('serviceWorker' in navigator) || !('PushManager' in window)) {
      return Promise.reject(new Error(t('push_unsupported')));
    }

    return registerSW().then(function (registration) {
      if (!registration) {
        throw new Error(t('push_unsupported'));
      }

      if (Notification.permission === 'denied') {
        throw new Error(t('push_denied'));
      }

      var requestPermission = Notification.permission === 'granted'
        ? Promise.resolve('granted')
        : Notification.requestPermission();

      return requestPermission.then(function (permission) {
        if (permission !== 'granted') {
          throw new Error(t('push_denied'));
        }

        return registration.pushManager.getSubscription().then(function (existing) {
          if (existing) return existing;
          var subOpts = { userVisibleOnly: true };
          if (vapidPublicKey) subOpts.applicationServerKey = base64ToUint8(vapidPublicKey);
          return registration.pushManager.subscribe(subOpts);
        }).then(function (subscription) {
          var payload = {
            endpoint: subscription.endpoint,
            keys: subscription.toJSON().keys || {}
          };

          if (!endpoint) {
            return { status: 'local-only', subscription: payload, message: t('push_local_only') };
          }

          return fetch(endpoint, {
            method: 'POST',
            headers: Object.assign({ 'Content-Type': 'application/json' }, authHeaders(token)),
            credentials: 'include',
            body: JSON.stringify(payload)
          }).then(function (response) {
            if (!response.ok) {
              return { status: 'local-only', subscription: payload, message: t('push_local_only') };
            }
            return { status: 'ok', subscription: payload, message: t('push_ready') };
          }).catch(function () {
            return { status: 'local-only', subscription: payload, message: t('push_local_only') };
          });
        });
      });
    });
  }

  function focusSearchInput() {
    var input = document.querySelector('[data-qcs-search], #qcs-search, input[type="search"]');
    if (input) {
      input.focus();
      if (typeof input.select === 'function') input.select();
    }
  }

  function showShortcutHelp() {
    var modal = document.getElementById('qcs-shortcut-help');
    if (!modal) {
      modal = document.createElement('div');
      modal.id = 'qcs-shortcut-help';
      modal.className = 'qcs-shortcuts-modal';
      modal.innerHTML = '' +
        '<div class="qcs-shortcuts-panel" role="dialog" aria-modal="true" aria-labelledby="qcs-shortcuts-title">' +
        '<h2 id="qcs-shortcuts-title">' + window.qcsEscapeHTML(t('shortcut_help')) + '</h2>' +
        '<ul>' +
        '<li><kbd>g d</kbd> Dashboard</li>' +
        '<li><kbd>g i</kbd> My Packages</li>' +
        '<li><kbd>g s</kbd> Shipments</li>' +
        '<li><kbd>g n</kbd> Notifications</li>' +
        '<li><kbd>/</kbd> Focus search</li>' +
        '<li><kbd>t</kbd> Toggle theme</li>' +
        '<li><kbd>?</kbd> Toggle this help</li>' +
        '</ul>' +
        '<button type="button" id="qcs-shortcuts-close">Close</button>' +
        '</div>';
      document.body.appendChild(modal);
      modal.addEventListener('click', function (event) {
        if (event.target === modal) modal.classList.remove('is-open');
      });
      var closeBtn = document.getElementById('qcs-shortcuts-close');
      if (closeBtn) closeBtn.addEventListener('click', function () { modal.classList.remove('is-open'); });
    }

    modal.classList.toggle('is-open');
    try {
      localStorage.setItem(LAST_SHORTCUT_KEY, '1');
    } catch (err) {
      // Ignore storage errors.
    }
  }

  function gotoRoute(routeKey) {
    var href = ROUTES[routeKey];
    if (href) window.location.href = href;
  }

  function initKeyboardShortcuts() {
    if (keyboardState.initialized) return;
    keyboardState.initialized = true;

    document.addEventListener('keydown', function (event) {
      if (!event) return;
      var target = event.target;
      var tag = target && target.tagName ? target.tagName.toLowerCase() : '';
      var inEditable = tag === 'input' || tag === 'textarea' || tag === 'select' || (target && target.isContentEditable);

      if (event.key === '?' && !inEditable) {
        event.preventDefault();
        showShortcutHelp();
        return;
      }

      if (event.key === '/' && !inEditable) {
        event.preventDefault();
        focusSearchInput();
        return;
      }

      if (event.key === 't' && !event.ctrlKey && !event.metaKey && !inEditable) {
        toggleTheme();
        return;
      }

      var lower = (event.key || '').toLowerCase();
      if (lower === 'g' && !inEditable) {
        keyboardState.gPressedAt = nowMs();
        return;
      }

      if (keyboardState.gPressedAt > 0 && nowMs() - keyboardState.gPressedAt < 1400) {
        keyboardState.gPressedAt = 0;
        if (lower === 'd') gotoRoute('dashboard');
        if (lower === 'i') gotoRoute('inbox');
        if (lower === 's') gotoRoute('shipments');
        if (lower === 'n') gotoRoute('notifications');
      }
    });
  }

  function ensureUtilityDock() {
    var dock = document.getElementById('qcs-pwa-tools');
    if (!dock) {
      dock = document.createElement('div');
      dock.id = 'qcs-pwa-tools';
      dock.className = 'qcs-pwa-tools';
      dock.innerHTML = '' +
        '<button type="button" id="qcs-theme-toggle" class="qcs-tools-btn" aria-label="Toggle theme"></button>' +
        '<label class="qcs-tools-label" for="qcs-locale-select">Lang</label>' +
        '<select id="qcs-locale-select" class="qcs-tools-select" aria-label="Language selector">' +
        '<option value="en">EN</option>' +
        '<option value="es">ES</option>' +
        '</select>' +
        '<button type="button" id="qcs-help-toggle" class="qcs-tools-btn" aria-label="Keyboard shortcuts">?</button>';
      document.body.appendChild(dock);

      var themeBtn = document.getElementById('qcs-theme-toggle');
      if (themeBtn) {
        themeBtn.addEventListener('click', function () {
          toggleTheme();
        });
      }

      var localeSelect = document.getElementById('qcs-locale-select');
      if (localeSelect) {
        localeSelect.addEventListener('change', function () {
          setLocale(localeSelect.value);
        });
      }

      var helpBtn = document.getElementById('qcs-help-toggle');
      if (helpBtn) {
        helpBtn.addEventListener('click', function () {
          showShortcutHelp();
        });
      }
    }

    var select = document.getElementById('qcs-locale-select');
    if (select) select.value = getLocale();
    updateUtilityLabels();
  }

  function updateUtilityLabels() {
    var themeBtn = document.getElementById('qcs-theme-toggle');
    if (themeBtn) {
      var dark = getTheme() === 'dark';
      themeBtn.textContent = dark ? t('theme_light') : t('theme_dark');
    }
  }

  function initBase(options) {
    var opts = options || {};
    ensureEscape();
    document.documentElement.setAttribute('lang', getLocale());
    applyTheme(getTheme());
    setLocale(getLocale());
    if (opts.registerSW !== false) registerSW();
    if (opts.keyboard !== false) initKeyboardShortcuts();
    if (opts.utilityDock !== false) ensureUtilityDock();
  }

  window.QCSPWA = {
    initBase: initBase,
    t: t,
    getLocale: getLocale,
    setLocale: setLocale,
    getTheme: getTheme,
    applyTheme: applyTheme,
    toggleTheme: toggleTheme,
    readToken: readToken,
    authHeaders: authHeaders,
    registerSW: registerSW,
    renderLoadingHTML: renderLoadingHTML,
    mountLoading: mountLoading,
    renderEmptyState: renderEmptyState,
    mountEmpty: mountEmpty,
    renderErrorState: renderErrorState,
    mountError: mountError,
    toast: toast,
    fetchJson: fetchJson,
    HttpError: QCSHttpError,
    statusBadge: statusBadge,
    formatMoney: formatMoney,
    formatDate: formatDate,
    safeURL: safeURL,
    renderSidebar: renderSidebar,
    bindLogout: bindLogout,
    copyToClipboard: copyToClipboard,
    sidebarLinks: SIDEBAR_LINKS,
    openNotificationStream: openNotificationStream,
    subscribePush: subscribePush,
    showShortcutHelp: showShortcutHelp,
    ROUTES: ROUTES
  };
})();
