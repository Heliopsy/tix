// SPDX-License-Identifier: AGPL-3.0-or-later

// Swipe a task list row on a touch screen, the way the app's rows swipe:
// towards the end of the line completes it, towards the start snoozes it
// until tomorrow morning. The row follows the finger over the action's colour
// and lets go back into place; past the threshold the action is posted as an
// ordinary form inside the row, so assets/feel.js swaps the row and brings
// the toast, whose Undo puts it back.
//
// It is a shortcut, never the only way: the tick and the task's own snooze
// menu do the same. Only a row that carries data-swipe-complete or
// data-swipe-snooze (templates/tasks.html, drawn only for what the reader may
// do) answers. A mouse is left alone. The decisions are in decide.js; this
// is the wiring, bound at the document because every boosted navigation
// swaps the body (see the note at the top of live.js).
(function (global) {
  "use strict";
  var tix = global.tix;
  var doc = global.document;
  if (!tix || !tix.swipeAction || !doc || !doc.addEventListener) {
    return;
  }

  function say(text) {
    return tix.t ? tix.t(text) : text;
  }

  var rows = "li.task[data-swipe-complete], li.task[data-swipe-snooze]";
  var drag = null;
  var swallow = null;

  function reduced() {
    var m = typeof global.matchMedia === "function" ? global.matchMedia("(prefers-reduced-motion: reduce)") : null;
    return !!(m && m.matches);
  }

  function behind(row) {
    var el = row.querySelector(".swipe-behind");
    if (!el) {
      el = doc.createElement("span");
      el.className = "swipe-behind";
      el.setAttribute("aria-hidden", "true");
      el.innerHTML = '<svg class="icon" width="20" height="20"><use href=""/></svg><span class="swipe-label"></span>';
      row.appendChild(el);
    }
    return el;
  }

  function draw(d) {
    var x = tix.swipeOffset(d.dx, d.width, d.rtl, d.offers);
    var side = tix.swipeSide(x, d.rtl, d.offers);
    var armed = tix.swipeAction(x, d.width, d.rtl, d.offers) !== "";
    d.row.style.setProperty("--swipe-x", x + "px");
    d.row.style.setProperty("--swipe-w", Math.abs(x) + "px");
    var el = behind(d.row);
    el.className = "swipe-behind" + (side ? " is-" + side : "") + (armed ? " is-armed" : "");
    if (side && el.getAttribute("data-side") !== side) {
      el.setAttribute("data-side", side);
      el.querySelector("use").setAttribute("href", "/assets/icons.svg#" + (side === "complete" ? "check" : "snooze"));
      el.querySelector(".swipe-label").textContent = side === "complete" ? say("Complete") : say("Snooze");
    }
  }

  function settle(row) {
    row.classList.remove("is-swipe-dragging");
    row.style.setProperty("--swipe-x", "0px");
    row.style.setProperty("--swipe-w", "0px");
    global.setTimeout(function () {
      row.classList.remove("is-swiping");
      row.style.removeProperty("--swipe-x");
      row.style.removeProperty("--swipe-w");
      var el = row.querySelector(".swipe-behind");
      if (el) {
        el.remove();
      }
    }, reduced() ? 0 : 180);
  }

  function act(row, action) {
    var tick = row.querySelector("form.tick");
    var route = row.getAttribute(action === "complete" ? "data-swipe-complete" : "data-swipe-snooze");
    if (!tick || !route) {
      return;
    }
    var csrf = tick.querySelector('input[name="csrf_token"]');
    var next = tick.querySelector('input[name="next"]');
    var form = doc.createElement("form");
    form.method = "post";
    form.action = route;
    form.hidden = true;
    form.className = "swipe-form";
    var fields = tix.swipeFields(action, csrf && csrf.value, next && next.value, row.getAttribute("data-swipe-until"));
    for (var i = 0; i < fields.length; i++) {
      var input = doc.createElement("input");
      input.type = "hidden";
      input.name = fields[i][0];
      input.value = fields[i][1];
      form.appendChild(input);
    }
    row.appendChild(form);
    if (global.htmx && typeof global.htmx.process === "function") {
      global.htmx.process(form);
    }
    if (typeof form.requestSubmit === "function") {
      form.requestSubmit();
    } else {
      form.submit();
    }
  }

  doc.addEventListener("pointerdown", function (event) {
    if (drag || !event.isPrimary || !tix.swipesWith(event.pointerType)) {
      return;
    }
    var row = event.target && event.target.closest ? event.target.closest(rows) : null;
    if (!row || row.classList.contains("is-pending") || tix.isTypingTarget(event.target)) {
      return;
    }
    drag = {
      row: row,
      id: event.pointerId,
      x0: event.clientX,
      y0: event.clientY,
      dx: 0,
      axis: "",
      width: row.getBoundingClientRect().width,
      rtl: global.getComputedStyle(row).direction === "rtl",
      offers: {
        complete: row.hasAttribute("data-swipe-complete"),
        snooze: row.hasAttribute("data-swipe-snooze")
      }
    };
  });

  doc.addEventListener("pointermove", function (event) {
    if (!drag || event.pointerId !== drag.id) {
      return;
    }
    var dx = event.clientX - drag.x0;
    if (!drag.axis) {
      drag.axis = tix.swipeAxis(dx, event.clientY - drag.y0, tix.swipeSlop);
      if (drag.axis === "y") {
        drag = null;
        return;
      }
      if (!drag.axis) {
        return;
      }
      drag.row.classList.add("is-swiping", "is-swipe-dragging");
    }
    drag.dx = dx;
    draw(drag);
  });

  function release(event, cancelled) {
    if (!drag || event.pointerId !== drag.id) {
      return;
    }
    var d = drag;
    drag = null;
    if (d.axis !== "x") {
      return;
    }
    var action = cancelled ? "" : tix.swipeAction(tix.swipeOffset(d.dx, d.width, d.rtl, d.offers), d.width, d.rtl, d.offers);
    swallow = { row: d.row, until: Date.now() + 500 };
    settle(d.row);
    if (action) {
      act(d.row, action);
    }
  }

  doc.addEventListener("pointerup", function (event) { release(event, false); });
  doc.addEventListener("pointercancel", function (event) { release(event, true); });

  // A swipe that started on the title or a chip must not also open it.
  doc.addEventListener("click", function (event) {
    if (!swallow) {
      return;
    }
    var s = swallow;
    swallow = null;
    if (Date.now() < s.until && s.row.contains(event.target)) {
      event.preventDefault();
      event.stopPropagation();
    }
  }, true);
})(window);
