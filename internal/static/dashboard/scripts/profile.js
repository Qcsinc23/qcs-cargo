// Auto-extracted from profile.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    (function () {
      'use strict';
      var QCS = window.QCSPWA;
      QCS.initBase({ registerSW: true, keyboard: true, utilityDock: true });

      var loginRedirect = '/login?redirect=' + encodeURIComponent('/dashboard/profile');
      var token = QCS.readToken();
      if (!token) { window.location.href = loginRedirect; return; }

      var app = document.getElementById('dashboard-app');
      QCS.mountLoading(app, QCS.t('loading'));

      QCS.fetchJson('/api/v1/me')
        .then(function (j) {
          if (!j || !j.data) { window.location.href = loginRedirect; return; }
          renderShell(j.data);
          QCS.bindLogout();
          bindForm();
        })
        .catch(function (err) {
          QCS.mountError(app, {
            title: 'Unable to load profile',
            description: (err && err.message) || 'Network error.',
            actionLabel: 'Sign in again',
            onRetry: function () { window.location.href = loginRedirect; }
          });
        });

      function renderShell(user) {
        var sidebar = QCS.renderSidebar('profile');
        var copyBtn = '<button type="button" id="copy-suite" class="text-xs text-[#2563EB] hover:underline ml-2">Copy</button>';
        var html = '<div class="qcs-page-wrap">' + sidebar
          + '<main id="qcs-main" class="qcs-page-main" tabindex="-1">'
          + '<nav aria-label="Breadcrumb" class="mb-4"><ol class="flex items-center gap-2 text-sm text-slate-600">'
          + '<li><a href="/dashboard" class="text-[#2563EB] hover:underline">Dashboard</a></li>'
          + '<li aria-hidden="true">/</li>'
          + '<li class="text-[#0F172A] font-medium" aria-current="page">Profile</li></ol></nav>'
          + '<div class="flex flex-wrap items-end justify-between gap-3 mb-6">'
          + '<div>'
          + '<h1 class="text-3xl font-bold text-[#0F172A]">Profile</h1>'
          + '<p class="text-sm text-slate-500 mt-1">Suite <span id="suite-code" class="font-mono font-semibold">' + window.qcsEscapeHTML(user.suite_code || '—') + '</span>' + copyBtn + '</p>'
          + '</div>'
          + '</div>'
          + '<form id="profile-form" class="bg-white rounded-xl border border-slate-200 shadow-sm p-6 max-w-2xl space-y-4">'
          + group('Account',
              field('name', 'Full name', 'text', user.name, true)
              + field('email', 'Email', 'email', user.email, false, true)
              + field('phone', 'Phone', 'tel', user.phone))
          + group('Mailing address',
              field('address_street', 'Street', 'text', user.address_street)
              + '<div class="grid grid-cols-1 sm:grid-cols-3 gap-4">'
              + field('address_city', 'City', 'text', user.address_city)
              + field('address_state', 'State', 'text', user.address_state)
              + field('address_zip', 'ZIP', 'text', user.address_zip)
              + '</div>')
          + '<div class="flex flex-wrap items-center gap-3 pt-3 border-t border-slate-100">'
          + '<button type="submit" class="bg-[#F97316] text-white px-6 py-2 rounded-lg font-semibold shadow-sm hover:opacity-95">Save changes</button>'
          + '<p id="profile-msg" class="text-sm hidden" aria-live="polite"></p>'
          + '</div>'
          + '</form></main></div>';
        app.innerHTML = html;
        var main = document.getElementById('qcs-main');
        if (main) main.focus({ preventScroll: true });

        var copy = document.getElementById('copy-suite');
        if (copy) {
          copy.addEventListener('click', function () {
            QCS.copyToClipboard(user.suite_code || '');
          });
        }
      }

      function group(title, body) {
        return '<fieldset class="space-y-4"><legend class="text-xs uppercase tracking-wider text-slate-500 font-semibold mb-1">' + window.qcsEscapeHTML(title) + '</legend>' + body + '</fieldset>';
      }

      function field(name, label, type, value, required, disabled) {
        var attrs = 'id="f-' + name + '" type="' + (type || 'text') + '" name="' + name + '" value="' + window.qcsEscapeHTML(value == null ? '' : value) + '"';
        if (required) attrs += ' required';
        if (disabled) attrs += ' disabled';
        var cls = disabled
          ? 'w-full border border-slate-200 rounded-lg px-3 py-2 bg-slate-50 text-slate-500'
          : 'w-full border border-slate-300 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-[#2563EB]';
        return '<div>'
          + '<label for="f-' + name + '" class="block text-sm font-medium text-slate-700 mb-1">' + window.qcsEscapeHTML(label) + '</label>'
          + '<input ' + attrs + ' class="' + cls + '">'
          + '</div>';
      }

      function bindForm() {
        var form = document.getElementById('profile-form');
        if (!form) return;
        form.addEventListener('submit', function (e) {
          e.preventDefault();
          var fd = new FormData(form);
          var body = {};
          ['name', 'phone', 'address_street', 'address_city', 'address_state', 'address_zip'].forEach(function (k) {
            var v = fd.get(k);
            if (v != null) body[k] = v;
          });
          QCS.fetchJson('/api/v1/me', { method: 'PATCH', body: body })
            .then(function () {
              setMsg('Profile updated.', true);
              QCS.toast('Profile updated', 'success');
            })
            .catch(function (err) {
              setMsg((err && err.message) || 'Could not save changes.', false);
            });
        });
      }

      function setMsg(text, ok) {
        var el = document.getElementById('profile-msg');
        if (!el) return;
        el.textContent = text;
        el.className = 'text-sm ' + (ok ? 'text-emerald-600' : 'text-red-600');
        el.classList.remove('hidden');
        if (ok) setTimeout(function () { el.classList.add('hidden'); }, 2500);
      }
    })();
  