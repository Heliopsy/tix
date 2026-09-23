// SPDX-License-Identifier: AGPL-3.0-or-later

(function () {
  "use strict";
  // The pure decisions this handler makes live in assets/decide.js, which is
  // covered by internal/web/jstest. What is left here is the wiring.
  var tix = window.tix;
  if (!tix) {
    return;
  }
  // The binding table is embedded once per page by internal/web/shortcuts.go
  // as JSON, and both the keydown handler below and the help overlay read it
  // at runtime. Neither ever hardcodes a key or a label, so the two cannot
  // drift apart.
  var dataEl = document.getElementById("tix-shortcuts-data");
  if (!dataEl) {
    return;
  }
  var data;
  try {
    data = JSON.parse(dataEl.textContent);
  } catch (err) {
    return;
  }
  if (!data || !data.bindings || !data.scheme || !data.bindings[data.scheme]) {
    return;
  }

  var comboToAction = tix.actionsByCombo(data.bindings[data.scheme]);

  // -- selection: a real DOM focus move over whichever rows this page has --

  function rows() {
    return Array.prototype.slice.call(document.querySelectorAll("[data-shortcut-row]"));
  }

  function focusRow(el) {
    if (el) {
      el.focus();
    }
  }

  function moveSelection(delta) {
    var list = rows();
    if (!list.length) {
      return;
    }
    var idx = list.indexOf(document.activeElement);
    var next;
    if (idx === -1) {
      next = delta > 0 ? 0 : list.length - 1;
    } else {
      next = Math.min(list.length - 1, Math.max(0, idx + delta));
    }
    focusRow(list[next]);
  }

  function jumpTo(which) {
    var list = rows();
    if (!list.length) {
      return;
    }
    focusRow(which === "top" ? list[0] : list[list.length - 1]);
  }

  function openSelected() {
    var el = document.activeElement;
    if (el && el.hasAttribute && el.hasAttribute("data-shortcut-row") && el.dataset.shortcutHref) {
      window.location.assign(el.dataset.shortcutHref);
    }
  }

  function goBack() {
    if (window.history.length > 1) {
      window.history.back();
    }
  }

  // focusTarget finds the one element this page marked for an action, if
  // any. A page that offers no such control (no comment box because the
  // reader may not comment, no filter box outside the task list) simply has
  // nothing for the key to find, rather than the key doing nothing useful.
  function focusTarget(action) {
    var el = document.querySelector('[data-shortcut-target="' + action + '"]');
    if (el) {
      el.focus();
    }
  }

  // -- help overlay: proper dialog role, focus trap, focus restored on close --

  var overlay = document.getElementById("shortcuts-help");
  var overlayList = document.getElementById("shortcuts-help-list");
  var overlayDialog = overlay && overlay.querySelector(".shortcuts-help-dialog");
  var returnFocus = null;

  function renderHelp() {
    if (!overlayList) {
      return;
    }
    overlayList.textContent = "";
    var table = data.bindings[data.scheme] || {};
    for (var i = 0; i < data.actions.length; i++) {
      var action = data.actions[i];
      var keys = table[action.name];
      if (!keys || !keys.length) {
        continue;
      }
      var dt = document.createElement("dt");
      for (var k = 0; k < keys.length; k++) {
        if (k > 0) {
          dt.appendChild(document.createTextNode(" or "));
        }
        var kbd = document.createElement("kbd");
        kbd.textContent = keys[k];
        dt.appendChild(kbd);
      }
      var dd = document.createElement("dd");
      dd.textContent = action.label;
      overlayList.appendChild(dt);
      overlayList.appendChild(dd);
    }
  }

  function focusableIn(container) {
    var found = container.querySelectorAll(
      'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
    );
    return Array.prototype.filter.call(found, function (el) {
      return !el.hasAttribute("disabled");
    });
  }

  function helpOpen() {
    return !!overlay && !overlay.hasAttribute("hidden");
  }

  function openHelp() {
    if (!overlay || helpOpen()) {
      return;
    }
    renderHelp();
    returnFocus = document.activeElement;
    overlay.removeAttribute("hidden");
    if (overlayDialog) {
      overlayDialog.focus();
    }
  }

  function closeHelp() {
    if (!overlay || !helpOpen()) {
      return;
    }
    overlay.setAttribute("hidden", "");
    if (returnFocus && document.contains(returnFocus)) {
      returnFocus.focus();
    }
    returnFocus = null;
  }

  if (overlay) {
    overlay.addEventListener("click", function (event) {
      if (event.target && event.target.hasAttribute("data-shortcuts-close")) {
        closeHelp();
      }
    });
    // The overlay owns its own keydown handling while it is open: Escape
    // closes it, and Tab is trapped inside it, ahead of the global handler
    // below which returns early whenever helpOpen() is true.
    overlay.addEventListener("keydown", function (event) {
      if (event.key === "Escape") {
        // stopPropagation, not just preventDefault: the event still bubbles
        // to the document-level listener below otherwise, and by the time it
        // arrives helpOpen() is already false, so Escape would additionally
        // fire "back" and navigate the page out from under the overlay.
        event.preventDefault();
        event.stopPropagation();
        closeHelp();
        return;
      }
      if (event.key !== "Tab" || !overlayDialog) {
        return;
      }
      event.stopPropagation();
      var focusable = focusableIn(overlayDialog);
      if (!focusable.length) {
        event.preventDefault();
        return;
      }
      var first = focusable[0];
      var last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    });
  }

  // -- dispatch ------------------------------------------------------------

  var handlers = {
    up: function () { moveSelection(-1); },
    down: function () { moveSelection(1); },
    top: function () { jumpTo("top"); },
    bottom: function () { jumpTo("bottom"); },
    open: openSelected,
    back: goBack,
    "new": function () { focusTarget("new"); },
    editTitle: function () { focusTarget("editTitle"); },
    comment: function () { focusTarget("comment"); },
    transition: function () { focusTarget("transition"); },
    filter: function () { focusTarget("filter"); },
    help: openHelp
  };

  document.addEventListener("keydown", function (event) {
    if (helpOpen()) {
      return;
    }
    if (tix.isTypingTarget(event.target)) {
      return;
    }
    var action = comboToAction[tix.comboOf(event)];
    var handler = action && handlers[action];
    if (!handler) {
      return;
    }
    event.preventDefault();
    handler();
  });
})();
