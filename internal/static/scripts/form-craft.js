// Minimum form-craft helpers used by public auth pages.
// Exported as plain globals on window.QcsFormCraft for CSP-friendly consumption.
(function(){
  function escHtml(s) {
    return String(s == null ? '' : s)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }
  function setBusy(form) {
    if (!form) return function(){};
    var submitBtn = form.querySelector('button[type="submit"], input[type="submit"]');
    var originalLabel = submitBtn ? (submitBtn.textContent || submitBtn.value) : null;
    if (submitBtn) {
      submitBtn.disabled = true;
      submitBtn.setAttribute('aria-busy', 'true');
      if (submitBtn.tagName === 'BUTTON') submitBtn.textContent = 'Working…';
    }
    return function release() {
      if (submitBtn) {
        submitBtn.disabled = false;
        submitBtn.removeAttribute('aria-busy');
        if (originalLabel != null && submitBtn.tagName === 'BUTTON') submitBtn.textContent = originalLabel;
      }
    };
  }
  function liveStatus(el, msg, opts) {
    if (!el) return;
    el.textContent = msg == null ? '' : String(msg);
    el.setAttribute('role', 'status');
    el.setAttribute('aria-live', (opts && opts.assertive) ? 'assertive' : 'polite');
  }
  function persistEmail(form, key) {
    if (!form) return;
    key = key || 'qcs:last-email';
    var input = form.querySelector('input[type="email"], input[name="email"]');
    if (!input) return;
    try {
      var saved = sessionStorage.getItem(key);
      if (saved && !input.value) input.value = saved;
    } catch(e) {}
    input.addEventListener('change', function(){
      try { sessionStorage.setItem(key, input.value || ''); } catch(e){}
    });
  }
  window.QcsFormCraft = { escHtml: escHtml, setBusy: setBusy, liveStatus: liveStatus, persistEmail: persistEmail };
})();
