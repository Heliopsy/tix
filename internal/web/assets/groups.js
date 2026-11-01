// SPDX-License-Identifier: AGPL-3.0-or-later

// Heading menus on board columns, lanes and list groups
// (templates/partials.html "group-menu"). Every action is a form that works
// without this file; with htmx it swaps the view's region in place, which
// takes the menu and the button that opened it with it. This puts focus back
// where the reader was: on the same group's three dots, or on the Hidden
// control when the group was just hidden, or on the first heading when there
// is neither. A refusal is said inside the menu rather than replacing the
// page, and the menu answers the arrow keys.
(function (global) {
  "use strict";
  var tix = (global.tix = global.tix || {});

  // groupFocusSelector is what to focus after a heading menu's form swapped
  // its region: the dots of the group acted on, or the Hidden control after
  // a hide. Keys are escaped, since a project key may hold anything a
  // selector reads as syntax.
  tix.groupFocusSelector = function (memo, escape) {
    if (!memo) return "";
    if (memo.hides) return "[data-group-hidden]";
    var key = escape ? escape(memo.key) : memo.key;
    return '[data-group-btn="' + key + '"]';
  };

  // groupMenuStep is where an arrow key moves focus among count items from
  // index, or -1 for a key that is not one of them. Down and Up wrap; Home
  // and End go to the ends.
  tix.groupMenuStep = function (count, index, key) {
    if (!count) return -1;
    switch (key) {
      case "ArrowDown":
        return index < 0 ? 0 : (index + 1) % count;
      case "ArrowUp":
        return index < 0 ? count - 1 : (index - 1 + count) % count;
      case "Home":
        return 0;
      case "End":
        return count - 1;
      default:
        return -1;
    }
  };

  function say(text) {
    return tix.t ? tix.t(text) : text;
  }

  var doc = global.document;
  if (!doc || !doc.addEventListener) return;

  function escape(value) {
    return global.CSS && typeof global.CSS.escape === "function" ? global.CSS.escape(value) : String(value).replace(/["\\]/g, "\\$&");
  }

  var ITEMS = 'a[href], button:not([disabled]), select:not([disabled]), input:not([type="hidden"]):not([disabled])';

  function items(menu) {
    var all = menu.querySelectorAll(ITEMS);
    var out = [];
    for (var i = 0; i < all.length; i++) out.push(all[i]);
    return out;
  }

  doc.addEventListener("keydown", function (event) {
    var target = event.target;
    var menu = target && target.closest ? target.closest("[data-group-menu]") : null;
    if (!menu || target.tagName === "INPUT" || target.tagName === "SELECT") return;
    var list = items(menu);
    var next = tix.groupMenuStep(list.length, list.indexOf(target), event.key);
    if (next < 0) return;
    event.preventDefault();
    list[next].focus();
  });

  var pending = null;
  doc.addEventListener("htmx:beforeRequest", function (event) {
    var elt = event.detail && event.detail.elt;
    var form = elt && elt.closest ? elt.closest("[data-group-control]") : null;
    if (!form) return;
    var region = doc.querySelector(form.getAttribute("hx-target") || "");
    pending = {
      key: form.getAttribute("data-group-control"),
      hides: form.hasAttribute("data-group-hides"),
      region: region && region.id ? region.id : "",
    };
    var old = form.closest("[data-group-menu]");
    var line = old && old.querySelector(".group-error");
    if (line && line.parentNode) line.parentNode.removeChild(line);
  });
  doc.addEventListener("htmx:beforeSwap", function (event) {
    var detail = event.detail || {};
    var elt = (detail.requestConfig && detail.requestConfig.elt) || detail.elt;
    var form = elt && elt.closest ? elt.closest("[data-group-control]") : null;
    if (!form || !detail.xhr || detail.xhr.status < 400) return;
    detail.shouldSwap = false;
    pending = null;
    var parsed = null;
    try {
      parsed = new global.DOMParser().parseFromString(detail.xhr.responseText || "", "text/html");
    } catch (e) {
      parsed = null;
    }
    var said = parsed && parsed.querySelector(".failure-message, .flash.error");
    var line = doc.createElement("p");
    line.className = "flowwhy group-error";
    line.setAttribute("role", "alert");
    line.textContent = said ? said.textContent.trim() : say("That did not work. Nothing was changed.");
    var menu = form.closest("[data-group-menu]") || form;
    menu.insertBefore(line, menu.firstChild ? menu.firstChild.nextSibling : null);
    var busy = form.querySelectorAll(".is-busy");
    for (var i = 0; i < busy.length; i++) {
      busy[i].classList.remove("is-busy");
      busy[i].disabled = false;
    }
    form.removeAttribute("aria-busy");
  });
  doc.addEventListener("htmx:afterSettle", function () {
    if (!pending) return;
    var memo = pending;
    pending = null;
    var region = (memo.region && doc.getElementById(memo.region)) || doc;
    var target = region.querySelector(tix.groupFocusSelector(memo, escape)) || region.querySelector("[data-group-btn]");
    if (target && typeof target.focus === "function") target.focus({ preventScroll: true });
  });
})(window);
