// Auto-extracted from invoice-detail.html
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
      if (!id || id === 'invoices') { window.location.href = '/dashboard/invoices'; return; }

      var app = document.getElementById('dashboard-app');
      QCS.mountLoading(app, QCS.t('loading'));

      Promise.all([
        QCS.fetchJson('/api/v1/me'),
        QCS.fetchJson('/api/v1/invoices/' + encodeURIComponent(id))
      ]).then(function (results) {
        var me = results[0];
        if (!me || !me.data) { window.location.href = loginRedirect; return; }
        if (!results[1] || !results[1].data) {
          QCS.mountError(app, {
            title: 'Invoice not found',
            actionLabel: 'Back to invoices',
            onRetry: function () { window.location.href = '/dashboard/invoices'; }
          });
          return;
        }
        renderShell(results[1].data);
        QCS.bindLogout();
      }).catch(function (err) {
        QCS.mountError(app, {
          title: 'Unable to load invoice',
          description: (err && err.message) || 'Network error.',
          actionLabel: 'Sign in again',
          onRetry: function () { window.location.href = loginRedirect; }
        });
      });

      function renderShell(payload) {
        var inv = payload.invoice || payload;
        var items = payload.items || [];
        var sidebar = QCS.renderSidebar('invoices');
        var code = inv.invoice_number || (inv.id ? String(inv.id).slice(0, 8) : '—');
        var subtotal = QCS.formatMoney(inv.subtotal != null ? Number(inv.subtotal) : 0, 'USD');
        var tax = QCS.formatMoney(inv.tax != null ? Number(inv.tax) : 0, 'USD');
        var total = QCS.formatMoney(inv.total != null ? Number(inv.total) : 0, 'USD');

        var html = '<div class="qcs-page-wrap">' + sidebar
          + '<main id="qcs-main" class="qcs-page-main" tabindex="-1">'
          + '<nav aria-label="Breadcrumb" class="mb-4"><ol class="flex items-center gap-2 text-sm text-slate-600">'
          + '<li><a href="/dashboard" class="text-[#2563EB] hover:underline">Dashboard</a></li>'
          + '<li aria-hidden="true">/</li>'
          + '<li><a href="/dashboard/invoices" class="text-[#2563EB] hover:underline">Invoices</a></li>'
          + '<li aria-hidden="true">/</li>'
          + '<li class="text-[#0F172A] font-medium" aria-current="page">' + window.qcsEscapeHTML(code) + '</li></ol></nav>'
          + '<div class="flex flex-wrap items-end justify-between gap-3 mb-6">'
          + '<div>'
          + '<h1 class="text-3xl font-bold text-[#0F172A]">Invoice ' + window.qcsEscapeHTML(code) + '</h1>'
          + '<p class="text-slate-500 mt-1">' + window.qcsEscapeHTML(inv.created_at ? QCS.formatDate(inv.created_at) : '—') + '</p>'
          + '</div>'
          + '<div class="flex items-center gap-2">' + QCS.statusBadge(inv.status || '—')
          + '<span class="text-2xl font-bold ml-3">' + window.qcsEscapeHTML(total) + '</span></div>'
          + '</div>'
          + '<div class="grid lg:grid-cols-3 gap-6 mb-6">'
          + '<section class="lg:col-span-2 bg-white rounded-xl border border-slate-200 shadow-sm p-6">'
          + '<h2 class="text-lg font-semibold mb-3">Line items</h2>'
          + (items.length === 0
              ? '<p class="text-slate-500 text-sm">No line items.</p>'
              : '<ul class="divide-y divide-slate-100">'
                + items.map(function (it) {
                    var lineTotal = QCS.formatMoney(it.total != null ? Number(it.total) : 0, 'USD');
                    return '<li class="py-3 flex items-baseline justify-between gap-3">'
                      + '<span>' + window.qcsEscapeHTML(it.description || '—') + '</span>'
                      + '<span class="font-medium">' + window.qcsEscapeHTML(lineTotal) + '</span>'
                      + '</li>';
                  }).join('')
                + '</ul>')
          + '</section>'
          + '<aside class="bg-white rounded-xl border border-slate-200 shadow-sm p-6 space-y-3">'
          + '<h2 class="text-lg font-semibold mb-2">Summary</h2>'
          + summaryRow('Subtotal', subtotal)
          + summaryRow('Tax', tax)
          + '<div class="flex justify-between text-base font-bold pt-3 border-t border-slate-200">'
          + '<span>Total</span><span>' + window.qcsEscapeHTML(total) + '</span></div>'
          + '<p class="text-xs text-slate-500 pt-2">PDF download coming soon.</p>'
          + '</aside>'
          + '</div>'
          + '</main></div>';
        app.innerHTML = html;
        var main = document.getElementById('qcs-main');
        if (main) main.focus({ preventScroll: true });
      }

      function summaryRow(label, value) {
        return '<div class="flex justify-between text-sm">'
          + '<span class="text-slate-500">' + window.qcsEscapeHTML(label) + '</span>'
          + '<span class="font-medium">' + window.qcsEscapeHTML(value) + '</span>'
          + '</div>';
      }
    })();
  