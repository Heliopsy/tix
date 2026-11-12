// SPDX-License-Identifier: AGPL-3.0-or-later
// A task opened in a dialog over the page or in a panel beside it, and a
// title renamed where it is listed. Both are preferences
// (internal/web/taskopen.go) and both stand down to plain links: with this
// file absent, or the preference off, every title is a link to the task's
// own page, which is the page a reload or a copied address of the dialog
// draws too.
//
// The panel is the dialog shown without a modal (show() rather than
// showModal()) and fixed to the right of the window, so everything below
// that keeps history, focus and a save's answer right serves both. On a
// window too narrow for it the same element opens as the dialog.
//
// The decisions are in decide.js and covered by internal/web/jstest. What
// is here is the wiring, bound at the document so a boosted navigation that
// swaps the body leaves nothing captured behind (see the note atop live.js).
(function () {
  "use strict";
  var tix = window.tix;
  if (!tix || !document.addEventListener) {
    return;
  }
  function say(text) {
    return tix.t ? tix.t(text) : text;
  }

  var origin = window.location.origin;

  function here() {
    return window.location.pathname + window.location.search;
  }

  function dialog() {
    return document.getElementById("taskdlg");
  }

  function dialogBody() {
    return document.getElementById("taskdlg-body");
  }

  function isOpen() {
    var dlg = dialog();
    return !!(dlg && dlg.open);
  }

  // -- the panel ----------------------------------------------------------------

  var PANE_COOKIE = "tix_pane";
  var YEAR = 60 * 60 * 24 * 365;

  function paneChosen() {
    var dlg = dialog();
    return !!(dlg && dlg.hasAttribute("data-task-pane"));
  }

  // inPane reports whether the dialog is open beside the listing.
  function inPane() {
    var dlg = dialog();
    return !!(dlg && dlg.open && dlg.getAttribute("data-shown") === "pane");
  }

  function windowWidth() {
    return window.innerWidth || 0;
  }

  function rootEl() {
    return document.documentElement || null;
  }

  function closest(target, selector) {
    return target && typeof target.closest === "function" ? target.closest(selector) : null;
  }

  function menuOpen() {
    try {
      return !!document.querySelector(":popover-open");
    } catch (err) {
      return false;
    }
  }

  // paneSpace is the room beside the sidebar, which the panel takes at most
  // 60% of.
  function paneSpace() {
    var side = document.querySelector("aside.side");
    var taken = side && side.getBoundingClientRect ? side.getBoundingClientRect().width : 0;
    return windowWidth() - taken;
  }

  function readPaneCookie() {
    var all = "";
    try {
      all = typeof document.cookie === "string" ? document.cookie : "";
    } catch (err) {
      return "";
    }
    var pairs = all.split(";");
    for (var i = 0; i < pairs.length; i++) {
      var pair = pairs[i].replace(/^\s+/, "");
      if (pair.indexOf(PANE_COOKIE + "=") === 0) {
        return pair.slice(PANE_COOKIE.length + 1);
      }
    }
    return "";
  }

  function paneWidth() {
    var kept = tix.paneParse(readPaneCookie());
    return tix.paneClamp(kept === null ? tix.paneLimits.def : kept, paneSpace());
  }

  // drawPaneWidth sets the panel's width and what its divider announces.
  function drawPaneWidth(width) {
    var root = rootEl();
    if (root && root.style && root.style.setProperty) {
      root.style.setProperty("--pane-w", width + "px");
    }
    var dlg = dialog();
    var handle = dlg && dlg.querySelector("[data-pane-resize]");
    if (handle) {
      handle.setAttribute("aria-valuemin", String(tix.paneLimits.min));
      handle.setAttribute("aria-valuemax", String(tix.paneMax(paneSpace())));
      handle.setAttribute("aria-valuenow", String(width));
    }
  }

  // savePaneWidth keeps a width for this browser, as the sidebar keeps its
  // own (sidebar.js), and draws it; null is the default.
  function savePaneWidth(width) {
    var value = tix.paneFormat(width);
    var secure = window.location.protocol === "https:" ? "; Secure" : "";
    try {
      document.cookie = PANE_COOKIE + "=" + value + "; Path=/; Max-Age=" + (value ? YEAR : 0) + "; SameSite=Lax" + secure;
    } catch (err) {
      // A browser refusing cookies keeps the width until the panel closes.
    }
    drawPaneWidth(tix.paneClamp(width === null ? tix.paneLimits.def : width, paneSpace()));
  }

  // markRow marks the row of the task in the panel, wherever the listing
  // draws one, and unmarks any other. Called again after a refresh redraws
  // the listing.
  function markRow() {
    var marked = document.querySelectorAll ? document.querySelectorAll("[data-pane-current]") : [];
    for (var i = 0; i < marked.length; i++) {
      marked[i].removeAttribute("aria-current");
      marked[i].removeAttribute("data-pane-current");
    }
    if (!inPane()) {
      return;
    }
    var body = dialogBody();
    var frag = body && body.querySelector("[data-task-dialog]");
    var ref = (frag && frag.getAttribute("data-task-dialog")) || tix.taskRefOf(here(), origin);
    if (!ref || !document.querySelectorAll) {
      return;
    }
    var esc = window.CSS && window.CSS.escape ? window.CSS.escape(ref) : ref;
    var rows = document.querySelectorAll('[data-shortcut-href="/tasks/' + esc + '"]');
    for (var j = 0; j < rows.length; j++) {
      rows[j].setAttribute("aria-current", "true");
      rows[j].setAttribute("data-pane-current", "");
    }
  }

  function setPaneOpen(open) {
    var root = rootEl();
    if (!root) {
      return;
    }
    if (open) {
      root.setAttribute("data-pane-open", "");
    } else {
      root.removeAttribute("data-pane-open");
    }
  }

  // showIn opens the closed dialog beside the listing or over it.
  function showIn(dlg, where) {
    if (where === "pane" && typeof dlg.show === "function") {
      dlg.setAttribute("data-shown", "pane");
      drawPaneWidth(paneWidth());
      setPaneOpen(true);
      dlg.show();
      return;
    }
    dlg.removeAttribute("data-shown");
    setPaneOpen(false);
    dlg.showModal();
  }

  function unpane() {
    var dlg = dialog();
    if (dlg) {
      dlg.removeAttribute("data-shown");
    }
    setPaneOpen(false);
    markRow();
  }

  // switching is set while an open task moves between the panel and the
  // dialog, so the close that move makes is not taken for the reader's.
  var switching = false;

  function reshow(where) {
    var dlg = dialog();
    if (!dlg || !dlg.open) {
      return;
    }
    switching = true;
    dlg.close();
    showIn(dlg, where);
    markRow();
  }

  // poke asks the page's live binding to refresh now (live.js), so the row
  // or card a save changed is redrawn whether or not a socket is open.
  function poke() {
    document.dispatchEvent(new CustomEvent("tix:refresh"));
  }

  // refusalText is the message and hint of the error screen a refused
  // request answered with (templates/error.html).
  function refusalText(html) {
    try {
      var doc = new DOMParser().parseFromString(html || "", "text/html");
      var parts = [];
      var msg = doc.querySelector(".failure-message");
      var hint = doc.querySelector(".failure-hint");
      if (msg) {
        parts.push(msg.textContent.trim());
      }
      if (hint) {
        parts.push(hint.textContent.trim());
      }
      return parts.join(" ");
    } catch (err) {
      return "";
    }
  }

  // -- the dialog -----------------------------------------------------------

  var opener = null;
  var openerRef = "";
  // under is the address of the page drawn beneath the dialog, kept current
  // through htmx's own history moves, so Forward onto a dialog entry can
  // tell the page it was opened over from another one.
  var under = here();
  var over = under;
  // pending is how the answer being fetched enters history: push for a
  // fresh open, replace for a task opened from inside the dialog, none for
  // an entry history itself moved to.
  var pending = null;
  var expectPop = false;
  var closing = false;

  function load(href, how, keepFocus) {
    var dlg = dialog();
    if (!dlg || !window.htmx) {
      return false;
    }
    pending = { href: href, how: how, keep: !!keepFocus };
    window.htmx.ajax("GET", href, {
      source: dlg,
      target: "#taskdlg-body",
      swap: "innerHTML show:none",
      headers: { "X-Tix-Dialog": "1" }
    });
    return true;
  }

  function openTask(href, from) {
    opener = from || document.activeElement;
    openerRef = tix.taskRefOf(href, origin);
    // Another task followed from the listing beside the open panel takes
    // the panel's place in history, so Back still closes it, and focus
    // stays in the listing the reader is going down.
    if (inPane()) {
      return load(href, "replace", true);
    }
    over = here();
    under = over;
    if (!load(href, "push")) {
      opener = null;
      return false;
    }
    return true;
  }

  function shown() {
    var dlg = dialog();
    var how = pending ? pending.how : "";
    var href = pending ? pending.href : "";
    var keep = pending ? pending.keep : false;
    pending = null;
    if (!dlg) {
      return;
    }
    if (how === "push") {
      window.history.pushState({ tixDialog: true, over: over }, "", href);
    } else if (how === "replace") {
      window.history.replaceState({ tixDialog: true, over: over }, "", href);
    }
    var body = dialogBody();
    if (body && how) {
      body.scrollTop = 0;
    }
    if (!dlg.open) {
      showIn(dlg, tix.paneOpensIn(paneChosen(), windowWidth()));
    }
    markRow();
    var close = dlg.querySelector("[data-dialog-close]");
    if (how && close && !keep) {
      close.focus();
    }
  }

  function restoreFocus() {
    var target = opener && document.contains(opener) ? opener : null;
    if (!target && openerRef) {
      var esc = window.CSS && window.CSS.escape ? window.CSS.escape(openerRef) : openerRef;
      var row = document.querySelector('[data-shortcut-href="/tasks/' + esc + '"]');
      if (row) {
        target = row.querySelector("a[href]") || row;
      }
    }
    opener = null;
    if (target && typeof target.focus === "function") {
      target.focus();
    }
  }

  // closeNow closes the dialog without touching history: the caller has
  // either moved history already or is about to leave the page.
  function closeNow() {
    var dlg = dialog();
    if (!dlg) {
      return;
    }
    closing = true;
    if (dlg.open) {
      dlg.close();
    }
    closing = false;
    unpane();
    var body = dialogBody();
    if (body) {
      body.innerHTML = "";
    }
    restoreFocus();
  }

  // requestClose is every way out a reader takes: Escape, Close, the
  // backdrop. While the dialog has an entry of its own it goes Back, so the
  // address returns to the page's and Forward can open it again.
  function requestClose() {
    var state = window.history.state;
    if (state && state.tixDialog) {
      expectPop = true;
      window.history.back();
      return;
    }
    closeNow();
  }

  // leaveTo follows a link out of the dialog to a screen of its own. The
  // dialog's entry becomes the page's again first, so Back from there
  // returns to the page rather than to a dialog over nothing.
  function leaveTo(href) {
    var state = window.history.state;
    if (state && state.tixDialog) {
      window.history.replaceState(null, "", state.over);
    }
    window.location.assign(href);
  }

  function showRefusal(text) {
    var body = dialogBody();
    if (!body) {
      return;
    }
    var note = body.querySelector(".taskdlg-error");
    if (!note) {
      note = document.createElement("p");
      note.className = "flash error taskdlg-error";
      note.setAttribute("role", "alert");
      body.insertBefore(note, body.firstChild);
    }
    note.textContent = text || say("That change was refused.");
  }

  // drawnFrom is the address the answer drawn last came from, after any
  // redirect. A form the dialog drew in place of the task, such as the copy
  // form, answers with the task screen, and the address follows it there so
  // a reload or Forward draws the task rather than the form again.
  var drawnFrom = "";

  function followAnswer() {
    var state = window.history.state;
    var path = drawnFrom.indexOf(origin + "/") === 0 ? drawnFrom.slice(origin.length) : drawnFrom;
    if (!state || !state.tixDialog || tix.taskRefOf(here(), origin) || !tix.taskRefOf(path, origin)) {
      return;
    }
    window.history.replaceState(state, "", path);
  }

  function fromDialog(detail) {
    var body = dialogBody();
    return !!(body && detail && detail.target === body);
  }

  document.addEventListener("htmx:beforeSwap", function (event) {
    var detail = event.detail;
    if (!fromDialog(detail)) {
      return;
    }
    var xhr = detail.xhr;
    var step = tix.dialogAnswer(xhr.status, xhr.responseText);
    if (step === "draw") {
      detail.shouldSwap = true;
      detail.isError = false;
      drawnFrom = xhr.responseURL || "";
      return;
    }
    detail.shouldSwap = false;
    if (step === "refused") {
      if (pending && !isOpen()) {
        // The task could not be opened at all: its own page explains why.
        var href = pending.href;
        pending = null;
        window.location.assign(href);
        return;
      }
      pending = null;
      showRefusal(refusalText(xhr.responseText));
      return;
    }
    pending = null;
    // The dialog's entry becomes the page's first, as leaveTo does: the
    // dialog's close event arrives after this, and finding its entry still
    // there it went Back, aborting the page the answer named and its toast.
    var state = window.history.state;
    if (state && state.tixDialog) {
      window.history.replaceState(null, "", state.over);
    }
    closeNow();
    window.location.replace(xhr.responseURL || over);
  });

  document.addEventListener("htmx:afterSwap", function (event) {
    if (!fromDialog(event.detail)) {
      return;
    }
    var fresh = !!pending;
    shown();
    if (!fresh) {
      followAnswer();
      poke();
    }
  });

  ["htmx:pushedIntoHistory", "htmx:replacedInHistory", "htmx:historyRestore"].forEach(function (name) {
    document.addEventListener(name, function () {
      under = here();
    });
  });

  // Captured on the window, ahead of htmx's own handler, so an entry the
  // dialog owns is never answered by htmx restoring a snapshot of the page
  // over the live one.
  window.addEventListener("popstate", function (event) {
    var dlg = dialog();
    if (expectPop) {
      expectPop = false;
      if (isOpen()) {
        closeNow();
      }
      event.stopImmediatePropagation();
      return;
    }
    var step = tix.dialogPop(event.state, isOpen(), under, here());
    if (step === "ignore") {
      return;
    }
    if (step === "leave") {
      closeNow();
      return;
    }
    event.stopImmediatePropagation();
    if (step === "close") {
      closeNow();
    } else if (step === "load") {
      load(window.location.href, "none");
    } else if (step === "reopen" && dlg) {
      over = event.state.over;
      opener = null;
      openerRef = tix.taskRefOf(window.location.pathname, origin);
      load(window.location.pathname + window.location.search, "none");
    } else {
      window.location.reload();
    }
  }, true);

  document.addEventListener("cancel", function (event) {
    if (event.target === dialog()) {
      event.preventDefault();
      requestClose();
    }
  }, true);

  // A close the page did not ask for, such as the browser's own escape
  // after the cancel above was refused, still has to put history back.
  document.addEventListener("close", function (event) {
    if (event.target === dialog() && switching) {
      switching = false;
      return;
    }
    if (closing || event.target !== dialog()) {
      return;
    }
    unpane();
    var state = window.history.state;
    if (state && state.tixDialog) {
      expectPop = true;
      window.history.back();
    }
    var body = dialogBody();
    if (body) {
      body.innerHTML = "";
    }
    restoreFocus();
  }, true);

  // -- editing a title in place ----------------------------------------------

  function csrf() {
    var field = document.querySelector('input[name="csrf_token"]');
    return field ? field.value : "";
  }

  // startEdit turns a title link into a field. The link names its task and
  // version itself when titles edit on click; the more actions menu's Rename
  // (templates/tasks.html) passes both, as holder, because the reader may
  // not have chosen that.
  function startEdit(link, holder) {
    if (!link || link.hidden || !link.parentNode) {
      return;
    }
    holder = holder || link;
    var ref = holder.getAttribute(holder === link ? "data-edit-title" : "data-rename");
    var original = link.textContent;
    var input = document.createElement("input");
    input.type = "text";
    input.className = "title-edit";
    input.value = original;
    input.setAttribute("aria-label", tix.titleLabel(ref));
    var note = document.createElement("span");
    note.className = "title-edit-error";
    note.id = "title-edit-error-" + ref;
    note.setAttribute("role", "alert");
    note.hidden = true;
    input.setAttribute("aria-describedby", note.id);
    link.hidden = true;
    link.parentNode.insertBefore(input, link.nextSibling);
    input.parentNode.insertBefore(note, input.nextSibling);

    var busy = false;
    var done = false;
    var refused = null;

    function finish(text, focus) {
      done = true;
      if (input.parentNode) {
        input.parentNode.removeChild(input);
      }
      if (note.parentNode) {
        note.parentNode.removeChild(note);
      }
      if (typeof text === "string") {
        link.textContent = text;
      }
      link.hidden = false;
      if (focus) {
        link.focus();
      }
    }

    function refuse(text, value) {
      refused = value;
      note.textContent = text;
      note.hidden = false;
      input.setAttribute("aria-invalid", "true");
    }

    function submit(onBlur) {
      if (busy || done) {
        return;
      }
      var step = tix.titleStep(original, input.value, onBlur ? refused : null);
      if (step === "cancel") {
        finish(undefined, !onBlur);
        return;
      }
      if (step === "keep") {
        return;
      }
      if (step === "empty") {
        refuse(say("A task needs a title."), "");
        return;
      }
      var value = input.value.trim();
      var form = new URLSearchParams();
      form.set("csrf_token", csrf());
      form.set("version", holder.getAttribute("data-version") || "");
      form.set("title", value);
      busy = true;
      input.setAttribute("aria-busy", "true");
      fetch("/tasks/" + encodeURIComponent(ref), {
        method: "POST",
        credentials: "same-origin",
        redirect: "manual",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: form.toString()
      }).then(function (resp) {
        busy = false;
        input.removeAttribute("aria-busy");
        if (tix.titleSaved(resp)) {
          var version = parseInt(holder.getAttribute("data-version"), 10) || 0;
          holder.setAttribute("data-version", String(version + 1));
          finish(value, document.activeElement === input);
          poke();
          return null;
        }
        return resp.text().then(function (html) {
          refuse(refusalText(html) || say("The title could not be saved."), value);
        });
      }).catch(function () {
        busy = false;
        input.removeAttribute("aria-busy");
        refuse(say("The title could not be saved."), value);
      });
    }

    input.addEventListener("keydown", function (event) {
      if (event.key === "Enter") {
        event.preventDefault();
        event.stopPropagation();
        submit(false);
      } else if (event.key === "Escape") {
        event.preventDefault();
        event.stopPropagation();
        finish(undefined, true);
      }
    });
    input.addEventListener("blur", function () {
      submit(true);
    });
    input.addEventListener("click", function (event) {
      event.stopPropagation();
    });
    input.focus();
    input.select();
  }

  // -- clicks and keys --------------------------------------------------------

  // Captured, so it is decided before htmx's boost handler on the link
  // itself sees the click.
  document.addEventListener("click", function (event) {
    var target = event.target;
    var dlg = dialog();
    if (dlg && target === dlg && isOpen() && !inPane()) {
      requestClose();
      return;
    }
    if (!target || typeof target.closest !== "function") {
      return;
    }
    if (target.closest("[data-dialog-close]")) {
      event.preventDefault();
      requestClose();
      return;
    }
    var link = target.closest("a[href]");
    if (!link || !tix.plainClick(event) || link.getAttribute("target") || link.hasAttribute("download")) {
      return;
    }
    var inside = !!(dlg && dlg.contains(link));
    if (link.hasAttribute("data-rename") && !inside) {
      var card = link.closest(".lane-card");
      var title = card && card.querySelector(".lane-title");
      if (title) {
        event.preventDefault();
        event.stopPropagation();
        startEdit(title, link);
        return;
      }
    }
    if (link.hasAttribute("data-edit-title") && !inside) {
      event.preventDefault();
      event.stopPropagation();
      startEdit(link);
      return;
    }
    if (!dlg) {
      return;
    }
    var href = link.getAttribute("href");
    if (!inside) {
      if (tix.taskRefOf(href, origin) || tix.dialogLinkStep(href, origin, link.hasAttribute("data-dialog-load")) === "load") {
        event.preventDefault();
        event.stopPropagation();
        openTask(href, link);
      }
      return;
    }
    var step = tix.dialogLinkStep(href, origin, link.hasAttribute("data-dialog-load"));
    if (!step) {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    if (step === "leave") {
      leaveTo(href);
    } else {
      load(href, "replace");
    }
  }, true);

  // Enter or F2 on a selected row edits its title, when titles edit.
  document.addEventListener("keydown", function (event) {
    var row = document.activeElement;
    if (!row || event.target !== row || !row.hasAttribute || !row.hasAttribute("data-shortcut-row")) {
      return;
    }
    if (!tix.editKey(event)) {
      return;
    }
    var link = row.querySelector("a[data-edit-title]");
    if (!link) {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    startEdit(link);
  }, true);

  // -- the panel's keys, divider and neighbours ---------------------------------

  // Escape inside the panel closes it, as it closes the dialog. A menu open
  // in it takes the key first, and so does a field: Escape there must not
  // throw away what is being typed beside a listing still in view.
  document.addEventListener("keydown", function (event) {
    if (event.key !== "Escape" || event.defaultPrevented || !inPane()) {
      return;
    }
    var dlg = dialog();
    if (!dlg.contains(event.target) || menuOpen() || tix.isTypingTarget(event.target)) {
      return;
    }
    event.preventDefault();
    requestClose();
  });

  document.addEventListener("keydown", function (event) {
    if (!closest(event.target, "[data-pane-resize]") || !inPane()) {
      return;
    }
    var next = tix.paneKey(event.key, event.shiftKey, paneWidth(), paneSpace());
    if (next === null) {
      return;
    }
    event.preventDefault();
    savePaneWidth(next === "reset" ? null : next);
  });

  document.addEventListener("dblclick", function (event) {
    if (closest(event.target, "[data-pane-resize]") && inPane()) {
      savePaneWidth(null);
    }
  });

  var drag = null;

  document.addEventListener("pointerdown", function (event) {
    var handle = closest(event.target, "[data-pane-resize]");
    if (!handle || event.button !== 0 || !inPane()) {
      return;
    }
    event.preventDefault();
    drag = { id: event.pointerId, width: null };
    if (handle.setPointerCapture) {
      handle.setPointerCapture(event.pointerId);
    }
    var root = rootEl();
    if (root) {
      root.setAttribute("data-pane-drag", "");
    }
  });

  document.addEventListener("pointermove", function (event) {
    if (!drag || event.pointerId !== drag.id) {
      return;
    }
    var root = rootEl();
    drag.width = tix.paneDrop(event.clientX, root && root.clientWidth ? root.clientWidth : windowWidth(), paneSpace());
    if (drag.width !== null) {
      drawPaneWidth(drag.width);
    }
  });

  function endDrag(event, keep) {
    if (!drag || event.pointerId !== drag.id) {
      return;
    }
    var width = drag.width;
    drag = null;
    var root = rootEl();
    if (root) {
      root.removeAttribute("data-pane-drag");
    }
    if (keep && width !== null) {
      savePaneWidth(width);
    } else {
      drawPaneWidth(paneWidth());
    }
  }

  document.addEventListener("pointerup", function (event) {
    endDrag(event, true);
  });
  document.addEventListener("pointercancel", function (event) {
    endDrag(event, false);
  });

  // A navigation of the page beside the panel makes the panel's entry the
  // listing's again and closes it before htmx takes its snapshot, so neither
  // Back nor the snapshot brings back a panel over a page it was not opened
  // over.
  document.addEventListener("htmx:beforeHistorySave", function () {
    if (!inPane()) {
      return;
    }
    var state = window.history.state;
    if (state && state.tixDialog) {
      window.history.replaceState(null, "", state.over);
    }
    opener = null;
    openerRef = "";
    closeNow();
  });

  // A refresh of the listing redraws its rows without the mark.
  document.addEventListener("htmx:load", markRow);
  document.addEventListener("tix:regions", markRow);

  // A window resized across the breakpoint moves an open task between the
  // panel and the dialog; within it, the panel keeps to its bounds.
  window.addEventListener("resize", function () {
    var dlg = dialog();
    if (!dlg || !dlg.open || !paneChosen()) {
      return;
    }
    var where = tix.paneOpensIn(true, windowWidth());
    if (where !== (inPane() ? "pane" : "dialog")) {
      reshow(where);
    } else if (where === "pane") {
      drawPaneWidth(paneWidth());
    }
  });

  // openTaskHref is what the command palette asks first (palette.js): a
  // task it offers opens in the dialog when the page has one, and focus
  // returns to from when the dialog closes.
  tix.openTaskHref = function (href, from) {
    if (!href || !dialog() || !tix.taskRefOf(href, origin)) {
      return false;
    }
    return openTask(href, from);
  };

  // openRow is what the "open" shortcut asks first (shortcuts.js): a
  // selected row opens in the dialog when the page has one.
  tix.openRow = function (row) {
    var href = row && row.getAttribute ? row.getAttribute("data-shortcut-href") : "";
    if (!href || !dialog()) {
      return false;
    }
    return openTask(href, row);
  };
})();
