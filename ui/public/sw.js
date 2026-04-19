// Minimal service worker — satisfies install criteria, no offline caching.
// docsiq is server-backed; offline UX isn't meaningful without the Go API running.

self.addEventListener("install", () => {
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(self.clients.claim());
});

// A fetch listener is required by Chrome's installability criteria, even if it
// doesn't intercept anything. Network pass-through by omission.
self.addEventListener("fetch", () => {});
