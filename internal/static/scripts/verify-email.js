// Auto-extracted from verify-email.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    (function () {
      const params = new URLSearchParams(location.search);
      const token = params.get('token');
      const status = document.getElementById('status');

      function safeParseJSON(response) {
        return response.text().then((text) => {
          if (!text) return {};
          try {
            return JSON.parse(text);
          } catch (_) {
            return { raw: text };
          }
        });
      }

      if (!token) {
        status.textContent = 'Missing token. Please use the verification link from your email.';
        return;
      }

      const controller = new AbortController();
      const timeout = setTimeout(() => controller.abort(), 15000);

      fetch('/api/v1/auth/verify-email', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token }),
        signal: controller.signal
      })
        .then((response) => safeParseJSON(response).then((payload) => ({ response, payload })))
        .then(({ response, payload }) => {
          if (response.ok) {
            status.textContent = 'Email verified. Redirecting to sign in...';
            setTimeout(() => {
              window.location.replace('/login?verified=1');
            }, 1200);
            return;
          }
          status.textContent = payload.error?.message || 'This verification link is invalid or expired.';
        })
        .catch((err) => {
          if (err && err.name === 'AbortError') {
            status.textContent = 'Verification is taking too long. Please try the link again.';
            return;
          }
          status.textContent = 'Network error. Please try again.';
        })
        .finally(() => clearTimeout(timeout));
    })();
  