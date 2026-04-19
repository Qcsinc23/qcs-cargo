// Auto-extracted from index.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    // Fetch latest blog posts for homepage
    document.addEventListener('DOMContentLoaded', () => {
      const grid = document.getElementById('latest-posts-grid');
      if (!grid) return;

      fetch('/api/v1/blog?limit=3')
        .then(r => r.ok ? r.json() : Promise.reject('Failed load'))
        .then(res => {
          const posts = res.data || [];
          if (posts.length === 0) {
            grid.innerHTML = '<p class="muted text-small text-center" style="grid-column: 1 / -1; padding: 40px 0;">Check back soon for news and updates.</p>';
            return;
          }
          grid.innerHTML = '';
          posts.forEach(p => {
            const colors = ['#e2e8f0', '#cbd5e1', '#94a3b8'];
            const bgColor = colors[Math.floor(Math.random() * colors.length)];
            const html = `
              <article class="card blog-card" style="padding: 0; overflow: hidden; border: none; box-shadow: var(--shadow-1);">
                <div style="height: 180px; background: url('/web/images/public/blog-fallback.png') center/cover no-repeat;"></div>
                <div style="padding: 24px;">
                  <p class="text-small" style="color: var(--accent); font-weight: 700; margin-bottom: 8px; text-transform: uppercase;">
                    ${p.category || 'NEWS'}
                  </p>
                  <h3 style="font-size: 1.25rem; margin-bottom: 12px; line-height: 1.4;">${p.title}</h3>
                  <p class="muted text-small" style="margin-bottom: 20px;">${p.excerpt}</p>
                  <a href="/blog/${p.slug}" style="color: var(--brand); font-weight: 700; font-size: 0.9rem;">
                    Read More &rarr;
                  </a>
                </div>
              </article>
            `;
            grid.innerHTML += html;
          });
        })
        .catch(err => {
          grid.innerHTML = '<p class="muted text-small text-center" style="grid-column: 1 / -1; font-style: italic;">Unable to load recent posts.</p>';
        });
    });
  