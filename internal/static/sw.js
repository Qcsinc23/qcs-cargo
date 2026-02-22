// Minimal service worker: cache-first for key static assets and shell.
const CACHE = 'qcs-cargo-v1';
const ASSETS = [
  '/',
  '/index.html',
  '/app.wasm',
  '/wasm_exec.js',
  '/login',
  '/login.html',
  '/dashboard',
  '/dashboard/index.html'
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
  e.respondWith(
    caches.match(e.request).then((cached) => cached || fetch(e.request).then((r) => {
      const clone = r.clone();
      if (r.ok && (u.pathname === '/' || u.pathname.endsWith('.html') || u.pathname.endsWith('.wasm') || u.pathname.endsWith('.js')))
        caches.open(CACHE).then((cache) => cache.put(e.request, clone));
      return r;
    }))
  );
});
