// SPDX-License-Identifier: AGPL-3.0-or-later
// Show completed, for the lanes view of the task screen.
//
// Loaded from the head without defer, so the root class is set before the
// body is parsed and a reader who hid finished cards never sees them flash in.
// The class sits on the root rather than on the lanes, so a live refresh or a
// boosted navigation replacing the listing does not undo it.
//
// localStorage holds one boolean under one key and nothing else: no project
// key, no task reference, no count. Every screen is served no-store because a
// page written to disk outlives the session that rendered it, and browser
// storage is the same disk with no logout hook, so only a preference that
// says nothing about anybody's work may live there. The default, shown,
// removes the key, so it leaves nothing behind.
//
// Stands alone: decide.js is deferred and has not run yet when this does.
(function (global) {
  "use strict";
  var tix = (global.tix = global.tix || {});
  var KEY = "tix.lanes.showCompleted";
  var HIDE = "lanes-hide-finished";

  // lanesShowCompleted reads the stored choice. Anything but "false",
  // including storage that throws, is the default.
  tix.lanesShowCompleted = function (storage) {
    try {
      return !storage || storage.getItem(KEY) !== "false";
    } catch (e) {
      return true;
    }
  };

  // lanesRemember stores the choice, removing the key for the default.
  tix.lanesRemember = function (storage, show) {
    try {
      if (!storage) return;
      if (show) {
        storage.removeItem(KEY);
      } else {
        storage.setItem(KEY, "false");
      }
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

  function apply(show) {
    var doc = global.document;
    if (!doc || !doc.documentElement) return;
    doc.documentElement.classList.toggle(HIDE, !show);
    var toggles = doc.querySelectorAll ? doc.querySelectorAll("[data-lanes-completed]") : [];
    for (var i = 0; i < toggles.length; i++) {
      toggles[i].hidden = false;
      toggles[i].setAttribute("aria-pressed", show ? "true" : "false");
    }
  }

  tix.lanesApply = function () {
    apply(tix.lanesShowCompleted(storage()));
  };

  tix.lanesToggle = function () {
    var show = !tix.lanesShowCompleted(storage());
    tix.lanesRemember(storage(), show);
    apply(show);
    return show;
  };

  tix.lanesApply();

  var doc = global.document;
  if (!doc || !doc.addEventListener) return;
  doc.addEventListener("DOMContentLoaded", tix.lanesApply);
  doc.addEventListener("htmx:load", tix.lanesApply);
  doc.addEventListener("click", function (event) {
    var target = event.target;
    var toggle = target && target.closest ? target.closest("[data-lanes-completed]") : null;
    if (toggle) tix.lanesToggle();
  });
  if (global.addEventListener) {
    global.addEventListener("storage", function (event) {
      if (event.key === KEY || event.key === null) tix.lanesApply();
    });
  }
})(window);
