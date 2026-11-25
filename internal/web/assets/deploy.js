// SPDX-License-Identifier: AGPL-3.0-or-later
// Picking up a new release in a tab that was open across the deploy.
//
// The assets are served at unversioned paths, so a tab left open while the
// server is upgraded keeps running the old scripts against the new server's
// fragments. The page carries the build it was drawn by on <body data-build>,
// and this compares it with the version /healthz reports: when the live
// socket comes back after a drop (assets/live.js fires tix:live-reconnected),
// when the tab is shown again, and every few minutes as a fallback for a
// screen with no socket. A boosted navigation keeps the body element, so the
// attribute keeps naming the build whose scripts are running.
//
// A different version reloads the page, but only when nothing would be lost
// (tix.reloadBlocked). Otherwise a banner with a Reload button says a new
// version is there, and the check is made again whenever the reader leaves a
// field, closes a dialog or a request settles. After the reload the page
// says which version it now runs, and goes back to where it was scrolled.
//
// Without scripting nothing here runs, and the next navigation simply loads
// the new build.
(function (global) {
  "use strict";
  var tix = (global.tix = global.tix || {});

  var NOTE_KEY = "tix.updatedTo";
  var SCROLL_KEY = "tix.updatedScroll";
  var HEALTH = "/healthz";
  var INTERVAL = 7 * 60 * 1000;
  var SPACING = 15 * 1000;
  var NOTE_FOR = 8000;

  function say(text) {
    return tix.t ? tix.t(text) : text;
  }

  // buildChanged reports whether the server now runs a different build than
  // the one that drew the page. An unknown version on either side is no change.
  tix.buildChanged = function (page, server) {
    return typeof page === "string" && typeof server === "string" && page !== "" && server !== "" &&
      page !== server;
  };

  function changedField(doc) {
    var fields = doc.querySelectorAll("input, textarea, select");
    for (var i = 0; i < fields.length; i++) {
      if (fields[i].type !== "hidden" && tix.fieldDirty && tix.fieldDirty(fields[i])) {
        return true;
      }
    }
    return false;
  }

  function openDialog(doc) {
    if (doc.querySelector("dialog[open]")) return true;
    try {
      return !!doc.querySelector(":popover-open");
    } catch (e) {
      return false;
    }
  }

  function selecting(doc) {
    return !!doc.querySelector('[data-select-toggle][aria-pressed="true"]') &&
      !!doc.querySelector("input[data-pick]:checked");
  }

  // reloadBlocked names what a reload would throw away, or answers "" when
  // there is nothing: focus in a field, a dialog or popover open, a request
  // in flight, rows selected, or a field changed and not yet submitted.
  tix.reloadBlocked = function (doc, inFlight) {
    if (!doc || typeof doc.querySelector !== "function") return "page";
    var active = doc.activeElement;
    if (active && active !== doc.body && tix.isTypingTarget && tix.isTypingTarget(active)) return "focus";
    if (openDialog(doc)) return "dialog";
    if (inFlight > 0 || doc.querySelector('.htmx-request, form[aria-busy="true"], .is-pending')) return "request";
    if (selecting(doc)) return "selection";
    if (changedField(doc)) return "edited";
    return "";
  };

  // updatedNote is what the page says after reloading into version v.
  tix.updatedNote = function (v) {
    return say("Updated to %s").replace("%s", v);
  };

  function storage() {
    try {
      return global.sessionStorage || null;
    } catch (e) {
      return null;
    }
  }

  function start() {
    var doc = global.document;
    var body = doc && doc.body;
    if (!body || !global.fetch) return;
    var page = body.getAttribute("data-build") || "";
    if (!page) return;

    var waiting = "";
    var inFlight = 0;
    var lastCheck = 0;
    var checking = false;
    var banner = null;
    var dismissed = "";

    function now() {
      return Date.now ? Date.now() : new Date().getTime();
    }

    function hideBanner() {
      if (banner) banner.hidden = true;
    }

    function showBanner() {
      if (dismissed === waiting) return;
      if (!banner) {
        banner = doc.createElement("div");
        banner.className = "deploybar";
        banner.id = "deploy-bar";
        banner.setAttribute("role", "status");
        var text = doc.createElement("span");
        text.textContent = say("A new version is available.");
        var reload = doc.createElement("button");
        reload.type = "button";
        reload.className = "deploybar-reload";
        reload.textContent = say("Reload");
        reload.addEventListener("click", function () { reloadNow(); });
        var close = doc.createElement("button");
        close.type = "button";
        close.className = "deploybar-close";
        close.textContent = say("Close");
        close.addEventListener("click", function () {
          dismissed = waiting;
          hideBanner();
        });
        banner.appendChild(text);
        banner.appendChild(reload);
        banner.appendChild(close);
        body.appendChild(banner);
      }
      if (!banner.isConnected && body.appendChild) body.appendChild(banner);
      banner.hidden = false;
    }

    function reloadNow() {
      var store = storage();
      if (store) {
        try {
          store.setItem(NOTE_KEY, waiting);
          store.setItem(SCROLL_KEY, JSON.stringify({
            at: global.location.pathname + global.location.search,
            y: global.scrollY || 0,
          }));
        } catch (e) { /* a full or blocked store only loses the note */ }
      }
      global.location.reload();
    }

    function attempt() {
      if (!waiting) return;
      if (tix.reloadBlocked(doc, inFlight)) {
        showBanner();
        return;
      }
      reloadNow();
    }

    function soon() {
      if (waiting) global.setTimeout(attempt, 0);
    }

    function check(force) {
      if (checking || waiting) return;
      if (!force && now() - lastCheck < SPACING) return;
      checking = true;
      lastCheck = now();
      global.fetch(HEALTH, { credentials: "same-origin", cache: "no-store" })
        .then(function (resp) { return resp.ok ? resp.json() : null; })
        .then(function (health) {
          checking = false;
          if (health && tix.buildChanged(page, health.version)) {
            waiting = health.version;
            attempt();
          }
        })
        .catch(function () { checking = false; });
    }

    doc.addEventListener("tix:live-reconnected", function () { check(true); });
    doc.addEventListener("visibilitychange", function () {
      if (!doc.hidden) check(false);
    });
    doc.addEventListener("htmx:beforeRequest", function () { inFlight++; });
    doc.addEventListener("htmx:afterRequest", function () {
      if (inFlight > 0) inFlight--;
      soon();
    });
    doc.addEventListener("htmx:afterSettle", soon);
    doc.addEventListener("focusout", soon);
    doc.addEventListener("submit", soon);
    doc.addEventListener("close", soon, true);
    doc.addEventListener("toggle", soon, true);
    global.setInterval(function () {
      if (!doc.hidden) check(false);
    }, INTERVAL);

    welcome(doc, body);
  }

  function welcome(doc, body) {
    var store = storage();
    if (!store) return;
    var note;
    var scroll;
    try {
      note = store.getItem(NOTE_KEY);
      scroll = store.getItem(SCROLL_KEY);
      store.removeItem(NOTE_KEY);
      store.removeItem(SCROLL_KEY);
    } catch (e) {
      return;
    }
    if (scroll) {
      try {
        var place = JSON.parse(scroll);
        if (place && place.at === global.location.pathname + global.location.search && place.y > 0) {
          global.scrollTo(0, place.y);
        }
      } catch (e) { /* a mangled entry is no place to go back to */ }
    }
    if (!note) return;
    var shown = doc.createElement("p");
    shown.className = "deploynote";
    shown.id = "deploy-note";
    shown.setAttribute("role", "status");
    shown.textContent = tix.updatedNote(note);
    body.appendChild(shown);
    global.setTimeout(function () {
      if (shown.parentNode) shown.parentNode.removeChild(shown);
    }, NOTE_FOR);
  }

  if (global.document && global.document.addEventListener) {
    if (global.document.readyState === "loading") {
      global.document.addEventListener("DOMContentLoaded", start);
    } else {
      start();
    }
  }
})(typeof globalThis !== "undefined" ? globalThis : this);
