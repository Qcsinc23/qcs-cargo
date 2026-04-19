// Auto-extracted from shipment-detail.html
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
      if (!id || id === 'shipments') { window.location.href = '/dashboard/shipments'; return; }

      var app = document.getElementById('dashboard-app');
      QCS.mountLoading(app, QCS.t('loading_shipments'));

      Promise.all([
        QCS.fetchJson('/api/v1/me'),
        QCS.fetchJson('/api/v1/shipments/' + encodeURIComponent(id))
      ]).then(function (results) {
        var me = results[0];
        if (!me || !me.data) { window.location.href = loginRedirect; return; }
        if (!results[1] || !results[1].data) {
          QCS.mountError(app, {
            title: 'Shipment not found',
            actionLabel: 'Back to shipments',
            onRetry: function () { window.location.href = '/dashboard/shipments'; }
          });
          return;
        }
        renderShell(results[1].data);
        QCS.bindLogout();
      }).catch(function (err) {
        QCS.mountError(app, {
          title: 'Unable to load shipment',
          description: (err && err.message) || 'Network error.',
          actionLabel: 'Sign in again',
          onRetry: function () { window.location.href = loginRedirect; }
        });
      });

      function renderShell(s) {
        var sidebar = QCS.renderSidebar('shipments');
        var trk = s.tracking_number || s.id || '—';
        var html = '<div class="qcs-page-wrap">' + sidebar
          + '<main id="qcs-main" class="qcs-page-main" tabindex="-1">'
          + '<nav aria-label="Breadcrumb" class="mb-4"><ol class="flex items-center gap-2 text-sm text-slate-600">'
          + '<li><a href="/dashboard" class="text-[#2563EB] hover:underline">Dashboard</a></li>'
          + '<li aria-hidden="true">/</li>'
          + '<li><a href="/dashboard/shipments" class="text-[#2563EB] hover:underline">Shipments</a></li>'
          + '<li aria-hidden="true">/</li>'
          + '<li class="text-[#0F172A] font-medium" aria-current="page">' + window.qcsEscapeHTML(trk) + '</li></ol></nav>'
          + '<div class="flex flex-wrap items-end justify-between gap-3 mb-6">'
          + '<div>'
          + '<h1 class="text-3xl font-bold text-[#0F172A]">Shipment</h1>'
          + '<p class="text-slate-500 mt-1 font-mono">' + window.qcsEscapeHTML(trk) + '</p>'
          + '</div>'
          + QCS.statusBadge(s.status || '—')
          + '</div>'
          + '<div class="grid lg:grid-cols-3 gap-6">'
          + '<section class="lg:col-span-2 bg-white rounded-xl border border-slate-200 shadow-sm p-6 space-y-3">'
          + '<h2 class="text-lg font-semibold mb-2">Details</h2>'
          + detailRow('Tracking', trk)
          + detailRow('Status', s.status || '—')
          + detailRow('Destination', s.destination_id || '—')
          + (s.carrier ? detailRow('Carrier', s.carrier) : '')
          + (s.estimated_delivery ? detailRow('Estimated delivery', QCS.formatDate(s.estimated_delivery)) : '')
          + (s.actual_delivery ? detailRow('Delivered', QCS.formatDate(s.actual_delivery, { dateStyle: 'medium', timeStyle: 'short' })) : '')
          + detailRow('Created', s.created_at ? QCS.formatDate(s.created_at, { dateStyle: 'medium', timeStyle: 'short' }) : '—')
          + '</section>'
          + '<aside class="bg-white rounded-xl border border-slate-200 shadow-sm p-6 space-y-3">'
          + '<h2 class="text-lg font-semibold">Actions</h2>'
          + '<button type="button" id="copy-trk" class="block w-full text-center bg-[#2563EB] text-white px-4 py-2 rounded-lg font-semibold shadow-sm hover:opacity-95">Copy tracking</button>'
          + '<a href="/track" class="block text-center bg-white border border-slate-300 px-4 py-2 rounded-lg font-medium hover:bg-slate-50">Public tracking</a>'
          + '<a href="/dashboard/shipments" class="block text-center bg-white border border-slate-300 px-4 py-2 rounded-lg font-medium hover:bg-slate-50">Back</a>'
          + '</aside>'
          + '</div></main></div>';
        app.innerHTML = html;
        var main = document.getElementById('qcs-main');
        if (main) main.focus({ preventScroll: true });

        var copy = document.getElementById('copy-trk');
        if (copy) copy.addEventListener('click', function () { QCS.copyToClipboard(trk); });
      }

      function detailRow(label, value) {
        return '<div class="flex justify-between gap-4 border-b border-slate-100 pb-2 last:border-0">'
          + '<span class="text-slate-500 text-sm">' + window.qcsEscapeHTML(label) + '</span>'
          + '<span class="font-medium text-right">' + window.qcsEscapeHTML(String(value)) + '</span>'
          + '</div>';
      }
    })();
  