// Auto-extracted from contact.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    document.getElementById('contact-form').addEventListener('submit', async (e) => {
      e.preventDefault();
      const fd = new FormData(e.target);
      const status = document.getElementById('form-status');
      status.style.display = 'block';
      status.style.color = '#576a83';
      status.textContent = 'Sending...';
      try {
        const r = await fetch('/api/v1/contact', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            name: fd.get('name'),
            email: fd.get('email'),
            subject: fd.get('subject') || '',
            message: fd.get('message'),
          }),
        });
        const j = await r.json();
        if (r.ok) {
          status.style.color = '#0f766e';
          status.textContent = j.data?.message || 'Thank you. We will get back to you soon.';
          e.target.reset();
        } else {
          status.style.color = '#b91c1c';
          status.textContent = (j.error && j.error.message) ? j.error.message : 'Something went wrong. Please try again.';
        }
      } catch (err) {
        status.style.color = '#b91c1c';
        status.textContent = 'Network error. Please try again.';
      }
    });
  