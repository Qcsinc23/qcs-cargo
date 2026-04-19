// Auto-extracted from settings.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    (function() {
      const token = localStorage.getItem('qcs_access_token');
      if (!token) { window.location.href = '/login?redirect=' + encodeURIComponent('/admin/settings'); return; }
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
          '<a href="/admin/reports" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Reports</a>' +
          '<a href="/admin/settings" class="block py-2 px-3 rounded-lg ' + (active === 'settings' ? 'bg-[#1E293B]' : 'hover:bg-[#1E293B]') + '">Settings</a>' +
          '</nav><a href="/dashboard" class="block mt-6 text-sm text-slate-400 hover:text-white">Customer Dashboard</a>' +
          '<button type="button" id="logout-btn" class="mt-4 text-sm text-slate-400 hover:text-white">Sign out</button></aside>';
      }
      function showComingSoon(btn) {
        const orig = btn.textContent;
        btn.textContent = 'Coming soon';
        btn.disabled = true;
        setTimeout(function() { btn.textContent = orig; btn.disabled = false; }, 2000);
      }
      fetch('/api/v1/me', auth).then(r => { if (!r.ok) { window.location.href = '/login'; return null; } return r.json(); }).then(me => {
        if (!me || me.data.role !== 'admin') { window.location.href = '/dashboard'; return; }
        const main = '<main class="flex-1 p-8">' +
          '<nav class="mb-4 text-sm text-slate-600"><a href="/admin" class="text-[#2563EB] hover:underline">Admin</a> / Settings</nav>' +
          '<h1 class="text-3xl font-bold mb-6">Settings</h1>' +
          '<div class="space-y-8">' +
          '<section class="bg-white rounded-xl border border-slate-200 p-6"><h2 class="text-xl font-semibold text-[#0F172A] mb-4">Pricing</h2><p class="text-slate-600 mb-4">Configure rates per destination and service type.</p><button type="button" class="settings-save bg-[#2563EB] text-white px-4 py-2 rounded-lg font-medium">Save</button></section>' +
          '<section class="bg-white rounded-xl border border-slate-200 p-6"><h2 class="text-xl font-semibold text-[#0F172A] mb-4">Storage config</h2><p class="text-slate-600 mb-4">Free storage days, overage rates, and billing cycle.</p><button type="button" class="settings-save bg-[#2563EB] text-white px-4 py-2 rounded-lg font-medium">Save</button></section>' +
          '<section class="bg-white rounded-xl border border-slate-200 p-6"><h2 class="text-xl font-semibold text-[#0F172A] mb-4">Business hours</h2><p class="text-slate-600 mb-4">Warehouse open hours and time slots for drop-off bookings.</p><button type="button" class="settings-save bg-[#2563EB] text-white px-4 py-2 rounded-lg font-medium">Save</button></section>' +
          '<section class="bg-white rounded-xl border border-slate-200 p-6"><h2 class="text-xl font-semibold text-[#0F172A] mb-4">General</h2><p class="text-slate-600 mb-4">Company name, contact, and global defaults.</p><button type="button" class="settings-save bg-[#2563EB] text-white px-4 py-2 rounded-lg font-medium">Save</button></section>' +
          '</div></main></div>';
        document.getElementById('app').innerHTML = adminSidebar('settings') + main;
        document.getElementById('logout-btn').onclick = () => { fetch('/api/v1/auth/logout', { method: 'POST', credentials: 'include' }).then(() => { localStorage.removeItem('qcs_access_token'); window.location.href = '/'; }); };
        document.querySelectorAll('.settings-save').forEach(function(btn) { btn.onclick = function() { showComingSoon(this); }; });
      });
    })();
  