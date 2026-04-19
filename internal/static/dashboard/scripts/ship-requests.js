// Auto-extracted from ship-requests.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    (function () {
      'use strict';
      var QCS = window.QCSPWA;
      QCS.initBase({ registerSW: true, keyboard: true, utilityDock: true });

      var loginRedirect = '/login?redirect=' + encodeURIComponent('/dashboard/ship-requests');
      var token = QCS.readToken();
      if (!token) { window.location.href = loginRedirect; return; }

      var app = document.getElementById('dashboard-app');
      QCS.mountLoading(app, QCS.t('loading'));

      Promise.all([
        QCS.fetchJson('/api/v1/me'),
        QCS.fetchJson('/api/v1/ship-requests').catch(function () { return { data: [] }; })
      ]).then(function (results) {
        var me = results[0];
        if (!me || !me.data) { window.location.href = loginRedirect; return; }
        var list = (results[1] && results[1].data) || [];
        renderShell(list);
        QCS.bindLogout();
      }).catch(function (err) {
        QCS.mountError(app, {
          title: 'Unable to load ship requests',
          description: (err && err.message) || 'Network error.',
          actionLabel: 'Sign in again',
          onRetry: function () { window.location.href = loginRedirect; }
        });
      });

      function renderShell(list) {
        var sidebar = QCS.renderSidebar('ship-requests');
        var html = '<div class="qcs-page-wrap">' + sidebar
          + '<main id="qcs-main" class="qcs-page-main" tabindex="-1">'
          + '<nav aria-label="Breadcrumb" class="mb-4"><ol class="flex items-center gap-2 text-sm text-slate-600">'
          + '<li><a href="/dashboard" class="text-[#2563EB] hover:underline">Dashboard</a></li>'
          + '<li aria-hidden="true">/</li>'
          + '<li class="text-[#0F172A] font-medium" aria-current="page">Ship Requests</li></ol></nav>'
          + '<div class="flex flex-wrap items-end justify-between gap-3 mb-6">'
          + '<div>'
          + '<h1 class="text-3xl font-bold text-[#0F172A]">Ship Requests</h1>'
          + '<p class="text-slate-600 mt-1">' + list.length + ' total</p>'
          + '</div>'
          + '<a href="/dashboard/ship" class="bg-[#F97316] text-white px-5 py-2 rounded-lg font-semibold shadow-sm hover:opacity-95">New ship request</a>'
          + '</div>'
          + (list.length === 0 ? QCS.renderEmptyState({
              title: 'No ship requests yet',
              description: 'Create your first ship request from packages in your locker.',
              actionHref: '/dashboard/ship',
              actionLabel: 'Start a ship request'
            }) : renderTable(list))
          + '</main></div>';
        app.innerHTML = html;
        var main = document.getElementById('qcs-main');
        if (main) main.focus({ preventScroll: true });
      }

      function renderTable(list) {
        var html = '<div class="bg-white rounded-xl border border-slate-200 shadow-sm overflow-hidden">'
          + '<div class="overflow-x-auto"><table class="w-full">'
          + '<thead class="bg-slate-50 border-b border-slate-200"><tr>'
          + th('Code') + th('Status') + th('Destination') + th('Total') + th('Created') + '<th class="py-3 px-4"></th>'
          + '</tr></thead><tbody>';
        list.forEach(function (sr) {
          var code = sr.confirmation_code || (sr.id ? String(sr.id).slice(0, 8) : '—');
          var dest = sr.destination_id || '—';
          var total = QCS.formatMoney(sr.total || 0, 'USD');
          var date = sr.created_at ? QCS.formatDate(sr.created_at) : '—';
          html += '<tr class="border-b border-slate-100 hover:bg-slate-50">'
            + '<td class="py-3 px-4 font-mono text-sm font-medium">' + window.qcsEscapeHTML(code) + '</td>'
            + '<td class="py-3 px-4">' + QCS.statusBadge(sr.status || 'draft') + '</td>'
            + '<td class="py-3 px-4">' + window.qcsEscapeHTML(dest) + '</td>'
            + '<td class="py-3 px-4 font-medium">' + window.qcsEscapeHTML(total) + '</td>'
            + '<td class="py-3 px-4 text-slate-500 text-sm">' + window.qcsEscapeHTML(date) + '</td>'
            + '<td class="py-3 px-4 text-right"><a href="/dashboard/ship-requests/' + encodeURIComponent(sr.id) + '" class="text-[#2563EB] hover:underline font-medium">View →</a></td>'
            + '</tr>';
        });
        html += '</tbody></table></div></div>';
        return html;
      }

      function th(label) {
        return '<th class="text-left py-3 px-4 font-medium text-slate-600 text-sm whitespace-nowrap">' + window.qcsEscapeHTML(label) + '</th>';
      }
    })();
  