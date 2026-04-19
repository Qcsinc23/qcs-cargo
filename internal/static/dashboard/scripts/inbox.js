// Auto-extracted from inbox.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    (function () {
      'use strict';
      var QCS = window.QCSPWA;
      QCS.initBase({ registerSW: true, keyboard: true, utilityDock: true });

      var loginRedirect = '/login?redirect=' + encodeURIComponent('/dashboard/inbox');
      var token = QCS.readToken();
      if (!token) { window.location.href = loginRedirect; return; }

      var app = document.getElementById('dashboard-app');
      QCS.mountLoading(app, QCS.t('loading_packages'));

      var inboxState = { user: null, list: [], sum: {}, filter: '', sortBy: 'date', selectedIds: [] };

      Promise.all([
        QCS.fetchJson('/api/v1/me'),
        QCS.fetchJson('/api/v1/locker').catch(function () { return { data: [] }; }),
        QCS.fetchJson('/api/v1/locker/summary').catch(function () { return { data: {} }; })
      ]).then(function (results) {
        var me = results[0];
        if (!me || !me.data) { window.location.href = loginRedirect; return; }
        inboxState.user = me.data;
        inboxState.list = (results[1] && results[1].data) || [];
        inboxState.sum = (results[2] && results[2].data) || {};
        renderShell();
        QCS.bindLogout();
      }).catch(function (err) {
        QCS.mountError(app, {
          title: 'Unable to load packages',
          description: (err && err.message) || 'Network error.',
          actionLabel: 'Sign in again',
          onRetry: function () { window.location.href = loginRedirect; }
        });
      });

      function daysLeft(expiresAt) {
        if (!expiresAt) return null;
        return Math.ceil((new Date(expiresAt) - new Date()) / (24 * 60 * 60 * 1000));
      }
      function storageColorByDaysLeft(d) {
        if (d == null) return 'bg-slate-300';
        if (d < 0) return 'bg-red-500';
        if (d <= 20) return 'bg-emerald-500';
        if (d <= 27) return 'bg-amber-500';
        return 'bg-red-500';
      }
      function storageBarPct(d) {
        if (d == null || d < 0) return 100;
        var remaining = Math.min(30, Math.max(0, d));
        return (remaining / 30) * 100;
      }

      function renderShell() {
        var sidebar = QCS.renderSidebar('inbox');
        var html = '<div class="qcs-page-wrap">' + sidebar
          + '<main id="qcs-main" class="qcs-page-main" tabindex="-1">'
          + '<nav aria-label="Breadcrumb" class="mb-4"><ol class="flex items-center gap-2 text-sm text-slate-600">'
          + '<li><a href="/dashboard" class="text-[#2563EB] hover:underline">Dashboard</a></li>'
          + '<li aria-hidden="true">/</li>'
          + '<li class="text-[#0F172A] font-medium" aria-current="page">My Packages</li></ol></nav>'
          + '<div id="inbox-banner"></div>'
          + '<div class="flex flex-wrap items-end justify-between gap-3 mb-6">'
          + '<div>'
          + '<h1 class="text-3xl font-bold text-[#0F172A]">My Packages</h1>'
          + '<p id="inbox-count" class="text-slate-600 mt-1"></p>'
          + '</div>'
          + '<div class="flex gap-2 flex-wrap">'
          + '<a href="/dashboard/ship" class="bg-[#F97316] text-white px-5 py-2 rounded-lg font-semibold shadow-sm hover:opacity-95">Ship my packages</a>'
          + '<a href="/dashboard/inbound" class="bg-white border border-slate-300 px-4 py-2 rounded-lg font-medium hover:bg-slate-50">Add inbound tracking</a>'
          + '</div>'
          + '</div>'
          + '<div class="bg-white border border-slate-200 rounded-xl shadow-sm p-4 mb-6 flex flex-wrap items-center gap-3 justify-between">'
          + '<div class="flex flex-wrap gap-2">'
          + filterBtn('', 'All')
          + filterBtn('stored', 'Stored')
          + filterBtn('expiring_soon', 'Expiring soon')
          + '</div>'
          + '<label class="text-sm text-slate-600">Sort: <select id="inbox-sort" class="border border-slate-300 rounded px-2 py-1 ml-1">'
          + '<option value="date">Date arrived</option>'
          + '<option value="sender">Sender</option>'
          + '<option value="expiring">Expiring soon</option>'
          + '</select></label>'
          + '</div>'
          + '<div id="inbox-list" aria-live="polite"></div>'
          + '<div id="ship-selected-bar" class="fixed bottom-0 left-0 right-0 bg-[#0F172A] text-white py-3 px-6 flex items-center justify-between shadow-lg hidden z-40">'
          + '<span id="ship-selected-count">0 selected</span>'
          + '<a href="/dashboard/ship" id="ship-selected-link" class="bg-[#F97316] text-white px-6 py-2 rounded-lg font-semibold">Ship Selected</a>'
          + '</div>'
          + '</main></div>';
        app.innerHTML = html;
        var main = document.getElementById('qcs-main');
        if (main) main.focus({ preventScroll: true });

        var sortEl = document.getElementById('inbox-sort');
        if (sortEl) {
          sortEl.value = inboxState.sortBy;
          sortEl.addEventListener('change', function () { inboxState.sortBy = sortEl.value; updateList(); });
        }
        document.querySelectorAll('.inbox-filter-btn').forEach(function (btn) {
          btn.addEventListener('click', function () { inboxState.filter = btn.getAttribute('data-filter'); updateList(); });
        });

        renderBanner();
        updateList();
      }

      function filterBtn(value, label) {
        var active = inboxState.filter === value;
        var cls = active
          ? 'bg-[#2563EB] text-white border-[#2563EB]'
          : 'bg-white text-slate-700 border-slate-300 hover:bg-slate-50';
        return '<button type="button" class="inbox-filter-btn px-4 py-2 rounded-lg border font-medium ' + cls + '" data-filter="' + window.qcsEscapeHTML(value) + '">' + window.qcsEscapeHTML(label) + '</button>';
      }

      function renderBanner() {
        var banner = document.getElementById('inbox-banner');
        if (!banner) return;
        var stored = inboxState.list.filter(function (p) { return (p.status || '') === 'stored'; }).length;
        var nextExpiry = inboxState.sum && inboxState.sum.next_expiry;
        var daysUntil = nextExpiry ? Math.ceil((new Date(nextExpiry) - new Date()) / (24 * 60 * 60 * 1000)) : null;
        var html = '';
        if (stored > 0 && daysUntil != null && daysUntil < 0) {
          html = '<div class="border rounded-xl p-4 mb-6 bg-red-50 border-red-200 text-red-900 flex flex-wrap items-center justify-between gap-3">'
            + '<div><p class="font-semibold">Free storage has expired</p><p class="text-sm opacity-90">Ship or dispose stored packages now to avoid daily fees.</p></div>'
            + '<a href="/dashboard/ship" class="font-medium underline whitespace-nowrap">Ship my packages →</a>'
            + '</div>';
        } else if (stored > 0 && daysUntil != null && daysUntil <= 5) {
          html = '<div class="border rounded-xl p-4 mb-6 bg-amber-50 border-amber-200 text-amber-900 flex flex-wrap items-center justify-between gap-3">'
            + '<div><p class="font-semibold">Free storage ends in ' + daysUntil + ' day' + (daysUntil === 1 ? '' : 's') + '</p><p class="text-sm opacity-90">Schedule shipping or pickup before ' + window.qcsEscapeHTML(QCS.formatDate(nextExpiry)) + '.</p></div>'
            + '<a href="/dashboard/ship" class="font-medium underline whitespace-nowrap">Ship now →</a>'
            + '</div>';
        }
        banner.innerHTML = html;
      }

      function updateList() {
        var countEl = document.getElementById('inbox-count');
        if (countEl) countEl.textContent = inboxState.list.length + ' packages in your locker';

        var sorted = inboxState.list.slice().sort(function (a, b) {
          if (inboxState.sortBy === 'sender') {
            var sa = (typeof a.sender_name === 'string' ? a.sender_name : (a.sender_name && a.sender_name.String)) || '';
            var sb = (typeof b.sender_name === 'string' ? b.sender_name : (b.sender_name && b.sender_name.String)) || '';
            return sa.localeCompare(sb);
          }
          if (inboxState.sortBy === 'expiring') {
            var da = daysLeft(a.free_storage_expires_at || a.FreeStorageExpiresAt);
            var db = daysLeft(b.free_storage_expires_at || b.FreeStorageExpiresAt);
            if (da == null && db == null) return 0;
            if (da == null) return 1;
            if (db == null) return -1;
            return da - db;
          }
          return (b.arrived_at || '').localeCompare(a.arrived_at || '');
        });

        var filtered = sorted;
        if (inboxState.filter === 'stored') {
          filtered = sorted.filter(function (p) { return (p.status || '') === 'stored'; });
        } else if (inboxState.filter === 'expiring_soon') {
          filtered = sorted.filter(function (p) {
            var d = daysLeft(p.free_storage_expires_at || p.FreeStorageExpiresAt);
            return (p.status || '') === 'stored' && d != null && d >= 0 && d <= 7;
          });
        }

        var container = document.getElementById('inbox-list');
        if (!container) return;
        if (filtered.length === 0) {
          container.innerHTML = QCS.renderEmptyState({
            title: 'No packages match',
            description: 'Use your QCS suite at checkout, or pre-alert a package to start receiving.',
            actionHref: '/dashboard/mailbox',
            actionLabel: 'Open mailbox'
          });
          updateShipSelectedBar();
          return;
        }

        var html = '<div class="grid grid-cols-1 md:grid-cols-2 gap-4">';
        filtered.forEach(function (p) {
          var sender = typeof p.sender_name === 'string' ? p.sender_name : (p.sender_name && p.sender_name.String) || 'Unknown';
          var weight = typeof p.weight_lbs === 'number' ? p.weight_lbs : (p.weight_lbs && p.weight_lbs.Float64) || 0;
          var status = p.status || 'stored';
          var checked = inboxState.selectedIds.indexOf(p.id) >= 0 ? ' checked' : '';
          var expiry = p.free_storage_expires_at || p.FreeStorageExpiresAt;
          var d = daysLeft(expiry);
          var barColor = (status === 'stored' && d != null) ? storageColorByDaysLeft(d) : 'bg-slate-300';
          var barPct = (status === 'stored' && d != null) ? storageBarPct(d) : 100;
          var subline = weight + ' lbs';
          if (d != null && status === 'stored') {
            subline += ' · ' + (d < 0 ? 'Expired' : (d + ' days left'));
          }
          var bar = (status === 'stored')
            ? '<div class="mt-2 h-1.5 bg-slate-200 rounded-full overflow-hidden"><div class="h-full rounded-full ' + barColor + '" style="width:' + barPct + '%"></div></div>'
            : '';
          html += '<article class="bg-white rounded-xl border border-slate-200 shadow-sm p-4 flex items-start gap-3 hover:shadow-md transition-shadow">'
            + '<input type="checkbox" class="inbox-cb mt-1" data-id="' + window.qcsEscapeHTML(p.id) + '"' + checked + '>'
            + '<a href="/dashboard/inbox/' + encodeURIComponent(p.id) + '" class="flex-1 min-w-0">'
            + '<div class="flex items-baseline justify-between gap-2 mb-1">'
            + '<p class="font-semibold text-[#0F172A] truncate">' + window.qcsEscapeHTML(sender) + '</p>'
            + QCS.statusBadge(status)
            + '</div>'
            + '<p class="text-sm text-slate-500">' + window.qcsEscapeHTML(subline) + '</p>'
            + bar
            + '</a></article>';
        });
        html += '</div>';
        container.innerHTML = html;

        document.querySelectorAll('.inbox-cb').forEach(function (cb) {
          cb.addEventListener('change', function () {
            var id = cb.getAttribute('data-id');
            if (cb.checked) {
              if (inboxState.selectedIds.indexOf(id) < 0) inboxState.selectedIds.push(id);
            } else {
              inboxState.selectedIds = inboxState.selectedIds.filter(function (x) { return x !== id; });
            }
            updateShipSelectedBar();
          });
        });

        updateShipSelectedBar();
      }

      function updateShipSelectedBar() {
        var bar = document.getElementById('ship-selected-bar');
        var link = document.getElementById('ship-selected-link');
        var countEl = document.getElementById('ship-selected-count');
        if (!bar) return;
        var n = inboxState.selectedIds.length;
        if (n > 0) {
          bar.classList.remove('hidden');
          if (countEl) countEl.textContent = n + ' selected';
          if (link) link.href = '/dashboard/ship?packages=' + inboxState.selectedIds.map(encodeURIComponent).join(',');
        } else {
          bar.classList.add('hidden');
        }
      }
    })();
  