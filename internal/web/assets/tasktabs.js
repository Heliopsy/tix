// SPDX-License-Identifier: AGPL-3.0-or-later

// The activity tabs on a task screen (templates/task.html): Comments, History
// and Artifacts under the description, one panel shown at a time.
//
// The tab drawn first is, in order: the panel the address fragment points
// into (#history, #comments, #artifacts, or a row inside one such as
// #comment-<id>), History when the server forced it for an older page or a
// raw entry the address names, the tab this browser last chose, and the
// server's default. Only a reader's own choice is remembered, in
// localStorage, and a browser that refuses storage still gets working tabs.
//
// Nothing a live update replaces holds the selection: the strip and the
// panels sit outside every region, so a refresh redraws the lists and the
// counts and leaves the chosen tab as it was. Without scripting the strip is
// not drawn and every panel shows.
//
// The Connect an agent screen (templates/connect.html) uses the same boxes
// with two differences. Its tabs are links to ?PARAM=NAME, which the server
// draws without scripting, so a click here prevents the navigation and
// switches in place. Its boxes carry data-tab-param, so the choice goes into
// the address with history.replaceState instead of into localStorage, every
// other tab link is rewritten to keep it, and a form marked data-tab-keep
// posts back to it. Boxes nest: a box owns only the tabs and panels whose
// nearest box it is.
(function (global) {
  "use strict";
  var tix = (global.tix = global.tix || {});
  var KEY = "tix.taskTab";

  // taskTabPick chooses the tab to show from what the page knows, each an
  // empty string when it says nothing; names are the tabs that exist.
  tix.taskTabPick = function (names, fromHash, forced, stored, initial) {
    var order = [fromHash, forced, stored, initial];
    for (var i = 0; i < order.length; i++) {
      if (order[i] && names.indexOf(order[i]) >= 0) {
        return order[i];
      }
    }
    return names[0] || "";
  };

  // taskTabStep is the index the arrow keys, Home and End move to from
  // current, or -1 for any other key.
  tix.taskTabStep = function (key, current, count) {
    switch (key) {
      case "ArrowRight":
        return (current + 1) % count;
      case "ArrowLeft":
        return (current - 1 + count) % count;
      case "Home":
        return 0;
      case "End":
        return count - 1;
      default:
        return -1;
    }
  };

  // tabSearch is the query string search becomes with param set to name,
  // every other parameter kept in its place.
  tix.tabSearch = function (search, param, name) {
    var params = new global.URLSearchParams(search || "");
    params.set(param, name);
    return "?" + params.toString();
  };

  var doc = global.document;
  if (!doc) {
    return;
  }

  function storage() {
    try {
      return global.localStorage || null;
    } catch (e) {
      return null;
    }
  }

  function stored() {
    var s = storage();
    try {
      return (s && s.getItem(KEY)) || "";
    } catch (e) {
      return "";
    }
  }

  function remember(name) {
    var s = storage();
    try {
      if (s) {
        s.setItem(KEY, name);
      }
    } catch (e) {
      // A full or refused store only means the choice is not kept.
    }
  }

  function ownedBy(box) {
    return function (el) {
      return el.closest("[data-task-tabs]") === box;
    };
  }

  function tabs(box) {
    return Array.prototype.slice.call(box.querySelectorAll('[role="tab"][data-tab-for]')).filter(ownedBy(box));
  }

  // keepInAddress writes a data-tab-param box's choice into the address and
  // points every tab link and data-tab-keep form at the address it makes.
  function keepInAddress(box, name) {
    var loc = global.location;
    var history = global.history;
    if (!loc || !history || typeof history.replaceState !== "function") {
      return;
    }
    var search = tix.tabSearch(loc.search, box.getAttribute("data-tab-param"), name);
    if (search !== loc.search) {
      history.replaceState(history.state, "", loc.pathname + search + loc.hash);
    }
    var links = doc.querySelectorAll('[data-task-tabs][data-tab-param] a[role="tab"][data-tab-for]');
    for (var i = 0; i < links.length; i++) {
      var owner = links[i].closest("[data-task-tabs]");
      links[i].setAttribute("href", loc.pathname + tix.tabSearch(search, owner.getAttribute("data-tab-param"), links[i].getAttribute("data-tab-for")));
    }
    var forms = doc.querySelectorAll("form[data-tab-keep]");
    for (var j = 0; j < forms.length; j++) {
      forms[j].setAttribute("action", (forms[j].getAttribute("action") || "").split("?")[0] + search);
    }
  }

  // choose shows name in box and keeps the choice where the box keeps it.
  function choose(box, name, focus) {
    select(box, name, focus);
    if (box.hasAttribute("data-tab-param")) {
      keepInAddress(box, name);
    } else {
      remember(name);
    }
  }

  function names(box) {
    return tabs(box).map(function (tab) {
      return tab.getAttribute("data-tab-for");
    });
  }

  // select shows the panel called name, marks its tab, and moves focus to
  // the tab when asked to.
  function select(box, name, focus) {
    box.setAttribute("data-tab", name);
    var list = tabs(box);
    for (var i = 0; i < list.length; i++) {
      var on = list[i].getAttribute("data-tab-for") === name;
      list[i].setAttribute("aria-selected", on ? "true" : "false");
      list[i].tabIndex = on ? 0 : -1;
      if (on && focus) {
        list[i].focus();
      }
    }
    var panels = box.querySelectorAll("[data-tab-panel]");
    for (var j = 0; j < panels.length; j++) {
      if (panels[j].parentNode === box) {
        panels[j].hidden = panels[j].getAttribute("data-tab-panel") !== name;
      }
    }
  }

  // panelOf names the panel of box an element sits in, or "".
  function panelOf(box, el) {
    var panel = el && el.closest ? el.closest("[data-tab-panel]") : null;
    while (panel && panel.parentNode !== box) {
      panel = panel.parentNode && panel.parentNode.closest ? panel.parentNode.closest("[data-tab-panel]") : null;
    }
    return panel ? panel.getAttribute("data-tab-panel") : "";
  }

  function hashTarget() {
    var id = global.location ? global.location.hash.slice(1) : "";
    if (!id) {
      return null;
    }
    try {
      id = decodeURIComponent(id);
    } catch (e) {
      // Not percent-encoded after all; look it up as written.
    }
    return doc.getElementById(id);
  }

  function setUp(box) {
    var fromHash = panelOf(box, hashTarget());
    var forced = box.hasAttribute("data-tab-forced") ? box.getAttribute("data-tab") : "";
    var drawn = box.getAttribute("data-tab") || "";
    var keep = box.hasAttribute("data-tab-param") ? "" : stored();
    var name = tix.taskTabPick(names(box), fromHash, forced, keep, drawn);
    select(box, name, false);
    if (name !== drawn && box.hasAttribute("data-tab-param")) {
      keepInAddress(box, name);
    }
  }

  function init(root) {
    var scope = root && root.querySelectorAll ? root : doc;
    var boxes = Array.prototype.slice.call(scope.querySelectorAll("[data-task-tabs]"));
    if (scope.matches && scope.matches("[data-task-tabs]")) {
      boxes.push(scope);
    }
    for (var i = 0; i < boxes.length; i++) {
      setUp(boxes[i]);
    }
  }

  // revealTab shows the panel an element sits in, for a link that names a
  // record inside one (assets/copy.js revealTarget).
  tix.revealTab = function (el) {
    var box = el && el.closest ? el.closest("[data-task-tabs]") : null;
    var name = box ? panelOf(box, el) : "";
    if (name && box.getAttribute("data-tab") !== name) {
      select(box, name, false);
    }
  };

  doc.addEventListener("click", function (event) {
    var tab = event.target && event.target.closest && event.target.closest("[data-task-tabs] [role=\"tab\"][data-tab-for]");
    if (!tab) {
      return;
    }
    if (tab.tagName === "A") {
      if (event.button || event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) {
        return;
      }
      event.preventDefault();
    }
    choose(tab.closest("[data-task-tabs]"), tab.getAttribute("data-tab-for"), false);
  });

  doc.addEventListener("keydown", function (event) {
    var tab = event.target && event.target.closest && event.target.closest("[data-task-tabs] [role=\"tab\"][data-tab-for]");
    if (!tab || event.altKey || event.ctrlKey || event.metaKey) {
      return;
    }
    var box = tab.closest("[data-task-tabs]");
    var list = tabs(box);
    var next = tix.taskTabStep(event.key, list.indexOf(tab), list.length);
    if (next < 0) {
      return;
    }
    event.preventDefault();
    choose(box, list[next].getAttribute("data-tab-for"), true);
  });

  global.addEventListener("hashchange", function () {
    var el = hashTarget();
    if (el) {
      tix.revealTab(el);
    }
  });

  doc.addEventListener("htmx:load", function (event) {
    init(event.target);
  });
  if (doc.readyState !== "loading") {
    init(doc);
  } else {
    doc.addEventListener("DOMContentLoaded", function () {
      init(doc);
    });
  }
})(window);
