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
//
// The rest of the file is how a lane card's own forms behave under htmx: the
// card is swapped by itself (templates/tasks.html), focus goes back to the
// control that was pressed on the new card, or to its neighbour when the card
// was deleted, and a refusal is said inside the card rather than replacing
// it. The card's menus also answer the arrow keys. Everything is bound at the document and
// resolved at event time, because the body is swapped by boosted navigation.
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

  // laneControl names the control a lane request came from: the form's
  // data-lane-control, or "" for anything that is not a lane card's form.
  tix.laneControl = function (elt) {
    if (!elt || typeof elt.closest !== "function") return "";
    var form = elt.closest("[data-lane-control]");
    if (!form || !form.closest(".lane-card")) return "";
    var name = form.getAttribute("data-lane-control") || "";
    return /^[a-z]+$/.test(name) ? name : "";
  };

  // laneRefusal is the sentence a refused lane action shows, read off the
  // error page the server answered with.
  tix.laneRefusal = function (doc, say) {
    var el = doc && doc.querySelector ? doc.querySelector(".failure-message, .flash.error") : null;
    var text = el && el.textContent ? el.textContent.trim() : "";
    return say("Not saved:") + (text ? " " + text : "");
  };

  // laneFocus puts focus on the named control of a card, or on the card.
  tix.laneFocus = function (card, control) {
    if (!card) return null;
    var target = (control && card.querySelector && card.querySelector('[data-focus="' + control + '"]')) || card;
    if (typeof target.focus === "function") target.focus({ preventScroll: true });
    return target;
  };

  // laneSettle undoes busy.js on a card whose answer is not swapped in: the
  // pressed button stays disabled otherwise, and a form left aria-busy holds
  // every live refresh back for good (decide.js regionWaiting).
  tix.laneSettle = function (card) {
    if (!card || !card.querySelectorAll) return;
    var busy = card.querySelectorAll(".is-busy");
    for (var i = 0; i < busy.length; i++) {
      busy[i].classList.remove("is-busy");
      busy[i].disabled = false;
    }
    var forms = card.querySelectorAll('form[aria-busy="true"]');
    for (var j = 0; j < forms.length; j++) forms[j].removeAttribute("aria-busy");
  };

  // laneMenuStep is where an arrow key moves focus among a menu's count
  // items from index, or -1 for a key that is not one of them. Down and Up
  // wrap, as a menu does; Home and End go to the ends. From outside the
  // items (index -1) Down starts at the top and Up at the bottom.
  tix.laneMenuStep = function (count, index, key) {
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

  // laneNeighbour is the card focus moves to when the one acted on is gone,
  // a delete: the next card in its lane, else the one before, else nothing.
  tix.laneNeighbour = function (card) {
    if (!card) return "";
    var sib = card.nextElementSibling;
    while (sib && !(sib.classList && sib.classList.contains("lane-card"))) sib = sib.nextElementSibling;
    if (!sib) {
      sib = card.previousElementSibling;
      while (sib && !(sib.classList && sib.classList.contains("lane-card"))) sib = sib.previousElementSibling;
    }
    return sib && sib.id ? sib.id : "";
  };

  function say(text) {
    return tix.t ? tix.t(text) : text;
  }

  var ITEMS = 'a[href], button:not([disabled]), input:not([type="hidden"]):not([disabled])';

  function menuItems(menu) {
    var all = menu.querySelectorAll(ITEMS);
    var out = [];
    for (var i = 0; i < all.length; i++) {
      if (!all[i].hidden && !all[i].closest("[hidden]")) out.push(all[i]);
    }
    return out;
  }

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

  // A lane menu answers the arrow keys, and the more actions menu takes
  // focus when it opens so they have somewhere to start. A text field keeps
  // its own arrows: the assignee field's suggestions live on them.
  doc.addEventListener("keydown", function (event) {
    var target = event.target;
    var menu = target && target.closest ? target.closest(".lanepop") : null;
    if (!menu || target.tagName === "INPUT") return;
    var items = menuItems(menu);
    var next = tix.laneMenuStep(items.length, items.indexOf(target), event.key);
    if (next < 0) return;
    event.preventDefault();
    items[next].focus();
  });
  doc.addEventListener("toggle", function (event) {
    var menu = event.target;
    if (!menu || event.newState !== "open" || !menu.hasAttribute || !menu.hasAttribute("data-lane-menu")) return;
    var items = menuItems(menu);
    if (items.length) items[0].focus({ preventScroll: true });
  }, true);
  // A link in a lane menu leaves the menu: it opens the task, in the dialog
  // perhaps, or starts the title editor (taskopen.js), and a menu left open
  // under either would hold every live refresh back. Captured, and this file
  // loads before taskopen.js, so the menu is shut before that one decides.
  doc.addEventListener("click", function (event) {
    var link = event.target && event.target.closest ? event.target.closest(".lanepop a[href]") : null;
    var menu = link && link.closest("[popover]");
    if (!menu || typeof menu.hidePopover !== "function") return;
    try {
      menu.hidePopover();
    } catch (e) {
      return;
    }
  }, true);

  var pending = null;
  doc.addEventListener("htmx:beforeRequest", function (event) {
    var elt = event.detail && event.detail.elt;
    var control = tix.laneControl(elt);
    if (!control) return;
    var card = elt.closest(".lane-card");
    pending = { id: card.id, control: control, neighbour: tix.laneNeighbour(card) };
    var old = card.querySelector(".lane-error");
    if (old && old.parentNode) old.parentNode.removeChild(old);
  });
  // The swap names the card it targets as elt; the form that asked is the
  // request's own elt.
  doc.addEventListener("htmx:beforeSwap", function (event) {
    var detail = event.detail || {};
    var elt = (detail.requestConfig && detail.requestConfig.elt) || detail.elt;
    var control = tix.laneControl(elt);
    if (!control || !detail.xhr || detail.xhr.status < 400) return;
    detail.shouldSwap = false;
    var card = elt.closest(".lane-card");
    var parsed = null;
    try {
      parsed = new global.DOMParser().parseFromString(detail.xhr.responseText || "", "text/html");
    } catch (e) {
      parsed = null;
    }
    var line = doc.createElement("p");
    line.className = "lane-error";
    line.setAttribute("role", "alert");
    line.textContent = tix.laneRefusal(parsed, say);
    var body = card.querySelector(".lane-body") || card;
    body.appendChild(line);
    tix.laneSettle(card);
    try {
      var open = card.querySelectorAll(":popover-open");
      for (var i = 0; i < open.length; i++) {
        if (typeof open[i].hidePopover === "function") open[i].hidePopover();
      }
    } catch (e) {
      // A browser without popovers has none open.
    }
    tix.laneFocus(card, control);
    pending = null;
  });
  doc.addEventListener("htmx:afterSettle", function () {
    if (!pending) return;
    var memo = pending;
    pending = null;
    var card = doc.getElementById(memo.id);
    if (card) {
      tix.laneFocus(card, memo.control);
    } else if (memo.neighbour) {
      tix.laneFocus(doc.getElementById(memo.neighbour), "");
    }
  });
  if (global.addEventListener) {
    global.addEventListener("storage", function (event) {
      if (event.key === KEY || event.key === null) tix.lanesApply();
    });
  }
})(window);
