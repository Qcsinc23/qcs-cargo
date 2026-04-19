// Auto-extracted from booking-detail.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    (function () {
      'use strict';
      var QCS = window.QCSPWA;
      QCS.initBase({ registerSW: true, keyboard: true, utilityDock: true });

      var loginRedirect = '/login?redirect=' + encodeURIComponent(window.location.pathname);
      var token = QCS.readToken();
      if (!token) { window.location.href = loginRedirect; return; }

      var id = window.location.pathname.split('/').pop();
      if (!id || id === 'bookings') { window.location.href = '/dashboard/bookings'; return; }

      var app = document.getElementById('dashboard-app');
      QCS.mountLoading(app, QCS.t('loading'));

      Promise.all([
        QCS.fetchJson('/api/v1/me'),
        QCS.fetchJson('/api/v1/bookings/' + encodeURIComponent(id))
      ]).then(function (results) {
        var me = results[0];
        if (!me || !me.data) { window.location.href = loginRedirect; return; }
        var b = results[1] && results[1].data;
        if (!b) {
          QCS.mountError(app, {
            title: 'Booking not found',
            actionLabel: 'Back to bookings',
            onRetry: function () { window.location.href = '/dashboard/bookings'; }
          });
          return;
        }
        renderShell(b);
        QCS.bindLogout();
      }).catch(function (err) {
        QCS.mountError(app, {
          title: 'Unable to load booking',
          description: (err && err.message) || 'Network error.',
          actionLabel: 'Sign in again',
          onRetry: function () { window.location.href = loginRedirect; }
        });
      });

      function strField(v) {
        if (typeof v === 'string') return v;
        if (v && v.String) return v.String;
        return '';
      }

      function renderShell(b) {
        var sidebar = QCS.renderSidebar('bookings');
        var rec = strField(b.recipient_id) || '—';
        var spec = strField(b.special_instructions) || '—';
        var pay = strField(b.payment_status) || '—';
        var code = b.confirmation_code || (b.id ? String(b.id).slice(0, 8) : '—');
        var totalLabel = QCS.formatMoney(b.total != null ? Number(b.total) : 0, 'USD');
        var html = '<div class="qcs-page-wrap">' + sidebar
          + '<main id="qcs-main" class="qcs-page-main" tabindex="-1">'
          + '<nav aria-label="Breadcrumb" class="mb-4"><ol class="flex items-center gap-2 text-sm text-slate-600">'
          + '<li><a href="/dashboard" class="text-[#2563EB] hover:underline">Dashboard</a></li>'
          + '<li aria-hidden="true">/</li>'
          + '<li><a href="/dashboard/bookings" class="text-[#2563EB] hover:underline">Bookings</a></li>'
          + '<li aria-hidden="true">/</li>'
          + '<li class="text-[#0F172A] font-medium" aria-current="page">' + window.qcsEscapeHTML(code) + '</li></ol></nav>'
          + '<div class="flex flex-wrap items-end justify-between gap-3 mb-6">'
          + '<div>'
          + '<h1 class="text-3xl font-bold text-[#0F172A]">Booking ' + window.qcsEscapeHTML(code) + '</h1>'
          + '<p class="text-slate-500 mt-1">' + window.qcsEscapeHTML(b.scheduled_date || '—') + ' · ' + window.qcsEscapeHTML(b.time_slot || '—') + '</p>'
          + '</div>'
          + '<div class="flex items-center gap-2">' + QCS.statusBadge(b.status || 'pending')
          + '<span class="text-xl font-bold ml-2">' + window.qcsEscapeHTML(totalLabel) + '</span></div>'
          + '</div>'
          + '<div class="bg-white rounded-xl border border-slate-200 shadow-sm p-6 max-w-2xl space-y-3">'
          + '<h2 class="text-lg font-semibold mb-2">Details</h2>'
          + detailRow('Status', b.status || '—')
          + detailRow('Scheduled date', b.scheduled_date || '—')
          + detailRow('Time slot', b.time_slot || '—')
          + detailRow('Destination', b.destination_id || '—')
          + detailRow('Recipient', rec)
          + detailRow('Special instructions', spec)
          + detailRow('Total', totalLabel)
          + detailRow('Payment status', pay)
          + '</div>'
          + '<div class="mt-4">'
          + '<a href="/dashboard/bookings" class="text-[#2563EB] hover:underline font-medium">← Back to bookings</a>'
          + '</div>'
          + '</main></div>';
        app.innerHTML = html;
        var main = document.getElementById('qcs-main');
        if (main) main.focus({ preventScroll: true });
      }

      function detailRow(label, value) {
        return '<div class="flex justify-between gap-4 border-b border-slate-100 pb-2 last:border-0">'
          + '<span class="text-slate-500 text-sm">' + window.qcsEscapeHTML(label) + '</span>'
          + '<span class="font-medium text-right">' + window.qcsEscapeHTML(String(value)) + '</span>'
          + '</div>';
      }
    })();
  