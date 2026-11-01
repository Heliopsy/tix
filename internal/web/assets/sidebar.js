// SPDX-License-Identifier: AGPL-3.0-or-later
// The sidebar's width, its collapsed rail and its folded groups.
//
// All three live in one cookie, tix_sidebar, which the server reads to draw
// the page (sidebar.go). The first paint of a load, and of every boosted
// navigation, is therefore the sidebar the reader left: nothing here has to
// correct a default a frame after it was drawn, which is what made the
// sidebar's entries jump on every navigation while this state lived in
// localStorage and the markup arrived without it.
//
// The cookie holds a number of pixels, a boolean and a list of group names,
// none of which says anything about anybody's work. The defaults are left
// out, and a cookie holding nothing but defaults is removed.
//
// Loaded from the head without defer. It puts the "js" class on the root
// before the body is parsed, which is what shows the controls that need a
// script ([data-needs-js]) from the first paint instead of revealing them
// after it, and it moves a state an earlier build kept in localStorage into
// the cookie before anything is drawn. Every listener is delegated at the
// document, so a swapped body needs nothing re-bound. Width and rail only
// matter from 900px up, where app.css draws the sidebar as a column.
(function (global) {
  "use strict";
  // say is text in the page's language; see i18n.js.
  function say(text) {
    return global.tix && global.tix.t ? global.tix.t(text) : text;
  }
  var tix = (global.tix = global.tix || {});
  var COOKIE = "tix_sidebar";
  var LEGACY_WIDTH = "tix.sidebar.width";
  var LEGACY_COLLAPSED = "tix.sidebar.collapsed";
  var LEGACY_RECENT = "tix.recent.closed";
  var GROUPS = ["saved", "recent", "configure", "data"];
  var CLASS = "side-collapsed";
  var DRAGGING = "side-dragging";
  var MIN = 180;
  var MAX = 480;
  var COLLAPSE_AT = 120;
  var STEP = 16;
  var BIG_STEP = 64;
  var YEAR = 60 * 60 * 24 * 365;

  tix.sidebarLimits = { min: MIN, max: MAX, collapseAt: COLLAPSE_AT };

  // sidebarClamp bounds a width to what the expanded sidebar may be.
  tix.sidebarClamp = function (px) {
    var n = Math.round(Number(px));
    if (!isFinite(n)) return null;
    return Math.min(MAX, Math.max(MIN, n));
  };

  // sidebarParse reads the cookie's value: {width, collapsed, fold}. A null
  // width is the default; anything unreadable is the default.
  tix.sidebarParse = function (value) {
    var state = { width: null, collapsed: false, fold: [] };
    var parts = String(value || "").split("&");
    for (var i = 0; i < parts.length; i++) {
      var eq = parts[i].indexOf("=");
      if (eq < 0) continue;
      var key = parts[i].slice(0, eq);
      var val = parts[i].slice(eq + 1);
      if (key === "w" && /^\d+$/.test(val) && Number(val) > 0) {
        state.width = tix.sidebarClamp(val);
      } else if (key === "rail") {
        state.collapsed = val === "1";
      } else if (key === "fold") {
        var names = val.split(".");
        for (var j = 0; j < names.length; j++) {
          if (GROUPS.indexOf(names[j]) >= 0 && state.fold.indexOf(names[j]) < 0) state.fold.push(names[j]);
        }
      }
    }
    return state;
  };

  // sidebarFormat writes a state as the cookie's value, leaving defaults out.
  // An empty string means there is nothing to keep.
  tix.sidebarFormat = function (state) {
    var parts = [];
    if (state.width !== null && state.width !== undefined) parts.push("w=" + tix.sidebarClamp(state.width));
    if (state.collapsed) parts.push("rail=1");
    var fold = [];
    for (var i = 0; i < GROUPS.length; i++) {
      if (state.fold && state.fold.indexOf(GROUPS[i]) >= 0) fold.push(GROUPS[i]);
    }
    if (fold.length) parts.push("fold=" + fold.join("."));
    return parts.join("&");
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

  var doc = global.document;

  function readCookie() {
    var all;
    try {
      all = doc && typeof doc.cookie === "string" ? doc.cookie : "";
    } catch (e) {
      return "";
    }
    var pairs = all.split(";");
    for (var i = 0; i < pairs.length; i++) {
      var pair = pairs[i].replace(/^\s+/, "");
      if (pair.indexOf(COOKIE + "=") === 0) return pair.slice(COOKIE.length + 1);
    }
    return "";
  }

  // tix.sidebarState is the state as the cookie holds it now.
  tix.sidebarState = function () {
    return tix.sidebarParse(readCookie());
  };

  function writeCookie(state) {
    var value = tix.sidebarFormat(state);
    var secure = global.location && global.location.protocol === "https:" ? "; Secure" : "";
    var age = value ? YEAR : 0;
    try {
      doc.cookie = COOKIE + "=" + value + "; Path=/; Max-Age=" + age + "; SameSite=Lax" + secure;
    } catch (e) {
      return;
    }
  }

  function storage() {
    try {
      return global.localStorage;
    } catch (e) {
      return null;
    }
  }

  // adopt moves what an earlier build kept in localStorage into the cookie,
  // once, and forgets it there.
  function adopt() {
    var s = storage();
    if (!s) return;
    try {
      var width = s.getItem(LEGACY_WIDTH);
      var collapsed = s.getItem(LEGACY_COLLAPSED);
      var recent = s.getItem(LEGACY_RECENT);
      if (width === null && collapsed === null && recent === null) return;
      var state = tix.sidebarState();
      if (!readCookie()) {
        if (width !== null && /^\d+$/.test(width)) state.width = tix.sidebarClamp(width);
        state.collapsed = collapsed === "true";
        if (recent === "true") state.fold.push("recent");
        writeCookie(state);
      }
      s.removeItem(LEGACY_WIDTH);
      s.removeItem(LEGACY_COLLAPSED);
      s.removeItem(LEGACY_RECENT);
    } catch (e) {
      return;
    }
  }

  function root() {
    return doc && doc.documentElement ? doc.documentElement : null;
  }

  function aside() {
    return doc && doc.querySelector ? doc.querySelector("aside.side") : null;
  }

  function currentWidth(width) {
    if (width !== null) return width;
    var side = aside();
    var measured = side && side.getBoundingClientRect ? side.getBoundingClientRect().width : 0;
    return tix.sidebarClamp(measured || 224);
  }

  function all(selector) {
    return doc && doc.querySelectorAll ? doc.querySelectorAll(selector) : [];
  }

  // apply draws a width and a rail. The server drew the same from the cookie,
  // so on a load or a swap this changes nothing that takes up room: it fills
  // in the separator's value and the rail's tooltips.
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
    var handles = all("[data-side-resize]");
    for (var i = 0; i < handles.length; i++) {
      handles[i].setAttribute("aria-valuemin", String(MIN));
      handles[i].setAttribute("aria-valuemax", String(MAX));
      handles[i].setAttribute("aria-valuenow", String(collapsed ? MIN : currentWidth(width)));
      if (collapsed) {
        handles[i].setAttribute("aria-valuetext", say("Collapsed"));
      } else {
        handles[i].removeAttribute("aria-valuetext");
      }
    }
    var toggles = all("[data-side-toggle]");
    for (var j = 0; j < toggles.length; j++) {
      toggles[j].setAttribute("aria-expanded", collapsed ? "false" : "true");
      var label = say(collapsed ? "Expand sidebar" : "Collapse sidebar");
      toggles[j].setAttribute("aria-label", label);
      toggles[j].setAttribute("title", label);
    }
    var named = all("[data-rail-name]");
    for (var k = 0; k < named.length; k++) {
      if (collapsed) {
        named[k].setAttribute("title", named[k].getAttribute("data-rail-name"));
      } else {
        named[k].removeAttribute("title");
      }
    }
  }

  function set(width, collapsed) {
    var state = tix.sidebarState();
    state.width = width;
    state.collapsed = collapsed;
    writeCookie(state);
    apply(width, collapsed);
  }

  tix.sidebarApply = function () {
    var state = tix.sidebarState();
    apply(state.width, state.collapsed);
  };

  tix.sidebarToggle = function () {
    var state = tix.sidebarState();
    set(state.width, !state.collapsed);
    return !state.collapsed;
  };

  // sidebarFold folds or unfolds one group, drawn and remembered.
  tix.sidebarFold = function (name, folded) {
    if (GROUPS.indexOf(name) < 0) return;
    var state = tix.sidebarState();
    var at = state.fold.indexOf(name);
    if (folded && at < 0) state.fold.push(name);
    if (!folded && at >= 0) state.fold.splice(at, 1);
    writeCookie(state);
    var groups = all('[data-navgroup="' + name + '"]');
    for (var i = 0; i < groups.length; i++) {
      groups[i].classList.toggle("is-folded", folded);
    }
    var buttons = all('[data-navgroup-toggle="' + name + '"]');
    for (var j = 0; j < buttons.length; j++) {
      buttons[j].setAttribute("aria-expanded", folded ? "false" : "true");
    }
  };

  var el = root();
  if (el && el.classList) el.classList.add("js");
  adopt();
  tix.sidebarApply();

  if (!doc || !doc.addEventListener) return;
  doc.addEventListener("DOMContentLoaded", tix.sidebarApply);
  doc.addEventListener("htmx:load", tix.sidebarApply);

  function closest(target, selector) {
    return target && target.closest ? target.closest(selector) : null;
  }

  doc.addEventListener("click", function (event) {
    if (closest(event.target, "[data-side-toggle]")) {
      tix.sidebarToggle();
      return;
    }
    var fold = closest(event.target, "[data-navgroup-toggle]");
    if (!fold) return;
    var name = fold.getAttribute("data-navgroup-toggle");
    var r = root();
    // On the rail a group has no room to unfold into; its heading opens the
    // sidebar instead, with the group open.
    if (r && r.classList.contains(CLASS)) {
      tix.sidebarToggle();
      tix.sidebarFold(name, false);
      return;
    }
    tix.sidebarFold(name, fold.getAttribute("aria-expanded") === "true");
  });

  doc.addEventListener("dblclick", function (event) {
    if (closest(event.target, "[data-side-resize]")) set(null, false);
  });

  doc.addEventListener("keydown", function (event) {
    if (!closest(event.target, "[data-side-resize]")) return;
    var state = tix.sidebarState();
    var width = currentWidth(state.width);
    var next = state.collapsed && event.key === "ArrowRight" ? "expand" : tix.sidebarKey(event.key, event.shiftKey, width);
    if (next === null) return;
    event.preventDefault();
    if (next === "expand") {
      set(state.width, false);
    } else if (next === "collapse") {
      set(state.width, true);
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
      apply(tix.sidebarState().width, true);
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
      set(tix.sidebarState().width, true);
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
})(window);
