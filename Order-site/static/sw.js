// Minimal service worker: lets the site be installed and shows a friendly page when offline.
// Pages and API calls are never cached, so orders, carts and prices are always live.
const CACHE = 'laikihub-shell-v5';
const OFFLINE = '/offline';
self.addEventListener('install', (e) => {
  e.waitUntil(caches.open(CACHE).then((c) => c.add(OFFLINE)).then(() => self.skipWaiting()));
});
self.addEventListener('activate', (e) => {
  e.waitUntil(caches.keys().then((ks) => Promise.all(ks.filter((k) => k !== CACHE).map((k) => caches.delete(k)))).then(() => self.clients.claim()));
});
self.addEventListener('fetch', (e) => {
  if (e.request.mode !== 'navigate') return;
  e.respondWith(fetch(e.request).catch(() => caches.match(OFFLINE)));
});

// ---- Push notifications ---------------------------------------------------------------------
// The server sends a small JSON message {title, body, url, tag}. Showing a notification for every
// push is required: phones (iPhone especially) stop allowing notifications to a site that
// receives a push and shows nothing.
self.addEventListener('push', (e) => {
  let data = {};
  try { data = e.data ? e.data.json() : {}; } catch (_) { data = { body: e.data ? e.data.text() : '' }; }
  const title = data.title || 'LaikiHub';
  const options = {
    body: data.body || '',
    icon: '/static/img/icon-192.png',
    badge: '/static/img/icon-192.png',
    tag: data.tag || undefined,
    renotify: !!data.tag,
    data: { url: typeof data.url === 'string' && data.url.charAt(0) === '/' ? data.url : '/notifications' },
  };
  e.waitUntil(self.registration.showNotification(title, options));
});

self.addEventListener('notificationclick', (e) => {
  e.notification.close();
  const target = new URL((e.notification.data && e.notification.data.url) || '/', self.location.origin).href;
  e.waitUntil((async () => {
    const all = await self.clients.matchAll({ type: 'window', includeUncontrolled: true });
    for (const c of all) {
      if (new URL(c.url).origin === self.location.origin && 'focus' in c) {
        await c.focus();
        if ('navigate' in c) { try { await c.navigate(target); } catch (_) {} }
        return;
      }
    }
    await self.clients.openWindow(target);
  })());
});

// The phone sometimes replaces a subscription (for example after a browser update). Re-subscribe
// quietly and tell the server; if that is not possible the next visit to the app does it.
self.addEventListener('pushsubscriptionchange', (e) => {
  e.waitUntil((async () => {
    try {
      const k = await (await fetch('/push/key', { cache: 'no-store' })).json();
      if (!k.enabled) return;
      const pad = '='.repeat((4 - (k.key.length % 4)) % 4);
      const raw = atob((k.key + pad).replace(/-/g, '+').replace(/_/g, '/'));
      const key = Uint8Array.from(raw, (c) => c.charCodeAt(0));
      const sub = await self.registration.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: key });
      await fetch('/push/subscribe', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(sub.toJSON()), credentials: 'same-origin' });
    } catch (_) { /* the app will fix it next time it is opened */ }
  })());
});
