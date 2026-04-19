// Auto-extracted from ship.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    (function () {
      'use strict';
      var QCS = window.QCSPWA;
      QCS.initBase({ registerSW: true, keyboard: true, utilityDock: true });

      var loginRedirect = '/login?redirect=' + encodeURIComponent('/dashboard/ship');
      var token = QCS.readToken();
      if (!token) { window.location.href = loginRedirect; return; }

      var params = new URLSearchParams(window.location.search);
      var preSelected = (params.get('packages') || '').split(',').filter(Boolean);

      var app = document.getElementById('dashboard-app');
      QCS.mountLoading(app, QCS.t('loading'));

      var step = 1;
      var state = {
        packages: [], selectedIds: [], destinations: [], destinationId: '',
        recipients: [], recipientId: '', serviceType: 'standard', specialInstructions: ''
      };

      Promise.all([
        QCS.fetchJson('/api/v1/me'),
        QCS.fetchJson('/api/v1/locker').catch(function () { return { data: [] }; }),
        QCS.fetchJson('/api/v1/destinations').catch(function () { return { data: [] }; }),
        QCS.fetchJson('/api/v1/recipients').catch(function () { return { data: [] }; })
      ]).then(function (results) {
        var me = results[0];
        if (!me || !me.data) { window.location.href = loginRedirect; return; }
        state.packages = (results[1] && results[1].data) || [];
        state.destinations = (results[2] && results[2].data) || [];
        state.recipients = (results[3] && results[3].data) || [];
        state.selectedIds = preSelected.length ? preSelected : (state.packages.length === 1 ? [state.packages[0].id] : []);
        renderShell();
        QCS.bindLogout();
      }).catch(function (err) {
        QCS.mountError(app, {
          title: 'Unable to start ship request',
          description: (err && err.message) || 'Network error.',
          actionLabel: 'Sign in again',
          onRetry: function () { window.location.href = loginRedirect; }
        });
      });

      function renderShell() {
        var sidebar = QCS.renderSidebar('ship');
        var html = '<div class="qcs-page-wrap">' + sidebar
          + '<main id="qcs-main" class="qcs-page-main" tabindex="-1">'
          + '<nav aria-label="Breadcrumb" class="mb-4"><ol class="flex items-center gap-2 text-sm text-slate-600">'
          + '<li><a href="/dashboard" class="text-[#2563EB] hover:underline">Dashboard</a></li>'
          + '<li aria-hidden="true">/</li>'
          + '<li class="text-[#0F172A] font-medium" aria-current="page">Ship my packages</li></ol></nav>'
          + '<h1 class="text-3xl font-bold text-[#0F172A] mb-2">Ship my packages</h1>'
          + '<p class="text-slate-600 mb-6">Pick the packages you want shipped, choose a destination and recipient, and confirm.</p>'
          + '<ol id="step-tracker" class="flex flex-wrap items-center gap-2 mb-6" aria-label="Steps"></ol>'
          + '<section id="step-content" class="bg-white rounded-xl border border-slate-200 shadow-sm p-6"></section>'
          + '<div id="step-nav" class="mt-4 flex flex-wrap items-center gap-3"></div>'
          + '</main></div>';
        app.innerHTML = html;
        var main = document.getElementById('qcs-main');
        if (main) main.focus({ preventScroll: true });
        update();
      }

      function update() {
        renderTracker();
        document.getElementById('step-content').innerHTML = renderStep();
        renderNav();
        bindStep();
      }

      function renderTracker() {
        var steps = ['Packages', 'Destination', 'Recipient', 'Review'];
        var tracker = document.getElementById('step-tracker');
        if (!tracker) return;
        tracker.innerHTML = steps.map(function (label, i) {
          var n = i + 1;
          var active = step === n;
          var done = step > n;
          var cls = active
            ? 'bg-[#2563EB] text-white border-[#2563EB]'
            : (done ? 'bg-emerald-50 text-emerald-700 border-emerald-200' : 'bg-white text-slate-600 border-slate-300');
          return '<li class="px-3 py-1 rounded-full text-sm border font-medium ' + cls + '">' + n + '. ' + window.qcsEscapeHTML(label) + '</li>';
        }).join('');
      }

      function renderStep() {
        if (step === 1) return renderPackages();
        if (step === 2) return renderDestinations();
        if (step === 3) return renderRecipients();
        return renderReview();
      }

      function renderPackages() {
        var list = state.packages;
        if (!list.length) {
          return '<p class="text-slate-600">You have no packages in your locker yet. <a href="/dashboard/inbound" class="text-[#2563EB] underline">Pre-alert a package</a> or <a href="/dashboard/mailbox" class="text-[#2563EB] underline">copy your US address</a> to start.</p>';
        }
        var html = '<h2 class="text-lg font-semibold mb-4">Select packages to ship</h2>'
          + '<div class="grid grid-cols-1 md:grid-cols-2 gap-3">';
        list.forEach(function (p) {
          var sender = typeof p.sender_name === 'string' ? p.sender_name : (p.sender_name && p.sender_name.String) || 'Unknown';
          var weight = typeof p.weight_lbs === 'number' ? p.weight_lbs : (p.weight_lbs && p.weight_lbs.Float64) || 0;
          var checked = state.selectedIds.indexOf(p.id) >= 0 ? ' checked' : '';
          html += '<label class="bg-white rounded-xl border border-slate-200 p-4 flex items-start gap-3 cursor-pointer hover:border-[#2563EB] hover:shadow-sm transition">'
            + '<input type="checkbox" class="pkg-cb mt-1 h-4 w-4 rounded border-slate-300 text-[#2563EB] focus:ring-[#2563EB]" data-id="' + window.qcsEscapeHTML(p.id) + '"' + checked + '>'
            + '<span><span class="block font-medium">' + window.qcsEscapeHTML(sender) + '</span>'
            + '<span class="block text-sm text-slate-500">' + weight + ' lbs · ' + window.qcsEscapeHTML(p.status || 'stored') + '</span></span></label>';
        });
        html += '</div>';
        return html;
      }

      function renderDestinations() {
        if (!state.destinations.length) return '<p class="text-slate-600">No destinations available right now.</p>';
        var html = '<h2 class="text-lg font-semibold mb-4">Choose destination</h2>'
          + '<div class="grid grid-cols-1 md:grid-cols-2 gap-3">';
        state.destinations.forEach(function (d) {
          var did = d.id || d.ID;
          var name = d.name || d.Name || did;
          var selected = state.destinationId === did;
          var cls = selected ? 'border-[#2563EB] bg-blue-50 ring-2 ring-[#2563EB]' : 'border-slate-200 hover:border-[#2563EB]';
          html += '<button type="button" class="dest-btn text-left border rounded-xl p-4 transition ' + cls + '" data-id="' + window.qcsEscapeHTML(did) + '">'
            + '<p class="font-semibold">' + window.qcsEscapeHTML(name) + '</p>'
            + (d.usd_per_lb ? '<p class="text-sm text-slate-500 mt-1">$' + window.qcsEscapeHTML(String(d.usd_per_lb)) + '/lb' + (d.transit ? ' · ' + window.qcsEscapeHTML(d.transit) : '') + '</p>' : '')
            + '</button>';
        });
        html += '</div>';
        return html;
      }

      function renderRecipients() {
        var html = '<div class="flex items-baseline justify-between mb-4">'
          + '<h2 class="text-lg font-semibold">Choose recipient</h2>'
          + '<a href="/dashboard/recipients" class="text-sm text-[#2563EB] hover:underline">+ Add recipient</a>'
          + '</div>';
        if (!state.recipients.length) {
          html += '<p class="text-slate-600">You have no recipients yet. <a href="/dashboard/recipients" class="text-[#2563EB] underline">Add one</a> first.</p>';
          return html;
        }
        html += '<div class="grid grid-cols-1 md:grid-cols-2 gap-3">';
        state.recipients.forEach(function (r) {
          var rid = r.id || r.ID;
          var name = (typeof r.name === 'string' ? r.name : (r.name && r.name.String)) || 'Unnamed';
          var city = (typeof r.city === 'string' ? r.city : (r.city && r.city.String)) || '';
          var selected = state.recipientId === rid;
          var cls = selected ? 'border-[#2563EB] bg-blue-50 ring-2 ring-[#2563EB]' : 'border-slate-200 hover:border-[#2563EB]';
          html += '<button type="button" class="recip-btn text-left border rounded-xl p-4 transition ' + cls + '" data-id="' + window.qcsEscapeHTML(rid) + '">'
            + '<p class="font-semibold">' + window.qcsEscapeHTML(name) + '</p>'
            + (city ? '<p class="text-sm text-slate-500 mt-1">' + window.qcsEscapeHTML(city) + '</p>' : '')
            + '</button>';
        });
        html += '</div>';
        return html;
      }

      function renderReview() {
        var dest = state.destinations.filter(function (d) { return (d.id || d.ID) === state.destinationId; })[0];
        var destName = dest ? (dest.name || dest.Name) : (state.destinationId || '—');
        var recip = state.recipients.filter(function (r) { return (r.id || r.ID) === state.recipientId; })[0];
        var recipName = recip
          ? ((typeof recip.name === 'string' ? recip.name : (recip.name && recip.name.String)) || '—')
          : (state.recipientId || '—');
        var html = '<h2 class="text-lg font-semibold mb-4">Review &amp; submit</h2>'
          + '<dl class="grid sm:grid-cols-2 gap-x-6 gap-y-3 text-sm mb-4">'
          + dlRow('Packages', String(state.selectedIds.length))
          + dlRow('Destination', destName)
          + dlRow('Recipient', recipName)
          + dlRow('Service', state.serviceType)
          + '</dl>'
          + '<div>'
          + '<label for="review-notes" class="block text-sm font-medium text-slate-700 mb-1">Special instructions (optional)</label>'
          + '<textarea id="review-notes" rows="3" class="w-full border border-slate-300 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-[#2563EB]" placeholder="e.g. fragile">' + window.qcsEscapeHTML(state.specialInstructions || '') + '</textarea>'
          + '</div>'
          + '<p id="submit-msg" class="mt-3 text-sm hidden" aria-live="polite"></p>';
        return html;
      }

      function dlRow(label, value) {
        return '<div><dt class="text-slate-500">' + window.qcsEscapeHTML(label) + '</dt><dd class="font-medium">' + window.qcsEscapeHTML(String(value)) + '</dd></div>';
      }

      function renderNav() {
        var nav = document.getElementById('step-nav');
        if (!nav) return;
        var html = '';
        if (step > 1) html += '<button type="button" id="back-btn" class="bg-white border border-slate-300 px-4 py-2 rounded-lg font-medium hover:bg-slate-50">← Back</button>';
        if (step < 4) html += '<button type="button" id="next-btn" class="bg-[#2563EB] text-white px-6 py-2 rounded-lg font-semibold shadow-sm hover:opacity-95">Next →</button>';
        else html += '<button type="button" id="submit-btn" class="bg-[#F97316] text-white px-6 py-3 rounded-lg font-semibold shadow-sm hover:opacity-95">Submit ship request</button>';
        nav.innerHTML = html;
      }

      function canProceed() {
        if (step === 1) return state.selectedIds.length > 0;
        if (step === 2) return !!state.destinationId;
        if (step === 3) return !!state.recipientId;
        return true;
      }

      function bindStep() {
        document.querySelectorAll('.pkg-cb').forEach(function (cb) {
          cb.addEventListener('change', function () {
            var id = cb.getAttribute('data-id');
            if (cb.checked) {
              if (state.selectedIds.indexOf(id) < 0) state.selectedIds.push(id);
            } else {
              state.selectedIds = state.selectedIds.filter(function (x) { return x !== id; });
            }
          });
        });
        document.querySelectorAll('.dest-btn').forEach(function (btn) {
          btn.addEventListener('click', function () { state.destinationId = btn.getAttribute('data-id'); update(); });
        });
        document.querySelectorAll('.recip-btn').forEach(function (btn) {
          btn.addEventListener('click', function () { state.recipientId = btn.getAttribute('data-id'); update(); });
        });

        var next = document.getElementById('next-btn');
        if (next) next.addEventListener('click', function () {
          if (!canProceed()) {
            QCS.toast('Pick an option to continue', 'warning');
            return;
          }
          step++; update();
        });
        var back = document.getElementById('back-btn');
        if (back) back.addEventListener('click', function () { step--; update(); });

        var submit = document.getElementById('submit-btn');
        if (submit) submit.addEventListener('click', function () {
          state.specialInstructions = (document.getElementById('review-notes') || {}).value || '';
          if (!state.selectedIds.length || !state.destinationId || !state.recipientId) {
            QCS.toast('Pick packages, destination, and recipient first', 'warning');
            return;
          }
          submit.disabled = true;
          QCS.fetchJson('/api/v1/ship-requests', {
            method: 'POST',
            body: {
              destination_id: state.destinationId,
              service_type: state.serviceType,
              recipient_id: state.recipientId || null,
              locker_package_ids: state.selectedIds,
              special_instructions: state.specialInstructions || null
            }
          }).then(function (d) {
            var data = (d && d.data) || {};
            var code = data.confirmation_code || data.id || '';
            var msg = document.getElementById('submit-msg');
            if (msg) {
              msg.textContent = 'Created: ' + code + '. Redirecting…';
              msg.className = 'mt-3 text-sm text-emerald-600';
              msg.classList.remove('hidden');
            }
            QCS.toast('Ship request created', 'success');
            setTimeout(function () { window.location.href = '/dashboard/ship-requests'; }, 1500);
          }).catch(function (err) {
            submit.disabled = false;
            var msg = document.getElementById('submit-msg');
            if (msg) {
              msg.textContent = (err && err.message) || 'Submit failed.';
              msg.className = 'mt-3 text-sm text-red-600';
              msg.classList.remove('hidden');
            }
          });
        });
      }
    })();
  