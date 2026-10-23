// SPDX-License-Identifier: AGPL-3.0-or-later

(function (global) {
  "use strict";
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
      if (!declared) {
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
    var fields = typeof region.querySelectorAll === "function"
      ? region.querySelectorAll("input, textarea, select") : [];
    for (var i = 0; i < fields.length; i++) {
      if (tix.fieldDirty(fields[i])) {
        return true;
      }
    }
    return false;
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
      return count + " files chosen";
    }
    if (count === 1) {
      return files[0].name;
    }
    return "No file chosen";
  };
})(typeof globalThis !== "undefined" ? globalThis : this);
