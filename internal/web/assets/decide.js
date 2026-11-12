// SPDX-License-Identifier: AGPL-3.0-or-later

(function (global) {
  "use strict";
  // say is text in the page's language; see i18n.js.
  function say(text) {
    return global.tix && global.tix.t ? global.tix.t(text) : text;
  }
  // The decisions the other scripts make, lifted out of their DOM handlers so
  // they can be exercised without a browser. Everything here is pure: it takes
  // values, returns values, and touches no document. The handlers in
  // shortcuts.js, live.js and copy.js keep only the wiring. See
  // internal/web/jstest for the suite that covers this file.
  var tix = global.tix = global.tix || {};

  // comboOf renders a keydown event the same way the binding table names a
  // key, so a lookup is a single map access. Ctrl and a Mac's command key
  // are represented, and Shift on a named key (Shift+ArrowDown extends the
  // selection); none of the shipped schemes use alt. Command used to read as the bare key, so command+g jumped to the
  // top of a list on a Mac instead of doing what the browser does with it.
  tix.comboOf = function (event) {
    var key = event && event.key;
    if (typeof key !== "string") {
      return "";
    }
    if (event.ctrlKey && key.length === 1) {
      return "ctrl+" + key.toLowerCase();
    }
    if (event.metaKey && key.length === 1) {
      return "meta+" + key.toLowerCase();
    }
    // A letter carries its shift in its case; a named key says so.
    if (event.shiftKey && key.length > 1) {
      return "shift+" + key;
    }
    return key;
  };

  // pressesNatively reports whether a key is the focused control's own way of
  // being pressed: Enter or Space on a button, Enter on a link. A shortcut
  // bound to either key took it first, so Enter on any focused button did
  // nothing at all and a menu could not be opened from the keyboard.
  tix.pressesNatively = function (el, combo) {
    if (!el || (combo !== "Enter" && combo !== " ")) {
      return false;
    }
    var tag = el.tagName;
    if (tag === "BUTTON" || tag === "SUMMARY") {
      return true;
    }
    return combo === "Enter" && tag === "A";
  };

  // actionsByCombo inverts one scheme's table, since the keydown handler looks
  // a key up but the binding table (and the help overlay) is written the other
  // way around, one or more keys per action.
  tix.actionsByCombo = function (table) {
    var out = {};
    if (!table) {
      return out;
    }
    for (var action in table) {
      if (!Object.prototype.hasOwnProperty.call(table, action)) {
        continue;
      }
      var keys = table[action] || [];
      for (var i = 0; i < keys.length; i++) {
        out[keys[i]] = action;
      }
    }
    return out;
  };

  // isTypingTarget reports whether a keystroke is text entry rather than a
  // shortcut. Every shortcut is gated on this, so a scheme never steals a
  // letter out of something the reader is typing.
  tix.isTypingTarget = function (el) {
    if (!el) {
      return false;
    }
    if (el.isContentEditable) {
      return true;
    }
    var tag = el.tagName;
    return tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT";
  };

  // coalescer returns a trigger that runs at once and then at most once more
  // per window. It is what every live update on every screen goes through.
  //
  // Leading edge first, because the first event is almost always the only
  // one: a single status change had nothing to coalesce and the quiet period
  // was pure added latency in front of the one thing the reader was waiting
  // to see. Trailing run after it, because a multi-hop transition writes its
  // hops as separate events milliseconds apart and the screen should settle
  // on the last of them, not flicker through each.
  //
  // maxWait bounds the trailing run. A pure quiet-period timer restarts on
  // every arrival, so a steady stream from several agents pushed the render
  // further away with each event and could starve it indefinitely. With the
  // bound, a screen under continuous change still redraws on a fixed beat.
  //
  // After a trailing run the window reopens with nothing queued, so two
  // renders never land back to back: the floor between any two is ms. The
  // timer API is a parameter so a test drives it by hand rather than by
  // waiting.
  tix.coalescer = function (ms, run, timers, maxWait) {
    var api = timers || global;
    var limit = maxWait > 0 ? maxWait : ms * 4;
    var quiet = null;
    var cap = null;
    var queued = false;

    function settle() {
      if (cap !== null) {
        api.clearTimeout(cap);
        cap = null;
      }
      if (quiet !== null) {
        api.clearTimeout(quiet);
        quiet = null;
      }
      if (!queued) {
        return;
      }
      queued = false;
      run();
      quiet = api.setTimeout(settle, ms);
    }

    var trigger = function () {
      if (quiet === null) {
        run();
        quiet = api.setTimeout(settle, ms);
        return;
      }
      queued = true;
      api.clearTimeout(quiet);
      quiet = api.setTimeout(settle, ms);
      if (cap === null) {
        cap = api.setTimeout(settle, limit);
      }
    };
    // queued reports a trailing run still to come.
    trigger.queued = function () {
      return queued;
    };
    return trigger;
  };

  // backoffDelay is how long to wait before the next attempt at a connection
  // that keeps failing: doubling from base, capped, and jittered over the
  // lower half of the interval so a server coming back up is not met by every
  // open browser at the same instant. random is a parameter for the same
  // reason the timers are.
  tix.backoffDelay = function (attempt, base, cap, random) {
    var start = base > 0 ? base : 500;
    var ceiling = cap > 0 ? cap : 30000;
    var n = attempt > 0 ? attempt : 0;
    var span = Math.min(ceiling, start * Math.pow(2, n));
    var roll = typeof random === "function" ? random() : Math.random();
    return Math.round(span / 2 + span / 2 * roll);
  };

  // staleNotice is what the indicator says while a screen is not receiving
  // events. It names the consequence rather than the plumbing: a reader does
  // not care that a socket closed, only that what they are looking at may
  // have moved on without them.
  tix.staleNotice = function (polling) {
    if (polling) {
      return "Not live. This view may be out of date; checking every 15 seconds.";
    }
    return "Not live. This view may be out of date; reconnecting.";
  };

  // resumeRefused reports whether a subscription was refused because the
  // cursor it resumed from has been retained out of the log. There is no
  // honest way to replay the gap at that point, so the screen stops trying to
  // and re-fetches its fragment whole instead.
  tix.resumeRefused = function (body) {
    if (!body || typeof body.message !== "string") {
      return false;
    }
    return body.message.indexOf("cursor") >= 0 && body.message.indexOf("no longer available") >= 0;
  };

  // targetKind reads the kind of record a URL fragment names, which is the
  // prefix the templates give every deep-linkable row ("#comment-<id>",
  // "#artifact-<id>"). A link from the activity feed can land on a record
  // that lives inside a closed disclosure, and the kind is what says which
  // disclosure to open.
  tix.targetKind = function (hash) {
    if (typeof hash !== "string") {
      return "";
    }
    var at = hash.charAt(0) === "#" ? hash.slice(1) : hash;
    var dash = at.indexOf("-");
    if (dash <= 0) {
      return "";
    }
    return at.slice(0, dash).toLowerCase();
  };

  // keepsPlace reports whether a boosted swap leaves the reader where they
  // were scrolled. A form that redraws its own page keeps the place, unless
  // the page now carries an answer to what was just asked, such as a sync
  // run's result drawn at the top: keeping the place hid it below the fold.
  tix.keepsPlace = function (keptPath, path, answered) {
    return !answered && typeof keptPath === "string" && keptPath === path;
  };

  // shouldRebind reports whether a setup bound to one element should move to
  // another. The layout boosts every internal navigation, so the element a
  // page-local setup connected to is replaced without the script running
  // again; re-running on htmx:load then has to tell "there is a new one" from
  // "this is the same one I already hold", or clicking between two pages
  // would open a second socket for every visit.
  tix.shouldRebind = function (next, bound) {
    return (next || null) !== (bound || null);
  };

  // wantsEvent reports whether one event is about what a screen is showing.
  // The subscription filter on the socket already narrows by project and by
  // event type, which is everything the server can decide; a screen about one
  // record narrows further here, because the stream carries no filter for
  // "this task" and a comment on a neighbouring task would otherwise rebuild
  // the page. An event naming no subject the screen knows is not for it.
  tix.wantsEvent = function (subject, event) {
    if (!event) {
      return false;
    }
    if (!subject) {
      return true;
    }
    if (event.subject_id === subject) {
      return true;
    }
    var payload = event.payload || {};
    return payload.task_id === subject;
  };

  // fieldDirty reports whether a form control has been changed away from what
  // the server rendered into it. The comparison is against the control's own
  // default, not against a copy taken at load, so it stays right for a
  // control that was itself swapped in by an earlier refresh.
  tix.fieldDirty = function (el) {
    if (!el) {
      return false;
    }
    if (el.type === "checkbox" || el.type === "radio") {
      return !!el.checked !== !!el.defaultChecked;
    }
    if (el.tagName === "SELECT") {
      var options = el.options || [];
      var declared = false;
      for (var i = 0; i < options.length; i++) {
        if (options[i].defaultSelected) {
          declared = true;
          break;
        }
      }
      // Markup that names no selected option still arrives with the first one
      // selected, because the browser picks it. That is not a reader's choice,
      // and reading it as one reports the region busy forever.
      //
      // A multiple select is the exception: the browser picks nothing for it,
      // so selectedIndex starts at -1 and any selection at all is the
      // reader's, the first option included. The scopes control on the token
      // form is exactly that, and under the single-select rule a reader who
      // picked only the first scope was reported as having changed nothing.
      if (!declared) {
        if (el.multiple) {
          return el.selectedIndex >= 0;
        }
        return el.selectedIndex > 0;
      }
      for (var j = 0; j < options.length; j++) {
        if (!!options[j].selected !== !!options[j].defaultSelected) {
          return true;
        }
      }
      return false;
    }
    return typeof el.defaultValue === "string" && el.value !== el.defaultValue;
  };

  // scrollPlaces records how far each scrolling element under root has been
  // scrolled, keyed so the same element can be found in a redrawn copy.
  tix.scrollPlaces = function (elements) {
    var out = {};
    var seen = {};
    for (var i = 0; i < elements.length; i++) {
      var el = elements[i];
      var key = el.tagName + "." + (el.className || "").toString().split(" ")[0];
      seen[key] = (seen[key] || 0) + 1;
      if (el.scrollTop > 0 || el.scrollLeft > 0) {
        out[key + "#" + seen[key]] = [el.scrollTop, el.scrollLeft];
      }
    }
    return out;
  };

  // restorePlaces puts recorded positions back onto a redrawn set.
  tix.restorePlaces = function (elements, places) {
    var seen = {};
    for (var i = 0; i < elements.length; i++) {
      var el = elements[i];
      var key = el.tagName + "." + (el.className || "").toString().split(" ")[0];
      seen[key] = (seen[key] || 0) + 1;
      var at = places[key + "#" + seen[key]];
      if (at) {
        el.scrollTop = at[0];
        el.scrollLeft = at[1];
      }
    }
  };

  // regionBusy reports whether a live region holds work in progress, which is
  // the one thing a refresh must never discard. Three ways it can: a card is
  // mid-drag, the caret is in a field inside the region, or a field in it has
  // been typed into and not yet submitted.
  tix.regionBusy = function (region, active, dragging) {
    if (dragging) {
      return true;
    }
    if (!region) {
      return false;
    }
    if (active && typeof region.contains === "function" && region.contains(active) &&
        tix.isTypingTarget(active)) {
      return true;
    }
    if (tix.regionWaiting(region)) {
      return true;
    }
    var fields = typeof region.querySelectorAll === "function"
      ? region.querySelectorAll("input, textarea, select") : [];
    for (var i = 0; i < fields.length; i++) {
      if (tix.fieldDirty(fields[i])) {
        return true;
      }
    }
    return false;
  };

  // regionWaiting reports whether a region holds a request still in flight
  // (htmx marks its indicator htmx-request, busy.js marks a submitted form
  // aria-busy, feel.js marks what it drew ahead of the answer is-pending) or
  // a menu the reader has open. A refresh landing then would detach the
  // element the response is meant for, draw over the state the reader was
  // just shown, or shut the menu under the pointer.
  tix.regionWaiting = function (region) {
    if (!region || typeof region.querySelector !== "function") {
      return false;
    }
    if (region.querySelector(".htmx-request, form[aria-busy=\"true\"], .is-pending")) {
      return true;
    }
    try {
      return !!region.querySelector(":popover-open");
    } catch (e) {
      return false;
    }
  };

  // focusMemo remembers which element in a region has focus, as the nearest
  // id around it and its data-focus name, so a refresh can put focus back on
  // the same control in the newly drawn region.
  tix.focusMemo = function (active, region) {
    if (!active || !region || typeof region.contains !== "function" || !region.contains(active) ||
        typeof active.closest !== "function") {
      return null;
    }
    // A choice in a menu is remembered as the button that opened the menu:
    // the menu closes with the answer, and its opener is what was pressed.
    var menu = active.closest("[popover][id]");
    var opener = menu && typeof region.querySelector === "function" &&
      region.querySelector('[popovertarget="' + menu.id + '"]');
    if (opener) {
      active = opener;
    }
    var holder = active.closest("[id]");
    if (!holder || !holder.id || holder === region) {
      return null;
    }
    var control = typeof active.getAttribute === "function" ? active.getAttribute("data-focus") || "" : "";
    return { id: holder.id, control: /^[a-z]+$/.test(control) ? control : "" };
  };

  // tickedNext is what a checklist item says while its post is on its way:
  // the opposite of what it said.
  tix.tickedNext = function (checked) {
    return checked === "true" ? "false" : "true";
  };

  // focusRestore focuses what a memo names in the document as it is now,
  // without scrolling, and returns it.
  tix.focusRestore = function (doc, memo) {
    if (!memo || !doc || typeof doc.getElementById !== "function") {
      return null;
    }
    var holder = doc.getElementById(memo.id);
    if (!holder) {
      return null;
    }
    var target = (memo.control && typeof holder.querySelector === "function" &&
      holder.querySelector('[data-focus="' + memo.control + '"]')) || holder;
    if (typeof target.focus === "function") {
      target.focus({ preventScroll: true });
    }
    return target;
  };

  // dragEnabled reports whether the board should attach drag handlers at all.
  // Native HTML5 drag-and-drop has no working touch equivalent, so this is
  // scoped to a fine pointer; a touch user gets the Move disclosure instead.
  tix.dragEnabled = function (dragAttr, matchMedia) {
    if (dragAttr !== "1" || typeof matchMedia !== "function") {
      return false;
    }
    var query = matchMedia("(pointer: fine)");
    return !!(query && query.matches);
  };

  // legalStates reads the states a card may move to out of its own Move
  // disclosure's options, which is the list the server already computed for
  // that card. A prototype-free object so a state named like an Object member
  // cannot pass canDrop by accident.
  //
  // Only the one-hop options count. The menu also offers routes through other
  // states, written "doing>done", and a drop names a column and nothing else:
  // lighting a far column up would walk the task through states the reader
  // never saw, and every one of those is an audit entry and an event. A route
  // is chosen from the menu, where its path is written out, or not at all.
  tix.legalStates = function (options) {
    var out = Object.create(null);
    for (var i = 0; options && i < options.length; i++) {
      var value = options[i] && options[i].value;
      if (typeof value === "string" && value !== "" && value.indexOf(">") < 0) {
        out[value] = true;
      }
    }
    return out;
  };

  // canDrop reports whether a column is one of those states.
  tix.canDrop = function (legal, state) {
    if (!legal || typeof state !== "string" || state === "") {
      return false;
    }
    return legal[state] === true;
  };

  // stripCount turns a column heading into the state's own name, dropping the
  // card count the heading carries.
  tix.stripCount = function (text) {
    if (typeof text !== "string") {
      return "";
    }
    return text.replace(/\s*\(\d+\)\s*$/, "").trim();
  };

  // movedMessage is what the board's live region says after the server
  // confirms a move.
  tix.movedMessage = function (ref, destination) {
    return ref + " moved to " + destination + ".";
  };

  // refusedMessage is what it says instead when the move was refused or the
  // request failed. The card has not moved: nothing moves it in the DOM
  // before the server confirms, so there is no position to roll back and the
  // explanation is the whole of the refusal path.
  tix.refusedMessage = function (ref, destination, reason) {
    var why = typeof reason === "string" ? reason.trim() : "";
    return ref + " was not moved to " + destination + ": " + (why || "the move was refused.");
  };

  // copyValue is what a button carrying its own value copies: the text of
  // data-copy-text, or data-copy-link made absolute against the page's
  // origin, since a pasted "/tasks/x-1" leads nowhere. Only a path of this
  // origin is widened; anything else is copied as written. null means the
  // button names no value and data-copy-target applies instead.
  tix.copyValue = function (btn, origin) {
    if (!btn || typeof btn.getAttribute !== "function") {
      return null;
    }
    var text = btn.getAttribute("data-copy-text");
    if (text !== null && text !== undefined) {
      return text;
    }
    var link = btn.getAttribute("data-copy-link");
    if (link === null || link === undefined) {
      return null;
    }
    if (origin && link.charAt(0) === "/" && link.charAt(1) !== "/") {
      return origin + link;
    }
    return link;
  };

  // copyText copies through whichever mechanism the browser offers and reports
  // the outcome to done. It never throws and never rejects: a clipboard the
  // browser refuses is a "Copy failed" label, not an unhandled error.
  tix.copyText = function (text, deps, done) {
    var d = deps || {};
    var finish = typeof done === "function" ? done : function () {};
    if (d.clipboard && typeof d.clipboard.writeText === "function") {
      var promise;
      try {
        promise = d.clipboard.writeText(text);
      } catch (err) {
        finish(false);
        return;
      }
      if (promise && typeof promise.then === "function") {
        promise.then(function () { finish(true); }, function () { finish(false); });
        return;
      }
      finish(true);
      return;
    }
    var ok = false;
    try {
      ok = !!(typeof d.legacy === "function" && d.legacy(text));
    } catch (err) {
      ok = false;
    }
    finish(ok);
  };

  // fileChoiceLabel reads out what a styled file field has selected, which the
  // hidden native widget would otherwise have said for free.
  tix.fileChoiceLabel = function (files) {
    var count = files && files.length ? files.length : 0;
    if (count > 1) {
      return say("%d files chosen").replace("%d", String(count));
    }
    if (count === 1) {
      return files[0].name;
    }
    return say("No file chosen");
  };

  // taskRefOf is the task a link leads to, when it leads to a task's own
  // screen on this origin, and "" for anything else: another site, a
  // listing, a task's live fragment or one of its action addresses. The
  // dialog opens only what this answers for.
  tix.taskRefOf = function (href, origin) {
    if (typeof href !== "string" || !href) {
      return "";
    }
    var path = href;
    if (origin && path.indexOf(origin + "/") === 0) {
      path = path.slice(origin.length);
    }
    if (path.charAt(0) !== "/" || path.charAt(1) === "/" || path.charAt(1) === "\\") {
      return "";
    }
    path = path.split("#")[0].split("?")[0];
    var m = /^\/tasks\/([^/]+)$/.exec(path);
    if (!m || m[1] === "live") {
      return "";
    }
    try {
      return decodeURIComponent(m[1]);
    } catch (err) {
      return "";
    }
  };

  // dialogLinkStep decides what a plain click on a link inside the open
  // dialog does: "task" draws that task in the dialog, "load" draws a screen
  // that asked to stay in the dialog (data-dialog-load, such as a task's copy
  // form), "leave" takes the whole page there, and "" is an anchor on this
  // page. Only a path on this origin may stay.
  tix.dialogLinkStep = function (href, origin, staysInDialog) {
    if (typeof href !== "string" || !href || href.charAt(0) === "#") {
      return "";
    }
    if (tix.taskRefOf(href, origin)) {
      return "task";
    }
    var path = href;
    if (origin && path.indexOf(origin + "/") === 0) {
      path = path.slice(origin.length);
    }
    var local = path.charAt(0) === "/" && path.charAt(1) !== "/" && path.charAt(1) !== "\\";
    return staysInDialog && local ? "load" : "leave";
  };

  // plainClick reports whether a click is the reader's ordinary primary
  // click. A modifier or another button means a new tab or window, which the
  // browser does by itself and the page must not take over.
  tix.plainClick = function (event) {
    return !!event && (event.button === 0 || event.button === undefined) &&
      !event.ctrlKey && !event.metaKey && !event.shiftKey && !event.altKey &&
      !event.defaultPrevented;
  };

  // dialogAnswer decides what an answer to a request made for the dialog
  // becomes: drawn in it, shown as a refusal above what it holds, or left for
  // the whole page because it is not the task screen (a delete redirects to
  // the listing).
  tix.dialogAnswer = function (status, text) {
    if (!(status >= 200 && status < 400)) {
      return "refused";
    }
    return /\sdata-task-dialog="/.test(text || "") ? "draw" : "leave";
  };

  // dialogPop decides what a move through history does to the dialog. state
  // is the entry arrived at, open whether the dialog is showing, under the
  // address of the page drawn beneath it and here the address now.
  //
  //   close   leave the dialog; the page under it is the entry, keep it
  //   leave   leave the dialog and let the page's own history restore run
  //   load    draw the entry's task in the open dialog
  //   reopen  open the dialog again over the page it was opened over
  //   reload  the entry belongs to another page: load it whole
  //   ignore  not the dialog's business
  tix.dialogPop = function (state, open, under, here) {
    var entry = !!(state && state.tixDialog);
    if (open) {
      if (entry) {
        return "load";
      }
      return here === under ? "close" : "leave";
    }
    if (!entry) {
      return "ignore";
    }
    return state.over === under ? "reopen" : "reload";
  };

  // The task panel: Task opens in set to Panel shows the task dialog beside
  // the listing on a window at least paneLimits.from wide (taskopen.js). The
  // widths are tix-mobile's task panel's (lib/src/core/theme/layout.dart);
  // the breakpoint is higher than its 840 because the web's sidebar is a
  // 224px column from 900px up, not a rail.
  var PANE = { from: 1100, min: 360, max: 720, def: 460, share: 0.6, step: 16, bigStep: 64 };
  tix.paneLimits = PANE;

  // paneOpensIn is where a task followed from a listing opens when the page
  // carries the task dialog: "pane" beside it, or "dialog" over it, which is
  // also what Panel falls back to on a narrow window.
  tix.paneOpensIn = function (paneChosen, windowWidth) {
    return paneChosen && Number(windowWidth) >= PANE.from ? "pane" : "dialog";
  };

  // paneMax is the widest the panel may be with space beside the sidebar:
  // 60% of it, within the panel's own bounds.
  tix.paneMax = function (space) {
    var n = Number(space);
    if (!isFinite(n) || n <= 0) {
      return PANE.max;
    }
    return Math.max(PANE.min, Math.min(PANE.max, Math.round(n * PANE.share)));
  };

  // paneClamp bounds a width to what the panel may be; null for no number.
  tix.paneClamp = function (px, space) {
    if (px === null || px === undefined || px === "") {
      return null;
    }
    var n = Math.round(Number(px));
    if (!isFinite(n)) {
      return null;
    }
    return Math.min(tix.paneMax(space), Math.max(PANE.min, n));
  };

  // paneKey is the width a key on the divider asks for: a number, "reset",
  // or null for a key it ignores. The panel is right of the divider, so Left
  // widens it.
  tix.paneKey = function (key, shift, width, space) {
    var step = shift ? PANE.bigStep : PANE.step;
    switch (key) {
      case "ArrowLeft":
        return tix.paneClamp(width + step, space);
      case "ArrowRight":
        return tix.paneClamp(width - step, space);
      case "Home":
        return PANE.min;
      case "End":
        return tix.paneMax(space);
      case "Enter":
        return "reset";
      default:
        return null;
    }
  };

  // paneDrop is the width a drag of the divider to clientX asks for, the
  // panel running to the window's right edge.
  tix.paneDrop = function (clientX, windowWidth, space) {
    return tix.paneClamp(Number(windowWidth) - Number(clientX), space);
  };

  // paneParse reads the tix_pane cookie's value, "w=NNN": the width, or null
  // for the default or anything unreadable.
  tix.paneParse = function (value) {
    var m = /(?:^|&)w=(\d+)(?:&|$)/.exec(String(value || ""));
    if (!m || Number(m[1]) <= 0) {
      return null;
    }
    return Math.min(PANE.max, Math.max(PANE.min, Number(m[1])));
  };

  // paneFormat writes a width as the cookie's value: "" for the default,
  // which removes the cookie.
  tix.paneFormat = function (width) {
    var n = width === null || width === undefined ? null : Math.round(Number(width));
    if (n === null || !isFinite(n) || n <= 0) {
      return "";
    }
    n = Math.min(PANE.max, Math.max(PANE.min, n));
    return n === PANE.def ? "" : "w=" + n;
  };

  // titleStep decides what finishing a title edit does. refused is the value
  // last refused, passed only when the field lost focus, so leaving a refused
  // value does not send it again; Enter always does.
  tix.titleStep = function (original, value, refused) {
    var v = String(value || "").trim();
    if (v === String(original || "").trim()) {
      return "cancel";
    }
    if (!v) {
      return "empty";
    }
    if (typeof refused === "string" && v === refused) {
      return "keep";
    }
    return "save";
  };

  // titleSaved reads a title save's answer. The save is posted with
  // redirect "manual", so success is the redirect itself, which a browser
  // reports as an opaque redirect with no status.
  tix.titleSaved = function (resp) {
    if (!resp) {
      return false;
    }
    return resp.type === "opaqueredirect" || (resp.status >= 200 && resp.status < 400);
  };

  // editKey reports whether a key on a selected row asks to edit its title.
  tix.editKey = function (event) {
    return !!event && (event.key === "Enter" || event.key === "F2") &&
      !event.ctrlKey && !event.metaKey && !event.altKey && !event.shiftKey;
  };

  // titleLabel names the field a title is edited in.
  tix.titleLabel = function (ref) {
    return say("Title of %s").replace("%s", String(ref || ""));
  };

  // applyBadge copies a freshly rendered unread badge onto the one on screen:
  // its count and whether it shows. mirrors are the other marks that say
  // there is something unread (the phone menu button's dot), shown with it.
  // attr names the attribute holding the count, the unread one by default.
  tix.applyBadge = function (here, fresh, mirrors, attr) {
    if (!here || !fresh) {
      return;
    }
    var name = attr || "data-inbox-count";
    var count = parseInt(fresh.getAttribute(name), 10) || 0;
    here.innerHTML = fresh.innerHTML;
    here.setAttribute(name, String(count));
    var marks = [here].concat(Array.prototype.slice.call(mirrors || []));
    for (var i = 0; i < marks.length; i++) {
      if (count > 0) {
        marks[i].removeAttribute("hidden");
      } else {
        marks[i].setAttribute("hidden", "hidden");
      }
    }
  };

  // mentionAt finds the @handle being typed at the caret: the position of
  // its @ and what follows it so far, or null when the caret is not in one.
  // An @ preceded by a word character is an address, not a mention.
  tix.mentionAt = function (text, caret) {
    var before = String(text || "").slice(0, caret);
    var m = /(^|[^\w@.])@([A-Za-z0-9._-]*)$/.exec(before);
    if (!m) {
      return null;
    }
    return { start: before.length - m[2].length - 1, query: m[2].toLowerCase() };
  };

  // mentionMatches lists the handles a query completes to: those starting
  // with it first, then those containing it, at most limit of them.
  tix.mentionMatches = function (handles, query, limit) {
    var q = String(query || "").toLowerCase();
    var starts = [];
    var inside = [];
    for (var i = 0; i < (handles || []).length; i++) {
      var h = String(handles[i]);
      var low = h.toLowerCase();
      if (low.indexOf(q) === 0) {
        starts.push(h);
      } else if (q && low.indexOf(q) > 0) {
        inside.push(h);
      }
    }
    return starts.concat(inside).slice(0, limit || 8);
  };

  // mentionInsert is the text once a handle is chosen: the @query replaced by
  // @handle and a space, and where the caret goes.
  tix.mentionInsert = function (text, caret, at, handle) {
    var value = String(text || "");
    var head = value.slice(0, at.start) + "@" + handle + " ";
    var rest = value.slice(caret).replace(/^\s/, "");
    return { value: head + rest, caret: head.length };
  };

  // The filter box's suggestions. filterTokenAt finds the word the caret is
  // in: its span, a leading minus, and the key it names ("" for a bare word,
  // "@" for an @handle), with the part typed after the key. A word inside an
  // open quote is free text and gets nothing.
  var filterAliases = { p: "project", s: "status", a: "assignee", prio: "priority" };
  tix.filterTokenAt = function (text, caret) {
    var value = String(text || "");
    var at = typeof caret === "number" ? caret : value.length;
    if ((value.slice(0, at).match(/"/g) || []).length % 2 === 1) {
      return null;
    }
    var start = value.lastIndexOf(" ", at - 1) + 1;
    var end = value.indexOf(" ", at);
    if (end < 0) {
      end = value.length;
    }
    var word = value.slice(start, end);
    var neg = word.charAt(0) === "-";
    if (neg) {
      word = word.slice(1);
    }
    if (word.charAt(0) === "@") {
      return { start: start, end: end, neg: neg, key: "@", typed: word.slice(1) };
    }
    var colon = word.indexOf(":");
    if (colon < 0) {
      return { start: start, end: end, neg: neg, key: "", typed: word };
    }
    var key = word.slice(0, colon).toLowerCase();
    return { start: start, end: end, neg: neg, key: filterAliases[key] || key, typed: word.slice(colon + 1) };
  };

  function rank(values, typed, show) {
    var q = String(typed || "").toLowerCase();
    var starts = [];
    var inside = [];
    for (var i = 0; i < values.length; i++) {
      var v = values[i];
      var hay = (v.match || v.value).toLowerCase();
      var at = hay.indexOf(q);
      if (at === 0 || (q && hay.indexOf(" " + q) > 0)) {
        starts.push(show(v));
      } else if (q && at > 0) {
        inside.push(show(v));
      }
    }
    return starts.concat(inside);
  }

  // filterSuggest lists what a token completes to, from the data the page
  // carries: each a label and the text that replaces the word. A word that is
  // already exactly its only suggestion gets nothing, so Enter applies it.
  tix.filterSuggest = function (data, tok, limit) {
    if (!data || !tok) {
      return [];
    }
    var list = function (name) {
      return (data[name] || []).map(function (v) { return { value: String(v) }; });
    };
    var out;
    var keyed = function (key) {
      return function (v) { return { label: key + ":" + v.value, insert: key + ":" + quoted(v.value), note: v.note || "" }; };
    };
    if (tok.key === "") {
      if (!tok.typed) {
        return [];
      }
      out = rank(list("keys"), tok.typed, function (v) { return { label: v.value + ":", insert: v.value + ":", note: "" }; });
    } else if (tok.key === "@") {
      out = rank(list("person"), tok.typed, function (v) { return { label: "@" + v.value, insert: "@" + v.value, note: "" }; });
    } else if (tok.key === "project") {
      var projects = (data.project || []).map(function (p) {
        return { value: p.k, note: p.n, match: p.k + " " + p.n };
      });
      out = rank(projects, tok.typed, keyed("project"));
    } else if (tok.key === "assignee" || tok.key === "creator" || tok.key === "claimed-by") {
      out = rank(list("person").concat([{ value: "me" }]), tok.typed, keyed(tok.key));
    } else if (data[tok.key]) {
      out = rank(list(tok.key), tok.typed, keyed(tok.key));
    } else {
      return [];
    }
    out = out.slice(0, limit || 8);
    var word = (tok.key === "" ? "" : tok.key === "@" ? "@" : tok.key + ":") + tok.typed;
    if (out.length === 1 && out[0].insert.toLowerCase() === word.toLowerCase()) {
      return [];
    }
    return out;
  };

  function quoted(v) {
    return /[\s"]/.test(v) ? '"' + v.replace(/"/g, "") + '"' : v;
  }

  // filterInsert is the box once a suggestion is taken: the word replaced,
  // its minus kept, a space after a whole term but not after a bare key, and
  // the caret at the end of what was inserted.
  tix.filterInsert = function (text, tok, insert) {
    var value = String(text || "");
    var term = (tok.neg ? "-" : "") + insert;
    var tail = value.slice(tok.end).replace(/^\s/, "");
    var gap = /:$/.test(insert) ? "" : " ";
    var head = value.slice(0, tok.start) + term + gap;
    return { value: head + tail, caret: head.length };
  };

  // filterKey decides what a key press in the box does while suggestions are
  // listed: move, take, close or nothing. Enter takes only an option an arrow
  // reached, so Enter on its own still applies the filter.
  tix.filterKey = function (key, open, count, index) {
    if (!open || !count) {
      return key === "ArrowDown" && count ? { act: "move", index: 0 } : { act: "none" };
    }
    switch (key) {
    case "ArrowDown":
      return { act: "move", index: index < 0 ? 0 : (index + 1) % count };
    case "ArrowUp":
      return { act: "move", index: index <= 0 ? count - 1 : index - 1 };
    case "Enter":
      return index >= 0 ? { act: "take", index: index } : { act: "none" };
    case "Tab":
      return { act: "take", index: index >= 0 ? index : 0 };
    case "Escape":
      return { act: "close" };
    }
    return { act: "none" };
  };

  // filterParams is what the box's in-place filter asks for: the form's
  // fields less the empty ones and the pickers nobody touched, whose
  // pick_KEY still equals the was_KEY it was drawn with. A changed picker
  // keeps both, which the server folds into the expression.
  tix.filterParams = function (pairs) {
    var was = {};
    var pick = {};
    for (var i = 0; i < pairs.length; i++) {
      var name = pairs[i][0];
      if (name.indexOf("was_") === 0) {
        was[name.slice(4)] = pairs[i][1];
      } else if (name.indexOf("pick_") === 0) {
        pick[name.slice(5)] = pairs[i][1];
      }
    }
    var out = [];
    for (var j = 0; j < pairs.length; j++) {
      var n = pairs[j][0];
      var v = pairs[j][1];
      var key = n.indexOf("pick_") === 0 ? n.slice(5) : n.indexOf("was_") === 0 ? n.slice(4) : null;
      if (key !== null && (pick[key] === undefined ? true : pick[key] === (was[key] === undefined ? "" : was[key]))) {
        continue;
      }
      if (String(v).trim() === "") {
        if (key === null) {
          continue;
        }
      }
      out.push([n, v]);
    }
    return out;
  };

  // freshIDs lists the identifiers in after that were not in before, in the
  // order after holds them: the rows a redraw brought.
  tix.freshIDs = function (before, after) {
    var seen = {};
    for (var i = 0; i < (before || []).length; i++) {
      seen[before[i]] = true;
    }
    var out = [];
    for (var j = 0; j < (after || []).length; j++) {
      if (!seen[after[j]]) {
        out.push(after[j]);
      }
    }
    return out;
  };

  // -- swipe ---------------------------------------------------------------
  // A list row on a touch screen swipes the way the app's rows do: towards
  // the end of the line (right, left to right) completes the task, towards
  // its start snoozes it, and a release short of swipeThreshold of the row's
  // width does nothing. assets/swipe.js keeps only the wiring.

  // swipeThreshold is the share of the row's width a swipe must cross, the
  // app's dismiss threshold.
  tix.swipeThreshold = 0.3;

  // swipeSlop is how far a finger moves before the gesture is read as a
  // swipe or a scroll; under it, it is still a tap.
  tix.swipeSlop = 10;

  // swipesWith reports whether a pointer swipes rows: a finger or a pen. A
  // mouse drags across a row to select text, as it always has.
  tix.swipesWith = function (pointerType) {
    return pointerType === "touch" || pointerType === "pen";
  };

  // swipeAxis reads which way a gesture is going once it has moved past
  // slop: "x" for a swipe, "y" for a scroll, "" while it could still be a
  // tap. A diagonal counts as a scroll, so the list is never held back.
  tix.swipeAxis = function (dx, dy, slop) {
    var ax = Math.abs(dx);
    var ay = Math.abs(dy);
    if (Math.max(ax, ay) < slop) {
      return "";
    }
    return ax > ay ? "x" : "y";
  };

  // swipeSide is the action a horizontal drag of dx is revealing, when the
  // row offers it: "complete" towards the line's end, "snooze" towards its
  // start, "" otherwise. offers is {complete: bool, snooze: bool}.
  tix.swipeSide = function (dx, rtl, offers) {
    if (!dx) {
      return "";
    }
    var forward = rtl ? dx < 0 : dx > 0;
    var side = forward ? "complete" : "snooze";
    return offers && offers[side] ? side : "";
  };

  // swipeAction is what letting go after a drag of dx across a row width
  // wide does: the side it reveals once it has crossed the threshold, and
  // nothing short of it.
  tix.swipeAction = function (dx, width, rtl, offers) {
    var side = tix.swipeSide(dx, rtl, offers);
    if (!side || !(width > 0) || Math.abs(dx) < width * tix.swipeThreshold) {
      return "";
    }
    return side;
  };

  // swipeOffset is how far the row follows the finger: not at all towards
  // an action it does not offer, and never past its own width.
  tix.swipeOffset = function (dx, width, rtl, offers) {
    if (!tix.swipeSide(dx, rtl, offers)) {
      return 0;
    }
    var w = width > 0 ? width : Math.abs(dx);
    return Math.max(-w, Math.min(w, dx));
  };

  // swipeFields is what a swiped row posts: the token and the page to come
  // back to, which its tick form already carries, and for a snooze the wake
  // time the row names. The route is the row's own data-swipe-* address.
  tix.swipeFields = function (action, csrf, next, until) {
    var out = [["csrf_token", csrf || ""], ["next", next || ""]];
    if (action === "snooze") {
      out.push(["until", until || ""]);
    }
    return out;
  };
})(typeof globalThis !== "undefined" ? globalThis : this);
