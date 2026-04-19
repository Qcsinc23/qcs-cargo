// Auto-extracted from settings.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    (function () {
      'use strict';
      var QCS = window.QCSPWA;
      QCS.initBase({ registerSW: true, keyboard: true, utilityDock: true });

      var loginRedirect = '/login?redirect=' + encodeURIComponent('/dashboard/settings');
      var token = QCS.readToken();
      if (!token) { window.location.href = loginRedirect; return; }

      var app = document.getElementById('dashboard-app');
      QCS.mountLoading(app, QCS.t('loading'));

      QCS.fetchJson('/api/v1/me')
        .then(function (j) {
          if (!j || !j.data) { window.location.href = loginRedirect; return; }
          renderShell();
          QCS.bindLogout();
        })
        .catch(function (err) {
          QCS.mountError(app, {
            title: 'Unable to load settings',
            description: (err && err.message) || 'Network error.',
            actionLabel: 'Sign in again',
            onRetry: function () { window.location.href = loginRedirect; }
          });
        });

      function renderShell() {
        var sidebar = QCS.renderSidebar('settings');
        var html = '<div class="qcs-page-wrap">' + sidebar
          + '<main id="qcs-main" class="qcs-page-main" tabindex="-1">'
          + '<nav aria-label="Breadcrumb" class="mb-4"><ol class="flex items-center gap-2 text-sm text-slate-600">'
          + '<li><a href="/dashboard" class="text-[#2563EB] hover:underline">Dashboard</a></li>'
          + '<li aria-hidden="true">/</li>'
          + '<li class="text-[#0F172A] font-medium" aria-current="page">Settings</li></ol></nav>'
          + '<h1 class="text-3xl font-bold text-[#0F172A] mb-6">Settings</h1>'
          + '<div class="bg-white rounded-xl border border-slate-200 shadow-sm divide-y divide-slate-100">'
          + settingsRow('/dashboard/settings/notifications', '🔔', 'Notifications', 'Email, SMS, and event preferences')
          + settingsRow('/dashboard/settings/security', '🔒', 'Security', 'Password, two-factor authentication')
          + settingsRow('/dashboard/settings/sessions', '🖥️', 'Sessions', 'Active devices and remote sign-out')
          + settingsRow('/dashboard/settings/delete-account', '⚠️', 'Account lifecycle', 'Deactivate or delete your account', true)
          + '</div></main></div>';
        app.innerHTML = html;
        var main = document.getElementById('qcs-main');
        if (main) main.focus({ preventScroll: true });
      }

      function settingsRow(href, icon, title, desc, danger) {
        var titleClass = danger ? 'text-red-600' : 'text-[#0F172A]';
        return '<a href="' + window.qcsEscapeHTML(href) + '" class="flex items-start gap-4 p-4 hover:bg-slate-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-[#2563EB]">'
          + '<span class="text-2xl" aria-hidden="true">' + icon + '</span>'
          + '<span class="flex-1 min-w-0">'
          + '<span class="block font-semibold ' + titleClass + '">' + window.qcsEscapeHTML(title) + '</span>'
          + '<span class="block text-sm text-slate-500">' + window.qcsEscapeHTML(desc) + '</span>'
          + '</span>'
          + '<span aria-hidden="true" class="text-slate-400">→</span>'
          + '</a>';
      }
    })();
  