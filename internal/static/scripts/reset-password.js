// Auto-extracted from reset-password.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    const token = new URLSearchParams(location.search).get('token');
    const formView = document.getElementById('form-view');
    const invalidView = document.getElementById('invalid-token');
    const successView = document.getElementById('success-view');

    if (!token) {
      formView.style.display = 'none';
      invalidView.style.display = 'block';
    }

    document.getElementById('reset-form').addEventListener('submit', async (e) => {
      e.preventDefault();
      const password = document.getElementById('password').value;
      const confirm = document.getElementById('confirm').value;
      const err = document.getElementById('form-error');
      const btn = document.getElementById('submit-btn');
      err.style.display = 'none';

      if (password !== confirm) {
        err.textContent = 'Passwords do not match.';
        err.style.display = 'block';
        return;
      }

      btn.disabled = true;
      btn.textContent = 'Updating...';

      try {
        const r = await fetch('/api/v1/auth/password/reset', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ token, password })
        });
        const j = await r.json();

        if (!r.ok) {
          if (r.status === 400 && j.error?.code === 'INVALID_TOKEN') {
            formView.style.display = 'none';
            invalidView.style.display = 'block';
          } else {
            err.textContent = j.error?.message || 'Update failed.';
            err.style.display = 'block';
            btn.disabled = false;
            btn.textContent = 'Update Password';
          }
          return;
        }

        formView.style.display = 'none';
        successView.style.display = 'block';
      } catch {
        err.textContent = 'Network error. Please try again.';
        err.style.display = 'block';
        btn.disabled = false;
        btn.textContent = 'Update Password';
      }
    });
  