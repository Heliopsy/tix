// SPDX-License-Identifier: AGPL-3.0-or-later
// The task screen's Hand to an agent dialog (templates/partials.html,
// "handoff-dlg"): one tab per client, showing that client's block.
//
// Without scripting the tab strip is not drawn and every client's block is
// on the page under its own heading. Here the strip shows one at a time, the
// arrow keys, Home and End move along it, and the last client chosen is kept
// in localStorage so the next task opens on it. Storage that throws, in a
// private window or with site data blocked, only means nothing is
// remembered.
(function (global) {
  "use strict";
  var tix = (global.tix = global.tix || {});
  var KEY = "tix.handoff.client";

  // handoffRemembered is the client this browser chose last, or "".
  tix.handoffRemembered = function (storage) {
    try {
      return (storage && storage.getItem(KEY)) || "";
    } catch (e) {
      return "";
    }
  };

  // handoffRemember keeps the client chosen, when storage lets it.
  tix.handoffRemember = function (storage, id) {
    try {
      if (storage) storage.setItem(KEY, id);
    } catch (e) {
      return;
    }
  };

  function storage() {
    try {
      return global.localStorage;
    } catch (e) {
      return null;
    }
  }

  // handoffSelect makes one client's tab and panel the current ones in one
  // dialog, and reports whether that client is there at all.
  tix.handoffSelect = function (root, id) {
    var tabs = root.querySelectorAll("[data-handoff-tab]");
    var found = false;
    for (var i = 0; i < tabs.length; i++) {
      if (tabs[i].getAttribute("data-handoff-tab") === id) found = true;
    }
    if (!found) return false;
    for (var j = 0; j < tabs.length; j++) {
      var on = tabs[j].getAttribute("data-handoff-tab") === id;
      tabs[j].setAttribute("aria-selected", on ? "true" : "false");
      tabs[j].setAttribute("tabindex", on ? "0" : "-1");
    }
    var panels = root.querySelectorAll("[data-handoff-panel]");
    for (var k = 0; k < panels.length; k++) {
      var current = panels[k].getAttribute("data-handoff-panel") === id;
      if (current) {
        panels[k].classList.add("is-current");
      } else {
        panels[k].classList.remove("is-current");
      }
    }
    return true;
  };

  // handoffRestore puts every hand-off dialog in scope on the remembered
  // client.
  tix.handoffRestore = function (scope, store) {
    var id = tix.handoffRemembered(store);
    if (!id || !scope || !scope.querySelectorAll) return;
    var roots = scope.querySelectorAll("[data-handoff]");
    for (var i = 0; i < roots.length; i++) tix.handoffSelect(roots[i], id);
  };

  // handoffStep is the tab a key moves to from index i of n, or -1.
  tix.handoffStep = function (key, i, n) {
    switch (key) {
      case "ArrowRight":
        return (i + 1) % n;
      case "ArrowLeft":
        return (i - 1 + n) % n;
      case "Home":
        return 0;
      case "End":
        return n - 1;
    }
    return -1;
  };

  if (typeof document === "undefined" || !document.addEventListener) return;

  document.addEventListener("click", function (event) {
    var target = event.target;
    if (!target || typeof target.closest !== "function") return;
    var tab = target.closest("[data-handoff-tab]");
    if (tab) {
      var root = tab.closest("[data-handoff]");
      var id = tab.getAttribute("data-handoff-tab");
      if (root && tix.handoffSelect(root, id)) tix.handoffRemember(storage(), id);
    }
  });

  // In the capture phase, because forms.js opens the dialog from there and
  // stops the click going any further.
  document.addEventListener("click", function (event) {
    var open = event.target && event.target.closest && event.target.closest("[data-dlg-open]");
    var dlg = open && document.getElementById(open.getAttribute("data-dlg-open"));
    if (dlg) tix.handoffRestore(dlg, storage());
  }, true);

  document.addEventListener("keydown", function (event) {
    var tab = event.target && event.target.closest && event.target.closest("[data-handoff-tab]");
    if (!tab) return;
    var tabs = tab.parentNode.querySelectorAll("[data-handoff-tab]");
    var i = Array.prototype.indexOf.call(tabs, tab);
    var next = tix.handoffStep(event.key, i, tabs.length);
    if (next < 0) return;
    event.preventDefault();
    tabs[next].focus();
    tabs[next].click();
  });

  tix.handoffRestore(document, storage());
})(window);
