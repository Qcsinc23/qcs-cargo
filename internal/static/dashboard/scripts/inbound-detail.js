// Auto-extracted from inbound-detail.html
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
      if (!id || id === 'inbound') { window.location.href = '/dashboard/inbound'; return; }

      var app = document.getElementById('dashboard-app');
      QCS.mountLoading(app, QCS.t('loading'));

      Promise.all([
        QCS.fetchJson('/api/v1/me'),
        QCS.fetchJson('/api/v1/inbound-tracking/' + encodeURIComponent(id))
      ]).then(function (results) {
        var me = results[0];
        if (!me || !me.data) { window.location.href = loginRedirect; return; }
        if (!results[1] || !results[1].data) {
          QCS.mountError(app, {
            title: 'Tracking not found',
            description: 'We could not find that inbound tracking entry.',
            actionLabel: 'Back to Expected Packages',
            onRetry: function () { window.location.href = '/dashboard/inbound'; }
          });
          return;
        }
        renderShell(results[1].data);
        QCS.bindLogout();
        bindActions();
      }).catch(function (err) {
        QCS.mountError(app, {
          title: 'Unable to load tracking',
          description: (err && err.message) || 'Network error.',
          actionLabel: 'Sign in again',
          onRetry: function () { window.location.href = loginRedirect; }
        });
      });

      function val(r, key) {
        var v = r[key];
        if (v && typeof v === 'object' && 'String' in v) return v.Valid ? v.String : '';
        return (typeof v === 'string' || typeof v === 'number') ? String(v) : '';
      }

      function renderShell(t) {
        var carrier = val(t, 'carrier') || '—';
        var trackingNumber = val(t, 'tracking_number') || '—';
        var status = val(t, 'status') || 'pending';
        var lastChecked = val(t, 'last_checked_at');
        var lockerPkgId = val(t, 'locker_package_id');
        var retailer = val(t, 'retailer_name');
        var expectedItems = val(t, 'expected_items');
        var title = retailer || carrier;

        var sidebar = QCS.renderSidebar('inbound');
        var html = '<div class="qcs-page-wrap">' + sidebar
          + '<main id="qcs-main" class="qcs-page-main" tabindex="-1">'
          + '<nav aria-label="Breadcrumb" class="mb-4"><ol class="flex items-center gap-2 text-sm text-slate-600">'
          + '<li><a href="/dashboard" class="text-[#2563EB] hover:underline">Dashboard</a></li>'
          + '<li aria-hidden="true">/</li>'
          + '<li><a href="/dashboard/inbound" class="text-[#2563EB] hover:underline">Expected Packages</a></li>'
          + '<li aria-hidden="true">/</li>'
          + '<li class="text-[#0F172A] font-medium" aria-current="page">Detail</li></ol></nav>'
          + '<div class="flex flex-wrap items-end justify-between gap-3 mb-6">'
          + '<div>'
          + '<h1 class="text-3xl font-bold text-[#0F172A]">' + window.qcsEscapeHTML(title) + '</h1>'
          + '<p class="text-slate-500 mt-1 font-mono">' + window.qcsEscapeHTML(trackingNumber) + '</p>'
          + '</div>'
          + QCS.statusBadge(status)
          + '</div>'
          + '<div class="bg-white rounded-xl border border-slate-200 shadow-sm p-6 mb-6 space-y-3">'
          + detailRow('Carrier', carrier)
          + detailRow('Tracking number', trackingNumber)
          + detailRow('Status', status)
          + (lastChecked ? detailRow('Last checked', QCS.formatDate(lastChecked, { dateStyle: 'medium', timeStyle: 'short' })) : '')
          + (expectedItems ? detailRow('Expected items', expectedItems) : '')
          + (lockerPkgId
              ? '<div class="flex justify-between gap-4"><span class="text-slate-500 text-sm">Linked package</span>'
                + '<a href="/dashboard/inbox/' + encodeURIComponent(lockerPkgId) + '" class="text-[#2563EB] hover:underline truncate font-medium">' + window.qcsEscapeHTML(lockerPkgId) + '</a></div>'
              : '')
          + '</div>'
          + '<div class="flex flex-wrap items-center gap-3">'
          + '<button type="button" id="delete-btn" class="bg-red-600 text-white px-6 py-2 rounded-lg font-semibold shadow-sm hover:bg-red-700">Remove tracking</button>'
          + '<a href="/dashboard/inbound" class="bg-white border border-slate-300 px-4 py-2 rounded-lg font-medium hover:bg-slate-50">Back</a>'
          + '<p id="msg" class="text-sm hidden" aria-live="polite"></p>'
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

      function bindActions() {
        var btn = document.getElementById('delete-btn');
        if (!btn) return;
        btn.addEventListener('click', function () {
          if (!confirm('Remove this tracking?')) return;
          btn.disabled = true;
          QCS.fetchJson('/api/v1/inbound-tracking/' + encodeURIComponent(id), { method: 'DELETE' })
            .then(function () {
              QCS.toast('Tracking removed', 'success');
              window.location.href = '/dashboard/inbound';
            })
            .catch(function (err) {
              btn.disabled = false;
              var msgEl = document.getElementById('msg');
              if (msgEl) {
                msgEl.textContent = (err && err.message) || 'Delete failed.';
                msgEl.className = 'text-sm text-red-600';
                msgEl.classList.remove('hidden');
              }
            });
        });
      }
    })();
  