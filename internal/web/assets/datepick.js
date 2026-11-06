// SPDX-License-Identifier: AGPL-3.0-or-later

// The calendar button beside a typed due date field (the date-pick partial).
// The field takes words and dates typed in the reader's format, which a
// native date control cannot, so the native control is kept hidden beside
// the field and only opened as a picker. The day it gives back, always
// YYYY-MM-DD, is written into the field in the pattern the server drew on
// the field (data-date-pattern), which is the order the server reads it in.
(function () {
  "use strict";
  var tix = window.tix;
  if (!tix) {
    return;
  }

  // writeDate fills a pattern such as dd.mm.yyyy with the day iso names.
  tix.writeDate = function (pattern, iso) {
    var parts = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso || "");
    if (!parts) {
      return "";
    }
    return (pattern || "yyyy-mm-dd").replace("yyyy", parts[1]).replace("mm", parts[2]).replace("dd", parts[3]);
  };

  function wire(root) {
    if (!root || !root.querySelectorAll) {
      return;
    }
    root.querySelectorAll("[data-datepick]").forEach(function (pick) {
      if (pick.dataset.datepickWired) {
        return;
      }
      var field = document.getElementById(pick.getAttribute("data-datepick"));
      var button = pick.querySelector(".datepick-open");
      var native = pick.querySelector(".datepick-native");
      if (!field || !button || !native) {
        return;
      }
      pick.dataset.datepickWired = "1";
      pick.hidden = false;
      button.addEventListener("click", function () {
        try {
          native.showPicker();
        } catch (e) {
          native.focus();
        }
      });
      native.addEventListener("change", function () {
        var value = tix.writeDate(field.getAttribute("data-date-pattern"), native.value);
        if (!value) {
          return;
        }
        field.value = value;
        field.dispatchEvent(new Event("input", { bubbles: true }));
        field.focus();
      });
    });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", function () { wire(document); });
  } else {
    wire(document);
  }
  document.addEventListener("htmx:afterSwap", function (event) {
    wire((event && event.target) || document);
  });
})();
