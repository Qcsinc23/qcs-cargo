// Auto-extracted from locker-receive.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    (function() {
      const token = localStorage.getItem('qcs_access_token');
      if (!token) { window.location.href = '/login?redirect=' + encodeURIComponent('/warehouse/locker-receive'); return; }
      const auth = { headers: { 'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json' } };
      fetch('/api/v1/me', auth).then(r => {
        if (!r.ok) { window.location.href = '/login?redirect=' + encodeURIComponent('/warehouse/locker-receive'); return null; }
        return r.json();
      }).then(me => {
        if (!me) return;
        const role = (me.data && me.data.role) || '';
        if (role !== 'staff' && role !== 'admin') { window.location.href = '/dashboard'; return; }
        const userID = (me.data && me.data.id) || '';
        const sidebar = '<aside class="w-64 bg-[#0F172A] text-white py-6 px-4"><a href="/" class="font-bold text-lg block mb-8">QCS Cargo</a><p class="text-amber-400 text-sm font-medium mb-2">Warehouse</p><nav class="space-y-1" aria-label="Main navigation">' +
          '<a href="/warehouse" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Dashboard</a>' +
          '<a href="/warehouse/locker-receive" class="block py-2 px-3 rounded-lg bg-[#1E293B]" aria-current="page">Locker Receive</a>' +
          '<a href="/warehouse/service-queue" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Service Queue</a>' +
          '<a href="/warehouse/ship-queue" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Ship Queue</a>' +
          '<a href="/warehouse/packages" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Packages</a>' +
          '<a href="/warehouse/receiving" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Receiving</a>' +
          '<a href="/warehouse/staging" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Staging</a>' +
          '<a href="/warehouse/manifests" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Manifests</a>' +
          '<a href="/warehouse/exceptions" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Exceptions</a></nav>' +
          '<a href="/admin" class="block mt-6 text-sm text-slate-400 hover:text-white">Admin</a><a href="/dashboard" class="block mt-2 text-sm text-slate-400 hover:text-white">Customer Dashboard</a><button type="button" class="sidebar-logout mt-4 text-sm text-slate-400 hover:text-white">Sign out</button></aside>';
        const main = '<main id="main" class="flex-1 p-8">' +
          '<nav aria-label="Breadcrumb" class="mb-4"><ol class="flex items-center gap-2 text-sm text-slate-600"><li><a href="/warehouse" class="hover:underline">Warehouse</a></li><li aria-current="page" class="font-medium">Locker Receive</li></ol></nav>' +
          '<h1 class="text-3xl font-bold mb-6">Receive Carrier Package</h1>' +
          '<p class="mb-4 text-slate-600 text-sm">Offline: receive actions will queue and sync when back online.</p>' +
          '<div id="msg" class="mb-4"></div>' +
          '<form id="form" class="max-w-xl space-y-4 bg-white rounded-xl border border-slate-200 p-6 shadow-sm">' +
          '<div><label for="suite_code" class="block text-sm font-medium text-slate-700 mb-1">Suite code <span class="text-red-500">*</span></label><input id="suite_code" type="text" name="suite_code" required placeholder="QCS-A1B2C3" class="w-full border border-slate-300 rounded-lg px-3 py-2" /></div>' +
          '<div><label for="tracking_inbound" class="block text-sm font-medium text-slate-700 mb-1">Tracking (optional)</label><input id="tracking_inbound" type="text" name="tracking_inbound" class="w-full border border-slate-300 rounded-lg px-3 py-2" /></div>' +
          '<div><label for="carrier_inbound" class="block text-sm font-medium text-slate-700 mb-1">Carrier (optional)</label><input id="carrier_inbound" type="text" name="carrier_inbound" placeholder="USPS, UPS, FedEx" class="w-full border border-slate-300 rounded-lg px-3 py-2" /></div>' +
          '<div><label for="sender_name" class="block text-sm font-medium text-slate-700 mb-1">Sender name (optional)</label><input id="sender_name" type="text" name="sender_name" placeholder="Amazon, Walmart" class="w-full border border-slate-300 rounded-lg px-3 py-2" /></div>' +
          '<div><label for="weight_lbs" class="block text-sm font-medium text-slate-700 mb-1">Weight (lbs, optional)</label><input id="weight_lbs" type="number" step="0.1" name="weight_lbs" class="w-full border border-slate-300 rounded-lg px-3 py-2" /></div>' +
          '<div><label for="condition" class="block text-sm font-medium text-slate-700 mb-1">Condition (optional)</label><select id="condition" name="condition" class="w-full border border-slate-300 rounded-lg px-3 py-2"><option value="">—</option><option value="good">Good</option><option value="damaged">Damaged</option></select></div>' +
          '<div><label for="storage_bay" class="block text-sm font-medium text-slate-700 mb-1">Storage bay (optional)</label><input id="storage_bay" type="text" name="storage_bay" placeholder="A1" class="w-full border border-slate-300 rounded-lg px-3 py-2" /></div>' +
          '<button type="submit" class="bg-[#0F172A] text-white px-6 py-2 rounded-lg font-medium hover:opacity-90">Receive Package</button></form>' +
          '</main>';
        document.getElementById('app').innerHTML = '<div class="flex min-h-screen">' + sidebar + main + '</div>';
        var lb = document.querySelector('.sidebar-logout'); if (lb) lb.onclick = function() { fetch('/api/v1/auth/logout', { method: 'POST', credentials: 'include' }).then(function() { localStorage.removeItem('qcs_access_token'); window.location.href = '/'; }); };
        document.getElementById('form').onsubmit = function(e) {
          e.preventDefault();
          const fd = new FormData(this);
          const payload = { suite_code: fd.get('suite_code') || '' };
          if (fd.get('tracking_inbound')) payload.tracking_inbound = fd.get('tracking_inbound');
          if (fd.get('carrier_inbound')) payload.carrier_inbound = fd.get('carrier_inbound');
          if (fd.get('sender_name')) payload.sender_name = fd.get('sender_name');
          const w = fd.get('weight_lbs'); if (w) payload.weight_lbs = parseFloat(w);
          if (fd.get('condition')) payload.condition = fd.get('condition');
          if (fd.get('storage_bay')) payload.storage_bay = fd.get('storage_bay');
          const msg = document.getElementById('msg');
          msg.className = 'mb-4 p-3 rounded-lg';
          fetch('/api/v1/warehouse/locker-receive', { method: 'POST', headers: auth.headers, body: JSON.stringify(payload) })
            .then(r => r.json().then(j => ({ ok: r.ok, j })))
            .then(({ ok, j }) => {
              if (ok) { msg.className = 'mb-4 p-3 rounded-lg bg-green-100 text-green-800'; msg.textContent = 'Package received. ID: ' + (j.data && j.data.id); this.reset(); }
              else { msg.className = 'mb-4 p-3 rounded-lg bg-red-100 text-red-800'; msg.textContent = (j.error && j.error.message) || j.message || 'Failed to receive package'; }
            })
            .catch(() => { msg.className = 'mb-4 p-3 rounded-lg bg-red-100 text-red-800'; msg.textContent = 'Network error'; });
        };
      });
    })();
  