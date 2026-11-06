// SPDX-License-Identifier: AGPL-3.0-or-later

// The Connect an agent screen (templates/connect.html): writes a token issued
// on the screen into every command, and turns each tab set into tabs.
//
// The token is read from the one-time secret block the server drew and
// copied into the [data-token-slot] spans. It is not stored, not sent
// anywhere and not put in the URL, so a reload shows $TIX_TOKEN again. The
// copy buttons copy textContent, so they copy the filled command.
(function () {
  "use strict";
  var tix = (window.tix = window.tix || {});

  // fillTokenSlots writes value into every slot and reports how many it filled.
  tix.fillTokenSlots = function (slots, value) {
    if (!value) {
      return 0;
    }
    for (var i = 0; i < slots.length; i++) {
      slots[i].textContent = value;
    }
    return slots.length;
  };

  // tabMove is the index the arrow keys, Home and End move to from current.
  tix.tabMove = function (key, current, count) {
    switch (key) {
      case "ArrowRight":
      case "ArrowDown":
        return (current + 1) % count;
      case "ArrowLeft":
      case "ArrowUp":
        return (current - 1 + count) % count;
      case "Home":
        return 0;
      case "End":
        return count - 1;
      default:
        return -1;
    }
  };

  if (typeof document === "undefined") {
    return;
  }

  function select(set, tab, focus) {
    var tabs = set.querySelectorAll('[role="tab"]');
    for (var i = 0; i < tabs.length; i++) {
      var on = tabs[i] === tab;
      tabs[i].setAttribute("aria-selected", on ? "true" : "false");
      tabs[i].tabIndex = on ? 0 : -1;
      var panel = document.getElementById(tabs[i].getAttribute("aria-controls"));
      if (panel) {
        panel.hidden = !on;
      }
    }
    if (focus) {
      tab.focus();
    }
  }

  function init() {
    var sets = document.querySelectorAll("[data-tabset]");
    for (var i = 0; i < sets.length; i++) {
      var current = sets[i].querySelector('[role="tab"][aria-selected="true"]');
      if (current) {
        select(sets[i], current, false);
      }
      sets[i].classList.add("is-ready");
    }
    var source = document.querySelector("[data-token-value]");
    if (source && tix.fillTokenSlots(document.querySelectorAll("[data-token-slot]"), source.textContent.trim())) {
      var block = source.closest("section");
      if (block && typeof block.scrollIntoView === "function") {
        block.scrollIntoView({ block: "center" });
      }
    }
  }

  document.addEventListener("click", function (event) {
    var tab = event.target && event.target.closest && event.target.closest('[data-tabset] [role="tab"]');
    if (tab) {
      select(tab.closest("[data-tabset]"), tab, false);
    }
  });

  document.addEventListener("keydown", function (event) {
    var tab = event.target && event.target.closest && event.target.closest('[data-tabset] [role="tab"]');
    if (!tab) {
      return;
    }
    var set = tab.closest("[data-tabset]");
    var tabs = Array.prototype.slice.call(set.querySelectorAll('[role="tab"]'));
    var next = tix.tabMove(event.key, tabs.indexOf(tab), tabs.length);
    if (next < 0) {
      return;
    }
    event.preventDefault();
    select(set, tabs[next], true);
  });

  document.addEventListener("htmx:load", init);
  if (document.readyState !== "loading") {
    init();
  } else {
    document.addEventListener("DOMContentLoaded", init);
  }
})();
