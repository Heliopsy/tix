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
  // key, so a lookup is a single map access. Only the ctrl modifier is
  // represented: none of the shipped schemes use alt or a bare shift chord.
  tix.comboOf = function (event) {
    var key = event && event.key;
    if (typeof key !== "string") {
      return "";
    }
    if (event.ctrlKey && key.length === 1) {
      return "ctrl+" + key.toLowerCase();
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

    return function () {
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
  // aria-busy) or a menu the reader has open. A refresh landing then would
  // detach the element the response is meant for, or shut the menu under the
  // pointer.
  tix.regionWaiting = function (region) {
    if (!region || typeof region.querySelector !== "function") {
      return false;
    }
    if (region.querySelector(".htmx-request, form[aria-busy=\"true\"]")) {
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
    var holder = active.closest("[id]");
    if (!holder || !holder.id || holder === region) {
      return null;
    }
    var control = typeof active.getAttribute === "function" ? active.getAttribute("data-focus") || "" : "";
    return { id: holder.id, control: /^[a-z]+$/.test(control) ? control : "" };
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
})(typeof globalThis !== "undefined" ? globalThis : this);
