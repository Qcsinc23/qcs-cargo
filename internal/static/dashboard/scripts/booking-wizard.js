// Auto-extracted from booking-wizard.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    (function () {
      'use strict';
      var QCS = window.QCSPWA;
      QCS.initBase({ registerSW: true, keyboard: true, utilityDock: true });

      var loginRedirect = '/login?redirect=' + encodeURIComponent('/dashboard/bookings/new');
      var token = QCS.readToken();
      if (!token) { window.location.href = loginRedirect; return; }

      var app = document.getElementById('dashboard-app');
      QCS.mountLoading(app, QCS.t('loading'));

      var STEPS = ['Date & time', 'Package description', 'Special instructions', 'Review', 'Confirm'];
      var state = {
        step: 1, date: '', timeSlot: '', packageDescription: '', specialInstructions: '',
        destinationId: 'guyana', serviceType: 'drop_off',
        subtotal: 0, discount: 0, insurance: 0, total: 0, slots: []
      };

      QCS.fetchJson('/api/v1/me')
        .then(function (j) {
          if (!j || !j.data) { window.location.href = loginRedirect; return; }
          renderShell();
          QCS.bindLogout();
        })
        .catch(function (err) {
          QCS.mountError(app, {
            title: 'Unable to start booking',
            description: (err && err.message) || 'Network error.',
            actionLabel: 'Sign in again',
            onRetry: function () { window.location.href = loginRedirect; }
          });
        });

      function formatDate(d) {
        var y = d.getFullYear();
        var m = String(d.getMonth() + 1).padStart(2, '0');
        var day = String(d.getDate()).padStart(2, '0');
        return y + '-' + m + '-' + day;
      }

      function renderShell() {
        var sidebar = QCS.renderSidebar('bookings');
        var html = '<div class="qcs-page-wrap">' + sidebar
          + '<main id="qcs-main" class="qcs-page-main" tabindex="-1">'
          + '<nav aria-label="Breadcrumb" class="mb-4"><ol class="flex items-center gap-2 text-sm text-slate-600">'
          + '<li><a href="/dashboard" class="text-[#2563EB] hover:underline">Dashboard</a></li>'
          + '<li aria-hidden="true">/</li>'
          + '<li><a href="/dashboard/bookings" class="text-[#2563EB] hover:underline">Bookings</a></li>'
          + '<li aria-hidden="true">/</li>'
          + '<li class="text-[#0F172A] font-medium" aria-current="page">New</li></ol></nav>'
          + '<h1 class="text-3xl font-bold text-[#0F172A] mb-2">New booking</h1>'
          + '<p class="text-slate-600 mb-6">Schedule a drop-off or pickup window in 5 quick steps.</p>'
          + '<ol id="step-tracker" class="flex flex-wrap items-center gap-2 mb-6" aria-label="Steps"></ol>'
          + '<section id="step-content" class="bg-white rounded-xl border border-slate-200 shadow-sm p-6 max-w-2xl"></section>'
          + '<div id="step-nav" class="mt-4 flex flex-wrap items-center gap-3 max-w-2xl"></div>'
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
        var tracker = document.getElementById('step-tracker');
        if (!tracker) return;
        tracker.innerHTML = STEPS.map(function (label, i) {
          var n = i + 1;
          var active = state.step === n;
          var done = state.step > n;
          var cls = active
            ? 'bg-[#2563EB] text-white border-[#2563EB]'
            : (done ? 'bg-emerald-50 text-emerald-700 border-emerald-200' : 'bg-white text-slate-600 border-slate-300');
          return '<li class="px-3 py-1 rounded-full text-sm border font-medium ' + cls + '">' + n + '. ' + window.qcsEscapeHTML(label) + '</li>';
        }).join('');
      }

      function renderStep() {
        var s = state.step;
        if (s === 1) {
          var today = formatDate(new Date());
          return '<h2 class="text-lg font-semibold mb-4">Pick a date and time</h2>'
            + '<div class="mb-4">'
            + '<label for="wizard-date" class="block text-sm font-medium text-slate-700 mb-1">Date</label>'
            + '<input type="date" id="wizard-date" class="w-full border border-slate-300 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-[#2563EB]" min="' + today + '" value="' + window.qcsEscapeHTML(state.date || '') + '">'
            + '</div>'
            + '<div>'
            + '<label class="block text-sm font-medium text-slate-700 mb-1">Time slot</label>'
            + '<div id="wizard-slots" class="flex flex-wrap gap-2 text-sm text-slate-500">Select a date first</div>'
            + '</div>';
        }
        if (s === 2) {
          var dests = ['guyana', 'jamaica', 'trinidad', 'barbados', 'suriname'];
          var opts = dests.map(function (d) {
            return '<option value="' + d + '"' + (state.destinationId === d ? ' selected' : '') + '>' + window.qcsEscapeHTML(d.charAt(0).toUpperCase() + d.slice(1)) + '</option>';
          }).join('');
          return '<h2 class="text-lg font-semibold mb-4">Package and destination</h2>'
            + '<div class="mb-4">'
            + '<label for="wizard-package-desc" class="block text-sm font-medium text-slate-700 mb-1">Package description</label>'
            + '<textarea id="wizard-package-desc" rows="3" class="w-full border border-slate-300 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-[#2563EB]" placeholder="e.g. 2 boxes, clothing and books">' + window.qcsEscapeHTML(state.packageDescription || '') + '</textarea>'
            + '</div>'
            + '<div>'
            + '<label for="wizard-destination" class="block text-sm font-medium text-slate-700 mb-1">Destination</label>'
            + '<select id="wizard-destination" class="w-full border border-slate-300 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-[#2563EB]">' + opts + '</select>'
            + '</div>';
        }
        if (s === 3) {
          return '<h2 class="text-lg font-semibold mb-4">Special instructions</h2>'
            + '<label for="wizard-special" class="block text-sm font-medium text-slate-700 mb-1">Anything we should know?</label>'
            + '<textarea id="wizard-special" rows="4" class="w-full border border-slate-300 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-[#2563EB]" placeholder="Fragile, leave at door, etc.">' + window.qcsEscapeHTML(state.specialInstructions || '') + '</textarea>';
        }
        if (s === 4) {
          var slot = state.slots.filter(function (x) { return x.id === state.timeSlot; })[0];
          var slotLabel = slot ? (slot.label || slot.id) : (state.timeSlot || '—');
          return '<h2 class="text-lg font-semibold mb-4">Review</h2>'
            + '<dl class="grid sm:grid-cols-2 gap-x-6 gap-y-3 text-sm">'
            + dlRow('Date', state.date || '—')
            + dlRow('Time', slotLabel)
            + dlRow('Destination', state.destinationId)
            + dlRow('Estimated total', QCS.formatMoney(state.total || 0, 'USD'))
            + (state.packageDescription ? '<div class="sm:col-span-2"><dt class="text-slate-500">Package</dt><dd class="font-medium whitespace-pre-line">' + window.qcsEscapeHTML(state.packageDescription) + '</dd></div>' : '')
            + (state.specialInstructions ? '<div class="sm:col-span-2"><dt class="text-slate-500">Special instructions</dt><dd class="font-medium whitespace-pre-line">' + window.qcsEscapeHTML(state.specialInstructions) + '</dd></div>' : '')
            + '</dl>';
        }
        return '<h2 class="text-lg font-semibold mb-4">Confirm booking</h2>'
          + '<p class="text-slate-600 mb-4">Click Confirm to create your booking.</p>'
          + '<button type="button" id="wizard-confirm-btn" class="bg-emerald-600 text-white px-6 py-3 rounded-lg font-semibold shadow-sm hover:bg-emerald-700">Confirm booking</button>'
          + '<p id="wizard-msg" class="mt-3 text-sm hidden" aria-live="polite"></p>';
      }

      function dlRow(label, value) {
        return '<div><dt class="text-slate-500">' + window.qcsEscapeHTML(label) + '</dt><dd class="font-medium">' + window.qcsEscapeHTML(String(value)) + '</dd></div>';
      }

      function renderNav() {
        var nav = document.getElementById('step-nav');
        if (!nav) return;
        var html = '';
        if (state.step > 1) html += '<button type="button" id="wizard-prev" class="bg-white border border-slate-300 px-4 py-2 rounded-lg font-medium hover:bg-slate-50">← Back</button>';
        if (state.step < 5) html += '<button type="button" id="wizard-next" class="bg-[#2563EB] text-white px-6 py-2 rounded-lg font-semibold shadow-sm hover:opacity-95">Next →</button>';
        nav.innerHTML = html;
      }

      function bindStep() {
        if (state.step === 1) {
          var dateEl = document.getElementById('wizard-date');
          if (dateEl) dateEl.addEventListener('change', function () { state.date = dateEl.value; loadSlots(); });
          loadSlots();
        }
        var prev = document.getElementById('wizard-prev');
        if (prev) prev.addEventListener('click', function () { state.step--; update(); });
        var next = document.getElementById('wizard-next');
        if (next) next.addEventListener('click', function () {
          if (state.step === 1) {
            state.date = (document.getElementById('wizard-date') || {}).value || state.date;
            state.timeSlot = state.timeSlot || (state.slots[0] && state.slots[0].id);
            if (!state.date || !state.timeSlot) { QCS.toast('Pick a date and time slot', 'warning'); return; }
          }
          if (state.step === 2) {
            var d = document.getElementById('wizard-package-desc');
            state.packageDescription = d ? d.value : state.packageDescription;
            var dst = document.getElementById('wizard-destination');
            state.destinationId = dst ? dst.value : state.destinationId;
          }
          if (state.step === 3) {
            var sp = document.getElementById('wizard-special');
            state.specialInstructions = sp ? sp.value : state.specialInstructions;
          }
          state.step++;
          update();
        });
        var confirm = document.getElementById('wizard-confirm-btn');
        if (confirm) confirm.addEventListener('click', submitBooking);
      }

      function loadSlots() {
        var el = document.getElementById('wizard-slots');
        if (!el) return;
        if (!state.date) { el.innerHTML = '<span class="text-sm text-slate-500">Select a date first</span>'; return; }
        el.innerHTML = '<span class="text-sm text-slate-500">Loading…</span>';
        QCS.fetchJson('/api/v1/bookings/time-slots?date=' + encodeURIComponent(state.date))
          .then(function (j) {
            var slots = (j && j.data && j.data.slots) || [];
            state.slots = slots;
            if (!slots.length) { el.innerHTML = '<span class="text-sm text-slate-500">No slots for that date.</span>'; return; }
            el.innerHTML = slots.map(function (slot) {
              var sel = state.timeSlot === slot.id;
              var cls = sel
                ? 'border-[#2563EB] bg-blue-50 ring-2 ring-[#2563EB]'
                : 'border-slate-300 hover:border-[#2563EB]';
              return '<button type="button" class="slot-btn border rounded-lg px-4 py-2 text-sm font-medium ' + cls + '" data-slot="' + window.qcsEscapeHTML(slot.id) + '">' + window.qcsEscapeHTML(slot.label || slot.id) + '</button>';
            }).join('');
            el.querySelectorAll('.slot-btn').forEach(function (btn) {
              btn.addEventListener('click', function () { state.timeSlot = btn.getAttribute('data-slot'); loadSlots(); });
            });
          })
          .catch(function () { el.innerHTML = '<span class="text-sm text-red-600">Could not load slots.</span>'; });
      }

      function submitBooking() {
        var btn = document.getElementById('wizard-confirm-btn');
        if (btn) btn.disabled = true;
        QCS.fetchJson('/api/v1/bookings', {
          method: 'POST',
          body: {
            service_type: state.serviceType,
            destination_id: state.destinationId,
            scheduled_date: state.date,
            time_slot: state.timeSlot,
            special_instructions: state.specialInstructions || null,
            subtotal: state.subtotal,
            discount: state.discount,
            insurance: state.insurance,
            total: state.total
          }
        }).then(function (j) {
          QCS.toast('Booking created', 'success');
          window.location.href = '/dashboard/bookings/' + (j && j.data && j.data.id ? encodeURIComponent(j.data.id) : '');
        }).catch(function (err) {
          if (btn) btn.disabled = false;
          var msg = document.getElementById('wizard-msg');
          if (msg) {
            msg.textContent = (err && err.message) || 'Failed to create booking.';
            msg.className = 'mt-3 text-sm text-red-600';
            msg.classList.remove('hidden');
          }
        });
      }
    })();
  