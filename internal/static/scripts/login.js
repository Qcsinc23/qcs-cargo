// Auto-extracted from login.html (Phase 2.4 / SEC-001a).
// Phase 3.4 (CRF-001) form-craft pass:
//   - Disable submit while the request is in flight; restore on response
//     via QcsFormCraft.setBusy.
//   - Persist the typed email across navigation via QcsFormCraft.persistEmail
//     so a bounced "go fix verification" detour does not lose the user's typing.
//   - Status updates use textContent (not innerHTML); the dev-only
//     "magic_link" branch builds a real anchor element instead of writing
//     attacker-influenceable HTML.

(function () {
  'use strict';

  var emailInput = document.getElementById('login-email');
  var submitBtn = document.getElementById('login-submit');
  var statusEl = document.getElementById('form-status');
  var form = document.getElementById('login-form');
  if (!emailInput || !submitBtn || !statusEl || !form) return;

  var FC = window.QcsFormCraft || {};

  if (typeof FC.persistEmail === 'function') {
    FC.persistEmail(form);
  }

  function showStatus(text, tone) {
    statusEl.style.display = 'block';
    statusEl.style.color = tone === 'error' ? '#b91c1c'
      : tone === 'success' ? '#0f766e'
      : '#576a83';
    if (typeof FC.liveStatus === 'function') {
      FC.liveStatus(statusEl, text);
    } else {
      statusEl.textContent = text;
    }
  }

  form.addEventListener('submit', async function (e) {
    e.preventDefault();
    var email = emailInput.value;
    var release = typeof FC.setBusy === 'function'
      ? FC.setBusy(form)
      : function () {};
    showStatus('Sending...', 'info');

    try {
      var r = await fetch('/api/v1/auth/magic-link/request', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: email })
      });
      var j = {};
      try { j = await r.json(); } catch (parseErr) { j = {}; }

      if (r.ok) {
        var data = j.data || {};
        if (data.magic_link) {
          statusEl.style.display = 'block';
          statusEl.style.color = '#0f766e';
          if (typeof FC.liveStatus === 'function') {
            FC.liveStatus(statusEl, 'Development link: ');
          } else {
            statusEl.textContent = 'Development link: ';
          }
          var a = document.createElement('a');
          a.className = 'text-mono';
          a.href = String(data.magic_link);
          a.textContent = String(data.magic_link);
          statusEl.appendChild(a);
        } else {
          showStatus('Check your email for the sign-in link.', 'success');
        }
      } else {
        var msg = (j.error && j.error.message) || 'Request failed.';
        showStatus(msg, 'error');
      }
    } catch (err) {
      showStatus('Network error. Please try again.', 'error');
    } finally {
      release();
    }
  });
})();
