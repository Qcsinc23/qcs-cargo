// Auto-extracted from verify.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    (function () {
      const params = new URLSearchParams(location.search);
      const token = params.get('token');
      const redirectTo = params.get('redirectTo') || '/dashboard';
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

      function withCacheBust(url) {
        return url + (url.includes('?') ? '&' : '?') + 't=' + Date.now();
      }

      async function clearLegacyCaches() {
        try {
          if ('serviceWorker' in navigator) {
            const regs = await navigator.serviceWorker.getRegistrations();
            await Promise.all(regs.map((reg) => reg.unregister()));
          }
          if ('caches' in window) {
            const keys = await caches.keys();
            const target = keys.filter((k) => k.indexOf('qcs-cargo-') === 0);
            await Promise.all(target.map((k) => caches.delete(k)));
          }
        } catch (_) {
          // Ignore cache cleanup errors; continue sign-in redirect.
        }
      }

      if (!token) {
        status.textContent = 'Missing token. Please use the link from your email.';
        return;
      }

      const controller = new AbortController();
      const timeout = setTimeout(() => controller.abort(), 15000);

      fetch('/api/v1/auth/magic-link/verify', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token }),
        credentials: 'include',
        signal: controller.signal
      })
        .then((r) => safeParseJSON(r).then((j) => ({ r, j })))
        .then(({ r, j }) => {
          if (j.data && j.data.access_token) {
            status.textContent = 'Success. Redirecting...';
            try {
              localStorage.setItem('qcs_access_token', j.data.access_token);
              if (j.data.session_id) {
                localStorage.setItem('qcs_session_id', j.data.session_id);
              }
            } catch (_) {
              status.textContent = 'Could not save sign-in state. Please disable private mode and try again.';
              return;
            }
            var redirectScheduled = false;
            function doRedirect() {
              if (redirectScheduled) return;
              redirectScheduled = true;
              window.location.replace(withCacheBust(redirectTo));
            }
            clearLegacyCaches().finally(() => { setTimeout(doRedirect, 100); });
            setTimeout(doRedirect, 4000);
          } else {
            document.querySelector('.page-title').textContent = 'Sign-in failed';
            status.textContent = j.error?.message || (r.ok ? 'Invalid or expired link.' : 'Could not complete sign-in.');
          }
        })
        .catch((err) => {
          document.querySelector('.page-title').textContent = 'Sign-in failed';
          if (err && err.name === 'AbortError') {
            status.textContent = 'Sign-in is taking too long. Please try the link again.';
            return;
          }
          status.textContent = 'Network error. Please try again.';
        })
        .finally(() => clearTimeout(timeout));
    })();
  