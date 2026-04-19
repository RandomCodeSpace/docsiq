/**
 * Force a full reload on any platform — especially useful on mobile where
 * there's no ⌘⇧R. Purges the service worker registration, all cache
 * storage (HTTP + CacheStorage), and reloads from network.
 */
export async function hardReload() {
  try {
    if ("serviceWorker" in navigator) {
      const regs = await navigator.serviceWorker.getRegistrations();
      await Promise.all(regs.map((r) => r.unregister()));
    }
  } catch { /* swallow — proceed to cache purge */ }
  try {
    if (typeof caches !== "undefined") {
      const keys = await caches.keys();
      await Promise.all(keys.map((k) => caches.delete(k)));
    }
  } catch { /* swallow */ }
  // Cache-busting query string forces a fresh network fetch on reload.
  const url = new URL(window.location.href);
  url.searchParams.set("_r", String(Date.now()));
  window.location.replace(url.toString());
}
