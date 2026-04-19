// Auto-extracted from login.html (Phase 2.4 / SEC-001a).
// Phase 3.4 (CRF-001) form-craft pass:
//   - Disable submit while the request is in flight; restore on response.
//   - Persist the typed email across navigation via sessionStorage so a
//     bounced "go fix verification" detour does not lose the user's typing.
//   - Status updates use textContent (not innerHTML); the dev-only
//     "magic_link" branch builds a real anchor element instead of writing
//     attacker-influenceable HTML.

(function () {
  'use strict';

  var STORAGE_KEY = 'qcs_login_email';
  var emailInput = document.getElementById('login-email');
  var submitBtn = document.getElementById('login-submit');
  var statusEl = document.getElementById('form-status');
  var form = document.getElementById('login-form');
  if (!emailInput || !submitBtn || !statusEl || !form) return;

  // Restore last-typed email so a back/forward navigation does not erase
  // the user's input.
  try {
    var saved = sessionStorage.getItem(STORAGE_KEY);
    if (saved && !emailInput.value) emailInput.value = saved;
  } catch (e) { /* sessionStorage unavailable */ }
  emailInput.addEventListener('input', function () {
    try { sessionStorage.setItem(STORAGE_KEY, emailInput.value); } catch (e) {}
  });

  function setStatus(text, tone) {
    statusEl.style.display = 'block';
    statusEl.style.color = tone === 'error' ? '#b91c1c'
      : tone === 'success' ? '#0f766e'
      : '#576a83';
    statusEl.textContent = text;
  }

  function setBusy(busy) {
    submitBtn.disabled = busy;
    submitBtn.setAttribute('aria-busy', busy ? 'true' : 'false');
    submitBtn.textContent = busy ? 'Sending...' : 'Send magic link';
  }

  form.addEventListener('submit', async function (e) {
    e.preventDefault();
    var email = emailInput.value;
    setBusy(true);
    setStatus('Sending...', 'info');

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
        // Dev-only echo of the magic link. Build the anchor as a real DOM
        // element so attacker-influenceable URLs cannot inject HTML.
        if (data.magic_link) {
          statusEl.style.display = 'block';
          statusEl.style.color = '#0f766e';
          statusEl.textContent = 'Development link: ';
          var a = document.createElement('a');
          a.className = 'text-mono';
          a.href = String(data.magic_link);
          a.textContent = String(data.magic_link);
          statusEl.appendChild(a);
        } else {
          setStatus('Check your email for the sign-in link.', 'success');
        }
      } else {
        var msg = (j.error && j.error.message) || 'Request failed.';
        setStatus(msg, 'error');
      }
    } catch (err) {
      setStatus('Network error. Please try again.', 'error');
    } finally {
      setBusy(false);
    }
  });
})();
