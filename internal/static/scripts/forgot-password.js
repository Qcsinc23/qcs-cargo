// Auto-extracted from forgot-password.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    document.getElementById('forgot-form').addEventListener('submit', async (e) => {
      e.preventDefault();
      const email = document.getElementById('email').value;
      const btn = document.getElementById('submit-btn');
      const err = document.getElementById('form-error');
      err.style.display = 'none';
      btn.disabled = true;
      btn.textContent = 'Sending...';

      try {
        await fetch('/api/v1/auth/password/forgot', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ email })
        });
        document.getElementById('sent-email').textContent = email;
        document.getElementById('form-view').style.display = 'none';
        document.getElementById('success-view').style.display = 'block';
      } catch {
        err.textContent = 'Network error. Please try again.';
        err.style.display = 'block';
        btn.disabled = false;
        btn.textContent = 'Send Reset Link';
      }
    });
  