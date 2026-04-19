// Auto-extracted from blog-post.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

        document.addEventListener('DOMContentLoaded', () => {
            // The slug is everything after /blog/
            const pathParts = window.location.pathname.split('/');
            const slug = pathParts[pathParts.length - 1] || pathParts[pathParts.length - 2];

            fetch('/api/v1/blog/' + slug)
                .then(r => r.ok ? r.json() : Promise.reject('Failed load'))
                .then(res => {
                    const post = res.data;
                    document.getElementById('loading').style.display = 'none';
                    document.getElementById('post-article').style.display = 'block';

                    document.title = post.title + ' | QCS Cargo';
                    document.getElementById('bc-title').textContent = post.title;
                    document.getElementById('post-title').textContent = post.title;
                    document.getElementById('post-category').textContent = post.category || 'NEWS';

                    if (post.published_at) {
                        const dt = new Date(post.published_at);
                        document.getElementById('post-date').textContent = dt.toLocaleDateString('en-US', { month: 'long', day: 'numeric', year: 'numeric' });
                    } else {
                        document.getElementById('post-date').textContent = 'Draft';
                    }

                    // Render Markdown
                    if (post.content_md) {
                        document.getElementById('post-content').innerHTML = marked.parse(post.content_md);
                    }
                })
                .catch(err => {
                    document.getElementById('loading').style.display = 'none';
                    document.getElementById('error').style.display = 'block';
                });
        });
    