// SPDX-License-Identifier: AGPL-3.0-or-later

// The service worker, served from /sw.js so its scope is the whole interface.
//
// It caches nothing and answers nothing. Every screen but the sign-in form is
// a tenant's data, and a cached copy is one more place that data outlives a
// sign-out on a shared machine. The fetch handler never calls respondWith, so
// the browser fetches every request itself, exactly as with no worker. It
// exists because some browsers offer Install only to a page a worker controls.
// TestTheServiceWorkerCachesNothing holds it to that.

self.addEventListener("install", function () {
  self.skipWaiting();
});

self.addEventListener("activate", function (event) {
  event.waitUntil(self.clients.claim());
});

self.addEventListener("fetch", function () {});
