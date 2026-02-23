window.qcsEscapeHTML = function (str) {
  if (typeof str !== 'string') return str || '';
  const map = {
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#039;'
  };
  return str.replace(/[&<>"']/g, function (m) { return map[m] || m; });
};

(function () {
  const MOBILE_MAX = 1023;

  function inferTitle(main) {
    if (!main) return 'Dashboard';
    const h1 = main.querySelector('h1');
    if (h1 && h1.textContent.trim()) return h1.textContent.trim();
    const currentCrumb = main.querySelector('nav[aria-label="Breadcrumb"] [aria-current="page"]');
    if (currentCrumb && currentCrumb.textContent.trim()) return currentCrumb.textContent.trim();
    return 'Dashboard';
  }

  function closeNav(shell) {
    shell.classList.remove('qcs-nav-open');
    document.body.classList.remove('qcs-nav-lock');
  }

  function openNav(shell) {
    shell.classList.add('qcs-nav-open');
    document.body.classList.add('qcs-nav-lock');
  }

  function syncViewportState(shell) {
    if (window.innerWidth > MOBILE_MAX) {
      closeNav(shell);
    }
  }

  function ensureTopbar(shell, main) {
    let topbar = shell.querySelector(':scope > .qcs-mobile-topbar');
    if (!topbar) {
      topbar = document.createElement('div');
      topbar.className = 'qcs-mobile-topbar';
      topbar.innerHTML =
        '<button type="button" class="qcs-mobile-menu-btn" aria-label="Open navigation" aria-expanded="false">☰</button>' +
        '<div class="qcs-mobile-title">Dashboard</div>' +
        '<a class="qcs-mobile-home" href="/dashboard">QCS</a>';
      shell.insertBefore(topbar, main || shell.firstChild);

      const menuBtn = topbar.querySelector('.qcs-mobile-menu-btn');
      menuBtn.addEventListener('click', function () {
        const open = shell.classList.contains('qcs-nav-open');
        if (open) {
          closeNav(shell);
          menuBtn.setAttribute('aria-expanded', 'false');
        } else {
          openNav(shell);
          menuBtn.setAttribute('aria-expanded', 'true');
        }
      });
    }

    const titleEl = topbar.querySelector('.qcs-mobile-title');
    if (titleEl) titleEl.textContent = inferTitle(main);
    return topbar;
  }

  function ensureOverlay(shell) {
    let overlay = shell.querySelector(':scope > .qcs-sidebar-overlay');
    if (!overlay) {
      overlay = document.createElement('div');
      overlay.className = 'qcs-sidebar-overlay';
      overlay.addEventListener('click', function () {
        closeNav(shell);
      });
      shell.appendChild(overlay);
    }
    return overlay;
  }

  function enhanceShell(shell) {
    if (!shell || shell.dataset.mobileNavEnhanced === '1') return;

    const aside = shell.querySelector(':scope > aside, aside');
    const main = shell.querySelector(':scope > main, main');
    if (!aside || !main) return;

    shell.dataset.mobileNavEnhanced = '1';
    shell.classList.add('qcs-shell');
    aside.classList.add('qcs-sidebar');
    main.classList.add('qcs-main');

    ensureTopbar(shell, main);
    ensureOverlay(shell);

    aside.addEventListener('click', function (event) {
      const link = event.target.closest('a[href]');
      if (!link) return;
      if (window.innerWidth <= MOBILE_MAX) {
        closeNav(shell);
      }
    });

    syncViewportState(shell);
  }

  function scanAndEnhance() {
    const shells = document.querySelectorAll('.flex.min-h-screen');
    shells.forEach(enhanceShell);

    // Keep the topbar title synced if main content is replaced dynamically.
    document.querySelectorAll('.qcs-shell').forEach((shell) => {
      const main = shell.querySelector('main');
      const title = shell.querySelector('.qcs-mobile-title');
      if (main && title) {
        const newTitle = inferTitle(main);
        if (title.textContent !== newTitle) {
          title.textContent = newTitle;
        }
      }
    });
  }

  const observer = new MutationObserver(function () {
    scanAndEnhance();
  });

  observer.observe(document.body, { childList: true, subtree: true });
  window.addEventListener('resize', function () {
    document.querySelectorAll('.qcs-shell').forEach(syncViewportState);
  });

  scanAndEnhance();
})();
