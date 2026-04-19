// Auto-extracted from ship-queue.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    (function() {
      const token = localStorage.getItem('qcs_access_token');
      if (!token) { window.location.href = '/login?redirect=' + encodeURIComponent('/warehouse/ship-queue'); return; }
      const auth = { headers: { 'Authorization': 'Bearer ' + token } };
      fetch('/api/v1/me', auth).then(r => {
        if (!r.ok) { window.location.href = '/login?redirect=' + encodeURIComponent('/warehouse/ship-queue'); return null; }
        return r.json();
      }).then(me => {
        if (!me) return;
        const role = (me.data && me.data.role) || '';
        if (role !== 'staff' && role !== 'admin') { window.location.href = '/dashboard'; return; }
        const sidebar = '<aside class="w-64 bg-[#0F172A] text-white py-6 px-4"><a href="/" class="font-bold text-lg block mb-8">QCS Cargo</a><p class="text-amber-400 text-sm font-medium mb-2">Warehouse</p><nav class="space-y-1">' +
          '<a href="/warehouse" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Dashboard</a>' +
          '<a href="/warehouse/locker-receive" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Locker Receive</a>' +
          '<a href="/warehouse/service-queue" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Service Queue</a>' +
          '<a href="/warehouse/ship-queue" class="block py-2 px-3 rounded-lg bg-[#1E293B]">Ship Queue</a>' +
          '<a href="/warehouse/packages" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Packages</a>' +
          '<a href="/warehouse/receiving" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Receiving</a>' +
          '<a href="/warehouse/staging" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Staging</a>' +
          '<a href="/warehouse/manifests" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Manifests</a>' +
          '<a href="/warehouse/exceptions" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Exceptions</a></nav>' +
          '<a href="/admin" class="block mt-6 text-sm text-slate-400 hover:text-white">Admin</a><a href="/dashboard" class="block mt-2 text-sm text-slate-400 hover:text-white">Customer Dashboard</a><button type="button" class="sidebar-logout mt-4 text-sm text-slate-400 hover:text-white">Sign out</button></aside>';
        function bindLogout() { var lb = document.querySelector('.sidebar-logout'); if (lb) lb.onclick = function() { fetch('/api/v1/auth/logout', { method: 'POST', credentials: 'include' }).then(function() { localStorage.removeItem('qcs_access_token'); window.location.href = '/'; }); }; }
        const list = [];
        function action(id, kind, body, label) {
          const opts = { method: 'PATCH', headers: auth.headers };
          if (body && Object.keys(body).length) opts.body = JSON.stringify(body); else opts.headers['Content-Type'] = 'application/json';
          fetch('/api/v1/warehouse/ship-queue/' + id + '/' + kind, opts).then(r => r.json()).then(j => {
            if (j.data) load(); else alert((j.error && j.error.message) || 'Failed');
          });
        }
        function load() {
          fetch('/api/v1/warehouse/ship-queue', auth).then(r => r.json()).then(j => {
            const data = (j && j.data) || [];
            const rows = data.map(sr => {
              const processBtn = '<button type="button" class="process-btn px-2 py-1 rounded bg-slate-600 text-white text-xs mr-1" data-id="' + sr.id + '">Process</button>';
              const weighedBtn = '<button type="button" class="weighed-btn px-2 py-1 rounded bg-amber-600 text-white text-xs mr-1" data-id="' + sr.id + '">Weighed</button>';
              const stagedBtn = '<button type="button" class="staged-btn px-2 py-1 rounded bg-emerald-600 text-white text-xs" data-id="' + sr.id + '">Staged</button>';
              return '<tr class="border-b border-slate-200"><td class="py-3 px-4">' + (sr.confirmation_code || '—') + '</td><td class="py-3 px-4">' + (sr.destination_id || '—') + '</td><td class="py-3 px-4">' + (sr.status || '') + '</td><td class="py-3 px-4">' + (sr.total != null ? '$' + sr.total : '—') + '</td><td class="py-3 px-4">' + processBtn + weighedBtn + stagedBtn + '</td></tr>';
            }).join('');
            document.getElementById('app').innerHTML = '<div class="flex min-h-screen">' + sidebar +
              '<main class="flex-1 p-8"><nav aria-label="Breadcrumb" class="mb-4"><ol class="flex items-center gap-2 text-sm text-slate-600"><li><a href="/warehouse" class="hover:underline">Warehouse</a></li><li aria-current="page" class="font-medium">Ship Queue</li></ol></nav><h1 class="text-3xl font-bold mb-6">Ship Queue</h1><div class="bg-white rounded-xl border border-slate-200 overflow-hidden"><table class="w-full"><thead class="bg-slate-50"><tr><th class="text-left py-3 px-4">Confirmation</th><th class="text-left py-3 px-4">Destination</th><th class="text-left py-3 px-4">Status</th><th class="text-left py-3 px-4">Total</th><th class="text-left py-3 px-4">Actions</th></tr></thead><tbody>' + rows + '</tbody></table></div></main></div>';
            document.querySelectorAll('.process-btn').forEach(b => b.onclick = () => action(b.dataset.id, 'process'));
            document.querySelectorAll('.weighed-btn').forEach(b => b.onclick = () => { const w = prompt('Consolidated weight (lbs)'); if (w != null && w !== '') action(b.dataset.id, 'weighed', { consolidated_weight_lbs: parseFloat(w) }); });
            document.querySelectorAll('.staged-btn').forEach(b => b.onclick = () => { const bay = prompt('Staging bay (optional)'); const mid = prompt('Manifest ID (optional)'); action(b.dataset.id, 'staged', { staging_bay: bay || undefined, manifest_id: mid || undefined }); });
            bindLogout();
          }).catch(() => { document.getElementById('app').innerHTML = '<div class="p-8">Failed to load ship queue.</div>'; });
        }
        load();
      });
    })();
  