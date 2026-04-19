// Auto-extracted from sessions.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    (function () {
      'use strict';
      var QCS = window.QCSPWA;
      QCS.initBase({ registerSW: true, keyboard: true, utilityDock: true });

      var loginRedirect = '/login?redirect=' + encodeURIComponent('/dashboard/settings/sessions');
      var token = QCS.readToken();
      var currentSessionID = '';
      try { currentSessionID = localStorage.getItem('qcs_session_id') || ''; } catch (e) {}
      if (!token) { window.location.href = loginRedirect; return; }

      var app = document.getElementById('dashboard-app');
      QCS.mountLoading(app, QCS.t('loading'));

      Promise.all([
        QCS.fetchJson('/api/v1/me'),
        QCS.fetchJson('/api/v1/sessions').catch(function () { return { data: [] }; })
      ]).then(function (results) {
        var me = results[0];
        if (!me || !me.data) { window.location.href = loginRedirect; return; }
        var list = (results[1] && results[1].data) || [];
        renderShell(list);
        QCS.bindLogout();
        bindRevokeActions();
      }).catch(function (err) {
        QCS.mountError(app, {
          title: 'Unable to load sessions',
          description: (err && err.message) || 'Network error.',
          actionLabel: 'Sign in again',
          onRetry: function () { window.location.href = loginRedirect; }
        });
      });

      function sessionLabel(s) {
        var agent = (s && s.user_agent) ? String(s.user_agent).trim() : '';
        var ip = (s && s.ip_address) ? String(s.ip_address).trim() : '';
        if (agent) return agent;
        if (ip) return ip;
        return (s && s.id) ? String(s.id).slice(0, 8) : 'Unknown device';
      }

      function safeDate(value) {
        if (!value) return '—';
        var d = new Date(value);
        if (isNaN(d.getTime())) return window.qcsEscapeHTML(String(value));
        return window.qcsEscapeHTML(QCS.formatDate(d, { dateStyle: 'medium', timeStyle: 'short' }));
      }

      function renderRows(list) {
        if (!Array.isArray(list) || list.length === 0) {
          return '<tr><td colspan="4" class="py-6 px-4 text-sm text-slate-500 text-center">No active sessions found.</td></tr>';
        }
        var html = '';
        list.forEach(function (s) {
          var sid = s && s.id ? String(s.id) : '';
          var isCurrent = currentSessionID && sid === currentSessionID;
          var currentBadge = isCurrent ? ' <span class="qcs-badge qcs-badge-success ml-2">Current</span>' : '';
          var revokeLabel = isCurrent ? 'Sign out this browser' : 'Revoke';
          html += '<tr class="border-b border-slate-100 hover:bg-slate-50">'
            + '<td class="py-3 px-4">' + window.qcsEscapeHTML(sessionLabel(s)) + currentBadge + '</td>'
            + '<td class="py-3 px-4 text-slate-500 text-sm">' + safeDate(s && s.created_at) + '</td>'
            + '<td class="py-3 px-4 text-slate-500 text-sm">' + safeDate(s && s.expires_at) + '</td>'
            + '<td class="py-3 px-4 text-right"><button type="button" class="text-red-600 hover:text-red-800 text-sm font-medium revoke-one" data-id="' + window.qcsEscapeHTML(sid) + '">' + window.qcsEscapeHTML(revokeLabel) + '</button></td></tr>';
        });
        return html;
      }

      function renderShell(list) {
        var sidebar = QCS.renderSidebar('settings');
        var html = '<div class="qcs-page-wrap">' + sidebar
          + '<main id="qcs-main" class="qcs-page-main" tabindex="-1">'
          + '<nav aria-label="Breadcrumb" class="mb-4"><ol class="flex items-center gap-2 text-sm text-slate-600">'
          + '<li><a href="/dashboard" class="text-[#2563EB] hover:underline">Dashboard</a></li>'
          + '<li aria-hidden="true">/</li>'
          + '<li><a href="/dashboard/settings" class="text-[#2563EB] hover:underline">Settings</a></li>'
          + '<li aria-hidden="true">/</li>'
          + '<li class="text-[#0F172A] font-medium" aria-current="page">Sessions</li></ol></nav>'
          + '<h1 class="text-3xl font-bold text-[#0F172A] mb-2">Active sessions</h1>'
          + '<p class="text-slate-600 mb-4">Revoke a session to sign that device out. Revoke all others to keep this browser signed in.</p>'
          + '<p id="sessions-msg" class="mb-4 text-sm text-slate-600" aria-live="polite"></p>'
          + '<button type="button" id="revoke-all-btn" class="mb-6 bg-white border border-slate-300 text-slate-800 px-4 py-2 rounded-lg font-medium hover:bg-slate-50">Revoke all other sessions</button>'
          + '<div class="bg-white rounded-xl border border-slate-200 shadow-sm overflow-hidden">'
          + '<table class="w-full"><thead class="bg-slate-50 border-b border-slate-200"><tr>'
          + '<th class="text-left py-3 px-4 font-medium text-slate-600 text-sm">Device / IP</th>'
          + '<th class="text-left py-3 px-4 font-medium text-slate-600 text-sm">Created</th>'
          + '<th class="text-left py-3 px-4 font-medium text-slate-600 text-sm">Expires</th>'
          + '<th class="text-right py-3 px-4 font-medium text-slate-600 text-sm">Action</th></tr></thead><tbody>'
          + renderRows(list)
          + '</tbody></table></div>'
          + '</main></div>';
        app.innerHTML = html;
        var main = document.getElementById('qcs-main');
        if (main) main.focus({ preventScroll: true });
      }

      function setStatus(message, isError) {
        var el = document.getElementById('sessions-msg');
        if (!el) return;
        el.textContent = message || '';
        el.className = 'mb-4 text-sm ' + (isError ? 'text-red-600' : 'text-slate-600');
      }

      function bindRevokeActions() {
        var revokeAllBtn = document.getElementById('revoke-all-btn');
        if (revokeAllBtn) {
          if (!currentSessionID) {
            revokeAllBtn.disabled = true;
            revokeAllBtn.className = 'mb-6 bg-slate-100 text-slate-400 px-4 py-2 rounded-lg font-medium cursor-not-allowed';
            setStatus('Current session ID is unavailable. Sign out and sign back in to enable "revoke all other sessions".', false);
          }
          revokeAllBtn.addEventListener('click', function () {
            if (!currentSessionID) return;
            if (!confirm('Sign out all other devices?')) return;
            QCS.fetchJson('/api/v1/sessions', { method: 'DELETE', body: { keep_session_id: currentSessionID } })
              .then(function () { window.location.reload(); })
              .catch(function (err) { setStatus((err && err.message) || 'Failed to revoke sessions.', true); });
          });
        }

        document.querySelectorAll('.revoke-one').forEach(function (btn) {
          btn.addEventListener('click', function () {
            var sid = btn.getAttribute('data-id');
            if (!sid) return;
            var isCurrent = currentSessionID && sid === currentSessionID;
            var confirmMsg = isCurrent ? 'Sign out this browser now?' : 'Revoke this session?';
            if (!confirm(confirmMsg)) return;
            QCS.fetchJson('/api/v1/sessions/' + encodeURIComponent(sid), { method: 'DELETE' })
              .then(function () {
                if (isCurrent) {
                  try {
                    localStorage.removeItem('qcs_access_token');
                    localStorage.removeItem('qcs_session_id');
                  } catch (e) {}
                  window.location.href = '/login';
                  return;
                }
                window.location.reload();
              })
              .catch(function (err) { setStatus((err && err.message) || 'Failed to revoke session.', true); });
          });
        });
      }
    })();
  