// SPDX-License-Identifier: AGPL-3.0-or-later

// The subtask tree on a task screen (templates/task.html "subtask-tree").
//
// Every node with subtasks carries a fold button the server draws hidden, so
// without scripting the tree is open and the button is not offered. This shows
// the buttons and makes them fold: pressing one hides or shows the list it
// controls. A node whose subtasks were never drawn has no list yet, only the
// Show subtasks link that htmx swaps for one; pressing its fold button presses
// that link, and the list that arrives is shown open.
(function (global) {
  "use strict";
  var tix = (global.tix = global.tix || {});

  // treeFold is what pressing a fold button does, from whether it reads
  // expanded and whether its list is on the page: "collapse", "expand", or
  // "load" for a list still to fetch.
  tix.treeFold = function (expanded, hasList) {
    if (!hasList) return "load";
    return expanded ? "collapse" : "expand";
  };

  var doc = global.document;
  if (!doc || !doc.addEventListener) return;

  function reveal(root) {
    var scope = root && root.querySelectorAll ? root : doc;
    var buttons = scope.querySelectorAll("button.treetoggle[hidden]");
    for (var i = 0; i < buttons.length; i++) buttons[i].hidden = false;
  }

  doc.addEventListener("click", function (event) {
    var btn = event.target && event.target.closest && event.target.closest("button.treetoggle");
    if (!btn) return;
    var id = btn.getAttribute("aria-controls");
    var list = id && doc.getElementById(id);
    switch (tix.treeFold(btn.getAttribute("aria-expanded") === "true", !!list)) {
      case "collapse":
        list.hidden = true;
        btn.setAttribute("aria-expanded", "false");
        break;
      case "expand":
        list.hidden = false;
        btn.setAttribute("aria-expanded", "true");
        break;
      default: {
        var more = doc.querySelector('[data-tree-load="' + id + '"]');
        if (more) {
          btn.setAttribute("aria-expanded", "true");
          more.click();
        }
      }
    }
  });

  // A loaded level arrives with fold buttons of its own, and the node that
  // asked for it now reads expanded.
  doc.addEventListener("htmx:load", function (event) {
    var el = event.target;
    reveal(el);
    if (el && el.id && el.classList && el.classList.contains("subtree")) {
      var owner = doc.querySelector('button.treetoggle[aria-controls="' + el.id + '"]');
      if (owner) owner.setAttribute("aria-expanded", "true");
    }
  });

  if (doc.readyState === "loading") {
    doc.addEventListener("DOMContentLoaded", function () {
      reveal(doc);
    });
  } else {
    reveal(doc);
  }
})(window);
