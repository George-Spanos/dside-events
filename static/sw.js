// dside events service worker. Served through text/template; VERSION changes with every build.
var VERSION = '{{.Version}}';
var CACHE = 'dside-' + VERSION;
var PRECACHE = ['/offline', '/static/style.css?v=' + VERSION, '/icon.svg'];

self.addEventListener('install', function (e) {
  e.waitUntil(caches.open(CACHE)
    .then(function (c) { return c.addAll(PRECACHE.map(function (u) { return new Request(u, { cache: 'reload' }); })); })
    .then(function () { return self.skipWaiting(); }));
});

self.addEventListener('activate', function (e) {
  e.waitUntil(caches.keys()
    .then(function (keys) { return Promise.all(keys.filter(function (k) { return k !== CACHE; }).map(function (k) { return caches.delete(k); })); })
    .then(function () { return self.clients.claim(); }));
});

// Pages: network only, offline page as fallback. HTML is never cached. Static files: cache first.
self.addEventListener('fetch', function (e) {
  var req = e.request, url = new URL(req.url);
  if (req.method !== 'GET' || url.origin !== self.location.origin) return;
  if (req.mode === 'navigate') {
    e.respondWith(fetch(req).catch(function () { return caches.match('/offline'); }));
    return;
  }
  if (url.pathname.indexOf('/static/') === 0 || url.pathname === '/icon.svg') {
    e.respondWith(caches.match(req).then(function (hit) { return hit || fetch(req); }));
  }
});
