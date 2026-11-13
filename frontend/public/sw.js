// Bump V when changing this file or the manifest/icon, so old caches are dropped.
const V = 'inventori-v1';
const SHELL = ['/', '/manifest.webmanifest', '/icon.svg'];

self.addEventListener('install', (e) => {
  e.waitUntil(caches.open(V).then((c) => c.addAll(SHELL)).then(() => self.skipWaiting()));
});

self.addEventListener('activate', (e) => {
  e.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== V).map((k) => caches.delete(k))))
      .then(() => self.clients.claim()),
  );
});

// The page lists the assets it already loaded, so the very first offline start works too.
self.addEventListener('message', (e) => {
  if (e.data && e.data.type === 'precache') {
    e.waitUntil(caches.open(V).then((c) => Promise.all(e.data.urls.map((u) => c.add(u).catch(() => {})))));
  }
});

self.addEventListener('fetch', (e) => {
  const req = e.request;
  const url = new URL(req.url);
  // API calls always go to the network; the app keeps its own data in IndexedDB.
  if (req.method !== 'GET' || url.origin !== location.origin || url.pathname.startsWith('/api/')) return;
  if (req.mode === 'navigate') {
    e.respondWith(
      fetch(req)
        .then((res) => { const copy = res.clone(); caches.open(V).then((c) => c.put('/', copy)); return res; })
        .catch(() => caches.match('/')),
    );
    return;
  }
  e.respondWith(
    caches.match(req).then((hit) => hit || fetch(req).then((res) => {
      if (res.ok) { const copy = res.clone(); caches.open(V).then((c) => c.put(req, copy)); }
      return res;
    })),
  );
});
