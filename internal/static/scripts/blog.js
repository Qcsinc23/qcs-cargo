// Auto-extracted from blog.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

        document.addEventListener('DOMContentLoaded', () => {
            fetch('/api/v1/blog?limit=50')
                .then(r => r.ok ? r.json() : Promise.reject('Failed load'))
                .then(res => {
                    const grid = document.getElementById('blog-grid');
                    const posts = res.data || [];
                    if (posts.length === 0) {
                        grid.innerHTML = '<p class="muted text-small text-center" style="grid-column: 1 / -1; padding: 40px 0;">We haven\'t published any posts yet. Check back soon!</p>';
                        return;
                    }
                    grid.innerHTML = '';
                    posts.forEach(p => {
                        const colors = ['#e2e8f0', '#cbd5e1', '#94a3b8'];
                        const bgColor = colors[Math.floor(Math.random() * colors.length)];
                        const dateStr = p.published_at ? new Date(p.published_at).toLocaleDateString('en-US', { month: 'long', day: 'numeric', year: 'numeric' }) : '';
                        grid.innerHTML += `
              <article class="card blog-card" style="padding: 0; overflow: hidden; border: none; box-shadow: var(--shadow-1);">
                <a href="/blog/${p.slug}" style="display: block; height: 200px; background: url('/web/images/public/blog-fallback.png') center/cover no-repeat; transition: opacity 0.2s;" onmouseover="this.style.opacity=0.9" onmouseout="this.style.opacity=1"></a>
                <div style="padding: 24px;">
                  <div class="flex-between" style="margin-bottom: 12px;">
                    <span class="text-small" style="color: var(--accent); font-weight: 700; letter-spacing: 0.5px;">${p.category || 'NEWS'}</span>
                    <span class="text-small muted">${dateStr}</span>
                  </div>
                  <h3 style="font-size: 1.35rem; margin-bottom: 12px; line-height: 1.4;">${p.title}</h3>
                  <p class="muted text-small" style="margin-bottom: 24px;">${p.excerpt}</p>
                  <a href="/blog/${p.slug}" style="color: var(--brand); font-weight: 700; font-size: 0.95rem;">Read Article &rarr;</a>
                </div>
              </article>
            `;
                    });
                })
                .catch(err => {
                    document.getElementById('blog-grid').innerHTML = '<p class="muted text-small text-center" style="grid-column: 1 / -1; color: #b91c1c;">Error loading blog posts. Please refresh the page.</p>';
                });
        });
    