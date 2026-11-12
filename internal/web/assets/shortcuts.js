// SPDX-License-Identifier: AGPL-3.0-or-later

(function () {
  "use strict";
  // say is text in the page's language; see i18n.js.
  function say(text) {
    return window.tix && window.tix.t ? window.tix.t(text) : text;
  }
  // The pure decisions this handler makes live in assets/decide.js, which is
  // covered by internal/web/jstest. What is left here is the wiring.
  var tix = window.tix;
  if (!tix) {
    return;
  }
  // The binding table is embedded once per page by internal/web/shortcuts.go
  // as JSON, and both the keydown handler below and the help overlay read it
  // at runtime. Neither ever hardcodes a key or a label, so the two cannot
  // drift apart.
  var dataEl = document.getElementById("tix-shortcuts-data");
  if (!dataEl) {
    return;
  }
  var data;
  try {
    data = JSON.parse(dataEl.textContent);
  } catch (err) {
    return;
  }
  if (!data || !data.bindings || !data.scheme || !data.bindings[data.scheme]) {
    return;
  }

  var comboToAction = tix.actionsByCombo(data.bindings[data.scheme]);
  var prefixes = tix.sequencePrefixes(comboToAction);

  // -- selection: a real DOM focus move over whichever rows this page has --

  function rows() {
    return Array.prototype.slice.call(document.querySelectorAll("[data-shortcut-row]"));
  }

  function focusRow(el) {
    if (el) {
      el.focus();
    }
  }

  function moveSelection(delta) {
    var list = rows();
    if (!list.length) {
      return;
    }
    var idx = list.indexOf(document.activeElement);
    var next;
    if (idx === -1) {
      next = delta > 0 ? 0 : list.length - 1;
    } else {
      next = Math.min(list.length - 1, Math.max(0, idx + delta));
    }
    focusRow(list[next]);
  }

  function jumpTo(which) {
    var list = rows();
    if (!list.length) {
      return;
    }
    focusRow(which === "top" ? list[0] : list[list.length - 1]);
  }

  function openSelected() {
    var el = document.activeElement;
    if (el && el.hasAttribute && el.hasAttribute("data-shortcut-row") && el.dataset.shortcutHref) {
      // In Dialog mode the task opens over the page (taskopen.js).
      if (typeof tix.openRow === "function" && tix.openRow(el)) {
        return;
      }
      window.location.assign(el.dataset.shortcutHref);
    }
  }

  function goBack() {
    if (window.history.length > 1) {
      window.history.back();
    }
  }

  // focusTarget finds the one element this page marked for an action, if
  // any. A page that offers no such control (no comment box because the
  // reader may not comment, no filter box outside the task list) simply has
  // nothing for the key to find, rather than the key doing nothing useful.
  //
  // A target inside a closed disclosure (the task title, which edits behind
  // Rename) opens it first, since a field nobody can see takes no focus.
  function focusTarget(action) {
    var el = document.querySelector('[data-shortcut-target="' + action + '"]');
    if (el) {
      var fold = typeof el.closest === "function" ? el.closest("details") : null;
      if (fold && !fold.open) {
        fold.open = true;
      }
      el.focus();
    }
  }

  // A row's own control first: the star, the priority or the copy on the
  // row the reader is on. With no row focused, the page's own control, never
  // one inside some row, so s on a list with nothing selected stars nothing.
  function focusedRow() {
    var el = document.activeElement;
    return el && typeof el.closest === "function" ? el.closest("[data-shortcut-row]") : null;
  }

  function controlFor(action) {
    var sel = '[data-shortcut-target="' + action + '"]';
    var row = focusedRow();
    if (row) {
      return row.querySelector(sel);
    }
    var all = document.querySelectorAll(sel);
    for (var i = 0; i < all.length; i++) {
      if (typeof all[i].closest !== "function" || !all[i].closest("[data-shortcut-row]")) {
        return all[i];
      }
    }
    return null;
  }

  // pressTarget focuses a field or presses a button the page marked for an
  // action. It answers false when there is none, so the key keeps whatever
  // the browser does with it. A field the rich text editor stands in for is
  // hidden, so the editor inside the same disclosure takes the focus.
  function pressTarget(action) {
    var el = controlFor(action);
    if (!el) {
      return false;
    }
    var fold = typeof el.closest === "function" ? el.closest("details") : null;
    if (fold && !fold.open) {
      fold.open = true;
    }
    if (tix.targetPress(el) !== "focus") {
      el.click();
      return true;
    }
    var editor = el.hidden && fold ? fold.querySelector('[contenteditable="true"]') : null;
    (editor || el).focus();
    return true;
  }

  function goTo(action) {
    var path = data.go && data.go[action];
    if (!path) {
      return false;
    }
    window.location.assign(path);
    return true;
  }

  // page moves the selection a screenful of rows at a time. With no rows on
  // the page the key keeps its native scroll.
  function page(delta) {
    var list = rows();
    var perPage = 10;
    var first = list[0];
    if (first && typeof first.getBoundingClientRect === "function") {
      var height = first.getBoundingClientRect().height;
      if (height > 0 && window.innerHeight > 0) {
        perPage = window.innerHeight / height - 1;
      }
    }
    var next = tix.pageIndex(list.indexOf(document.activeElement), list.length, perPage, delta);
    if (next < 0) {
      return false;
    }
    focusRow(list[next]);
    return true;
  }

  // refresh redraws the screen through its live region when it has one,
  // which keeps the scroll, the focus and anything half typed, and reloads
  // the page otherwise.
  function refresh() {
    if (typeof tix.liveRefresh === "function" && tix.liveRefresh()) {
      return true;
    }
    window.location.reload();
    return true;
  }

  // switchView presses the next of the list, board and lanes links, where
  // the page draws them.
  function switchView() {
    var nav = document.querySelector('[data-shortcut-target="switchView"]');
    var links = nav ? Array.prototype.slice.call(nav.querySelectorAll("a[data-view]")) : [];
    var current = -1;
    for (var i = 0; i < links.length; i++) {
      if (links[i].getAttribute("aria-current") === "page") {
        current = i;
      }
    }
    var next = tix.nextView(links.length, current);
    if (next < 0) {
      return false;
    }
    links[next].click();
    return true;
  }

  // copyRef copies the focused row's reference, or the open task's.
  var toastTimer = null;
  function copyRef() {
    var row = focusedRow();
    var ref = "";
    if (row) {
      ref = tix.refOfRow(row.getAttribute("data-shortcut-href") || "");
    } else {
      var own = controlFor("copyRef");
      ref = own ? own.getAttribute("data-shortcut-ref") || "" : "";
    }
    if (!ref || typeof tix.copyText !== "function") {
      return false;
    }
    var nav = window.navigator || {};
    tix.copyText(ref, { clipboard: nav.clipboard, legacy: tix.legacyCopy }, function (ok) {
      toast(ok ? say("Copied %s").replace("%s", ref) : say("Copy failed"), ok);
    });
    return true;
  }

  function toast(text, ok) {
    var host = document.getElementById("toast");
    if (!host || typeof document.createElement !== "function") {
      return;
    }
    var p = document.createElement("p");
    p.className = "flash" + (ok ? "" : " error");
    p.setAttribute("role", ok ? "status" : "alert");
    host.appendChild(p);
    window.setTimeout(function () {
      p.textContent = text;
    }, 50);
    if (toastTimer) {
      window.clearTimeout(toastTimer);
    }
    toastTimer = window.setTimeout(function () {
      p.remove();
    }, 4000);
  }

  // -- help overlay: proper dialog role, focus trap, focus restored on close --

  // Looked up when used, not when this file ran. The overlay is part of the
  // body, which every boosted navigation replaces, so a reference taken at
  // load pointed at a detached copy after the first click: "?" opened an
  // overlay nobody could see.
  var overlay = null;
  var overlayList = null;
  var overlayDialog = null;
  var returnFocus = null;

  function resolveOverlay() {
    overlay = document.getElementById("shortcuts-help");
    overlayList = document.getElementById("shortcuts-help-list");
    overlayDialog = overlay && overlay.querySelector(".shortcuts-help-dialog");
  }

  function keyLabel(key) {
    var wrap = document.createElement("span");
    var parts = key.split(" ");
    for (var p = 0; p < parts.length; p++) {
      if (p > 0) {
        wrap.appendChild(document.createTextNode(" "));
      }
      var kbd = document.createElement("kbd");
      kbd.textContent = parts[p];
      wrap.appendChild(kbd);
    }
    return wrap;
  }

  // renderHelp draws the active scheme's keys under the four group headings,
  // leaving out an action it binds no key to and a group left empty.
  function renderHelp() {
    if (!overlayList) {
      return;
    }
    overlayList.textContent = "";
    var table = data.bindings[data.scheme] || {};
    var groups = data.groups || [];
    for (var g = 0; g < groups.length; g++) {
      var list = document.createElement("dl");
      var count = 0;
      for (var i = 0; i < data.actions.length; i++) {
        var action = data.actions[i];
        var keys = table[action.name];
        if (action.group !== groups[g].value || !keys || !keys.length) {
          continue;
        }
        var dt = document.createElement("dt");
        for (var k = 0; k < keys.length; k++) {
          if (k > 0) {
            dt.appendChild(document.createTextNode(" " + say("or") + " "));
          }
          dt.appendChild(keyLabel(keys[k]));
        }
        var dd = document.createElement("dd");
        dd.textContent = action.label;
        list.appendChild(dt);
        list.appendChild(dd);
        count++;
      }
      if (!count) {
        continue;
      }
      var head = document.createElement("h3");
      head.textContent = groups[g].label;
      overlayList.appendChild(head);
      overlayList.appendChild(list);
    }
  }

  function focusableIn(container) {
    var found = container.querySelectorAll(
      'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
    );
    return Array.prototype.filter.call(found, function (el) {
      return !el.hasAttribute("disabled");
    });
  }

  function helpOpen() {
    resolveOverlay();
    return !!overlay && !overlay.hasAttribute("hidden");
  }

  function openHelp() {
    if (helpOpen() || !overlay) {
      return;
    }
    renderHelp();
    returnFocus = document.activeElement;
    overlay.removeAttribute("hidden");
    if (overlayDialog) {
      overlayDialog.focus();
    }
  }

  function closeHelp() {
    if (!helpOpen()) {
      return;
    }
    overlay.setAttribute("hidden", "");
    if (returnFocus && document.contains(returnFocus)) {
      returnFocus.focus();
    }
    returnFocus = null;
  }

  document.addEventListener("click", function (event) {
    var el = event.target;
    if (!el || !el.closest) {
      return;
    }
    if (el.closest("[data-shortcuts-open]")) {
      event.preventDefault();
      openHelp();
      return;
    }
    if (el.hasAttribute("data-shortcuts-close") && el.closest("#shortcuts-help")) {
      closeHelp();
    }
  });
  // The overlay owns its own keydown handling while it is open: Escape
  // closes it, and Tab is trapped inside it, ahead of the global handler
  // below which returns early whenever helpOpen() is true.
  document.addEventListener("keydown", function (event) {
      if (!helpOpen()) {
        return;
      }
      if (event.key === "Escape") {
        // stopImmediatePropagation, not just preventDefault: the dispatch
        // listener below is on the document too, and by the time it ran
        // helpOpen() would already be false, so Escape would additionally
        // fire "back" and navigate the page out from under the overlay.
        event.preventDefault();
        event.stopImmediatePropagation();
        closeHelp();
        return;
      }
      if (event.key !== "Tab" || !overlayDialog) {
        return;
      }
      event.stopImmediatePropagation();
      var focusable = focusableIn(overlayDialog);
      if (!focusable.length) {
        event.preventDefault();
        return;
      }
      var first = focusable[0];
      var last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    });

  function revealOpeners() {
    var openers = document.querySelectorAll("[data-shortcuts-open][hidden]");
    for (var i = 0; i < openers.length; i++) {
      openers[i].removeAttribute("hidden");
    }
  }
  revealOpeners();
  document.addEventListener("htmx:load", revealOpeners);

  // -- dispatch ------------------------------------------------------------

  var handlers = {
    goInbox: function () { return goTo("goInbox"); },
    goMyWork: function () { return goTo("goMyWork"); },
    goTasks: function () { return goTo("goTasks"); },
    goProjects: function () { return goTo("goProjects"); },
    goActivity: function () { return goTo("goActivity"); },
    goSettings: function () { return goTo("goSettings"); },
    goStats: function () { return goTo("goStats"); },
    pageDown: function () { return page(1); },
    pageUp: function () { return page(-1); },
    refresh: refresh,
    switchView: switchView,
    editDescription: function () { return pressTarget("editDescription"); },
    tag: function () { return pressTarget("tag"); },
    star: function () { return pressTarget("star"); },
    priority: function () { return pressTarget("priority"); },
    assign: function () { return pressTarget("assign"); },
    due: function () { return pressTarget("due"); },
    snooze: function () { return pressTarget("snooze"); },
    copyRef: copyRef,
    up: function () { moveSelection(-1); },
    down: function () { moveSelection(1); },
    top: function () { jumpTo("top"); },
    bottom: function () { jumpTo("bottom"); },
    open: openSelected,
    back: goBack,
    "new": function () { focusTarget("new"); },
    // Answers false where there is no quick-add box, so the key is the
    // browser's there.
    quickPaste: function () { return typeof tix.quickPaste === "function" && tix.quickPaste(); },
    editTitle: function () { focusTarget("editTitle"); },
    comment: function () { focusTarget("comment"); },
    transition: function () { focusTarget("transition"); },
    // Both answer false when the page has no selection to act on, and the
    // key then keeps whatever the browser does with it.
    select: function () { return typeof tix.togglePick === "function" && tix.togglePick(); },
    selectAll: function () { return typeof tix.pickAll === "function" && tix.pickAll(); },
    extendUp: function () { return typeof tix.extendPick === "function" && tix.extendPick(-1); },
    extendDown: function () { return typeof tix.extendPick === "function" && tix.extendPick(1); },
    complete: function () { return typeof tix.completeFocused === "function" && tix.completeFocused(); },
    filter: function () { focusTarget("filter"); },
    palette: function () {
      if (typeof tix.openPalette === "function") {
        tix.openPalette();
      }
    },
    help: openHelp
  };

  // The palette's Keyboard shortcuts action opens this same overlay.
  tix.openShortcutsHelp = openHelp;

  function menuOpen() {
    try {
      return !!(document.querySelector && document.querySelector(":popover-open"));
    } catch (e) {
      return false;
    }
  }

  // A key that starts a two-key sequence (g in g i) waits up to a second for
  // the second key. When none comes, the key's own action runs, if it has
  // one: a lone g still jumps to the top in the default scheme.
  var pending = "";
  var pendingTimer = null;
  // Holding Shift to type the S of g S is a keydown of its own, which must
  // not end the wait.
  var modifiers = { Shift: true, Control: true, Alt: true, Meta: true, AltGraph: true };

  function stopWaiting() {
    pending = "";
    if (pendingTimer !== null) {
      window.clearTimeout(pendingTimer);
      pendingTimer = null;
    }
  }

  function wait(combo, lone) {
    pending = combo;
    pendingTimer = window.setTimeout(function () {
      pendingTimer = null;
      pending = "";
      var handler = lone && handlers[lone];
      if (handler) {
        handler();
      }
    }, 1000);
  }

  function standsDown(event) {
    if (helpOpen()) {
      return true;
    }
    // A task open in the dialog owns the keyboard: Escape closes it there,
    // and a key meant for the page behind it would act on a page the reader
    // cannot see. The task panel (taskopen.js) owns only the keys pressed in
    // it; the listing beside it is in view and keeps its own.
    var open = document.querySelector && document.querySelector("dialog[open]");
    if (open && (!open.getAttribute || open.getAttribute("data-shown") !== "pane" || open.contains(event.target))) {
      return true;
    }
    // So does an open menu. Escape is the popover's own light dismissal, and
    // the back shortcut taking it as well closed a status menu and then left
    // the page.
    if (menuOpen()) {
      return true;
    }
    return tix.isTypingTarget(event.target);
  }

  document.addEventListener("keydown", function (event) {
    if (standsDown(event)) {
      stopWaiting();
      return;
    }
    if (modifiers[event.key]) {
      return;
    }
    var combo = tix.comboOf(event);
    if (tix.pressesNatively(event.target, combo)) {
      stopWaiting();
      return;
    }
    var step = tix.keyStep(comboToAction, prefixes, pending, combo);
    stopWaiting();
    if (step.pending) {
      wait(step.pending, step.lone);
      event.preventDefault();
      return;
    }
    var handler = step.action && handlers[step.action];
    if (!handler) {
      return;
    }
    if (handler() === false) {
      return;
    }
    event.preventDefault();
  });
})();
