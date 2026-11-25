// SPDX-License-Identifier: AGPL-3.0-or-later

// Registers the service worker that makes the interface installable. It runs
// once per document load and binds to nothing on the page. The worker caches
// nothing (assets/sw.js), so a browser without one loses only Install.

(function () {
  "use strict";
  var sw = window.navigator && window.navigator.serviceWorker;
  if (!sw || typeof sw.register !== "function") {
    return;
  }
  sw.register("/sw.js", { scope: "/" }).catch(function () {});
})();
