/// <reference lib="webworker" />
import { precacheAndRoute, cleanupOutdatedCaches } from "workbox-precaching";
import { registerRoute, NavigationRoute } from "workbox-routing";
import {
  CacheFirst,
  NetworkFirst,
  NetworkOnly,
} from "workbox-strategies";
import { ExpirationPlugin } from "workbox-expiration";
import { CacheableResponsePlugin } from "workbox-cacheable-response";

declare let self: ServiceWorkerGlobalScope;

// Precache app shell (manifest injected by workbox at build time)
precacheAndRoute(self.__WB_MANIFEST);
cleanupOutdatedCaches();

// Offline fallback for navigation requests
const offlineFallback = new NavigationRoute(
  new NetworkFirst({
    cacheName: "stash-navigations",
    plugins: [
      new CacheableResponsePlugin({ statuses: [0, 200] }),
    ],
  }),
  {
    // Don't intercept API, stream, image, or login routes
    denylist: [
      /\/graphql/,
      /\/scene\/.*\/stream/,
      /\/image\//,
      /\/login/,
      /\/logout/,
    ],
  }
);
registerRoute(offlineFallback);

// Scene/performer/studio thumbnails and screenshots - cache first
registerRoute(
  /\/(scene|performer|studio|tag|image|gallery)\/\d+\/(screenshot|image|thumbnail|preview)/,
  new CacheFirst({
    cacheName: "stash-images",
    plugins: [
      new ExpirationPlugin({
        // 500 was small for large libraries — browsing a big grid evicts and
        // refetches thumbnails mid-scroll. Thumbnails are tiny, so keep more.
        // (The Go backend already sends immutable Cache-Control on versioned
        // media URLs, so even an evicted entry is served from the HTTP cache
        // without a network round-trip; this mainly smooths the SW layer.)
        maxEntries: 2000,
        maxAgeSeconds: 7 * 24 * 60 * 60, // 7 days
      }),
      new CacheableResponsePlugin({ statuses: [0, 200] }),
    ],
  })
);

// Static assets (already immutable-cached by Go backend)
registerRoute(
  /\/assets\//,
  new CacheFirst({
    cacheName: "stash-assets",
    plugins: [
      new ExpirationPlugin({
        maxEntries: 200,
        maxAgeSeconds: 365 * 24 * 60 * 60, // 1 year
      }),
    ],
  })
);

// Video/audio streams - always network, never cache
registerRoute(/\/stream/, new NetworkOnly());

// Listen for skip waiting message from the client
self.addEventListener("message", (event) => {
  if (event.data && event.data.type === "SKIP_WAITING") {
    self.skipWaiting();
  }

  // Clear caches on logout
  if (event.data && event.data.type === "CLEAR_CACHES") {
    caches.keys().then((names) => {
      names.forEach((name) => caches.delete(name));
    });
  }
});
