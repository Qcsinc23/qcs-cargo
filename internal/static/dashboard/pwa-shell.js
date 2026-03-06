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
      push_local_only: 'Push subscribed locally; server endpoint not available.'
    },
    es: {
      loading: 'Cargando...',
      loading_dashboard: 'Cargando panel...',
      loading_packages: 'Cargando paquetes...',
      loading_shipments: 'Cargando envios...',
      loading_notifications: 'Cargando configuracion de notificaciones...',
      empty_title: 'Aun no hay datos',
      empty_desc: 'Todavia no hay informacion para mostrar.',
      empty_action: 'Ir al panel',
      realtime_connecting: 'Conectando actualizaciones en tiempo real...',
      realtime_connected: 'Actualizaciones en tiempo real conectadas',
      realtime_retry: 'Reconectando actualizaciones en tiempo real...',
      realtime_offline: 'Actualizaciones en tiempo real en pausa (sin conexion)',
      theme_light: 'Claro',
      theme_dark: 'Oscuro',
      shortcut_help: 'Atajos del teclado',
      push_unsupported: 'Las notificaciones push no son compatibles con este navegador.',
      push_denied: 'El permiso de push fue denegado.',
      push_ready: 'Notificaciones push habilitadas en este dispositivo.',
      push_local_only: 'Push suscrito localmente; endpoint del servidor no disponible.'
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
    target.innerHTML = renderLoadingHTML(message);
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
    openNotificationStream: openNotificationStream,
    subscribePush: subscribePush,
    showShortcutHelp: showShortcutHelp,
    ROUTES: ROUTES
  };
})();
