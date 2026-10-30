// SPDX-License-Identifier: AGPL-3.0-or-later
// The sidebar's width and its collapsed rail.
//
// Loaded from the head without defer, like lanes.js, so the width and the
// collapsed class are on the root before the body is parsed and a reload never
// draws the default sidebar first. Both sit on the root rather than on the
// aside, so a boosted navigation replacing the body does not undo them.
//
// localStorage holds two keys and nothing else: a number of pixels and a
// boolean. Neither says anything about anybody's work, which is the condition
// for living in browser storage (see lanes.js). The default removes each key.
//
// The handle and the toggle are rendered hidden and shown from here, so with
// scripting off neither is offered. Every listener is delegated at the
// document, so a swapped body needs nothing re-bound. All of it only matters
// from 900px up, where app.css draws the sidebar as a column.
(function (global) {
  "use strict";
  var tix = (global.tix = global.tix || {});
  var WIDTH = "tix.sidebar.width";
  var COLLAPSED = "tix.sidebar.collapsed";
  var CLASS = "side-collapsed";
  var DRAGGING = "side-dragging";
  var MIN = 180;
  var MAX = 480;
  var COLLAPSE_AT = 120;
  var STEP = 16;
  var BIG_STEP = 64;

  tix.sidebarLimits = { min: MIN, max: MAX, collapseAt: COLLAPSE_AT };

  // sidebarClamp bounds a width to what the expanded sidebar may be.
  tix.sidebarClamp = function (px) {
    var n = Math.round(Number(px));
    if (!isFinite(n)) return null;
    return Math.min(MAX, Math.max(MIN, n));
  };

  // sidebarWidth reads the stored width, or null for the default. Anything
  // unreadable, including storage that throws, is the default.
  tix.sidebarWidth = function (storage) {
    try {
      var raw = storage ? storage.getItem(WIDTH) : null;
      if (raw === null || !/^\d+$/.test(raw)) return null;
      return tix.sidebarClamp(raw);
    } catch (e) {
      return null;
    }
  };

  // sidebarCollapsed reads the stored collapsed state.
  tix.sidebarCollapsed = function (storage) {
    try {
      return !!storage && storage.getItem(COLLAPSED) === "true";
    } catch (e) {
      return false;
    }
  };

  // sidebarRemember stores a width and a collapsed state, removing each key
  // that holds the default. A null width is the default.
  tix.sidebarRemember = function (storage, width, collapsed) {
    try {
      if (!storage) return;
      if (width === null) {
        storage.removeItem(WIDTH);
      } else {
        storage.setItem(WIDTH, String(tix.sidebarClamp(width)));
      }
      if (collapsed) {
        storage.setItem(COLLAPSED, "true");
      } else {
        storage.removeItem(COLLAPSED);
      }
    } catch (e) {
      return;
    }
  };

  // sidebarKey is the width a key press on the separator asks for, given the
  // current one: a number, "collapse", "reset", or null for a key it ignores.
  tix.sidebarKey = function (key, shift, width) {
    var step = shift ? BIG_STEP : STEP;
    switch (key) {
      case "ArrowLeft":
        return width <= MIN ? "collapse" : tix.sidebarClamp(width - step);
      case "ArrowRight":
        return tix.sidebarClamp(width + step);
      case "Home":
        return MIN;
      case "End":
        return MAX;
      case "Enter":
        return "reset";
      default:
        return null;
    }
  };

  // sidebarDrop is what letting go of a drag at a pointer distance means.
  tix.sidebarDrop = function (px) {
    return px < COLLAPSE_AT ? "collapse" : tix.sidebarClamp(px);
  };

  function storage() {
    try {
      return global.localStorage;
    } catch (e) {
      return null;
    }
  }

  function root() {
    var doc = global.document;
    return doc && doc.documentElement ? doc.documentElement : null;
  }

  function aside() {
    var doc = global.document;
    return doc && doc.querySelector ? doc.querySelector("aside.side") : null;
  }

  function currentWidth(width) {
    if (width !== null) return width;
    var side = aside();
    var measured = side && side.getBoundingClientRect ? side.getBoundingClientRect().width : 0;
    return tix.sidebarClamp(measured || 224);
  }

  function apply(width, collapsed) {
    var el = root();
    if (!el) return;
    if (el.style && el.style.setProperty) {
      if (width === null) {
        el.style.removeProperty("--side-w");
      } else {
        el.style.setProperty("--side-w", width + "px");
      }
    }
    el.classList.toggle(CLASS, collapsed);
    var doc = global.document;
    if (!doc.querySelectorAll) return;
    var handles = doc.querySelectorAll("[data-side-resize]");
    for (var i = 0; i < handles.length; i++) {
      handles[i].hidden = false;
      handles[i].setAttribute("aria-valuemin", String(MIN));
      handles[i].setAttribute("aria-valuemax", String(MAX));
      handles[i].setAttribute("aria-valuenow", String(collapsed ? MIN : currentWidth(width)));
      if (collapsed) {
        handles[i].setAttribute("aria-valuetext", "Collapsed");
      } else {
        handles[i].removeAttribute("aria-valuetext");
      }
    }
    var toggles = doc.querySelectorAll("[data-side-toggle]");
    for (var j = 0; j < toggles.length; j++) {
      toggles[j].hidden = false;
      toggles[j].setAttribute("aria-expanded", collapsed ? "false" : "true");
      toggles[j].setAttribute("aria-label", collapsed ? "Expand sidebar" : "Collapse sidebar");
      toggles[j].setAttribute("title", collapsed ? "Expand sidebar" : "Collapse sidebar");
    }
    var named = doc.querySelectorAll("[data-rail-name]");
    for (var k = 0; k < named.length; k++) {
      if (collapsed) {
        named[k].setAttribute("title", named[k].getAttribute("data-rail-name"));
      } else {
        named[k].removeAttribute("title");
      }
    }
  }

  function set(width, collapsed) {
    tix.sidebarRemember(storage(), width, collapsed);
    apply(width, collapsed);
  }

  tix.sidebarApply = function () {
    var s = storage();
    apply(tix.sidebarWidth(s), tix.sidebarCollapsed(s));
  };

  tix.sidebarToggle = function () {
    var s = storage();
    var collapsed = !tix.sidebarCollapsed(s);
    set(tix.sidebarWidth(s), collapsed);
    return collapsed;
  };

  tix.sidebarApply();

  var doc = global.document;
  if (!doc || !doc.addEventListener) return;
  doc.addEventListener("DOMContentLoaded", tix.sidebarApply);
  doc.addEventListener("htmx:load", tix.sidebarApply);

  function closest(target, selector) {
    return target && target.closest ? target.closest(selector) : null;
  }

  doc.addEventListener("click", function (event) {
    if (closest(event.target, "[data-side-toggle]")) tix.sidebarToggle();
  });

  doc.addEventListener("dblclick", function (event) {
    if (closest(event.target, "[data-side-resize]")) set(null, false);
  });

  doc.addEventListener("keydown", function (event) {
    if (!closest(event.target, "[data-side-resize]")) return;
    var s = storage();
    var collapsed = tix.sidebarCollapsed(s);
    var width = currentWidth(tix.sidebarWidth(s));
    var next = collapsed && event.key === "ArrowRight" ? "expand" : tix.sidebarKey(event.key, event.shiftKey, width);
    if (next === null) return;
    event.preventDefault();
    if (next === "expand") {
      set(tix.sidebarWidth(s), false);
    } else if (next === "collapse") {
      set(tix.sidebarWidth(s), true);
    } else if (next === "reset") {
      set(null, false);
    } else {
      set(next, false);
    }
  });

  var drag = null;

  doc.addEventListener("pointerdown", function (event) {
    var handle = closest(event.target, "[data-side-resize]");
    if (!handle || event.button !== 0) return;
    var side = aside();
    if (!side) return;
    event.preventDefault();
    drag = { left: side.getBoundingClientRect().left, id: event.pointerId, handle: handle };
    if (handle.setPointerCapture) handle.setPointerCapture(event.pointerId);
    root().classList.add(DRAGGING);
  });

  doc.addEventListener("pointermove", function (event) {
    if (!drag || event.pointerId !== drag.id) return;
    var next = tix.sidebarDrop(event.clientX - drag.left);
    if (next === "collapse") {
      apply(tix.sidebarWidth(storage()), true);
    } else {
      apply(next, false);
    }
  });

  function finish(event) {
    if (!drag || event.pointerId !== drag.id) return;
    var next = tix.sidebarDrop(event.clientX - drag.left);
    root().classList.remove(DRAGGING);
    drag = null;
    if (next === "collapse") {
      set(tix.sidebarWidth(storage()), true);
    } else {
      set(next, false);
    }
  }

  doc.addEventListener("pointerup", finish);
  doc.addEventListener("pointercancel", function (event) {
    if (!drag || event.pointerId !== drag.id) return;
    root().classList.remove(DRAGGING);
    drag = null;
    tix.sidebarApply();
  });

  if (global.addEventListener) {
    global.addEventListener("storage", function (event) {
      if (event.key === WIDTH || event.key === COLLAPSED || event.key === null) tix.sidebarApply();
    });
  }
})(window);
