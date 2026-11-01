// SPDX-License-Identifier: AGPL-3.0-or-later

// Bulk selection on the task list (templates/tasks.html "bulk-bar"). The row
// boxes belong to the bar's form and every action is a submit button, so all
// of it works without this file. This adds what a plain form cannot: the bar
// hides until a row is ticked and counts the selection, shift-click ticks a
// range, x toggles the focused row, Ctrl or Cmd+A ticks every row while focus
// is in the list, and Escape clears the selection before it does anything
// else. A ticked box is a changed field, so a live refresh already waits
// while a selection exists (decide.js regionBusy).
(function (global) {
  "use strict";
  var tix = (global.tix = global.tix || {});

  // pickRange is the indices a shift-click sets: every box from the anchor to
  // the one clicked, either way round. No anchor, or one that left the list,
  // is a click on that box alone.
  tix.pickRange = function (anchor, index, count) {
    if (index < 0 || index >= count) return [];
    var from = anchor >= 0 && anchor < count ? anchor : index;
    var lo = Math.min(from, index);
    var hi = Math.max(from, index);
    var out = [];
    for (var i = lo; i <= hi; i++) out.push(i);
    return out;
  };

  // pickCountText is what the bar says for n selected rows.
  tix.pickCountText = function (n, say) {
    var text = (say || function (s) { return s; })(n === 1 ? "%d task selected" : "%d tasks selected");
    return text.replace("%d", String(n));
  };

  // pickAllApplies reports whether Ctrl or Cmd+A belongs to the selection:
  // only when focus is on a row or inside the list, never in a field, where
  // it selects the field's text as it always has.
  tix.pickAllApplies = function (active) {
    if (!active || typeof active.closest !== "function") return false;
    if (tix.isTypingTarget && tix.isTypingTarget(active)) return false;
    return !!active.closest(".tasklist, [data-shortcut-row]");
  };

  var doc = global.document;
  if (!doc || !doc.addEventListener) return;

  var anchor = -1;

  function boxes() {
    return Array.prototype.slice.call(doc.querySelectorAll("input[data-pick]"));
  }

  function bar() {
    return doc.querySelector("form[data-bulkbar]");
  }

  function picked() {
    return boxes().filter(function (b) { return b.checked; });
  }

  function say(text) {
    return tix.t ? tix.t(text) : text;
  }

  function refresh() {
    var form = bar();
    if (!form) return;
    var n = picked().length;
    form.hidden = n === 0;
    var count = form.querySelector("[data-bulk-count]");
    if (count) count.textContent = tix.pickCountText(n, say);
  }

  function clear() {
    boxes().forEach(function (b) { b.checked = false; });
    anchor = -1;
    refresh();
  }

  function setAll(on) {
    boxes().forEach(function (b) { b.checked = on; });
    refresh();
  }

  tix.togglePick = function () {
    var active = doc.activeElement;
    var row = active && active.closest ? active.closest("[data-shortcut-row]") : null;
    var box = row ? row.querySelector("input[data-pick]") : null;
    if (!box) return false;
    box.checked = !box.checked;
    anchor = boxes().indexOf(box);
    refresh();
    return true;
  };

  tix.pickAll = function () {
    if (!bar() || !tix.pickAllApplies(doc.activeElement)) return false;
    setAll(true);
    return true;
  };

  doc.addEventListener("click", function (event) {
    var box = event.target;
    if (!box || !box.matches || !box.matches("input[data-pick]")) return;
    var list = boxes();
    var index = list.indexOf(box);
    if (event.shiftKey && anchor >= 0) {
      tix.pickRange(anchor, index, list.length).forEach(function (i) { list[i].checked = box.checked; });
    }
    anchor = index;
    refresh();
  });

  doc.addEventListener("reset", function (event) {
    if (event.target === bar()) {
      global.setTimeout(clear, 0);
    }
  });

  // Enter in the tag or handle box presses the action beside it, rather than
  // the disabled default the form leads with.
  doc.addEventListener("keydown", function (event) {
    var el = event.target;
    if (event.key !== "Enter" || !el || !el.getAttribute) return;
    var id = el.getAttribute("data-bulk-enter");
    var target = id ? doc.getElementById(id) : null;
    if (target) {
      event.preventDefault();
      target.click();
    }
  });

  // Escape clears a selection first. Capture phase, so it runs before the
  // shortcut layer turns Escape into Back; an open menu or dialog keeps its
  // own Escape, and a reader typing in a field keeps theirs.
  doc.addEventListener("keydown", function (event) {
    if (event.key !== "Escape" || !picked().length) return;
    if (doc.querySelector("dialog[open]")) return;
    try {
      if (doc.querySelector(":popover-open")) return;
    } catch (e) {
      // no popover support: nothing can be open
    }
    if (tix.isTypingTarget && tix.isTypingTarget(event.target)) return;
    var help = doc.getElementById("shortcuts-help");
    if (help && !help.hasAttribute("hidden")) return;
    event.preventDefault();
    event.stopImmediatePropagation();
    clear();
  }, true);

  refresh();
  doc.addEventListener("htmx:load", refresh);
})(typeof globalThis !== "undefined" ? globalThis : this);
