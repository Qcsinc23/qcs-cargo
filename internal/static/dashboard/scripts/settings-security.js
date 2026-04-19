// Auto-extracted from security.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    (function () {
      'use strict';
      var QCS = window.QCSPWA;
      QCS.initBase({ registerSW: true, keyboard: true, utilityDock: true });

      var loginRedirect = '/login?redirect=' + encodeURIComponent('/dashboard/settings/security');
      var token = QCS.readToken();
      if (!token) { window.location.href = loginRedirect; return; }

      var app = document.getElementById('dashboard-app');
      QCS.mountLoading(app, QCS.t('loading'));

      QCS.fetchJson('/api/v1/me')
        .then(function (j) {
          if (!j || !j.data) { window.location.href = loginRedirect; return; }
          renderShell();
          QCS.bindLogout();
          bindForm();
        })
        .catch(function (err) {
          QCS.mountError(app, {
            title: 'Unable to load security settings',
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
          + '<li><a href="/dashboard/settings" class="text-[#2563EB] hover:underline">Settings</a></li>'
          + '<li aria-hidden="true">/</li>'
          + '<li class="text-[#0F172A] font-medium" aria-current="page">Security</li></ol></nav>'
          + '<h1 class="text-3xl font-bold text-[#0F172A] mb-2">Security</h1>'
          + '<p class="text-sm text-slate-500 mb-6">Keep your account safe with a strong, unique password.</p>'
          + '<div class="bg-white rounded-xl border border-slate-200 shadow-sm p-6 max-w-lg">'
          + '<h2 class="text-lg font-semibold mb-4">Change password</h2>'
          + '<form id="pwd-form" class="space-y-4">'
          + field('current_password', 'Current password')
          + field('new_password', 'New password', 'At least 8 characters with a mix of letters and numbers.')
          + '<div class="flex flex-wrap items-center gap-3 pt-2">'
          + '<button type="submit" class="bg-[#F97316] text-white px-6 py-2 rounded-lg font-semibold shadow-sm hover:opacity-95">Update password</button>'
          + '<p id="pwd-msg" class="text-sm hidden" aria-live="polite"></p>'
          + '</div>'
          + '</form></div>'
          + '</main></div>';
        app.innerHTML = html;
        var main = document.getElementById('qcs-main');
        if (main) main.focus({ preventScroll: true });
      }

      function field(name, label, hint) {
        return '<div>'
          + '<label for="f-' + name + '" class="block text-sm font-medium text-slate-700 mb-1">' + window.qcsEscapeHTML(label) + '</label>'
          + '<input id="f-' + name + '" type="password" name="' + name + '" class="w-full border border-slate-300 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-[#2563EB]" required>'
          + (hint ? '<p class="text-xs text-slate-500 mt-1">' + window.qcsEscapeHTML(hint) + '</p>' : '')
          + '</div>';
      }

      function bindForm() {
        var form = document.getElementById('pwd-form');
        if (!form) return;
        form.addEventListener('submit', function (e) {
          e.preventDefault();
          var fd = new FormData(form);
          var body = {
            current_password: fd.get('current_password'),
            new_password: fd.get('new_password')
          };
          QCS.fetchJson('/api/v1/auth/password/change', { method: 'PATCH', body: body })
            .then(function () {
              setMsg('Password updated.', true);
              QCS.toast('Password updated', 'success');
              form.reset();
            })
            .catch(function (err) {
              setMsg((err && err.message) || 'Failed to update password.', false);
            });
        });
      }

      function setMsg(text, ok) {
        var el = document.getElementById('pwd-msg');
        if (!el) return;
        el.textContent = text;
        el.className = 'text-sm ' + (ok ? 'text-emerald-600' : 'text-red-600');
        el.classList.remove('hidden');
      }
    })();
  