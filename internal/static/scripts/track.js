// Auto-extracted from track.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    const params = new URLSearchParams(location.search);
    const input = document.getElementById('track-input');
    const result = document.getElementById('result');

    if (params.get('q')) {
      input.value = params.get('q');
      doTrack(params.get('q'));
    }

    document.getElementById('track-form').addEventListener('submit', (e) => {
      e.preventDefault();
      doTrack(input.value.trim());
    });

    async function doTrack(number) {
      if (!number) return;
      result.innerHTML = '<div class="notice notice-info">Looking up shipment...</div>';
      try {
        const r = await fetch('/api/v1/track/' + encodeURIComponent(number));
        const j = await r.json();
        if (!r.ok) {
          result.innerHTML = `
            <div class="notice notice-warn">
              <p><strong>Shipment not found.</strong> No match for <span class="text-mono">${escHtml(number)}</span>. Check the code or <a href="/contact">contact support</a>.</p>
            </div>`;
          return;
        }

        const d = j.data || {};
        const statusMap = {
          delivered: ['Delivered', 'ok'],
          in_transit: ['In Transit', 'ok'],
          processing: ['Processing', 'warn'],
          pending: ['Pending', 'warn'],
          cancelled: ['Cancelled', 'err'],
          exception: ['Exception', 'err'],
        };
        const entry = statusMap[d.status] || [d.status || 'Unknown', 'unk'];
        result.innerHTML = `
          <article class="card">
            <p class="text-small text-mono">${escHtml(d.tracking_number || number)}</p>
            <div class="status-line" style="margin:8px 0 12px;"><span class="dot ${entry[1]}"></span><strong>${entry[0]}</strong></div>
            <div class="grid-2">
              ${d.destination_id ? `<div><p class="text-small">Destination</p><p>${escHtml(d.destination_id)}</p></div>` : ''}
              ${d.carrier ? `<div><p class="text-small">Carrier</p><p>${escHtml(d.carrier)}</p></div>` : ''}
              ${d.total_weight ? `<div><p class="text-small">Weight</p><p>${d.total_weight} lbs</p></div>` : ''}
              ${d.package_count ? `<div><p class="text-small">Packages</p><p>${d.package_count}</p></div>` : ''}
              ${d.estimated_delivery ? `<div><p class="text-small">Estimated delivery</p><p>${escHtml(d.estimated_delivery)}</p></div>` : ''}
            </div>
          </article>`;
      } catch (e) {
        result.innerHTML = '<div class="notice notice-danger"><p>Network error. Please try again.</p></div>';
      }
    }

    function escHtml(s) {
      return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
    }
  