// SPDX-License-Identifier: AGPL-3.0-or-later

// The scope picker every token form draws (the "scope-picker" partial):
// presets, Select all, Clear, a tri-state checkbox per group and a summary of
// what the token will be able to do. The checkboxes post on their own, so
// everything here is an extra and is drawn only with scripting.
(function () {
  "use strict";
  function say(text) {
    return window.tix && window.tix.t ? window.tix.t(text) : text;
  }
  var tix = (window.tix = window.tix || {});

  // scopeGroupState is what a group's own checkbox shows for its boxes: all,
  // none or some, counting only the ones the reader may tick.
  tix.scopeGroupState = function (boxes) {
    var open = 0;
    var ticked = 0;
    for (var i = 0; i < boxes.length; i++) {
      if (boxes[i].disabled) {
        continue;
      }
      open++;
      if (boxes[i].checked) {
        ticked++;
      }
    }
    if (open === 0 || ticked === 0) {
      return "none";
    }
    return ticked === open ? "all" : "some";
  };

  // scopeSet ticks exactly the boxes want names, leaving disabled ones alone.
  // want of null ticks every enabled box, which is Select all: the scopes
  // the reader holds, one by one, never "*".
  tix.scopeSet = function (boxes, want) {
    for (var i = 0; i < boxes.length; i++) {
      if (boxes[i].disabled) {
        continue;
      }
      boxes[i].checked = want === null ? true : want.indexOf(boxes[i].value) >= 0;
    }
  };

  // scopeSummary is the line under the picker.
  tix.scopeSummary = function (values) {
    if (!values.length) {
      return say("Nothing ticked yet. A token needs at least one scope.");
    }
    return say("This token can use: %s").replace("%s", values.join(", "));
  };

  if (typeof document === "undefined") {
    return;
  }

  function scopeBoxes(root) {
    return Array.prototype.slice.call(root.querySelectorAll('input[name="scopes"]:not([data-scope-star])'));
  }

  function refresh(picker) {
    var groups = picker.querySelectorAll("[data-scope-group]");
    for (var i = 0; i < groups.length; i++) {
      var all = groups[i].querySelector("[data-scope-group-all]");
      var state = tix.scopeGroupState(scopeBoxes(groups[i]));
      if (all) {
        all.checked = state === "all";
        all.indeterminate = state === "some";
      }
    }
    var ticked = [];
    var boxes = picker.querySelectorAll('input[name="scopes"]');
    for (var j = 0; j < boxes.length; j++) {
      if (boxes[j].checked && !boxes[j].disabled) {
        ticked.push(boxes[j].value);
      }
    }
    var summary = picker.querySelector("[data-scope-summary]");
    if (summary) {
      summary.textContent = tix.scopeSummary(ticked);
    }
  }

  document.addEventListener("click", function (event) {
    var btn = event.target && event.target.closest && event.target.closest("[data-scope-preset], [data-scope-select]");
    var picker = btn && btn.closest("[data-scope-picker]");
    if (!picker) {
      return;
    }
    var star = picker.querySelector("[data-scope-star]");
    if (star) {
      star.checked = false;
    }
    var preset = btn.getAttribute("data-scope-preset");
    if (preset !== null) {
      tix.scopeSet(scopeBoxes(picker), preset.split(" "));
    } else {
      tix.scopeSet(scopeBoxes(picker), btn.getAttribute("data-scope-select") === "all" ? null : []);
    }
    refresh(picker);
  });

  document.addEventListener("change", function (event) {
    var box = event.target;
    var picker = box && box.closest && box.closest("[data-scope-picker]");
    if (!picker) {
      return;
    }
    if (box.hasAttribute("data-scope-group-all")) {
      tix.scopeSet(scopeBoxes(box.closest("[data-scope-group]")), box.checked ? null : []);
    }
    refresh(picker);
  });

  function init() {
    var pickers = document.querySelectorAll("[data-scope-picker]");
    for (var i = 0; i < pickers.length; i++) {
      refresh(pickers[i]);
    }
  }
  document.addEventListener("htmx:load", init);
  if (document.readyState !== "loading") {
    init();
  } else {
    document.addEventListener("DOMContentLoaded", init);
  }
})();
