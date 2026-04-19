// Auto-extracted from inbox-detail.html
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
      if (!id || id === 'inbox') { window.location.href = '/dashboard/inbox'; return; }

      var SERVICE_TYPES = [
        { id: 'photo_detail', label: 'Detailed Photos', price: '$3' },
        { id: 'content_inspection', label: 'Content Inspection', price: '$5' },
        { id: 'repackage', label: 'Repackaging', price: '$5' },
        { id: 'remove_invoice', label: 'Remove Invoices', price: '$2' },
        { id: 'fragile_wrap', label: 'Fragile Wrap', price: '$3' },
        { id: 'gift_wrap', label: 'Gift Wrap', price: '$5' }
      ];

      var app = document.getElementById('dashboard-app');
      QCS.mountLoading(app, QCS.t('loading_packages'));

      var pkgState = null;

      Promise.all([
        QCS.fetchJson('/api/v1/me'),
        QCS.fetchJson('/api/v1/locker/' + encodeURIComponent(id)),
        QCS.fetchJson('/api/v1/locker/' + encodeURIComponent(id) + '/service-requests').catch(function () { return { data: [] }; })
      ]).then(function (results) {
        var me = results[0];
        if (!me || !me.data) { window.location.href = loginRedirect; return; }
        if (!results[1] || !results[1].data) {
          QCS.mountError(app, {
            title: 'Package not found',
            description: 'We could not find that package in your locker.',
            actionLabel: 'Back to inbox',
            onRetry: function () { window.location.href = '/dashboard/inbox'; }
          });
          return;
        }
        pkgState = { pkg: results[1].data, services: (results[2] && results[2].data) || [] };
        renderShell();
        QCS.bindLogout();
      }).catch(function (err) {
        QCS.mountError(app, {
          title: 'Unable to load package',
          description: (err && err.message) || 'Network error.',
          actionLabel: 'Sign in again',
          onRetry: function () { window.location.href = loginRedirect; }
        });
      });

      function statusByType(list) {
        var map = {};
        list.forEach(function (sr) {
          var t = sr.service_type || sr.ServiceType;
          map[t] = (sr.status || sr.Status) === 'completed' ? 'Completed' : 'Pending';
        });
        return map;
      }

      function servicesHTML() {
        var map = statusByType(pkgState.services);
        var html = '<div class="grid grid-cols-1 sm:grid-cols-2 gap-3">';
        SERVICE_TYPES.forEach(function (svc) {
          var status = map[svc.id] || (svc.id === 'photo_detail' ? map['photo'] : null);
          var label = window.qcsEscapeHTML(svc.label);
          var price = window.qcsEscapeHTML(svc.price);
          if (status) {
            var pillCls = status === 'Completed' ? 'qcs-badge-success' : 'qcs-badge-warning';
            html += '<div class="flex items-center justify-between rounded-lg border border-slate-200 bg-slate-50 px-4 py-3">'
              + '<span class="font-medium">' + label + ' <span class="text-slate-500 font-normal">' + price + '</span></span>'
              + '<span class="qcs-badge ' + pillCls + '">' + window.qcsEscapeHTML(status) + '</span></div>';
          } else {
            html += '<div class="flex items-center justify-between rounded-lg border border-slate-200 px-4 py-3">'
              + '<span class="font-medium">' + label + ' <span class="text-slate-500 font-normal">' + price + '</span></span>'
              + '<button type="button" class="service-req-btn text-sm bg-[#2563EB] text-white px-3 py-1.5 rounded-lg font-semibold hover:opacity-95" data-type="' + window.qcsEscapeHTML(svc.id) + '">Request</button></div>';
          }
        });
        html += '</div>';
        return html;
      }

      function renderShell() {
        var p = pkgState.pkg;
        var sender = typeof p.sender_name === 'string' ? p.sender_name : (p.sender_name && p.sender_name.String) || '—';
        var weight = typeof p.weight_lbs === 'number' ? p.weight_lbs : (p.weight_lbs && p.weight_lbs.Float64) || '—';
        var arrived = p.arrived_at ? QCS.formatDate(p.arrived_at, { dateStyle: 'medium', timeStyle: 'short' }) : '—';
        var sidebar = QCS.renderSidebar('inbox');
        var html = '<div class="qcs-page-wrap">' + sidebar
          + '<main id="qcs-main" class="qcs-page-main" tabindex="-1">'
          + '<nav aria-label="Breadcrumb" class="mb-4"><ol class="flex items-center gap-2 text-sm text-slate-600">'
          + '<li><a href="/dashboard" class="text-[#2563EB] hover:underline">Dashboard</a></li>'
          + '<li aria-hidden="true">/</li>'
          + '<li><a href="/dashboard/inbox" class="text-[#2563EB] hover:underline">My Packages</a></li>'
          + '<li aria-hidden="true">/</li>'
          + '<li class="text-[#0F172A] font-medium" aria-current="page">Package</li></ol></nav>'
          + '<div class="flex flex-wrap items-end justify-between gap-3 mb-6">'
          + '<div>'
          + '<h1 class="text-3xl font-bold text-[#0F172A]">' + window.qcsEscapeHTML(sender) + '</h1>'
          + '<p class="text-slate-500 mt-1">Arrived ' + window.qcsEscapeHTML(arrived) + '</p>'
          + '</div>'
          + QCS.statusBadge(p.status || 'stored')
          + '</div>'
          + '<div class="grid lg:grid-cols-3 gap-6 mb-6">'
          + '<section class="lg:col-span-2 bg-white rounded-xl border border-slate-200 shadow-sm p-6 space-y-3">'
          + '<h2 class="text-lg font-semibold mb-2">Package details</h2>'
          + detailRow('Weight', weight + ' lbs')
          + detailRow('Condition', p.condition || '—')
          + detailRow('Tracking', p.tracking_number || '—')
          + '</section>'
          + '<aside class="bg-white rounded-xl border border-slate-200 shadow-sm p-6 space-y-3">'
          + '<h2 class="text-lg font-semibold">Actions</h2>'
          + '<a href="/dashboard/ship?packages=' + encodeURIComponent(p.id) + '" class="block text-center bg-[#F97316] text-white px-4 py-2 rounded-lg font-semibold shadow-sm hover:opacity-95">Ship this package</a>'
          + '<a href="/dashboard/inbox" class="block text-center bg-white border border-slate-300 px-4 py-2 rounded-lg font-medium hover:bg-slate-50">Back to inbox</a>'
          + '</aside>'
          + '</div>'
          + '<section class="bg-white rounded-xl border border-slate-200 shadow-sm p-6">'
          + '<div class="flex items-baseline justify-between mb-1">'
          + '<h2 class="text-lg font-semibold">Value-added services</h2>'
          + '<span class="text-xs text-slate-500">Fees apply</span>'
          + '</div>'
          + '<p class="text-sm text-slate-600 mb-4">Request photos, inspection, repackaging, and more.</p>'
          + '<div id="services-card">' + servicesHTML() + '</div>'
          + '<p id="request-msg" class="mt-3 text-sm hidden" aria-live="polite"></p>'
          + '</section>'
          + '</main></div>';
        app.innerHTML = html;
        var main = document.getElementById('qcs-main');
        if (main) main.focus({ preventScroll: true });
        bindServiceButtons();
      }

      function detailRow(label, value) {
        return '<div class="flex justify-between gap-4 border-b border-slate-100 pb-2">'
          + '<span class="text-slate-500 text-sm">' + window.qcsEscapeHTML(label) + '</span>'
          + '<span class="font-medium text-right">' + window.qcsEscapeHTML(String(value)) + '</span>'
          + '</div>';
      }

      function showMsg(text, isErr) {
        var el = document.getElementById('request-msg');
        if (!el) return;
        el.textContent = text;
        el.className = 'mt-3 text-sm ' + (isErr ? 'text-red-600' : 'text-emerald-600');
        el.classList.remove('hidden');
      }

      function refreshServices() {
        QCS.fetchJson('/api/v1/locker/' + encodeURIComponent(pkgState.pkg.id) + '/service-requests')
          .then(function (d) {
            pkgState.services = (d && d.data) || [];
            var card = document.getElementById('services-card');
            if (card) { card.innerHTML = servicesHTML(); bindServiceButtons(); }
          })
          .catch(function () {});
      }

      function bindServiceButtons() {
        document.querySelectorAll('.service-req-btn').forEach(function (btn) {
          btn.addEventListener('click', function () {
            var type = btn.getAttribute('data-type');
            btn.disabled = true;
            QCS.fetchJson('/api/v1/locker/' + encodeURIComponent(pkgState.pkg.id) + '/service-request', {
              method: 'POST',
              body: { service_type: type, notes: '' }
            }).then(function () {
              showMsg('Request submitted.');
              QCS.toast('Service request submitted', 'success');
              refreshServices();
            }).catch(function (err) {
              showMsg((err && err.message) || 'Request failed.', true);
              btn.disabled = false;
            });
          });
        });
      }
    })();
  