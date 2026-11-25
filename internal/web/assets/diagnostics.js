// SPDX-License-Identifier: AGPL-3.0-or-later
// Copy diagnostics on the release screen (templates/release.html): the
// server's version lines the page was drawn with, and this browser's facts
// added here, refreshed the moment Copy is pressed so the connection state is
// the one at that moment. assets/copy.js does the copying. The lines stay in
// English: they are for whoever reads the bug report.
(function (global) {
  "use strict";
  var tix = (global.tix = global.tix || {});

  // diagnosticsText is the report, one fact per line. The server address is
  // left out unless withServer, since a private server's name is nobody
  // else's business.
  tix.diagnosticsText = function (f, withServer) {
    var lines = [
      "tix " + (f.version || "unknown"),
      "Newest published: " + (f.latest || "not known"),
      "Browser: " + (f.userAgent || "unknown"),
      "Language: " + (f.language || "unknown") + (f.languages ? " (browser asks for " + f.languages + ")" : ""),
      "Time zone: " + (f.zone || "unknown"),
      "Screen: " + (f.screen || "unknown"),
      "Theme: " + (f.theme || "system") + (f.motion ? ", reduce motion" : ""),
      "Connection: " + (f.online === false ? "offline" : "online") + ", live updates " +
        (f.live === "on" ? "connected" : f.live === "off" ? "not connected" : "not started")
    ];
    if (withServer && f.server) {
      lines.push("Server address: " + f.server);
    }
    return lines.join("\n");
  };

  if (!global.document || !global.document.addEventListener) {
    return;
  }
  var document = global.document;

  function facts(pre) {
    var root = document.documentElement;
    var nav = global.navigator || {};
    var zone = "";
    try {
      zone = Intl.DateTimeFormat().resolvedOptions().timeZone || "";
    } catch (e) {
      zone = "";
    }
    var scr = global.screen ? global.screen.width + "x" + global.screen.height : "";
    if (scr && global.devicePixelRatio) {
      scr += " @" + global.devicePixelRatio + "x, window " + global.innerWidth + "x" + global.innerHeight;
    }
    return {
      version: pre.getAttribute("data-version"),
      latest: pre.getAttribute("data-latest"),
      userAgent: nav.userAgent,
      language: root.getAttribute("lang"),
      languages: nav.languages ? Array.prototype.join.call(nav.languages, ", ") : "",
      zone: zone,
      screen: scr,
      theme: root.getAttribute("data-theme"),
      motion: root.getAttribute("data-motion") === "reduce",
      online: nav.onLine,
      live: root.getAttribute("data-live"),
      server: global.location ? global.location.origin : ""
    };
  }

  function refresh() {
    var pre = document.querySelector("[data-diagnostics]");
    if (!pre) {
      return;
    }
    var box = document.querySelector("[data-diagnostics-server]");
    pre.textContent = tix.diagnosticsText(facts(pre), !!(box && box.checked));
  }

  document.addEventListener("click", function (event) {
    var btn = event.target && event.target.closest ? event.target.closest('[data-copy-target="diagnostics-text"]') : null;
    if (btn) {
      refresh();
    }
  }, true);
  document.addEventListener("change", function (event) {
    if (event.target && event.target.matches && event.target.matches("[data-diagnostics-server]")) {
      refresh();
    }
  });
  document.addEventListener("htmx:load", refresh);
  document.addEventListener("DOMContentLoaded", refresh);
})(typeof globalThis !== "undefined" ? globalThis : this);
