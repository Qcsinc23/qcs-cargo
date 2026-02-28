const SHELL_CACHE = 'qcs-cargo-shell-v4';
const RUNTIME_CACHE = 'qcs-cargo-runtime-v4';
const QUEUE_DB = 'qcs-cargo-offline';
const QUEUE_STORE = 'warehouse_actions';
const SYNC_TAG = 'qcs-warehouse-replay';

const SHELL_ASSETS = [
  '/',
  '/index.html',
  '/login',
  '/login.html',
  '/dashboard',
  '/dashboard/index.html',
  '/dashboard/inbox',
  '/dashboard/inbox.html',
  '/dashboard/shipments',
  '/dashboard/shipments.html',
  '/dashboard/settings/notifications',
  '/dashboard/settings/notifications.html',
  '/dashboard/mobile-nav.css',
  '/dashboard/mobile-nav.js',
  '/dashboard/pwa-shell.css',
  '/dashboard/pwa-shell.js',
  '/warehouse',
  '/warehouse/index.html',
  '/warehouse/ship-queue.html',
  '/warehouse/staging.html',
  '/warehouse/receiving.html'
];

const WAREHOUSE_ACTION_PATTERNS = [
  /^\/api\/v1\/warehouse\/packages\/receive-from-booking$/,
  /^\/api\/v1\/warehouse\/locker-receive$/,
  /^\/api\/v1\/warehouse\/ship-queue\/[^/]+\/(process|weighed|staged)$/,
  /^\/api\/v1\/warehouse\/bays\/move$/,
  /^\/api\/v1\/warehouse\/exceptions\/[^/]+\/resolve$/,
  /^\/api\/v1\/warehouse\/manifests$/
];

self.addEventListener('install', (event) => {
  event.waitUntil((async () => {
    const cache = await caches.open(SHELL_CACHE);
    await Promise.allSettled(SHELL_ASSETS.map(async (asset) => {
      try {
        await cache.add(new Request(asset, { cache: 'reload' }));
      } catch (err) {
        // Asset may not exist in all deployments.
      }
    }));
    await self.skipWaiting();
  })());
});

self.addEventListener('activate', (event) => {
  event.waitUntil((async () => {
    const keys = await caches.keys();
    await Promise.all(
      keys
        .filter((key) => key !== SHELL_CACHE && key !== RUNTIME_CACHE)
        .map((key) => caches.delete(key))
    );
    await self.clients.claim();
    await replayQueuedWarehouseActions();
  })());
});

self.addEventListener('sync', (event) => {
  if (event.tag === SYNC_TAG) {
    event.waitUntil(replayQueuedWarehouseActions());
  }
});

self.addEventListener('message', (event) => {
  const data = event && event.data;
  if (!data) return;
  if (data.type === 'QCS_REPLAY_QUEUE') {
    replayQueuedWarehouseActions();
  }
});

self.addEventListener('fetch', (event) => {
  const request = event.request;
  const url = new URL(request.url);

  if (url.origin !== self.location.origin) return;

  if (request.method !== 'GET' && isQueueableWarehouseAction(request, url)) {
    event.respondWith(networkWriteWithQueue(request));
    return;
  }

  if (url.pathname.startsWith('/api/')) {
    return;
  }

  if (request.mode === 'navigate' && isShellRoute(url.pathname)) {
    event.respondWith(networkFirst(request, '/dashboard/index.html'));
    return;
  }

  if (isStaticAsset(url.pathname)) {
    event.respondWith(staleWhileRevalidate(request));
    return;
  }
});

function isShellRoute(pathname) {
  return pathname === '/'
    || pathname === '/dashboard'
    || pathname.startsWith('/dashboard/')
    || pathname === '/warehouse'
    || pathname.startsWith('/warehouse/');
}

function isStaticAsset(pathname) {
  return pathname.endsWith('.css')
    || pathname.endsWith('.js')
    || pathname.endsWith('.html')
    || pathname.endsWith('.svg')
    || pathname.endsWith('.png')
    || pathname.endsWith('.jpg')
    || pathname.endsWith('.jpeg')
    || pathname.endsWith('.webp')
    || pathname.endsWith('.woff2')
    || pathname.endsWith('.wasm');
}

function isQueueableWarehouseAction(request, url) {
  const method = request.method.toUpperCase();
  if (method !== 'POST' && method !== 'PATCH') return false;
  if (!url.pathname.startsWith('/api/v1/warehouse/')) return false;
  return WAREHOUSE_ACTION_PATTERNS.some((pattern) => pattern.test(url.pathname));
}

async function networkFirst(request, fallbackAsset) {
  const cache = await caches.open(SHELL_CACHE);
  try {
    const response = await fetch(request);
    if (response && response.ok) {
      cache.put(request, response.clone());
    }
    return response;
  } catch (err) {
    const cached = await cache.match(request);
    if (cached) return cached;
    if (fallbackAsset) {
      const fallback = await cache.match(fallbackAsset);
      if (fallback) return fallback;
    }
    return new Response('Offline', { status: 503, statusText: 'Offline' });
  }
}

async function staleWhileRevalidate(request) {
  const cache = await caches.open(RUNTIME_CACHE);
  const cached = await cache.match(request);

  const networkPromise = fetch(request).then((response) => {
    if (response && response.ok) {
      cache.put(request, response.clone());
    }
    return response;
  }).catch(() => null);

  if (cached) {
    return cached;
  }

  const network = await networkPromise;
  if (network) return network;
  return new Response('Offline', { status: 503, statusText: 'Offline' });
}

async function networkWriteWithQueue(request) {
  try {
    return await fetch(request);
  } catch (err) {
    await queueWarehouseAction(request);
    return new Response(JSON.stringify({
      queued: true,
      offline: true,
      message: 'Action queued and will retry when connection returns.'
    }), {
      status: 202,
      headers: {
        'Content-Type': 'application/json',
        'X-Offline-Queued': '1'
      }
    });
  }
}

async function queueWarehouseAction(request) {
  const entry = await requestToQueueEntry(request);
  const db = await openQueueDB();
  await new Promise((resolve, reject) => {
    const tx = db.transaction(QUEUE_STORE, 'readwrite');
    tx.oncomplete = () => resolve();
    tx.onerror = () => reject(tx.error || new Error('failed to queue request'));
    tx.objectStore(QUEUE_STORE).put(entry);
  });

  if (self.registration && self.registration.sync) {
    try {
      await self.registration.sync.register(SYNC_TAG);
    } catch (err) {
      // Background sync unsupported or denied.
    }
  }
}

async function requestToQueueEntry(request) {
  const headers = {};
  request.headers.forEach((value, key) => {
    if (key.toLowerCase() === 'content-length') return;
    headers[key] = value;
  });

  let body = '';
  if (request.method !== 'GET' && request.method !== 'HEAD') {
    try {
      body = await request.clone().text();
    } catch (err) {
      body = '';
    }
  }

  return {
    id: (self.crypto && self.crypto.randomUUID) ? self.crypto.randomUUID() : String(Date.now()) + '-' + Math.random(),
    url: request.url,
    method: request.method,
    headers,
    body,
    createdAt: new Date().toISOString()
  };
}

async function replayQueuedWarehouseActions() {
  const db = await openQueueDB();
  const entries = await readAllQueueEntries(db);
  if (!entries.length) return;

  for (const entry of entries) {
    try {
      const init = {
        method: entry.method,
        headers: entry.headers || {},
        credentials: 'include'
      };
      if (entry.method !== 'GET' && entry.method !== 'HEAD') {
        init.body = entry.body || '';
      }

      const response = await fetch(entry.url, init);
      if (response && response.ok) {
        await deleteQueueEntry(db, entry.id);
      }
    } catch (err) {
      // Keep entry for next replay attempt.
    }
  }
}

function openQueueDB() {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(QUEUE_DB, 1);
    req.onupgradeneeded = () => {
      const db = req.result;
      if (!db.objectStoreNames.contains(QUEUE_STORE)) {
        db.createObjectStore(QUEUE_STORE, { keyPath: 'id' });
      }
    };
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error || new Error('failed to open queue db'));
  });
}

function readAllQueueEntries(db) {
  return new Promise((resolve, reject) => {
    const tx = db.transaction(QUEUE_STORE, 'readonly');
    const store = tx.objectStore(QUEUE_STORE);
    const req = store.getAll();
    req.onsuccess = () => resolve(req.result || []);
    req.onerror = () => reject(req.error || new Error('failed to read queue entries'));
  });
}

function deleteQueueEntry(db, id) {
  return new Promise((resolve, reject) => {
    const tx = db.transaction(QUEUE_STORE, 'readwrite');
    tx.oncomplete = () => resolve();
    tx.onerror = () => reject(tx.error || new Error('failed to delete queue entry'));
    tx.objectStore(QUEUE_STORE).delete(id);
  });
}
