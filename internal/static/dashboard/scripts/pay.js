// Auto-extracted from pay.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    (function () {
      'use strict';
      var QCS = window.QCSPWA;
      QCS.initBase({ registerSW: true, keyboard: true, utilityDock: true });

      var pathParts = window.location.pathname.split('/').filter(Boolean);
      var idx = pathParts.indexOf('ship-requests');
      var id = idx >= 0 && pathParts[idx + 1] && pathParts[idx + 1] !== 'pay' ? pathParts[idx + 1] : null;
      if (!id) { window.location.href = '/dashboard/ship-requests'; return; }

      var app = document.getElementById('app');
      QCS.mountLoading(app, 'Preparing payment…');

      // Concurrently load: current user (auth + sidebar), ship request
      // (status + total), and start/reuse the PaymentIntent. The server
      // reuses an existing reusable intent (audit fix C-2) so this is safe
      // to call multiple times.
      Promise.all([
        QCS.fetchJson('/api/v1/me'),
        QCS.fetchJson('/api/v1/ship-requests/' + encodeURIComponent(id)),
        QCS.fetchJson('/api/v1/ship-requests/' + encodeURIComponent(id) + '/pay', { method: 'POST', body: {} }).catch(function (e) { return { __error: e }; })
      ]).then(function (results) {
        var me = results[0] && results[0].data;
        var srRes = results[1] && results[1].data;
        var sr = (srRes && (srRes.ship_request || srRes)) || null;
        var payResult = results[2];

        if (!sr) {
          QCS.mountError(app, {
            title: 'Ship request not found',
            description: 'We could not find a ship request with that identifier.',
            actionLabel: 'Back to ship requests',
            onRetry: function () { window.location.href = '/dashboard/ship-requests'; }
          });
          return;
        }

        var totalLabel = QCS.formatMoney(sr.total || 0, 'USD');
        var sidebar = QCS.renderSidebar('ship-requests');
        var bc = '<nav aria-label="Breadcrumb" class="mb-4">'
          + '<ol class="flex items-center gap-2 text-sm text-slate-600">'
          + '<li><a href="/dashboard" class="text-[#2563EB] hover:underline">Dashboard</a></li>'
          + '<li aria-hidden="true">/</li>'
          + '<li><a href="/dashboard/ship-requests" class="text-[#2563EB] hover:underline">Ship Requests</a></li>'
          + '<li aria-hidden="true">/</li>'
          + '<li><a href="/dashboard/ship-requests/' + encodeURIComponent(id) + '" class="text-[#2563EB] hover:underline">Request</a></li>'
          + '<li aria-hidden="true">/</li>'
          + '<li class="text-[#0F172A] font-medium" aria-current="page">Pay</li>'
          + '</ol></nav>';

        // Already paid?
        if (sr.status === 'paid' || (sr.payment_status && String(sr.payment_status).toLowerCase() === 'paid')) {
          app.innerHTML = '<div class="qcs-page-wrap">' + sidebar
            + '<main id="qcs-main" class="qcs-page-main" tabindex="-1">'
            + bc
            + '<div class="bg-white rounded-xl border border-slate-200 p-6 max-w-xl shadow-sm">'
            + '<div class="qcs-badge qcs-badge-success mb-3">Paid</div>'
            + '<h1 class="text-2xl font-bold mb-2">This ship request is already paid</h1>'
            + '<p class="text-slate-600 mb-4">Total charged: <strong>' + window.qcsEscapeHTML(totalLabel) + '</strong>.</p>'
            + '<div class="flex gap-3"><a href="/dashboard/ship-requests/' + encodeURIComponent(id) + '" class="bg-[#2563EB] text-white px-4 py-2 rounded-lg font-medium">View ship request</a>'
            + '<a href="/dashboard/ship-requests" class="px-4 py-2 rounded-lg border border-slate-300">Back to list</a></div>'
            + '</div></main></div>';
          QCS.bindLogout();
          return;
        }

        // Stripe not configured (501) or pay endpoint failed for other reasons
        if (payResult && payResult.__error) {
          var err = payResult.__error;
          if (err.status === 501) {
            renderUnavailable('Online payment is not configured yet. Please contact support to arrange payment for this shipment.');
            return;
          }
          if (err.status === 409 && err.body && err.body.error && err.body.error.code === 'ALREADY_PAID') {
            // Server says paid; refresh the page to pick up new state.
            window.location.reload();
            return;
          }
          renderUnavailable(err.message || 'We could not start payment right now.');
          return;
        }

        var clientSecret = payResult && payResult.data && payResult.data.client_secret;
        if (!clientSecret) {
          renderUnavailable('Payment provider did not return a client secret.');
          return;
        }

        app.innerHTML = '<div class="qcs-page-wrap">' + sidebar
          + '<main id="qcs-main" class="qcs-page-main" tabindex="-1">'
          + bc
          + '<a href="/dashboard/ship-requests/' + encodeURIComponent(id) + '" class="text-[#2563EB] mb-6 inline-block">&larr; Back to ship request</a>'
          + '<h1 class="text-3xl font-bold mb-1">Complete Payment</h1>'
          + '<p class="text-slate-500 mb-6">Total: <strong class="text-[#0F172A]">' + window.qcsEscapeHTML(totalLabel) + '</strong></p>'
          + '<div class="bg-white rounded-xl border border-slate-200 p-6 max-w-xl shadow-sm">'
          + '<div id="payment-element" aria-label="Payment details"></div>'
          + '<button type="button" id="pay-btn" class="mt-5 w-full bg-[#2563EB] text-white px-6 py-3 rounded-lg font-semibold disabled:opacity-60">Pay ' + window.qcsEscapeHTML(totalLabel) + '</button>'
          + '<p id="pay-status" class="mt-3 text-sm text-slate-500" aria-live="polite"></p>'
          + '</div></main></div>';
        QCS.bindLogout();

        if (typeof Stripe === 'undefined') {
          QCS.toast('Stripe.js failed to load — check your network/ad-blocker', 'error');
          var btn = document.getElementById('pay-btn');
          if (btn) { btn.disabled = true; btn.textContent = 'Payment unavailable'; }
          return;
        }

        var pk = window.STRIPE_PUBLISHABLE_KEY || '';
        var ensurePK = pk
          ? Promise.resolve(pk)
          : QCS.fetchJson('/api/v1/config').then(function (d) {
              return (d && d.data && d.data.stripe_publishable_key) || '';
            }).catch(function () { return ''; });

        ensurePK.then(function (key) {
          if (!key) {
            QCS.toast('Stripe is not configured on this site.', 'error');
            document.getElementById('pay-btn').disabled = true;
            return;
          }
          var stripe = Stripe(key);
          var elements = stripe.elements({ clientSecret: clientSecret });
          var paymentElement = elements.create('payment');
          paymentElement.mount('#payment-element');
          var btn = document.getElementById('pay-btn');
          var statusEl = document.getElementById('pay-status');
          btn.addEventListener('click', function () {
            btn.disabled = true;
            statusEl.textContent = 'Processing payment…';
            // Stripe's return_url drives the post-payment redirect. The
            // server's /api/webhooks/stripe handler atomically marks the
            // ship request paid (audit fix C-3); the confirmation page
            // simply polls the request to surface the new state — no
            // client-side reconcile call needed.
            stripe.confirmPayment({
              elements: elements,
              confirmParams: { return_url: window.location.origin + '/dashboard/ship-requests/' + encodeURIComponent(id) + '/confirmation' }
            }).then(function (result) {
              if (result && result.error) {
                statusEl.textContent = result.error.message || 'Payment failed';
                QCS.toast(result.error.message || 'Payment failed', 'error');
                btn.disabled = false;
                return;
              }
              // If we get here without redirect (rare for non-card methods),
              // route the user to the confirmation page so it can poll status.
              window.location.href = '/dashboard/ship-requests/' + encodeURIComponent(id) + '/confirmation';
            }).catch(function (err) {
              statusEl.textContent = (err && err.message) || 'Payment failed';
              QCS.toast(statusEl.textContent, 'error');
              btn.disabled = false;
            });
          });
        });

        function renderUnavailable(message) {
          app.innerHTML = '<div class="qcs-page-wrap">' + sidebar
            + '<main id="qcs-main" class="qcs-page-main" tabindex="-1">'
            + bc
            + '<div class="bg-amber-50 border border-amber-200 rounded-xl p-6 max-w-xl">'
            + '<h1 class="text-xl font-semibold text-amber-900 mb-2">Payment unavailable</h1>'
            + '<p class="text-amber-800">' + window.qcsEscapeHTML(message) + '</p>'
            + '<a href="/dashboard/ship-requests/' + encodeURIComponent(id) + '" class="inline-block mt-4 text-[#2563EB] font-medium">Back to ship request</a>'
            + '</div></main></div>';
          QCS.bindLogout();
        }
      }).catch(function (err) {
        QCS.mountError(app, {
          title: 'Could not load payment page',
          description: (err && err.message) || 'Network error.',
          onRetry: function () { window.location.reload(); }
        });
      });
    })();
  