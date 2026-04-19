// Auto-extracted from reports.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    (function() {
      const token = localStorage.getItem('qcs_access_token');
      if (!token) { window.location.href = '/login?redirect=' + encodeURIComponent('/admin/reports'); return; }
      const auth = { headers: { 'Authorization': 'Bearer ' + token } };
      function adminSidebar(active) {
        return '<div class="flex min-h-screen"><aside class="w-64 bg-[#0F172A] text-white py-6 px-4">' +
          '<a href="/" class="font-bold text-lg block mb-8">QCS Cargo</a><p class="text-amber-400 text-sm font-medium mb-2">Admin</p>' +
          '<nav class="space-y-1">' +
          '<a href="/admin" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Dashboard</a>' +
          '<a href="/admin/ship-requests" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Ship Requests</a>' +
          '<a href="/admin/locker-packages" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Locker Packages</a>' +
          '<a href="/admin/service-queue" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Service Queue</a>' +
          '<a href="/admin/unmatched" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Unmatched</a>' +
          '<a href="/admin/bookings" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Bookings</a>' +
          '<a href="/admin/users" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Users</a>' +
          '<a href="/admin/storage-report" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Storage Report</a>' +
          '<a href="/admin/reports" class="block py-2 px-3 rounded-lg ' + (active === 'reports' ? 'bg-[#1E293B]' : 'hover:bg-[#1E293B]') + '">Reports</a>' +
          '<a href="/admin/settings" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Settings</a>' +
          '</nav><a href="/dashboard" class="block mt-6 text-sm text-slate-400 hover:text-white">Customer Dashboard</a>' +
          '<button type="button" id="logout-btn" class="mt-4 text-sm text-slate-400 hover:text-white">Sign out</button></aside>';
      }
      fetch('/api/v1/me', auth).then(r => { if (!r.ok) { window.location.href = '/login'; return null; } return r.json(); }).then(me => {
        if (!me || me.data.role !== 'admin') { window.location.href = '/dashboard'; return; }
        const from = document.createElement('input');
        const to = document.createElement('input');
        function loadReports() {
          const fromVal = document.getElementById('report-from') ? document.getElementById('report-from').value : '';
          const toVal = document.getElementById('report-to') ? document.getElementById('report-to').value : '';
          const q = (fromVal ? '&from=' + encodeURIComponent(fromVal) : '') + (toVal ? '&to=' + encodeURIComponent(toVal) : '');
          Promise.all([
            fetch('/api/v1/admin/reports/revenue?' + q.replace(/^&/, ''), auth).then(r => r.ok ? r.json() : null),
            fetch('/api/v1/admin/reports/shipments?' + q.replace(/^&/, ''), auth).then(r => r.ok ? r.json() : null),
            fetch('/api/v1/admin/reports/customers', auth).then(r => r.ok ? r.json() : null)
          ]).then(([revRes, shipRes, custRes]) => {
            const rev = (revRes && revRes.data) || {};
            const ship = (shipRes && shipRes.data) || {};
            const cust = (custRes && custRes.data) || {};
            document.getElementById('report-revenue').textContent = rev.revenue != null ? '$' + Number(rev.revenue).toFixed(2) : '—';
            document.getElementById('report-shipments').textContent = ship.count != null ? ship.count : '—';
            document.getElementById('report-customers').textContent = cust.count != null ? cust.count : '—';
          });
        }
        const main = '<main class="flex-1 p-8">' +
          '<nav class="mb-4 text-sm text-slate-600"><a href="/admin" class="text-[#2563EB] hover:underline">Admin</a> / Reports</nav>' +
          '<h1 class="text-3xl font-bold mb-6">Reports</h1>' +
          '<div class="mb-6 flex flex-wrap gap-4 items-end">' +
          '<div><label class="block text-sm font-medium text-slate-600 mb-1">From (optional)</label><input type="date" id="report-from" class="border border-slate-300 rounded-lg px-3 py-2" /></div>' +
          '<div><label class="block text-sm font-medium text-slate-600 mb-1">To (optional)</label><input type="date" id="report-to" class="border border-slate-300 rounded-lg px-3 py-2" /></div>' +
          '<button type="button" id="report-apply" class="bg-[#2563EB] text-white px-4 py-2 rounded-lg font-medium">Apply</button>' +
          '</div>' +
          '<div class="grid grid-cols-1 md:grid-cols-3 gap-6">' +
          '<div class="bg-white rounded-xl border border-slate-200 p-6"><p class="text-slate-500 text-sm">Revenue (shipped/paid)</p><p id="report-revenue" class="text-2xl font-bold text-[#0F172A]">—</p></div>' +
          '<div class="bg-white rounded-xl border border-slate-200 p-6"><p class="text-slate-500 text-sm">Shipments count</p><p id="report-shipments" class="text-2xl font-bold text-[#0F172A]">—</p></div>' +
          '<div class="bg-white rounded-xl border border-slate-200 p-6"><p class="text-slate-500 text-sm">Total customers</p><p id="report-customers" class="text-2xl font-bold text-[#0F172A]">—</p></div>' +
          '</div><p class="mt-4 text-slate-500 text-sm">Revenue and shipments use optional from/to date range. Leave empty for all time.</p></main></div>';
        document.getElementById('app').innerHTML = adminSidebar('reports') + main;
        document.getElementById('logout-btn').onclick = () => { fetch('/api/v1/auth/logout', { method: 'POST', credentials: 'include' }).then(() => { localStorage.removeItem('qcs_access_token'); window.location.href = '/'; }); };
        document.getElementById('report-apply').onclick = loadReports;
        loadReports();
      });
    })();
  