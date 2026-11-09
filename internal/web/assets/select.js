// SPDX-License-Identifier: AGPL-3.0-or-later

// Select mode on the task screen (templates/tasks.html "bulk-bar" and the
// Select toggle). The row boxes belong to the bar's form and every action is
// a submit button, so all of it works without this file: the toggle is a
// link to ?select=1 and the stylesheet shows the boxes and the bar while it
// is pressed. This adds what a link cannot: the toggle presses in place, x
// on a focused row, Shift or Ctrl/Cmd-click on a row's empty space and a long
// press on a touch screen enter the mode, Shift-click ticks a range,
// Shift+arrows extend it, Ctrl or Cmd+A ticks every row, d presses the
// focused row's done control, and Escape, Done or unticking the last box
// leaves. The selection is kept by reference on the toggle, so a row
// redrawn by a swap comes back ticked; a navigation brings a new toggle and
// an empty selection. A ticked box is a changed field, so a live refresh
// already waits while a selection exists (decide.js regionBusy).
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

  // pressHeld reports whether a touch held still long enough to be a long
  // press: within slop pixels of where it started.
  tix.pressHeld = function (start, at, slop) {
    if (!start || !at) return false;
    var dx = at.x - start.x;
    var dy = at.y - start.y;
    return dx * dx + dy * dy <= slop * slop;
  };

  // LONG_PRESS_MS is how long a still finger has to rest on a row.
  tix.LONG_PRESS_MS = 500;
  var SLOP = 10;

  // A press on any of these in a row is the control's own, never an empty
  // area: a link opens, a button presses, a field takes focus.
  var CONTROL = "a, button, input, select, textarea, label, summary, [popover]";
  var NO_HOLD = "button, input, select, textarea, label, summary, [popover]";
  var ROW = "[data-shortcut-row]";

  var doc = global.document;
  if (!doc || !doc.addEventListener) return;

  function toggle() {
    return doc.querySelector("[data-select-toggle]");
  }

  // state is the selection this page keeps: the references ticked, the
  // anchor of the next range, and how many were ticked a moment ago, which
  // is how unticking the last one is told from never having ticked any.
  function state() {
    var t = toggle();
    if (!t) return null;
    if (!t.tixSelect) {
      t.tixSelect = { picked: {}, anchor: "", count: 0 };
      if (t.getAttribute("aria-pressed") === "true") {
        boxes().forEach(function (b) { if (b.checked) t.tixSelect.picked[b.value] = true; });
      }
    }
    return t.tixSelect;
  }

  function boxes() {
    return Array.prototype.slice.call(doc.querySelectorAll("input[data-pick]"));
  }

  function bar() {
    return doc.querySelector("form[data-bulkbar]");
  }

  function say(text) {
    return tix.t ? tix.t(text) : text;
  }

  function on() {
    var t = toggle();
    return !!t && t.getAttribute("aria-pressed") === "true";
  }

  function count(st) {
    var n = 0;
    for (var k in st.picked) if (Object.prototype.hasOwnProperty.call(st.picked, k)) n++;
    return n;
  }

  // sync writes the selection back to the boxes and the bar, and leaves the
  // mode when the reader has just unticked the last box.
  function sync() {
    var st = state();
    if (!st) return;
    boxes().forEach(function (b) { b.checked = !!st.picked[b.value]; });
    var n = count(st);
    if (n === 0 && st.count > 0) setMode(false);
    st.count = n;
    var form = bar();
    var label = form && form.querySelector("[data-bulk-count]");
    if (label) label.textContent = tix.pickCountText(n, say);
  }

  function setMode(want) {
    var t = toggle();
    if (!t) return false;
    t.setAttribute("aria-pressed", want ? "true" : "false");
    if (!want) {
      var st = state();
      st.picked = {};
      st.anchor = "";
      st.count = 0;
      boxes().forEach(function (b) { b.checked = false; });
    }
    return true;
  }

  tix.selectMode = function () { return on(); };
  tix.setSelectMode = function (want) {
    if (!setMode(want)) return false;
    sync();
    return true;
  };

  function boxOf(row) {
    return row && row.querySelector ? row.querySelector("input[data-pick]") : null;
  }

  function rowOf(el) {
    return el && el.closest ? el.closest(ROW) : null;
  }

  function pick(box, value) {
    var st = state();
    if (value) st.picked[box.value] = true;
    else delete st.picked[box.value];
  }

  // range ticks every box from the anchor to this one, with this one's state.
  function range(box, value) {
    var st = state();
    var list = boxes();
    var anchor = -1;
    for (var i = 0; i < list.length; i++) if (list[i].value === st.anchor) anchor = i;
    tix.pickRange(anchor, list.indexOf(box), list.length).forEach(function (j) { pick(list[j], value); });
    st.anchor = box.value;
  }

  function flip(box) {
    var st = state();
    pick(box, !st.picked[box.value]);
    st.anchor = box.value;
  }

  tix.togglePick = function () {
    var box = boxOf(rowOf(doc.activeElement));
    if (!box || !state()) return false;
    setMode(true);
    flip(box);
    sync();
    return true;
  };

  // extendPick moves focus one row and ticks both the row it left and the
  // one it reached, entering the mode first.
  tix.extendPick = function (delta) {
    var box = boxOf(rowOf(doc.activeElement));
    if (!box || !state()) return false;
    var list = boxes();
    var next = list[Math.min(list.length - 1, Math.max(0, list.indexOf(box) + delta))];
    setMode(true);
    pick(box, true);
    pick(next, true);
    state().anchor = next.value;
    sync();
    var row = rowOf(next);
    if (row && row.focus) row.focus();
    return true;
  };

  tix.pickAll = function () {
    if (!bar() || !state() || !tix.pickAllApplies(doc.activeElement)) return false;
    setMode(true);
    boxes().forEach(function (b) { pick(b, true); });
    sync();
    return true;
  };

  // completeFocused presses the focused row's round done control. A task
  // already done, one that cannot be ticked from here, and a board card,
  // which has no done control, leave the key alone.
  tix.completeFocused = function () {
    var row = rowOf(doc.activeElement);
    var check = row && row.querySelector ? row.querySelector("button.check") : null;
    if (!check || !check.classList || check.classList.contains("is-done") || check.classList.contains("is-locked")) return false;
    check.click();
    return true;
  };

  // A press of the toggle or Done answers in place. Capture phase, so the
  // boosted link never navigates.
  doc.addEventListener("click", function (event) {
    var el = event.target;
    if (!el || !el.closest) return;
    var t = el.closest("[data-select-toggle]");
    var exit = el.closest("[data-select-exit]");
    if (!t && !exit) return;
    event.preventDefault();
    event.stopPropagation();
    tix.setSelectMode(t ? !on() : false);
  }, true);

  // The toggle is a link drawn as a button, so Space presses it too.
  doc.addEventListener("keydown", function (event) {
    var el = event.target;
    if (event.key !== " " || !el || !el.closest || !el.closest("[data-select-toggle]")) return;
    event.preventDefault();
    tix.setSelectMode(!on());
  });

  var held = null;
  var swallowUntil = 0;

  function now() {
    return global.Date ? global.Date.now() : 0;
  }

  // The click a long press's release produces is the press, not a click on
  // whatever it landed on. Capture phase, ahead of the link it would follow.
  doc.addEventListener("click", function (event) {
    if (!swallowUntil || now() >= swallowUntil) return;
    swallowUntil = 0;
    event.preventDefault();
    event.stopPropagation();
    if (event.stopImmediatePropagation) event.stopImmediatePropagation();
  }, true);

  doc.addEventListener("click", function (event) {
    var el = event.target;
    if (!el || !el.matches) return;
    if (el.matches("input[data-pick]")) {
      if (!state()) return;
      if (event.shiftKey) range(el, el.checked);
      else {
        pick(el, el.checked);
        state().anchor = el.value;
      }
      sync();
      return;
    }
    var row = rowOf(el);
    var box = boxOf(row);
    if (!box || !state() || (el.closest && el.closest(CONTROL))) return;
    var mod = event.ctrlKey || event.metaKey;
    if (!event.shiftKey && !mod && !on()) return;
    event.preventDefault();
    setMode(true);
    if (event.shiftKey) range(box, true);
    else flip(box);
    sync();
  });

  // Shift-click on a row would otherwise also select its text.
  doc.addEventListener("mousedown", function (event) {
    var el = event.target;
    if (!event.shiftKey || !el || !el.closest || !boxOf(rowOf(el)) || el.closest(CONTROL)) return;
    event.preventDefault();
  });

  function cancelHold() {
    if (held && held.timer) global.clearTimeout(held.timer);
    held = null;
  }

  doc.addEventListener("pointerdown", function (event) {
    cancelHold();
    var el = event.target;
    if (event.pointerType !== "touch" || !el || !el.closest) return;
    var box = boxOf(rowOf(el));
    if (!box || !state() || el.closest(NO_HOLD)) return;
    var start = { x: event.clientX, y: event.clientY };
    held = { start: start, timer: global.setTimeout(function () {
      held = null;
      setMode(true);
      pick(box, true);
      state().anchor = box.value;
      sync();
      swallowUntil = now() + 1000;
    }, tix.LONG_PRESS_MS) };
  });

  doc.addEventListener("pointermove", function (event) {
    if (held && !tix.pressHeld(held.start, { x: event.clientX, y: event.clientY }, SLOP)) cancelHold();
  });
  ["pointerup", "pointercancel"].forEach(function (type) { doc.addEventListener(type, cancelHold); });
  doc.addEventListener("scroll", cancelHold, true);

  // A long press on a link raises the browser's own menu; once the press
  // has selected the row that menu is not what was asked for.
  doc.addEventListener("contextmenu", function (event) {
    if (swallowUntil && now() < swallowUntil) event.preventDefault();
  });
  doc.addEventListener("touchend", function (event) {
    if (swallowUntil && now() < swallowUntil && event.cancelable !== false) event.preventDefault();
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

  // Escape leaves select mode first. Capture phase, so it runs before the
  // shortcut layer turns Escape into Back; an open menu or dialog keeps its
  // own Escape, and a reader typing in a field keeps theirs.
  doc.addEventListener("keydown", function (event) {
    if (event.key !== "Escape" || !on()) return;
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
    tix.setSelectMode(false);
  }, true);

  sync();
  doc.addEventListener("htmx:load", sync);
})(typeof globalThis !== "undefined" ? globalThis : this);
