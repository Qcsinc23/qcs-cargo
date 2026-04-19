// Auto-extracted from status.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    async function loadStatus() {
      const banner = document.getElementById('overall-banner');
      const statusEl = document.getElementById('overall-status');
      const msgEl = document.getElementById('overall-message');
      document.getElementById('last-updated').textContent = new Date().toLocaleString();
      try {
        const r = await fetch('/api/v1/status');
        const j = await r.json();
        const s = j.data?.status || 'unknown';

        if (s === 'operational') {
          banner.className = 'notice notice-info';
          statusEl.innerHTML = '<strong>All systems operational</strong>';
          msgEl.textContent = j.data?.message || 'No active incidents.';
        } else {
          banner.className = 'notice notice-warn';
          statusEl.innerHTML = '<strong>Degraded performance</strong>';
          msgEl.textContent = j.data?.message || 'Some services may be affected.';
        }

        const h = await fetch('/api/v1/health');
        const hj = await h.json();
        setService('svc-api', h.ok ? 'operational' : 'degraded');
        setService('svc-db', hj.db === 'ok' ? 'operational' : 'degraded');
        setService('svc-email', 'operational');
        setService('svc-payment', 'operational');
      } catch (e) {
        banner.className = 'notice notice-danger';
        statusEl.innerHTML = '<strong>Service disruption</strong>';
        msgEl.textContent = 'Unable to reach status endpoints. Please try again shortly.';
        ['svc-api', 'svc-db', 'svc-email', 'svc-payment'].forEach((id) => setService(id, 'unknown'));
      }
    }

    function setService(id, status) {
      const el = document.getElementById(id);
      if (!el) return;

      const map = {
        operational: ['ok', 'Operational'],
        degraded: ['warn', 'Degraded'],
        down: ['err', 'Down'],
        unknown: ['unk', 'Unknown'],
      };
      const entry = map[status] || map.unknown;
      el.innerHTML = '<span class="dot ' + entry[0] + '"></span>' + entry[1];
    }

    loadStatus();
  