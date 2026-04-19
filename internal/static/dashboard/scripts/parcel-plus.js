// Auto-extracted from parcel-plus.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    (function () {
      'use strict';
      var QCS = window.QCSPWA;
      QCS.initBase({ registerSW: true, keyboard: true, utilityDock: true });
      var token = QCS.readToken();
      if (!token) { window.location.href = '/login?redirect=' + encodeURIComponent('/dashboard/parcel-plus'); return; }
      var app = document.getElementById('app');
      QCS.mountLoading(app, 'Loading parcel services…');

      Promise.all([
        QCS.fetchJson('/api/v1/me'),
        QCS.fetchJson('/api/v1/parcel/photos').catch(function () { return { data: [] }; }),
        QCS.fetchJson('/api/v1/parcel/loyalty-summary').catch(function () { return { data: {} }; }),
        QCS.fetchJson('/api/v1/locker').catch(function () { return { data: [] }; })
      ]).then(function (results) {
        var me = results[0] && results[0].data;
        var photos = results[1] && results[1].data || [];
        var loyalty = results[2] && results[2].data || {};
        var locker = results[3] && results[3].data || [];
        if (!me) { window.location.href = '/login'; return; }

        var ids = locker.slice(0, 5).map(function (x) { return x.id; });
        var previewPromise = ids.length >= 2
          ? QCS.fetchJson('/api/v1/parcel/consolidation-preview', { method: 'POST', body: { locker_package_ids: ids, destination_id: 'guyana' } })
              .catch(function () { return null; })
          : Promise.resolve(null);

        previewPromise.then(function (preview) {
          render(photos, loyalty, preview && preview.data);
        });
      }).catch(function (err) {
        QCS.mountError(app, {
          title: 'Could not load parcel services',
          description: (err && err.message) || 'Network error.',
          onRetry: function () { window.location.reload(); }
        });
      });

      function render(photos, loyalty, preview) {
        var sidebar = QCS.renderSidebar('parcel-plus');

        // Photo cards: ESCAPE every server-supplied field. file_url is run
        // through QCS.safeURL so a malicious javascript:/data: URL cannot end
        // up in <img src>. (Audit fix C-1 / H-5 follow-on for the customer UI.)
        var photoCards = '';
        photos.slice(0, 8).forEach(function (pkg) {
          var firstPhoto = pkg.photos && pkg.photos[0] && pkg.photos[0].photo_url;
          var safeImg = firstPhoto ? QCS.safeURL(firstPhoto) : '';
          photoCards += '<article class="bg-white rounded-xl border border-slate-200 p-4 shadow-sm">'
            + '<p class="font-medium mb-2 truncate">' + window.qcsEscapeHTML(pkg.sender_name || 'Package') + '</p>'
            + (safeImg && safeImg !== '#'
                ? '<img src="' + window.qcsEscapeHTML(safeImg) + '" alt="Package photo" loading="lazy" class="rounded-lg border border-slate-200 mb-3 w-full h-40 object-cover" />'
                : '<div class="rounded-lg bg-slate-100 mb-3 w-full h-40 flex items-center justify-center text-slate-400">📷</div>')
            + '<p class="text-sm text-slate-500 truncate">' + window.qcsEscapeHTML(pkg.locker_package_id || '') + '</p>'
            + '</article>';
        });

        var consolidation = preview
          ? '<dl class="grid grid-cols-3 gap-4 text-center">'
            + statLine('Packages', String(preview.package_count || 0))
            + statLine('Saved (lbs)', String(Math.round((preview.estimated_savings_lbs || 0) * 10) / 10))
            + statLine('Billable (lbs)', String(Math.round((preview.post_consolidation_billable_lbs || 0) * 10) / 10))
            + '</dl>'
            + '<a href="/dashboard/ship" class="inline-block mt-4 bg-[#F97316] text-white px-4 py-2 rounded-lg font-medium">Ship consolidated →</a>'
          : '<p class="text-slate-500">Need at least 2 packages in your locker for a preview.</p>'
            + '<a href="/dashboard/inbox" class="inline-block mt-3 text-[#2563EB] font-medium">Check my packages →</a>';

        var tier = String(loyalty.tier || 'basic');
        var pts = Number(loyalty.current_points || 0);
        var next = Number(loyalty.next_tier_at || 500);
        var pct = Math.min(100, Math.round((pts / Math.max(1, next)) * 100));

        app.innerHTML = '<div class="qcs-page-wrap">' + sidebar
          + '<main id="qcs-main" class="qcs-page-main" tabindex="-1">'
          + '<nav aria-label="Breadcrumb" class="mb-4"><ol class="flex items-center gap-2 text-sm text-slate-600"><li><a href="/dashboard" class="text-[#2563EB] hover:underline">Dashboard</a></li><li aria-hidden="true">/</li><li class="text-[#0F172A] font-medium" aria-current="page">Parcel+</li></ol></nav>'
          + '<div class="flex flex-wrap items-end justify-between gap-3 mb-6">'
          + '<div><p class="text-slate-500 text-xs uppercase tracking-[0.2em]">Parcel+</p>'
          + '<h1 class="text-3xl font-bold">Consolidation, photos, and loyalty</h1></div>'
          + '</div>'

          + '<section class="grid lg:grid-cols-3 gap-4 mb-6">'
          + '<article class="bg-white rounded-xl border border-slate-200 p-6 shadow-sm">'
          + '<h2 class="text-lg font-semibold mb-3">Consolidation preview</h2>' + consolidation
          + '</article>'
          + '<article class="bg-white rounded-xl border border-slate-200 p-6 shadow-sm">'
          + '<h2 class="text-lg font-semibold mb-3">Loyalty</h2>'
          + '<div class="flex items-center justify-between mb-2"><div><p class="text-2xl font-bold">' + window.qcsEscapeHTML(String(pts)) + ' <span class="text-sm text-slate-500 font-normal">pts</span></p>'
          + '<p class="text-xs text-slate-500 capitalize">Current tier: ' + window.qcsEscapeHTML(tier) + '</p></div>'
          + '<span class="qcs-badge qcs-badge-info">' + window.qcsEscapeHTML(tier) + '</span></div>'
          + '<div class="h-2 bg-slate-200 rounded-full overflow-hidden mt-2"><div class="h-full bg-[#2563EB]" style="width:' + pct + '%"></div></div>'
          + '<p class="text-xs text-slate-500 mt-1">' + (next - pts) + ' pts to next tier</p>'
          + '</article>'
          + '<article class="bg-white rounded-xl border border-slate-200 p-6 shadow-sm">'
          + '<h2 class="text-lg font-semibold mb-3">Quick actions</h2>'
          + '<div class="grid gap-2 text-sm">'
          + actionLink('/dashboard/inbox', '📦 View package inbox')
          + actionLink('/dashboard/ship', '✈️ Create ship request')
          + actionLink('/dashboard/settings/notifications', '🔔 Notification preferences')
          + '</div></article>'
          + '</section>'

          + '<section><h2 class="text-2xl font-semibold mb-4">Package photos</h2>'
          + '<div class="grid sm:grid-cols-2 lg:grid-cols-4 gap-4">'
          + (photoCards || '<p class="text-slate-500 col-span-full">No package photos available yet. Photos appear automatically when our warehouse logs your incoming packages.</p>')
          + '</div></section>'

          + '</main></div>';

        QCS.bindLogout();
      }

      function statLine(label, value) {
        return '<div><dt class="text-xs text-slate-500 uppercase tracking-wider">' + window.qcsEscapeHTML(label) + '</dt>'
          + '<dd class="text-xl font-bold">' + window.qcsEscapeHTML(value) + '</dd></div>';
      }
      function actionLink(href, label) {
        return '<a href="' + window.qcsEscapeHTML(href) + '" class="block py-2 px-3 rounded-lg border border-slate-200 hover:bg-slate-50 hover:border-slate-300 focus:outline-none focus-visible:ring-2 focus-visible:ring-[#2563EB]">' + window.qcsEscapeHTML(label) + '</a>';
      }
    })();
  