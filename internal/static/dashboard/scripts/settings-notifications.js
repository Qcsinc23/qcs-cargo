// Auto-extracted from notifications.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    (function () {
      'use strict';
      var QCS = window.QCSPWA;
      QCS.initBase({ registerSW: true, keyboard: true, utilityDock: true });

      var loginRedirect = '/login?redirect=' + encodeURIComponent('/dashboard/settings/notifications');
      var token = QCS.readToken();
      if (!token) { window.location.href = loginRedirect; return; }

      var app = document.getElementById('dashboard-app');
      QCS.mountLoading(app, QCS.t('loading_notifications'));

      var streamHandle = null;

      Promise.all([
        QCS.fetchJson('/api/v1/me'),
        QCS.fetchJson('/api/v1/notifications/preferences').catch(function () { return { data: {} }; })
      ]).then(function (results) {
        var me = results[0];
        if (!me || !me.data) { window.location.href = loginRedirect; return; }
        renderShell(((results[1] && results[1].data) || {}));
        QCS.bindLogout();
        try {
          streamHandle = QCS.openNotificationStream({ statusSelector: '#notification-stream-status' });
          window.addEventListener('beforeunload', function () {
            if (streamHandle && typeof streamHandle.close === 'function') streamHandle.close();
          });
        } catch (e) {}
      }).catch(function (err) {
        QCS.mountError(app, {
          title: 'Unable to load notification settings',
          description: (err && err.message) || 'Network error.',
          actionLabel: 'Sign in again',
          onRetry: function () { window.location.href = loginRedirect; }
        });
      });

      function checked(v) { return (v === true || v === 1) ? ' checked' : ''; }

      function renderShell(p) {
        var sidebar = QCS.renderSidebar('settings');
        var html = '<div class="qcs-page-wrap">' + sidebar
          + '<main id="qcs-main" class="qcs-page-main" tabindex="-1">'
          + '<nav aria-label="Breadcrumb" class="mb-4"><ol class="flex items-center gap-2 text-sm text-slate-600">'
          + '<li><a href="/dashboard" class="text-[#2563EB] hover:underline">Dashboard</a></li>'
          + '<li aria-hidden="true">/</li>'
          + '<li><a href="/dashboard/settings" class="text-[#2563EB] hover:underline">Settings</a></li>'
          + '<li aria-hidden="true">/</li>'
          + '<li class="text-[#0F172A] font-medium" aria-current="page">Notifications</li></ol></nav>'
          + '<div class="flex flex-wrap items-end justify-between gap-3 mb-6">'
          + '<div>'
          + '<h1 class="text-3xl font-bold text-[#0F172A]">Notification preferences</h1>'
          + '<p class="text-sm text-slate-500 mt-1">Choose how QCS Cargo keeps you in the loop.</p>'
          + '</div>'
          + '<p id="notification-stream-status" class="text-xs text-slate-500" aria-live="polite">Realtime updates connecting…</p>'
          + '</div>'
          + '<form id="prefs-form" class="bg-white rounded-xl border border-slate-200 shadow-sm p-6 max-w-2xl space-y-5">'
          + section('Channels',
              toggle('email_enabled', 'Email notifications', p.email_enabled)
              + toggle('sms_enabled', 'SMS notifications', p.sms_enabled)
              + toggle('push_enabled', 'Push notifications', p.push_enabled))
          + section('Events',
              toggle('on_package_arrived', 'When a package arrives', p.on_package_arrived)
              + toggle('on_storage_expiry', 'Storage expiry reminder', p.on_storage_expiry)
              + toggle('on_ship_updates', 'Shipment updates', p.on_ship_updates)
              + toggle('on_inbound_updates', 'Inbound tracking updates', p.on_inbound_updates))
          + '<p class="text-slate-600 text-sm">Daily digest: <span class="font-medium">' + window.qcsEscapeHTML(String(p.daily_digest || 'off')) + '</span></p>'
          + '<div class="flex flex-wrap gap-3 pt-2 border-t border-slate-100">'
          + '<button type="submit" class="bg-[#F97316] text-white px-6 py-2 rounded-lg font-semibold shadow-sm hover:opacity-95">Save preferences</button>'
          + '<button type="button" id="push-enable-btn" class="bg-white border border-slate-300 px-4 py-2 rounded-lg font-medium hover:bg-slate-50">Enable push on this device</button>'
          + '</div>'
          + '<p id="prefs-msg" class="text-sm text-emerald-600 hidden" aria-live="polite">Saved.</p>'
          + '</form>'
          + '</main></div>';
        app.innerHTML = html;
        var main = document.getElementById('qcs-main');
        if (main) main.focus({ preventScroll: true });

        var pushBtn = document.getElementById('push-enable-btn');
        if (pushBtn) {
          pushBtn.addEventListener('click', function () {
            QCS.subscribePush({ token: token, endpoint: '/api/v1/notifications/push/subscribe' })
              .then(function (msg) {
                QCS.toast((msg && msg.message) || 'Push enabled', (msg && msg.status === 'ok') ? 'success' : 'info');
              })
              .catch(function (err) {
                QCS.toast((err && err.message) || 'Could not enable push', 'error');
              });
          });
        }

        document.getElementById('prefs-form').addEventListener('submit', function (e) {
          e.preventDefault();
          var body = {
            email_enabled: this.email_enabled.checked ? 1 : 0,
            sms_enabled: this.sms_enabled.checked ? 1 : 0,
            push_enabled: this.push_enabled.checked ? 1 : 0,
            on_package_arrived: this.on_package_arrived.checked ? 1 : 0,
            on_storage_expiry: this.on_storage_expiry.checked ? 1 : 0,
            on_ship_updates: this.on_ship_updates.checked ? 1 : 0,
            on_inbound_updates: this.on_inbound_updates.checked ? 1 : 0
          };
          QCS.fetchJson('/api/v1/notifications/preferences', { method: 'PUT', body: body })
            .then(function () {
              var msg = document.getElementById('prefs-msg');
              if (msg) {
                msg.textContent = 'Preferences saved.';
                msg.classList.remove('hidden');
                setTimeout(function () { msg.classList.add('hidden'); }, 2200);
              }
              QCS.toast('Preferences saved', 'success');
            })
            .catch(function (err) {
              QCS.toast((err && err.message) || 'Could not save preferences', 'error');
            });
        });
      }

      function section(title, body) {
        return '<fieldset class="space-y-2"><legend class="text-xs uppercase tracking-wider text-slate-500 font-semibold mb-2">' + window.qcsEscapeHTML(title) + '</legend>' + body + '</fieldset>';
      }

      function toggle(name, label, value) {
        return '<label class="flex items-center gap-3 py-1 text-sm">'
          + '<input type="checkbox" name="' + window.qcsEscapeHTML(name) + '" class="h-4 w-4 rounded border-slate-300 text-[#2563EB] focus:ring-[#2563EB]"' + checked(value) + '>'
          + '<span>' + window.qcsEscapeHTML(label) + '</span>'
          + '</label>';
      }
    })();
  