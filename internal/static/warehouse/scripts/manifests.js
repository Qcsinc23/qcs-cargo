// Auto-extracted from manifests.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    (function() {
      const token = localStorage.getItem('qcs_access_token');
      if (!token) { window.location.href = '/login?redirect=' + encodeURIComponent('/warehouse/manifests'); return; }
      const auth = { headers: { 'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json' } };
      fetch('/api/v1/me', auth).then(r => {
        if (!r.ok) { window.location.href = '/login?redirect=' + encodeURIComponent('/warehouse/manifests'); return null; }
        return r.json();
      }).then(me => {
        if (!me) return;
        const role = (me.data && me.data.role) || '';
        if (role !== 'staff' && role !== 'admin') { window.location.href = '/dashboard'; return; }
        const sidebar = '<aside class="w-64 bg-[#0F172A] text-white py-6 px-4"><a href="/" class="font-bold text-lg block mb-8">QCS Cargo</a><p class="text-amber-400 text-sm font-medium mb-2">Warehouse</p><nav class="space-y-1">' +
          '<a href="/warehouse" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Dashboard</a>' +
          '<a href="/warehouse/locker-receive" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Locker Receive</a>' +
          '<a href="/warehouse/service-queue" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Service Queue</a>' +
          '<a href="/warehouse/ship-queue" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Ship Queue</a>' +
          '<a href="/warehouse/packages" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Packages</a>' +
          '<a href="/warehouse/receiving" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Receiving</a>' +
          '<a href="/warehouse/staging" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Staging</a>' +
          '<a href="/warehouse/manifests" class="block py-2 px-3 rounded-lg bg-[#1E293B]">Manifests</a>' +
          '<a href="/warehouse/exceptions" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Exceptions</a></nav>' +
          '<a href="/admin" class="block mt-6 text-sm text-slate-400 hover:text-white">Admin</a><a href="/dashboard" class="block mt-2 text-sm text-slate-400 hover:text-white">Customer Dashboard</a><button type="button" class="sidebar-logout mt-4 text-sm text-slate-400 hover:text-white">Sign out</button></aside>';
        function load() {
          fetch('/api/v1/warehouse/manifests', auth).then(r => r.json()).then(j => {
            const list = (j && j.data) || [];
            // Pass 2 audit fix C-1: HTML-escape every server-supplied field.
            const esc = window.QCSAdmin.escapeHTML;
            const rows = list.map(m => '<tr class="border-b border-slate-200"><td class="py-3 px-4">' + esc(m.id ? m.id.substring(0, 8) + '…' : '—') + '</td><td class="py-3 px-4">' + esc(m.destination_id || '—') + '</td><td class="py-3 px-4">' + esc(m.status || '') + '</td><td class="py-3 px-4">' + esc(m.created_at || '') + '</td><td class="py-3 px-4"><a href="/api/v1/warehouse/manifests/' + encodeURIComponent(m.id) + '/documents" target="_blank" class="text-blue-600 hover:underline">Documents</a></td></tr>').join('');
            function bindLogout() { var lb = document.querySelector('.sidebar-logout'); if (lb) lb.onclick = function() { fetch('/api/v1/auth/logout', { method: 'POST', credentials: 'include' }).then(function() { localStorage.removeItem('qcs_access_token'); window.location.href = '/'; }); }; }
            document.getElementById('app').innerHTML = '<div class="flex min-h-screen">' + sidebar +
              '<main class="flex-1 p-8"><nav class="mb-4 text-sm text-slate-600"><a href="/warehouse" class="hover:underline">Warehouse</a> / Manifests</nav><h1 class="text-3xl font-bold mb-6">Manifests</h1><div class="bg-white rounded-xl border overflow-hidden"><table class="w-full"><thead class="bg-slate-50"><tr><th class="text-left py-3 px-4">ID</th><th class="text-left py-3 px-4">Destination</th><th class="text-left py-3 px-4">Status</th><th class="text-left py-3 px-4">Created</th><th class="text-left py-3 px-4">Documents</th></tr></thead><tbody>' + rows + '</tbody></table></div></main></div>';
            bindLogout();
          }).catch(() => { document.getElementById('app').innerHTML = '<div class="p-8">Failed to load.</div>'; });
        }
        load();
      });
    })();
  