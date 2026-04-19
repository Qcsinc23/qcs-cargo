// Auto-extracted from storage-report.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    (function() {
      const token = localStorage.getItem('qcs_access_token');
      if (!token) { window.location.href = '/login?redirect=' + encodeURIComponent('/admin/storage-report'); return; }
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
          '<a href="/admin/storage-report" class="block py-2 px-3 rounded-lg ' + (active === 'storage-report' ? 'bg-[#1E293B]' : 'hover:bg-[#1E293B]') + '">Storage Report</a>' +
          '<a href="/admin/reports" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Reports</a>' +
          '<a href="/admin/settings" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Settings</a>' +
          '</nav><a href="/dashboard" class="block mt-6 text-sm text-slate-400 hover:text-white">Customer Dashboard</a>' +
          '<button type="button" id="logout-btn" class="mt-4 text-sm text-slate-400 hover:text-white">Sign out</button></aside>';
      }
      fetch('/api/v1/me', auth).then(r => { if (!r.ok) { window.location.href = '/login'; return null; } return r.json(); }).then(me => {
        if (!me || me.data.role !== 'admin') { window.location.href = '/dashboard'; return; }
        fetch('/api/v1/admin/storage-report', auth).then(r => r.ok ? r.json() : null).then(res => {
          const d = (res && res.data) || {};
          const main = '<main class="flex-1 p-8">' +
            '<nav class="mb-4 text-sm text-slate-600"><a href="/admin" class="text-[#2563EB] hover:underline">Admin</a> / Storage Report</nav>' +
            '<h1 class="text-3xl font-bold mb-6">Storage Report</h1>' +
            '<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">' +
            '<div class="bg-white rounded-xl border border-slate-200 p-6"><p class="text-slate-500 text-sm">Packages stored</p><p class="text-2xl font-bold text-[#0F172A]">' + (d.total_packages_stored ?? '—') + '</p></div>' +
            '<div class="bg-white rounded-xl border border-slate-200 p-6"><p class="text-slate-500 text-sm">Total weight (lbs)</p><p class="text-2xl font-bold text-[#0F172A]">' + (d.total_weight ?? '—') + '</p></div>' +
            '<div class="bg-white rounded-xl border border-slate-200 p-6"><p class="text-slate-500 text-sm">Utilization</p><p class="text-2xl font-bold text-[#0F172A]">' + (d.utilization_pct != null ? d.utilization_pct + '%' : '—') + '</p></div>' +
            '<div class="bg-white rounded-xl border border-slate-200 p-6"><p class="text-slate-500 text-sm">Expiring in 5 days</p><p class="text-2xl font-bold text-[#0F172A]">' + (d.packages_expiring_soon ?? '—') + '</p></div>' +
            '</div>' +
            '<div class="mt-6 bg-white rounded-xl border border-slate-200 p-6"><p class="text-slate-500 text-sm">Storage fees collected today</p><p class="text-2xl font-bold text-[#0F172A]">$' + (d.storage_fees_collected_today != null ? Number(d.storage_fees_collected_today).toFixed(2) : '—') + '</p></div>' +
            '</main></div>';
          document.getElementById('app').innerHTML = adminSidebar('storage-report') + main;
          document.getElementById('logout-btn').onclick = () => { fetch('/api/v1/auth/logout', { method: 'POST', credentials: 'include' }).then(() => { localStorage.removeItem('qcs_access_token'); window.location.href = '/'; }); };
        }).catch(() => {
          document.getElementById('app').innerHTML = adminSidebar('storage-report') + '<main class="flex-1 p-8"><p class="text-red-600">Failed to load storage report.</p></main></div>';
        });
      });
    })();
  