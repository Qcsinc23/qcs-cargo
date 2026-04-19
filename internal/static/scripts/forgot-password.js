// Auto-extracted from forgot-password.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).
// Phase 3.4 (CRF-001): wired through QcsFormCraft for shared busy/aria-live
// behavior and cross-page email persistence.

(function () {
  'use strict';

  var form = document.getElementById('forgot-form');
  var emailInput = document.getElementById('email');
  var err = document.getElementById('form-error');
  if (!form || !emailInput || !err) return;

  var FC = window.QcsFormCraft || {};

  if (typeof FC.persistEmail === 'function') {
    FC.persistEmail(form);
  }

  form.addEventListener('submit', async function (e) {
    e.preventDefault();
    var email = emailInput.value;
    err.style.display = 'none';
    var release = typeof FC.setBusy === 'function'
      ? FC.setBusy(form)
      : function () {};

    try {
      await fetch('/api/v1/auth/password/forgot', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: email })
      });
      document.getElementById('sent-email').textContent = email;
      document.getElementById('form-view').style.display = 'none';
      document.getElementById('success-view').style.display = 'block';
      release();
    } catch (netErr) {
      err.style.display = 'block';
      if (typeof FC.liveStatus === 'function') {
        FC.liveStatus(err, 'Network error. Please try again.');
      } else {
        err.textContent = 'Network error. Please try again.';
      }
      release();
    }
  });
})();
