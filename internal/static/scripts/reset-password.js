// Auto-extracted from reset-password.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).
// Phase 3.4 (CRF-001): wired through QcsFormCraft for shared busy/aria-live
// behavior.

(function () {
  'use strict';

  var token = new URLSearchParams(location.search).get('token');
  var formView = document.getElementById('form-view');
  var invalidView = document.getElementById('invalid-token');
  var successView = document.getElementById('success-view');
  var form = document.getElementById('reset-form');
  var err = document.getElementById('form-error');
  if (!formView || !invalidView || !successView || !form || !err) return;

  var FC = window.QcsFormCraft || {};

  if (!token) {
    formView.style.display = 'none';
    invalidView.style.display = 'block';
  }

  function showError(text) {
    err.style.display = 'block';
    if (typeof FC.liveStatus === 'function') {
      FC.liveStatus(err, text);
    } else {
      err.textContent = text;
    }
  }

  form.addEventListener('submit', async function (e) {
    e.preventDefault();
    var password = document.getElementById('password').value;
    var confirm = document.getElementById('confirm').value;
    err.style.display = 'none';

    if (password !== confirm) {
      showError('Passwords do not match.');
      return;
    }

    var release = typeof FC.setBusy === 'function'
      ? FC.setBusy(form)
      : function () {};

    try {
      var r = await fetch('/api/v1/auth/password/reset', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token: token, password: password })
      });
      var j = {};
      try { j = await r.json(); } catch (parseErr) { j = {}; }

      if (!r.ok) {
        if (r.status === 400 && j.error && j.error.code === 'INVALID_TOKEN') {
          formView.style.display = 'none';
          invalidView.style.display = 'block';
        } else {
          showError((j.error && j.error.message) || 'Update failed.');
          release();
        }
        return;
      }

      formView.style.display = 'none';
      successView.style.display = 'block';
      release();
    } catch (netErr) {
      showError('Network error. Please try again.');
      release();
    }
  });
})();
