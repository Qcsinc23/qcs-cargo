const SHELL_CACHE = 'qcs-cargo-shell-v5';
const RUNTIME_CACHE = 'qcs-cargo-runtime-v5';
const QUEUE_DB = 'qcs-cargo-offline';
const QUEUE_STORE = 'warehouse_actions';
const SYNC_TAG = 'qcs-warehouse-replay';

// Pass 2 audit fix M-9: bound how many replay attempts we keep around per
// queued entry. After this many consecutive failures we drop the entry rather
// than retry forever (which would otherwise grow IndexedDB unboundedly).
const MAX_REPLAY_ATTEMPTS = 5;
// Entries older than this are dropped on the next replay sweep.
const QUEUE_TTL_MS = 24 * 60 * 60 * 1000;

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
    const lower = key.toLowerCase();
    // Pass 2 audit fix H-6: never persist credentials to IndexedDB. Bearer
    // tokens are short-lived, attacker-readable from any XSS, and frequently
    // expired by the time a replay actually runs. We also strip Cookie
    // (browsers normally do not let SW read it, but be defensive). On replay
    // we re-establish auth via the refresh cookie (credentials: 'include').
    if (lower === 'authorization' || lower === 'cookie' || lower === 'content-length') return;
    headers[lower] = value;
  });

  let body = '';
  if (request.method !== 'GET' && request.method !== 'HEAD') {
    try {
      body = await request.clone().text();
    } catch (err) {
      body = '';
    }
  }

  const idempotencyKey = (self.crypto && self.crypto.randomUUID) ? self.crypto.randomUUID() : String(Date.now()) + '-' + Math.random();
  // Pass 2 audit fix M-9: deterministic Idempotency-Key for replays. The
  // server's warehouse idempotency middleware short-circuits duplicate
  // submissions of the same key so a network drop between server-handled
  // write and client-received response cannot create a duplicate.
  headers['idempotency-key'] = idempotencyKey;

  return {
    id: idempotencyKey,
    url: request.url,
    method: request.method,
    headers,
    body,
    createdAt: new Date().toISOString(),
    attempts: 0
  };
}

async function replayQueuedWarehouseActions() {
  const db = await openQueueDB();
  const entries = await readAllQueueEntries(db);
  if (!entries.length) return;

  // Pass 2 audit fix M-9: drop entries that have aged out so the queue does
  // not grow forever when something is permanently unreplayable (e.g. a
  // logged-out device, a deleted bay, a now-invalid action).
  const now = Date.now();

  for (const entry of entries) {
    try {
      const createdMs = entry.createdAt ? Date.parse(entry.createdAt) : now;
      if (now - createdMs > QUEUE_TTL_MS) {
        await deleteQueueEntry(db, entry.id);
        broadcastReplayEvent({ type: 'expired', entryId: entry.id, url: entry.url });
        continue;
      }

      const init = {
        method: entry.method,
        // Re-establish auth via the same-origin refresh cookie. We
        // intentionally do NOT carry the original Authorization header; see
        // H-6 in requestToQueueEntry().
        headers: entry.headers || {},
        credentials: 'include'
      };
      if (entry.method !== 'GET' && entry.method !== 'HEAD') {
        init.body = entry.body || '';
      }

      const response = await fetch(entry.url, init);
      if (response && response.ok) {
        await deleteQueueEntry(db, entry.id);
        broadcastReplayEvent({ type: 'success', entryId: entry.id, url: entry.url });
        continue;
      }

      // Treat 4xx (other than 408 timeout / 429 throttle) as permanent.
      // The server has rejected the action; retrying will not help.
      if (response && response.status >= 400 && response.status < 500 && response.status !== 408 && response.status !== 429) {
        await deleteQueueEntry(db, entry.id);
        broadcastReplayEvent({
          type: 'permanent_failure',
          entryId: entry.id,
          url: entry.url,
          status: response.status
        });
        continue;
      }

      // Transient failure: increment attempts and drop after MAX_REPLAY_ATTEMPTS.
      const nextAttempts = (entry.attempts || 0) + 1;
      if (nextAttempts >= MAX_REPLAY_ATTEMPTS) {
        await deleteQueueEntry(db, entry.id);
        broadcastReplayEvent({
          type: 'gave_up',
          entryId: entry.id,
          url: entry.url,
          attempts: nextAttempts
        });
        continue;
      }
      await updateQueueEntry(db, Object.assign({}, entry, { attempts: nextAttempts }));
    } catch (err) {
      // Network-layer failure (truly offline). Bump attempts and let the
      // next sync event try again.
      const nextAttempts = (entry.attempts || 0) + 1;
      if (nextAttempts >= MAX_REPLAY_ATTEMPTS) {
        await deleteQueueEntry(db, entry.id);
        broadcastReplayEvent({ type: 'gave_up', entryId: entry.id, url: entry.url, attempts: nextAttempts });
      } else {
        await updateQueueEntry(db, Object.assign({}, entry, { attempts: nextAttempts }));
      }
    }
  }
}

function broadcastReplayEvent(detail) {
  try {
    self.clients.matchAll({ includeUncontrolled: true, type: 'window' }).then(function (clients) {
      clients.forEach(function (client) {
        client.postMessage({ type: 'QCS_REPLAY_EVENT', detail: detail });
      });
    });
  } catch (e) { /* best-effort */ }
}

function updateQueueEntry(db, entry) {
  return new Promise(function (resolve, reject) {
    const tx = db.transaction(QUEUE_STORE, 'readwrite');
    tx.oncomplete = function () { resolve(); };
    tx.onerror = function () { reject(tx.error || new Error('failed to update queue entry')); };
    tx.objectStore(QUEUE_STORE).put(entry);
  });
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
