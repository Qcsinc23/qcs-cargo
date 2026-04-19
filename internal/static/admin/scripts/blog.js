// Auto-extracted from blog.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

        (function () {
            const token = localStorage.getItem('qcs_access_token');
            if (!token) { window.location.href = '/login?redirect=' + encodeURIComponent('/admin/blog'); return; }
            const auth = { headers: { 'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json' } };

            function adminSidebar(active) {
                var notif = '<div class="relative mb-4"><button type="button" id="admin-notification-btn" class="p-2 rounded-lg hover:bg-[#1E293B] text-white" aria-label="Notifications">' +
                    '<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"></path><path d="M13.73 21a2 2 0 0 1-3.46 0"></path></svg></button>' +
                    '<div id="admin-notification-dropdown" class="hidden absolute left-0 top-full mt-1 w-64 bg-white rounded-lg shadow-lg border border-slate-200 py-2 z-10">' +
                    '<p class="px-4 py-2 text-sm font-medium text-slate-700 border-b border-slate-100">Notifications</p><p class="px-4 py-4 text-slate-500 text-sm">No new notifications</p></div></div>';
                return '<div class="flex min-h-screen"><aside class="w-64 bg-[#0F172A] text-white py-6 px-4 shrink-0">' +
                    '<a href="/" class="font-bold text-lg block mb-2">QCS Cargo</a>' + notif + '<p class="text-amber-400 text-sm font-medium mb-2">Admin</p>' +
                    '<nav class="space-y-1">' +
                    '<a href="/admin" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Dashboard</a>' +
                    '<a href="/admin/ship-requests" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Ship Requests</a>' +
                    '<a href="/admin/locker-packages" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Locker Packages</a>' +
                    '<a href="/admin/service-queue" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Service Queue</a>' +
                    '<a href="/admin/unmatched" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Unmatched</a>' +
                    '<a href="/admin/bookings" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Bookings</a>' +
                    '<a href="/admin/users" class="block py-2 px-3 rounded-lg hover:bg-[#1E293B]">Users</a>' +
                    '<a href="/admin/blog" class="block py-2 px-3 rounded-lg ' + (active === 'blog' ? 'bg-[#1E293B]' : 'hover:bg-[#1E293B]') + '">Blog Posts</a>' +
                    '</nav><a href="/dashboard" class="block mt-6 text-sm text-slate-400 hover:text-white">Customer Dashboard</a>' +
                    '<button type="button" id="logout-btn" class="mt-4 text-sm text-slate-400 hover:text-white">Sign out</button></aside>';
            }

            function bindNotificationDropdown() {
                var notifBtn = document.getElementById('admin-notification-btn');
                var notifDrop = document.getElementById('admin-notification-dropdown');
                if (notifBtn && notifDrop) {
                    notifBtn.onclick = function () { notifDrop.classList.toggle('hidden'); };
                    document.addEventListener('click', function (e) { if (!notifDrop.contains(e.target) && !notifBtn.contains(e.target)) notifDrop.classList.add('hidden'); });
                }
            }

            function renderApp() {
                fetch('/api/v1/me', { headers: { 'Authorization': 'Bearer ' + token } }).then(r => { if (!r.ok) { window.location.href = '/login'; return null; } return r.json(); }).then(me => {
                    if (!me || me.data.role !== 'admin') { window.location.href = '/dashboard'; return; }
                    loadPosts();
                });
            }

            let posts = [];

            function loadPosts() {
                fetch('/api/v1/admin/blog?limit=100', { headers: { 'Authorization': 'Bearer ' + token } })
                    .then(r => r.ok ? r.json() : null)
                    .then(res => {
                        posts = (res && res.data) || [];
                        renderMain();
                    });
            }

            function renderMain() {
                let main = '<main class="flex-1 p-8 h-screen overflow-y-auto"><nav class="mb-4 text-sm text-slate-600"><a href="/admin" class="text-[#2563EB] hover:underline">Admin</a> / Blog</nav>' +
                    '<div class="flex justify-between items-center mb-6"><h1 class="text-3xl font-bold">Blog Posts</h1><button class="bg-[#2563EB] text-white px-4 py-2 rounded-lg font-medium hover:bg-blue-700" onclick="window.editPost()">+ New Post</button></div>' +
                    '<div class="bg-white rounded-xl border border-slate-200 overflow-hidden"><table class="w-full"><thead class="bg-slate-50 border-b"><tr>' +
                    '<th class="text-left py-3 px-4 font-medium text-slate-600">Title</th><th class="text-left py-3 px-4 font-medium text-slate-600">Slug</th><th class="text-left py-3 px-4 font-medium text-slate-600">Status</th><th class="text-left py-3 px-4 font-medium text-slate-600">Visible At</th><th class="text-left py-3 px-4 font-medium text-slate-600">Actions</th></tr></thead><tbody>';

                if (posts.length === 0) {
                    main += '<tr><td colspan="5" class="py-6 text-center text-slate-500">No blog posts found.</td></tr>';
                } else {
                    posts.forEach(p => {
                        main += '<tr class="border-b border-slate-100 hover:bg-slate-50">' +
                            '<td class="py-3 px-4 font-medium">' + p.title + '</td>' +
                            '<td class="py-3 px-4 font-mono text-sm text-slate-500">' + p.slug + '</td>' +
                            '<td class="py-3 px-4"><span class="px-2 py-1 rounded text-sm ' + (p.status === 'published' ? 'bg-green-100 text-green-800' : 'bg-slate-100 text-slate-700') + '">' + p.status + '</span></td>' +
                            '<td class="py-3 px-4 text-sm text-slate-500">' + (p.published_at ? new Date(p.published_at).toLocaleString() : '—') + '</td>' +
                            '<td class="py-3 px-4"><button onclick="window.editPost(\'' + p.id + '\')" class="text-blue-600 hover:underline mr-4">Edit</button><button onclick="window.deletePost(\'' + p.id + '\')" class="text-red-500 hover:underline">Delete</button></td></tr>';
                    });
                }
                main += '</tbody></table></div></main>';

                // Editor Modal
                main += '<div id="post-modal" class="hidden fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">' +
                    '<div class="bg-white rounded-xl shadow-xl w-full max-w-4xl max-h-[90vh] flex flex-col">' +
                    '<div class="p-4 border-b flex justify-between items-center"><h2 id="modal-title" class="text-lg font-bold">New Post</h2><button onclick="document.getElementById(\'post-modal\').classList.add(\'hidden\')" class="text-slate-500 text-xl">&times;</button></div>' +
                    '<div class="p-6 overflow-y-auto flex-1"><form id="post-form" class="space-y-4">' +
                    '<input type="hidden" id="post-id" />' +
                    '<div class="grid grid-cols-2 gap-4"><div><label class="block text-sm font-medium mb-1">Title</label><input type="text" id="post-title" required class="w-full border rounded-lg px-3 py-2" /></div><div><label class="block text-sm font-medium mb-1">Slug</label><input type="text" id="post-slug" required class="w-full border rounded-lg px-3 py-2 font-mono text-sm" /></div></div>' +
                    '<div class="grid grid-cols-2 gap-4"><div><label class="block text-sm font-medium mb-1">Status</label><select id="post-status" class="w-full border rounded-lg px-3 py-2"><option value="draft">Draft</option><option value="published">Published</option></select></div><div><label class="block text-sm font-medium mb-1">Published At (UTC, Optional)</label><input type="datetime-local" id="post-date" class="w-full border rounded-lg px-3 py-2" /></div></div>' +
                    '<div><label class="block text-sm font-medium mb-1">Excerpt</label><textarea id="post-excerpt" required rows="2" class="w-full border rounded-lg px-3 py-2"></textarea></div>' +
                    '<div><label class="block text-sm font-medium mb-1">Markdown Content</label><textarea id="post-md" required rows="10" class="w-full border rounded-lg px-3 py-2 font-mono text-sm"></textarea></div>' +
                    '<div id="form-error" class="text-red-600 hidden"></div>' +
                    '</form></div>' +
                    '<div class="p-4 border-t flex justify-end gap-3"><button type="button" onclick="document.getElementById(\'post-modal\').classList.add(\'hidden\')" class="px-4 py-2 text-slate-600 hover:bg-slate-100 rounded-lg">Cancel</button><button type="button" onclick="window.savePost()" class="px-4 py-2 bg-[#2563EB] text-white rounded-lg hover:bg-blue-700">Save Post</button></div>' +
                    '</div></div>';

                document.getElementById('app').innerHTML = adminSidebar('blog') + main;
                document.getElementById('logout-btn').onclick = () => { fetch('/api/v1/auth/logout', { method: 'POST', credentials: 'include' }).then(() => { localStorage.removeItem('qcs_access_token'); window.location.href = '/'; }); };
                bindNotificationDropdown();

                // Auto-slug generator listener
                document.getElementById('post-title')?.addEventListener('input', e => {
                    const v = e.target.value.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)/g, '');
                    if (!document.getElementById('post-id').value) document.getElementById('post-slug').value = v;
                });
            }

            window.editPost = function (id) {
                const form = document.getElementById('post-form');
                const modal = document.getElementById('post-modal');
                const err = document.getElementById('form-error');
                form.reset();
                err.classList.add('hidden');

                if (!id) {
                    document.getElementById('modal-title').innerText = "New Post";
                    document.getElementById('post-id').value = "";
                } else {
                    document.getElementById('modal-title').innerText = "Edit Post";
                    const p = posts.find(x => x.id === id);
                    if (p) {
                        document.getElementById('post-id').value = p.id;
                        document.getElementById('post-title').value = p.title;
                        document.getElementById('post-slug').value = p.slug;
                        document.getElementById('post-status').value = p.status;
                        document.getElementById('post-excerpt').value = p.excerpt;
                        document.getElementById('post-md').value = p.content_md || '';
                        if (p.published_at) {
                            const dt = new Date(p.published_at);
                            const tzoffset = dt.getTimezoneOffset() * 60000;
                            document.getElementById('post-date').value = (new Date(dt - tzoffset)).toISOString().slice(0, 16);
                        }
                    }
                }
                modal.classList.remove('hidden');
            };

            window.savePost = function () {
                const form = document.getElementById('post-form');
                if (!form.checkValidity()) { form.reportValidity(); return; }

                const id = document.getElementById('post-id').value;
                const method = id ? 'PUT' : 'POST';
                const url = id ? '/api/v1/admin/blog/' + id : '/api/v1/admin/blog';

                let dateVal = document.getElementById('post-date').value;
                if (dateVal) { dateVal = new Date(dateVal).toISOString(); }

                const payload = {
                    title: document.getElementById('post-title').value,
                    slug: document.getElementById('post-slug').value,
                    status: document.getElementById('post-status').value,
                    excerpt: document.getElementById('post-excerpt').value,
                    content_md: document.getElementById('post-md').value,
                    published_at: dateVal || null,
                    category: "ANNOUNCEMENTS"
                };

                fetch(url, { method: method, ...auth, body: JSON.stringify(payload) })
                    .then(r => r.ok ? r.json() : r.json().then(e => Promise.reject(e)))
                    .then(() => {
                        document.getElementById('post-modal').classList.add('hidden');
                        loadPosts();
                    })
                    .catch(e => {
                        const err = document.getElementById('form-error');
                        err.textContent = e.error?.message || 'Failed to save';
                        err.classList.remove('hidden');
                    });
            };

            window.deletePost = function (id) {
                if (!confirm('Are you sure you want to delete this post?')) return;
                fetch('/api/v1/admin/blog/' + id, { method: 'DELETE', headers: auth.headers })
                    .then(r => r.ok ? loadPosts() : alert('Failed to delete'));
            };

            renderApp();
        })();
    