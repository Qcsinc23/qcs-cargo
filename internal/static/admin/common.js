/**
 * Admin shared utilities. Loaded on every admin page.
 *
 * Exports on window:
 *   QCSAdmin.escapeHTML(str)                    -> HTML-escaped string
 *   QCSAdmin.escapeAttr(str)                    -> attribute-safe escape
 *   QCSAdmin.safeURL(url)                       -> sanitized URL or "#"
 *   QCSAdmin.text(el, str)                      -> el.textContent = str
 *   adminSearchOpen()                           -> Cmd+K modal opener (back-compat)
 *
 * Pass 2 audit fix C-1: server-supplied strings are HTML-escaped on every
 * admin/warehouse render path so a malicious customer-controlled value
 * (e.g. user.name = "<img src=x onerror=...>") cannot execute when an admin
 * opens a list view that includes the value.
 */
(function () {
  function escapeHTML(value) {
    if (value === null || value === undefined) return '';
    var s = String(value);
    var out = '';
    for (var i = 0; i < s.length; i++) {
      var ch = s.charCodeAt(i);
      switch (ch) {
        case 38: out += '&amp;';  break; // &
        case 60: out += '&lt;';   break; // <
        case 62: out += '&gt;';   break; // >
        case 34: out += '&quot;'; break; // "
        case 39: out += '&#39;';  break; // '
        case 96: out += '&#96;';  break; // `
        default: out += s.charAt(i);
      }
    }
    return out;
  }

  function escapeAttr(value) {
    return escapeHTML(value);
  }

  // Returns the URL if it is a same-origin absolute path or an http(s) URL on
  // the same origin; otherwise returns "#". Prevents javascript:, data:, and
  // other non-navigational schemes from sneaking into href/src.
  function safeURL(url) {
    if (url === null || url === undefined) return '#';
    var s = String(url).trim();
    if (s === '') return '#';
    if (s.charAt(0) === '/' && (s.length === 1 || s.charAt(1) !== '/')) return s; // absolute path
    var lower = s.toLowerCase();
    if (lower.indexOf('http://') === 0 || lower.indexOf('https://') === 0) {
      try {
        var parsed = new URL(s);
        if (parsed.origin === window.location.origin) return parsed.pathname + parsed.search + parsed.hash;
      } catch (e) { /* fallthrough */ }
    }
    return '#';
  }

  function text(el, value) {
    if (!el) return;
    el.textContent = (value === null || value === undefined) ? '' : String(value);
  }

  window.QCSAdmin = window.QCSAdmin || {};
  window.QCSAdmin.escapeHTML = escapeHTML;
  window.QCSAdmin.escapeAttr = escapeAttr;
  window.QCSAdmin.safeURL = safeURL;
  window.QCSAdmin.text = text;
})();

/**
 * Admin shared: Cmd+K / Ctrl+K global search modal.
 */
(function () {
  var modal = null;
  var inputEl = null;
  var resultsEl = null;
  var debounceTimer = null;
  var esc = (window.QCSAdmin && window.QCSAdmin.escapeHTML) || function (v) { return String(v == null ? '' : v); };
  var safeURL = (window.QCSAdmin && window.QCSAdmin.safeURL) || function (u) { return String(u == null ? '#' : u); };

  function authHeaders() {
    var token = localStorage.getItem('qcs_access_token');
    return token ? { 'Authorization': 'Bearer ' + token } : {};
  }

  function openModal() {
    if (modal) {
      modal.style.display = 'flex';
      if (inputEl) {
        inputEl.value = '';
        inputEl.focus();
      }
      resultsEl.innerHTML = '<p class="text-slate-500 text-sm p-4">Type to search…</p>';
      return;
    }
    modal = document.createElement('div');
    modal.id = 'admin-search-modal';
    modal.setAttribute('role', 'dialog');
    modal.setAttribute('aria-label', 'Search');
    modal.className = 'fixed inset-0 z-50 flex items-start justify-center pt-[15vh] px-4 bg-black/50';
    modal.style.display = 'flex';
    modal.innerHTML =
      '<div class="bg-white rounded-xl shadow-xl w-full max-w-xl max-h-[70vh] flex flex-col overflow-hidden" onclick="event.stopPropagation()">' +
      '<div class="p-3 border-b border-slate-200 flex items-center gap-2">' +
      '<input type="text" id="admin-search-input" placeholder="Search users, ship requests, packages…" ' +
      'class="flex-1 px-3 py-2 border border-slate-200 rounded-lg focus:ring-2 focus:ring-[#2563EB] focus:border-[#2563EB] outline-none" />' +
      '<button type="button" id="admin-search-close" class="px-3 py-2 text-slate-500 hover:text-slate-700">Esc</button>' +
      '</div>' +
      '<div id="admin-search-results" class="flex-1 overflow-y-auto p-4"></div>' +
      '</div>';
    document.body.appendChild(modal);
    inputEl = document.getElementById('admin-search-input');
    resultsEl = document.getElementById('admin-search-results');
    resultsEl.innerHTML = '<p class="text-slate-500 text-sm">Type to search…</p>';

    inputEl.addEventListener('input', function () {
      clearTimeout(debounceTimer);
      var q = (inputEl.value || '').trim();
      if (!q) {
        resultsEl.innerHTML = '<p class="text-slate-500 text-sm">Type to search…</p>';
        return;
      }
      debounceTimer = setTimeout(function () {
        fetch('/api/v1/admin/search?q=' + encodeURIComponent(q), { headers: authHeaders() })
          .then(function (r) { return r.ok ? r.json() : null; })
          .then(function (data) {
            if (!data) {
              resultsEl.innerHTML = '<p class="text-slate-500 text-sm">Search failed.</p>';
              return;
            }
            renderResults(data);
          })
          .catch(function () {
            resultsEl.innerHTML = '<p class="text-slate-500 text-sm">Search failed.</p>';
          });
      }, 200);
    });

    inputEl.addEventListener('keydown', function (e) {
      if (e.key === 'Escape') closeModal();
    });

    modal.querySelector('#admin-search-close').onclick = closeModal;
    modal.addEventListener('click', function (e) {
      if (e.target === modal) closeModal();
    });
    inputEl.focus();
  }

  function closeModal() {
    if (modal) modal.style.display = 'none';
  }

  function renderResults(data) {
    var users = data.users || [];
    var shipRequests = data.ship_requests || [];
    var lockerPackages = data.locker_packages || [];
    var html = '';
    if (users.length) {
      html += '<p class="text-xs font-medium text-slate-400 uppercase tracking-wide mb-2">Users</p><ul class="mb-4">';
      users.forEach(function (u) {
        var href = safeURL('/admin/users/' + (u.id || ''));
        html += '<li><a href="' + esc(href) + '" class="block py-2 px-3 rounded-lg hover:bg-slate-100 text-[#0F172A]">' +
          esc(u.name || u.email || u.id) + ' <span class="text-slate-500 text-sm">' + esc(u.email || '') + '</span> ' +
          (u.suite_code ? '<span class="font-mono text-xs text-slate-400">' + esc(u.suite_code) + '</span>' : '') + '</a></li>';
      });
      html += '</ul>';
    }
    if (shipRequests.length) {
      html += '<p class="text-xs font-medium text-slate-400 uppercase tracking-wide mb-2">Ship Requests</p><ul class="mb-4">';
      shipRequests.forEach(function (sr) {
        html += '<li><a href="/admin/ship-requests?code=' + encodeURIComponent(sr.confirmation_code || sr.id) + '" class="block py-2 px-3 rounded-lg hover:bg-slate-100 text-[#0F172A]">' +
          esc(sr.confirmation_code || sr.id) + ' <span class="text-slate-500 text-sm">' + esc(sr.status || '') + '</span></a></li>';
      });
      html += '</ul>';
    }
    if (lockerPackages.length) {
      html += '<p class="text-xs font-medium text-slate-400 uppercase tracking-wide mb-2">Locker Packages</p><ul class="mb-4">';
      lockerPackages.forEach(function (p) {
        html += '<li><a href="/admin/locker-packages?suite=' + encodeURIComponent(p.suite_code || '') + '" class="block py-2 px-3 rounded-lg hover:bg-slate-100 text-[#0F172A]">' +
          esc(p.suite_code || p.id) + ' ' + (p.sender_name ? '<span class="text-slate-500 text-sm">' + esc(p.sender_name) + '</span>' : '') + '</a></li>';
      });
      html += '</ul>';
    }
    if (!html) html = '<p class="text-slate-500 text-sm">No results.</p>';
    resultsEl.innerHTML = html;
  }

  document.addEventListener('keydown', function (e) {
    if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
      e.preventDefault();
      openModal();
    }
  });

  window.adminSearchOpen = openModal;
})();
