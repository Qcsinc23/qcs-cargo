// Auto-extracted from ship-request-detail.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    (function () {
      'use strict';
      var QCS = window.QCSPWA;
      QCS.initBase({ registerSW: true, keyboard: true, utilityDock: true });

      var loginRedirect = '/login?redirect=' + encodeURIComponent(window.location.pathname);
      var token = QCS.readToken();
      if (!token) { window.location.href = loginRedirect; return; }

      var pathParts = window.location.pathname.split('/');
      var id = pathParts[pathParts.length - 1];
      if (!id || id === 'ship-requests') { window.location.href = '/dashboard/ship-requests'; return; }

      var app = document.getElementById('dashboard-app');
      QCS.mountLoading(app, QCS.t('loading'));

      Promise.all([
        QCS.fetchJson('/api/v1/me'),
        QCS.fetchJson('/api/v1/ship-requests/' + encodeURIComponent(id))
      ]).then(function (results) {
        var me = results[0];
        if (!me || !me.data) { window.location.href = loginRedirect; return; }
        if (!results[1] || !results[1].data) {
          QCS.mountError(app, {
            title: 'Ship request not found',
            description: 'We could not find that ship request.',
            actionLabel: 'Back to ship requests',
            onRetry: function () { window.location.href = '/dashboard/ship-requests'; }
          });
          return;
        }
        renderShell(results[1].data);
        QCS.bindLogout();
      }).catch(function (err) {
        QCS.mountError(app, {
          title: 'Unable to load ship request',
          description: (err && err.message) || 'Network error.',
          actionLabel: 'Sign in again',
          onRetry: function () { window.location.href = loginRedirect; }
        });
      });

      function renderShell(payload) {
        var sr = payload.ship_request || payload;
        var items = payload.items || [];
        var status = sr.status || 'draft';
        var instr = typeof sr.special_instructions === 'string'
          ? sr.special_instructions
          : (sr.special_instructions && sr.special_instructions.String) || '';
        var totalCents = sr.total != null ? Number(sr.total) : 0;
        var totalLabel = QCS.formatMoney(totalCents, 'USD');
        var code = sr.confirmation_code || (sr.id ? String(sr.id).slice(0, 8) : '—');

        var sidebar = QCS.renderSidebar('ship-requests');
        var html = '<div class="qcs-page-wrap">' + sidebar
          + '<main id="qcs-main" class="qcs-page-main" tabindex="-1">'
          + '<nav aria-label="Breadcrumb" class="mb-4"><ol class="flex items-center gap-2 text-sm text-slate-600">'
          + '<li><a href="/dashboard" class="text-[#2563EB] hover:underline">Dashboard</a></li>'
          + '<li aria-hidden="true">/</li>'
          + '<li><a href="/dashboard/ship-requests" class="text-[#2563EB] hover:underline">Ship Requests</a></li>'
          + '<li aria-hidden="true">/</li>'
          + '<li class="text-[#0F172A] font-medium" aria-current="page">' + window.qcsEscapeHTML(code) + '</li></ol></nav>'
          + '<div class="flex flex-wrap items-end justify-between gap-3 mb-6">'
          + '<div>'
          + '<h1 class="text-3xl font-bold text-[#0F172A]">' + window.qcsEscapeHTML(code) + '</h1>'
          + '<p class="text-slate-500 mt-1">' + window.qcsEscapeHTML(sr.destination_id || '') + '</p>'
          + '</div>'
          + '<div class="flex items-center gap-2">' + QCS.statusBadge(status)
          + '<span class="text-xl font-bold ml-2">' + window.qcsEscapeHTML(totalLabel) + '</span></div>'
          + '</div>'
          + '<div class="grid lg:grid-cols-3 gap-6 mb-6">'
          + '<section class="lg:col-span-2 bg-white rounded-xl border border-slate-200 shadow-sm p-6">'
          + '<h2 class="text-lg font-semibold mb-3">Summary</h2>'
          + '<dl class="grid sm:grid-cols-2 gap-x-6 gap-y-3 text-sm">'
          + dlRow('Destination', sr.destination_id || '—')
          + dlRow('Service', sr.service_type || '—')
          + dlRow('Total', totalLabel)
          + dlRow('Created', sr.created_at ? QCS.formatDate(sr.created_at, { dateStyle: 'medium', timeStyle: 'short' }) : '—')
          + (instr ? '<div class="sm:col-span-2"><dt class="text-slate-500">Special instructions</dt><dd class="font-medium whitespace-pre-line">' + window.qcsEscapeHTML(instr) + '</dd></div>' : '')
          + '</dl>'
          + '</section>'
          + '<aside class="bg-white rounded-xl border border-slate-200 shadow-sm p-6 space-y-3">'
          + '<h2 class="text-lg font-semibold">Actions</h2>'
          + actionsHTML(sr.id, status, totalLabel)
          + '<a href="/dashboard/ship-requests" class="block text-center bg-white border border-slate-300 px-4 py-2 rounded-lg font-medium hover:bg-slate-50">Back</a>'
          + '</aside>'
          + '</div>'
          + '<section class="bg-white rounded-xl border border-slate-200 shadow-sm p-6">'
          + '<h2 class="text-lg font-semibold mb-3">Packages (' + items.length + ')</h2>'
          + (items.length === 0
              ? '<p class="text-slate-500 text-sm">No packages attached.</p>'
              : '<ul class="divide-y divide-slate-100">'
                + items.map(function (it) {
                    return '<li class="py-2 flex items-center justify-between"><span class="font-mono text-sm">' + window.qcsEscapeHTML(it.locker_package_id || it.id) + '</span></li>';
                  }).join('')
                + '</ul>')
          + '</section>'
          + '</main></div>';
        app.innerHTML = html;
        var main = document.getElementById('qcs-main');
        if (main) main.focus({ preventScroll: true });
      }

      function dlRow(label, value) {
        return '<div><dt class="text-slate-500">' + window.qcsEscapeHTML(label) + '</dt><dd class="font-medium">' + window.qcsEscapeHTML(String(value)) + '</dd></div>';
      }

      function actionsHTML(srId, status, totalLabel) {
        var html = '';
        if (status === 'draft' || status === 'pending_customs') {
          html += '<a href="/dashboard/ship-requests/' + encodeURIComponent(srId) + '/customs" class="block text-center bg-[#2563EB] text-white px-4 py-2 rounded-lg font-semibold shadow-sm hover:opacity-95">Customs declaration</a>';
          html += '<a href="/dashboard/ship-requests/' + encodeURIComponent(srId) + '/pay" class="block text-center bg-[#F97316] text-white px-4 py-2 rounded-lg font-semibold shadow-sm hover:opacity-95">Pay now</a>';
        } else if (status === 'pending_payment') {
          html += '<a href="/dashboard/ship-requests/' + encodeURIComponent(srId) + '/pay" class="block text-center bg-[#F97316] text-white px-4 py-2 rounded-lg font-semibold shadow-sm hover:opacity-95">Pay ' + window.qcsEscapeHTML(totalLabel) + '</a>';
        } else {
          html += '<p class="text-sm text-slate-500">No further action required.</p>';
        }
        return html;
      }
    })();
  