// Auto-extracted from faq.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    function toggle(btn) {
      const item = btn.closest('.faq-item');
      item.classList.toggle('open');
    }
  