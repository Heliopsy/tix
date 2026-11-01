// SPDX-License-Identifier: AGPL-3.0-or-later
// Recently viewed: the last tasks this browser opened, in the sidebar.
//
// Kept in localStorage, per tenant and per signed-in actor, so two people
// sharing a browser do not see each other's list. Each entry is a task's ref,
// its title as it was when opened, and when. A task is recorded when its
// screen is drawn, as a page or in the task dialog: both carry
// [data-recent-task] on the title bar (templates/task.html).
//
// The list is drawn into the sidebar's "recent-tasks" block, which is
// rendered hidden and empty, so with scripting off nothing is offered. On the
// collapsed rail only its icon shows, and pressing it opens the sidebar.
//
// tix.recentTasks() is the public face, for the command palette and anything
// else that wants it: the entries newest first, each {ref, title, href, at},
// at most ten. A "tix:recent" event on the document says the list changed.
(function (global) {
  "use strict";
  var tix = (global.tix = global.tix || {});
  var MAX = 10;
  var PREFIX = "tix.recent.";
  var OPEN = "tix.recent.closed";

  // recentKey is the storage key one reader's list lives under.
  tix.recentKey = function (who) {
    return PREFIX + (who || "anonymous");
  };

  // recentParse reads a stored list, dropping anything malformed.
  tix.recentParse = function (raw) {
    var list;
    try {
      list = JSON.parse(raw || "[]");
    } catch (e) {
      return [];
    }
    if (!Array.isArray(list)) return [];
    var out = [];
    for (var i = 0; i < list.length && out.length < MAX; i++) {
      var e = list[i];
      if (e && typeof e.ref === "string" && e.ref && typeof e.title === "string") {
        out.push({ ref: e.ref, title: e.title, at: typeof e.at === "string" ? e.at : "" });
      }
    }
    return out;
  };

  // recentAdd puts a task first, once, keeping at most max entries.
  tix.recentAdd = function (list, entry, max) {
    var out = [entry];
    for (var i = 0; i < list.length; i++) {
      if (list[i].ref !== entry.ref) out.push(list[i]);
    }
    return out.slice(0, max || MAX);
  };

  var doc = global.document;

  function storage() {
    try {
      return global.localStorage;
    } catch (e) {
      return null;
    }
  }

  function owner() {
    var el = doc && doc.querySelector("[data-recent-for]");
    return el ? el.getAttribute("data-recent-for") : "";
  }

  function read() {
    var s = storage();
    try {
      return s ? tix.recentParse(s.getItem(tix.recentKey(owner()))) : [];
    } catch (e) {
      return [];
    }
  }

  function write(list) {
    var s = storage();
    try {
      if (s) s.setItem(tix.recentKey(owner()), JSON.stringify(list));
    } catch (e) {
      return;
    }
  }

  // recentTasks answers the reader's recently viewed tasks, newest first.
  tix.recentTasks = function () {
    return read().map(function (e) {
      return { ref: e.ref, title: e.title, href: "/tasks/" + encodeURIComponent(e.ref), at: e.at };
    });
  };

  if (!doc || !doc.addEventListener) return;

  function record(root) {
    var marks = (root || doc).querySelectorAll ? (root || doc).querySelectorAll("[data-recent-task]") : [];
    if (!marks.length || !owner()) return false;
    var list = read();
    var before = JSON.stringify(list);
    for (var i = 0; i < marks.length; i++) {
      var ref = marks[i].getAttribute("data-recent-task");
      if (!ref) continue;
      list = tix.recentAdd(list, { ref: ref, title: marks[i].getAttribute("data-recent-title") || ref, at: new Date().toISOString() });
    }
    if (JSON.stringify(list) === before) return false;
    write(list);
    return true;
  }

  function draw() {
    var box = doc.querySelector("[data-recent]");
    var ul = box && box.querySelector("[data-recent-list]");
    if (!ul) return;
    var list = tix.recentTasks();
    while (ul.firstChild) ul.removeChild(ul.firstChild);
    for (var i = 0; i < list.length; i++) {
      var li = doc.createElement("li");
      var a = doc.createElement("a");
      a.href = list[i].href;
      a.className = "recentitem";
      a.setAttribute("data-rail-name", list[i].ref + " " + list[i].title);
      if (global.location && global.location.pathname === list[i].href) a.setAttribute("aria-current", "page");
      var ref = doc.createElement("span");
      ref.className = "ref";
      ref.textContent = list[i].ref;
      var title = doc.createElement("span");
      title.className = "recenttitle";
      title.textContent = list[i].title;
      a.appendChild(ref);
      a.appendChild(title);
      li.appendChild(a);
      ul.appendChild(li);
    }
    box.hidden = list.length === 0;
    var s = storage();
    var closed = false;
    try {
      closed = !!s && s.getItem(OPEN) === "true";
    } catch (e) {
      closed = false;
    }
    box.open = !closed;
  }

  function refresh(root) {
    var changed = record(root);
    draw();
    if (changed && typeof global.CustomEvent === "function") {
      doc.dispatchEvent(new global.CustomEvent("tix:recent", { detail: tix.recentTasks() }));
    }
  }

  doc.addEventListener(
    "toggle",
    function (event) {
      var box = event.target;
      if (!box || !box.hasAttribute || !box.hasAttribute("data-recent") || box.hidden) return;
      var s = storage();
      try {
        if (!s) return;
        if (box.open) s.removeItem(OPEN);
        else s.setItem(OPEN, "true");
      } catch (e) {
        return;
      }
    },
    true
  );

  // On the rail the list has no room; its icon opens the sidebar instead.
  doc.addEventListener("click", function (event) {
    var summary = event.target && event.target.closest ? event.target.closest("[data-recent] > summary") : null;
    var root = doc.documentElement;
    if (!summary || !root.classList.contains("side-collapsed") || typeof tix.sidebarToggle !== "function") return;
    event.preventDefault();
    tix.sidebarToggle();
    summary.parentNode.open = true;
  });

  doc.addEventListener("htmx:afterSettle", function (event) {
    refresh(event.detail && event.detail.elt);
  });
  if (doc.readyState === "loading") {
    doc.addEventListener("DOMContentLoaded", function () {
      refresh();
    });
  } else {
    refresh();
  }
})(window);
