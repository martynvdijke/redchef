/* RedChef service worker — Web Push delivery + notification clicks.
 *
 * Registered by app.js on / (scope /). It intentionally does no caching:
 * RedChef's HTML is rendered per-request, so a cache layer would go stale.
 */

const ICON =
  "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 96 96'%3E%3Crect width='96' height='96' rx='24' fill='%23D42B2B'/%3E%3Cpath d='M48 22c-9.4 0-16 7-16 16.4V49l-5.6 9.2c-1 1.6.2 3.6 2 3.6h39.2c1.8 0 3-2 2-3.6L64 49V38.4C64 29 57.4 22 48 22z' fill='%23FFF8F0'/%3E%3Ccircle cx='48' cy='70' r='6' fill='%23FFF8F0'/%3E%3C/svg%3E";

const BADGE =
  "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 96 96'%3E%3Cpath d='M48 22c-9.4 0-16 7-16 16.4V49l-5.6 9.2c-1 1.6.2 3.6 2 3.6h39.2c1.8 0 3-2 2-3.6L64 49V38.4C64 29 57.4 22 48 22z' fill='%23000'/%3E%3Ccircle cx='48' cy='70' r='6' fill='%23000'/%3E%3C/svg%3E";

self.addEventListener('install', () => {
  // Activate this version immediately; the page re-registers on every load.
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  event.waitUntil(self.clients.claim());
});

// ── Push ──
self.addEventListener('push', (event) => {
  let payload = {};
  if (event.data) {
    try {
      payload = event.data.json();
    } catch (_) {
      payload = { title: 'Nieuwe post', body: event.data.text() };
    }
  }
  if (!payload || typeof payload !== 'object') payload = {};

  const title = typeof payload.title === 'string' && payload.title ? payload.title : 'Nieuwe post';
  const body = typeof payload.body === 'string' ? payload.body : '';
  const url = typeof payload.url === 'string' && payload.url ? payload.url : '/';
  const tag = typeof payload.id === 'string' && payload.id ? payload.id : 'redchef-new-post';

  event.waitUntil(
    self.registration.showNotification(title, {
      body,
      icon: ICON,
      badge: BADGE,
      tag,
      renotify: true,
      data: { url },
    })
  );
});

// ── Click ──
self.addEventListener('notificationclick', (event) => {
  event.notification.close();

  const data = event.notification.data || {};
  let target = typeof data.url === 'string' && data.url ? data.url : '/';
  try {
    target = new URL(target, self.location.origin).href;
  } catch (_) {
    target = self.location.origin + '/';
  }

  event.waitUntil(
    (async () => {
      const windows = await self.clients.matchAll({ type: 'window', includeUncontrolled: true });

      // Exact page already open: just focus it.
      for (const client of windows) {
        if (client.url === target && 'focus' in client) {
          return client.focus();
        }
      }

      // Another RedChef tab is open: reuse it and navigate.
      for (const client of windows) {
        if ('focus' in client) {
          await client.focus();
          if ('navigate' in client) {
            try {
              await client.navigate(target);
              return;
            } catch (_) {
              // Navigation refused (e.g. the client went away); fall through.
            }
          }
          return;
        }
      }

      if (self.clients.openWindow) {
        return self.clients.openWindow(target);
      }
      return undefined;
    })()
  );
});
