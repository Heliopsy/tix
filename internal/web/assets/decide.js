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

  // debouncer returns a trigger that runs at most once per quiet period. A
  // multi-hop transition writes its hops as separate events a few
  // milliseconds apart; waiting for them all to land is what lets the feed
  // group them the way a page load would. The timer API is a parameter so a
  // test drives it by hand rather than by waiting.
  tix.debouncer = function (ms, run, timers) {
    var api = timers || global;
    var timer = null;
    return function () {
      if (timer !== null) {
        api.clearTimeout(timer);
      }
      timer = api.setTimeout(function () {
        timer = null;
        run();
      }, ms);
    };
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
  tix.legalStates = function (options) {
    var out = Object.create(null);
    for (var i = 0; options && i < options.length; i++) {
      var value = options[i] && options[i].value;
      if (typeof value === "string" && value !== "") {
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
