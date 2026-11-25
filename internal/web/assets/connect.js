// SPDX-License-Identifier: AGPL-3.0-or-later

// The Connect an agent screen (templates/connect.html): writes a token issued
// on the screen into every command. The screen's tabs are the task screen's,
// in assets/tasktabs.js.
//
// The token is read from the one-time secret block the server drew and
// copied into the [data-token-slot] spans, in hidden panels too. It is not
// stored, not sent anywhere and not put in the URL, so a reload shows
// $TIX_TOKEN again. The copy buttons copy textContent, so they copy the
// filled command.
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

  if (typeof document === "undefined") {
    return;
  }

  function init() {
    var source = document.querySelector("[data-token-value]");
    if (source && tix.fillTokenSlots(document.querySelectorAll("[data-token-slot]"), source.textContent.trim())) {
      var block = source.closest("section");
      if (block && typeof block.scrollIntoView === "function") {
        block.scrollIntoView({ block: "center" });
      }
    }
  }

  document.addEventListener("htmx:load", init);
  if (document.readyState !== "loading") {
    init();
  } else {
    document.addEventListener("DOMContentLoaded", init);
  }
})();
