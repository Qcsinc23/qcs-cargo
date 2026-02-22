// Minimal service worker: cache-first for public shell assets only.
const CACHE = 'qcs-cargo-v2';
const ASSETS = [
  '/',
  '/index.html',
  '/login',
  '/login.html'
];

self.addEventListener('install', (e) => {
  e.waitUntil(
    caches.open(CACHE).then((cache) => cache.addAll(ASSETS)).then(() => self.skipWaiting())
  );
});

self.addEventListener('activate', (e) => {
  e.waitUntil(caches.keys().then((keys) => Promise.all(
    keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))
  )).then(() => self.clients.claim()));
});

self.addEventListener('fetch', (e) => {
  const u = new URL(e.request.url);
  if (u.origin !== location.origin || u.pathname.startsWith('/api/')) return;
  if (
    u.pathname.startsWith('/dashboard') ||
    u.pathname.startsWith('/admin') ||
    u.pathname.startsWith('/warehouse') ||
    u.pathname.startsWith('/verify')
  ) {
    return;
  }
  e.respondWith(
    caches.match(e.request).then((cached) => cached || fetch(e.request).then((r) => {
      const clone = r.clone();
      if (r.ok && (u.pathname === '/' || u.pathname.endsWith('.html') || u.pathname.endsWith('.wasm') || u.pathname.endsWith('.js')))
        caches.open(CACHE).then((cache) => cache.put(e.request, clone));
      return r;
    }))
  );
});
