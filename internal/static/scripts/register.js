// Auto-extracted from register.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    const form = document.getElementById('register-form');
    const steps = document.querySelectorAll('.reg-step');
    const dots = document.querySelectorAll('.step-dot');
    const lines = document.querySelectorAll('.step-line');
    const headerTitle = document.querySelector('#registration-header h1');
    const headerText = document.querySelector('#registration-header p');
    const status = document.getElementById('form-status');
    const submitBtn = document.getElementById('register-submit');

    const stepInfo = [
      { title: 'Step 1: Account Details', text: 'Create your credentials to get started.' },
      { title: 'Step 2: Shipping Route', text: 'Choose your primary destination for best rates.' },
      { title: 'Step 3: Verification', text: 'Review international shipping requirements.' },
      { title: 'Verify Your Email', text: 'Use the link in your inbox to activate your account.' }
    ];

    function showStep(stepNum) {
      steps.forEach(s => s.style.display = 'none');
      document.querySelector(`.reg-step[data-step="${stepNum}"]`).style.display = 'block';

      dots.forEach((dot, i) => {
        const dStep = i + 1;
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

    document.querySelectorAll('.next-step').forEach(btn => {
      btn.addEventListener('click', () => {
        const currentStep = btn.closest('.reg-step');
        const stepNum = parseInt(currentStep.getAttribute('data-step'));

        // Validation for step 1
        if (stepNum === 1) {
          const inputs = currentStep.querySelectorAll('input[required]');
          let valid = true;
          inputs.forEach(input => {
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

    document.querySelectorAll('.prev-step').forEach(btn => {
      btn.addEventListener('click', () => {
        const currentStep = btn.closest('.reg-step');
        const stepNum = parseInt(currentStep.getAttribute('data-step'));
        showStep(stepNum - 1);
      });
    });

    form.addEventListener('submit', async (e) => {
      e.preventDefault();
      const formData = new FormData(form);
      const data = Object.fromEntries(formData.entries());
      // Handle prefix
      data.phone = '+592' + data.phone.replace(/[^0-9]/g, '');

      status.style.display = 'block';
      status.style.color = 'var(--muted)';
      status.textContent = 'Creating your account...';
      submitBtn.disabled = true;
      submitBtn.textContent = 'Creating...';

      try {
        const r = await fetch('/api/v1/auth/register', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(data)
        });
        const j = await r.json();
        if (r.ok) {
          const user = j.data?.user || {};
          const email = user.email || data.email;
          const suiteCode = user.suite_code || 'Suite code assigned';
          document.getElementById('suite-code-display').textContent = suiteCode;
          document.getElementById('verification-message').textContent = (j.data?.message || 'Check your inbox for the verification link.') + ' Sent to ' + email + '.';
          status.style.display = 'none';
          showStep(4);
        } else {
          status.style.color = '#b91c1c';
          status.textContent = j.error?.message || 'Registration failed. Please check your details.';
        }
      } catch (err) {
        status.style.color = '#b91c1c';
        status.textContent = 'Connection error. Please try again.';
      } finally {
        submitBtn.disabled = false;
        submitBtn.textContent = 'Agree & Finalize';
      }
    });
  