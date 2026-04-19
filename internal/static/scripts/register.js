// Auto-extracted from register.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).
// Phase 3.4 (CRF-001): wired through QcsFormCraft for shared busy/aria-live
// behavior. Note: register has no email-persist call by design (the
// multi-step wizard owns its own state and we don't want to leak a
// half-typed email between sessions on shared devices).

(function () {
  'use strict';

  var form = document.getElementById('register-form');
  var steps = document.querySelectorAll('.reg-step');
  var dots = document.querySelectorAll('.step-dot');
  var lines = document.querySelectorAll('.step-line');
  var headerTitle = document.querySelector('#registration-header h1');
  var headerText = document.querySelector('#registration-header p');
  var status = document.getElementById('form-status');
  var submitBtn = document.getElementById('register-submit');
  if (!form || !status || !submitBtn) return;

  var FC = window.QcsFormCraft || {};

  var stepInfo = [
    { title: 'Step 1: Account Details', text: 'Create your credentials to get started.' },
    { title: 'Step 2: Shipping Route', text: 'Choose your primary destination for best rates.' },
    { title: 'Step 3: Verification', text: 'Review international shipping requirements.' },
    { title: 'Verify Your Email', text: 'Use the link in your inbox to activate your account.' }
  ];

  function showStep(stepNum) {
    steps.forEach(function (s) { s.style.display = 'none'; });
    document.querySelector('.reg-step[data-step="' + stepNum + '"]').style.display = 'block';

    dots.forEach(function (dot, i) {
      var dStep = i + 1;
      dot.classList.remove('active', 'completed');
      if (dStep < stepNum) dot.classList.add('completed');
      if (dStep === stepNum) dot.classList.add('active');

      if (i < lines.length) {
        lines[i].classList.remove('active');
        if (dStep < stepNum) lines[i].classList.add('active');
      }
    });

    headerTitle.textContent = stepInfo[stepNum - 1].title;
    headerText.textContent = stepInfo[stepNum - 1].text;

    if (stepNum === 4) {
      document.getElementById('step-indicator').style.display = 'none';
      document.getElementById('login-link').style.display = 'none';
    }
  }

  function setStatus(text, tone) {
    status.style.display = 'block';
    status.style.color = tone === 'error' ? '#b91c1c'
      : tone === 'success' ? '#0f766e'
      : 'var(--muted)';
    if (typeof FC.liveStatus === 'function') {
      FC.liveStatus(status, text);
    } else {
      status.textContent = text;
    }
  }

  document.querySelectorAll('.next-step').forEach(function (btn) {
    btn.addEventListener('click', function () {
      var currentStep = btn.closest('.reg-step');
      var stepNum = parseInt(currentStep.getAttribute('data-step'), 10);

      if (stepNum === 1) {
        var inputs = currentStep.querySelectorAll('input[required]');
        var valid = true;
        inputs.forEach(function (input) {
          if (!input.checkValidity()) {
            input.reportValidity();
            valid = false;
          }
        });
        if (!valid) return;
      }

      showStep(stepNum + 1);
    });
  });

  document.querySelectorAll('.prev-step').forEach(function (btn) {
    btn.addEventListener('click', function () {
      var currentStep = btn.closest('.reg-step');
      var stepNum = parseInt(currentStep.getAttribute('data-step'), 10);
      showStep(stepNum - 1);
    });
  });

  form.addEventListener('submit', async function (e) {
    e.preventDefault();
    var formData = new FormData(form);
    var data = Object.fromEntries(formData.entries());
    data.phone = '+592' + String(data.phone || '').replace(/[^0-9]/g, '');

    setStatus('Creating your account...', 'info');
    var release = typeof FC.setBusy === 'function'
      ? FC.setBusy(form)
      : function () {};

    try {
      var r = await fetch('/api/v1/auth/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
      });
      var j = {};
      try { j = await r.json(); } catch (parseErr) { j = {}; }
      if (r.ok) {
        var user = (j.data && j.data.user) || {};
        var email = user.email || data.email;
        var suiteCode = user.suite_code || 'Suite code assigned';
        document.getElementById('suite-code-display').textContent = suiteCode;
        document.getElementById('verification-message').textContent =
          ((j.data && j.data.message) || 'Check your inbox for the verification link.') +
          ' Sent to ' + email + '.';
        status.style.display = 'none';
        showStep(4);
      } else {
        setStatus((j.error && j.error.message) || 'Registration failed. Please check your details.', 'error');
      }
    } catch (err) {
      setStatus('Connection error. Please try again.', 'error');
    } finally {
      release();
    }
  });
})();
