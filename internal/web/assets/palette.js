// SPDX-License-Identifier: AGPL-3.0-or-later
// The command palette: ctrl+k (command+k on a Mac) or the Search button opens
// a dialog that finds a task, a project, a screen or an action by name.
//
// Screens and actions come from tix-palette-data, which the server embeds in
// every page already translated (internal/web/palette.go), so they draw the
// moment the palette opens. Projects are read once from /palette/projects.
// Tasks are asked for only once something is typed, debounced, from
// /palette/tasks, which runs the filter language the filter bar runs.
//
// The first half of this file is pure and covered by internal/web/jstest;
// the second is the wiring, bound at the document so a boosted navigation
// that swaps the body leaves nothing captured behind.
(function (global) {
  "use strict";
  var tix = (global.tix = global.tix || {});
  function say(text) {
    return tix.t ? tix.t(text) : text;
  }

  // -- decisions --------------------------------------------------------------

  // fuzzyWord scores one query word against a text: a substring scores
  // highest, more so at the start or at a word start; failing that, the
  // letters in order with gaps score lower the more they are spread. -1 is no
  // match.
  // A ref or project key is kept lower case and drawn upper case, as INFRA-8.
  function shownRef(s) {
    return String(s || "").toUpperCase();
  }
  function fuzzyWord(word, text) {
    var at = text.indexOf(word);
    if (at === 0) {
      return 300 - text.length;
    }
    if (at > 0) {
      var boundary = /[\s\-_/:.]/.test(text.charAt(at - 1));
      return (boundary ? 250 : 200) - at - text.length / 10;
    }
    var score = 100;
    var last = -1;
    for (var i = 0; i < word.length; i++) {
      var next = text.indexOf(word.charAt(i), last + 1);
      if (next < 0) {
        return -1;
      }
      if (last >= 0) {
        score -= next - last - 1;
      }
      last = next;
    }
    return Math.max(1, score);
  }

  // paletteScore scores a whole query against a text, every word of the
  // query required. The empty query matches everything equally.
  tix.paletteScore = function (query, text) {
    var words = String(query || "").toLowerCase().split(/\s+/).filter(Boolean);
    var haystack = String(text || "").toLowerCase();
    var total = 0;
    for (var i = 0; i < words.length; i++) {
      var s = fuzzyWord(words[i], haystack);
      if (s < 0) {
        return -1;
      }
      total += s;
    }
    return total;
  };

  // paletteRank filters items by the query and orders them best first. An
  // item used recently is lifted: to the top, in recency order, when nothing
  // is typed, and by a bonus otherwise. Ties keep the order items came in.
  tix.paletteRank = function (items, query, recent) {
    var order = {};
    var keys = recent || [];
    for (var r = 0; r < keys.length; r++) {
      if (!Object.prototype.hasOwnProperty.call(order, keys[r])) {
        order[keys[r]] = r;
      }
    }
    var typed = String(query || "").trim() !== "";
    var scored = [];
    for (var i = 0; i < (items || []).length; i++) {
      var item = items[i];
      var text = [item.label, item.hint, item.terms].filter(Boolean).join(" ");
      var s = tix.paletteScore(query, text);
      if (s < 0) {
        continue;
      }
      var seen = Object.prototype.hasOwnProperty.call(order, item.key) ? order[item.key] : -1;
      if (seen >= 0) {
        s += typed ? 60 - Math.min(seen, 50) : 1000 - seen;
      }
      scored.push({ item: item, score: s, index: i });
    }
    scored.sort(function (a, b) {
      return b.score - a.score || a.index - b.index;
    });
    return scored.map(function (x) { return x.item; });
  };

  // paletteOrderGroups puts the group holding the best match first, so the
  // option Enter takes is the one the query names best: "board" is the
  // board view, not the Keyboard section of Settings. A task the server
  // matched on its description alone still counts as a match. Ties keep the
  // order the groups came in.
  tix.paletteOrderGroups = function (groups, query) {
    var scored = (groups || []).map(function (g, i) {
      var top = -1;
      for (var k = 0; k < g.items.length; k++) {
        var it = g.items[k];
        top = Math.max(top, tix.paletteScore(query, [it.label, it.hint, it.terms].filter(Boolean).join(" ")), 1);
      }
      return { group: g, top: top, index: i };
    });
    scored.sort(function (a, b) {
      return b.top - a.top || a.index - b.index;
    });
    return scored.map(function (x) { return x.group; });
  };

  // paletteRemember puts one item at the front of a recent list, once,
  // keeping at most max.
  tix.paletteRemember = function (list, item, max) {
    var out = [item];
    for (var i = 0; list && i < list.length && out.length < max; i++) {
      if (list[i] && list[i].key !== item.key) {
        out.push(list[i]);
      }
    }
    return out;
  };

  // paletteRecentTasks reads whatever tix.recentTasks() answered, when a page
  // has one, as palette items: a ref, or an object naming a ref and perhaps
  // a title and an address. Anything else is dropped rather than drawn.
  tix.paletteRecentTasks = function (raw) {
    var out = [];
    if (!raw || typeof raw.length !== "number") {
      return out;
    }
    for (var i = 0; i < raw.length; i++) {
      var entry = raw[i];
      var ref = typeof entry === "string" ? entry : entry && typeof entry.ref === "string" ? entry.ref : "";
      if (!ref) {
        continue;
      }
      var title = entry && typeof entry.title === "string" ? entry.title : "";
      var href = entry && typeof entry.href === "string" && entry.href.charAt(0) === "/" ? entry.href : "/tasks/" + encodeURIComponent(ref);
      out.push({ key: "task:" + ref, kind: "task", label: title || shownRef(ref), hint: title ? shownRef(ref) : "", href: href });
    }
    return out;
  };

  // paletteSearchFor is the task search a query asks for, or "" for none: a
  // palette with nothing typed sends nothing.
  tix.paletteSearchFor = function (query) {
    var q = String(query || "").trim();
    return q ? "/palette/tasks?q=" + encodeURIComponent(q) : "";
  };

  // -- wiring -------------------------------------------------------------------

  var doc = global.document;
  if (!doc || typeof doc.addEventListener !== "function") {
    return;
  }

  var RECENT_KEY = "tix.palette.recent";
  var PENDING_KEY = "tix.palette.pending";
  var RECENT_MAX = 8;
  var GROUP_MAX = 6;
  var DEBOUNCE_MS = 150;
  var PROJECTS_TTL = 60000;

  var returnFocus = null;
  var restoreOnClose = true;
  var active = 0;
  var activeKey = "";
  var shown = [];
  var projects = null;
  var projectsAt = 0;
  var tasks = null;
  var timer = null;
  var seq = 0;
  var inflight = null;

  function el(id) {
    return doc.getElementById(id);
  }

  function dialog() {
    return el("palette");
  }

  function isOpen() {
    var dlg = dialog();
    return !!(dlg && dlg.open);
  }

  function readData() {
    var node = el("tix-palette-data");
    try {
      var data = node ? JSON.parse(node.textContent) : null;
      return data && typeof data === "object" ? data : {};
    } catch (err) {
      return {};
    }
  }

  function store(kind) {
    try {
      return kind === "session" ? global.sessionStorage : global.localStorage;
    } catch (err) {
      return null;
    }
  }

  function loadRecent() {
    var s = store("local");
    try {
      var list = s ? JSON.parse(s.getItem(RECENT_KEY) || "[]") : [];
      return Array.isArray(list) ? list.filter(function (x) { return x && typeof x.key === "string"; }) : [];
    } catch (err) {
      return [];
    }
  }

  function remember(item) {
    var s = store("local");
    if (!s) {
      return;
    }
    var kept = { key: item.key, kind: item.kind, label: item.label, hint: item.hint || "" };
    if (item.href) {
      kept.href = item.href;
    }
    if (item.action) {
      kept.action = item.action;
    }
    if (item.project) {
      kept.project = item.project;
    }
    try {
      s.setItem(RECENT_KEY, JSON.stringify(tix.paletteRemember(loadRecent(), kept, RECENT_MAX)));
    } catch (err) {
      return;
    }
  }

  function recentTasks() {
    if (typeof tix.recentTasks !== "function") {
      return [];
    }
    try {
      return tix.paletteRecentTasks(tix.recentTasks());
    } catch (err) {
      return [];
    }
  }

  // staticItems are the screens and actions, and the projects once read.
  // "New task in" a project is offered only once something is typed: with
  // nothing typed it would be one row per project ahead of everything else.
  function staticItems(typed) {
    var data = readData();
    var screens = (data.screens || []).map(function (s) {
      return { key: s.id, kind: "screen", label: s.label, hint: s.hint || "", href: s.href };
    });
    var actions = (data.actions || []).map(function (a) {
      return { key: "action:" + a.id, kind: "action", action: a.id, label: a.label, hint: "" };
    });
    var projs = (projects || []).map(function (p) {
      return { key: "project:" + p.key, kind: "project", label: p.name, hint: shownRef(p.key), href: p.href };
    });
    if (typed) {
      (projects || []).forEach(function (p) {
        actions.push({ key: "action:new-task:" + p.key, kind: "action", action: "new-task", project: p.key,
          label: say("New task in %s").replace("%s", p.name), hint: shownRef(p.key) });
      });
    }
    return { screens: screens, actions: actions, projects: projs };
  }

  function groupsFor(query) {
    var q = query.trim();
    var recent = loadRecent();
    var recentKeys = recentTasks().map(function (x) { return x.key; })
      .concat(recent.map(function (x) { return x.key; }));
    var items = staticItems(q !== "");
    var groups = [];
    var used = {};
    function take(list, max) {
      var out = [];
      for (var i = 0; i < list.length && out.length < max; i++) {
        if (!used[list[i].key]) {
          used[list[i].key] = true;
          out.push(list[i]);
        }
      }
      return out;
    }
    if (!q) {
      groups.push({ name: say("Recent"), items: take(recentTasks().concat(recent), GROUP_MAX) });
      groups.push({ name: say("Actions"), items: take(items.actions, items.actions.length) });
      groups.push({ name: say("Screens"), items: take(items.screens, items.screens.length) });
      groups.push({ name: say("Projects"), items: take(items.projects, items.projects.length) });
      return groups;
    }
    var known = recentTasks().concat(recent.filter(function (x) { return x.kind === "task"; }));
    var found = tix.paletteRank(known, q, recentKeys);
    if (tasks && tasks.q === q) {
      found = found.concat(tasks.items.map(function (t) {
        return { key: "task:" + t.ref, kind: "task", label: t.title, hint: shownRef(t.ref), href: t.href, status: t.status };
      }));
    }
    groups.push({ name: say("Tasks"), items: take(found, 10) });
    groups.push({ name: say("Projects"), items: take(tix.paletteRank(items.projects, q, recentKeys), GROUP_MAX) });
    groups.push({ name: say("Screens"), items: take(tix.paletteRank(items.screens, q, recentKeys), GROUP_MAX) });
    groups.push({ name: say("Actions"), items: take(tix.paletteRank(items.actions, q, recentKeys), GROUP_MAX) });
    return tix.paletteOrderGroups(groups, q);
  }

  function statusText(query) {
    var q = query.trim();
    if (q && (!tasks || tasks.q !== q)) {
      return say("Searching tasks");
    }
    if (q && tasks && tasks.error) {
      return say("The task search did not answer.");
    }
    if (!shown.length) {
      return say("No matches");
    }
    return "";
  }

  function render() {
    var list = el("palette-list");
    var input = el("palette-input");
    if (!list || !input) {
      return;
    }
    var groups = groupsFor(input.value);
    list.textContent = "";
    shown = [];
    groups.forEach(function (g, gi) {
      if (!g.items.length) {
        return;
      }
      var group = doc.createElement("div");
      group.setAttribute("role", "group");
      group.className = "palette-group";
      var head = doc.createElement("div");
      head.className = "palette-group-name";
      head.id = "palette-group-" + gi;
      head.setAttribute("role", "presentation");
      head.textContent = g.name;
      group.setAttribute("aria-labelledby", head.id);
      group.appendChild(head);
      g.items.forEach(function (item) {
        var opt = doc.createElement("div");
        opt.setAttribute("role", "option");
        opt.className = "palette-item palette-" + item.kind;
        opt.id = "palette-opt-" + shown.length;
        opt.setAttribute("data-palette-index", String(shown.length));
        var label = doc.createElement("span");
        label.className = "palette-label";
        label.textContent = item.label;
        opt.appendChild(label);
        if (item.hint) {
          var hint = doc.createElement("span");
          hint.className = "palette-hint";
          hint.textContent = item.hint;
          opt.appendChild(hint);
        }
        group.appendChild(opt);
        shown.push(item);
      });
      list.appendChild(group);
    });
    var keep = -1;
    for (var i = 0; i < shown.length; i++) {
      if (shown[i].key === activeKey) {
        keep = i;
        break;
      }
    }
    setActive(keep >= 0 ? keep : 0, false);
    var status = el("palette-status");
    if (status) {
      status.textContent = statusText(input.value);
    }
  }

  function setActive(index, scroll) {
    var input = el("palette-input");
    var list = el("palette-list");
    if (!list || !input) {
      return;
    }
    active = shown.length ? (index + shown.length) % shown.length : 0;
    activeKey = shown.length ? shown[active].key : "";
    var opts = list.querySelectorAll("[role=option]");
    for (var i = 0; i < opts.length; i++) {
      var on = i === active;
      opts[i].setAttribute("aria-selected", on ? "true" : "false");
      opts[i].classList.toggle("is-active", on);
      if (on && scroll && typeof opts[i].scrollIntoView === "function") {
        opts[i].scrollIntoView({ block: "nearest" });
      }
    }
    if (shown.length) {
      input.setAttribute("aria-activedescendant", "palette-opt-" + active);
    } else {
      input.removeAttribute("aria-activedescendant");
    }
  }

  function loadProjects() {
    if (projects && Date.now() - projectsAt < PROJECTS_TTL) {
      return;
    }
    projectsAt = Date.now();
    fetch("/palette/projects", { credentials: "same-origin", headers: { Accept: "application/json" } })
      .then(function (resp) { return resp.ok ? resp.json() : null; })
      .then(function (list) {
        if (Array.isArray(list)) {
          projects = list;
          if (isOpen()) {
            render();
          }
        }
      })
      .catch(function () { projectsAt = 0; });
  }

  function search() {
    var input = el("palette-input");
    var q = input ? input.value.trim() : "";
    var url = tix.paletteSearchFor(q);
    if (inflight) {
      inflight.abort();
      inflight = null;
    }
    if (!url) {
      return;
    }
    var mine = ++seq;
    var ctrl = typeof AbortController === "function" ? new AbortController() : null;
    inflight = ctrl;
    fetch(url, { credentials: "same-origin", headers: { Accept: "application/json" }, signal: ctrl ? ctrl.signal : undefined })
      .then(function (resp) { return resp.ok ? resp.json() : Promise.reject(new Error("refused")); })
      .then(function (list) {
        if (mine !== seq) {
          return;
        }
        tasks = { q: q, items: Array.isArray(list) ? list : [], error: !Array.isArray(list) };
        inflight = null;
        render();
      })
      .catch(function (err) {
        if (mine !== seq || (err && err.name === "AbortError")) {
          return;
        }
        tasks = { q: q, items: [], error: true };
        inflight = null;
        render();
      });
  }

  function typed() {
    activeKey = "";
    render();
    if (timer !== null) {
      clearTimeout(timer);
    }
    timer = setTimeout(function () {
      timer = null;
      search();
    }, DEBOUNCE_MS);
  }

  function open() {
    var dlg = dialog();
    var input = el("palette-input");
    if (!dlg || !input || dlg.open || typeof dlg.showModal !== "function") {
      return false;
    }
    returnFocus = doc.activeElement;
    restoreOnClose = true;
    input.value = "";
    tasks = null;
    activeKey = "";
    render();
    dlg.showModal();
    input.focus();
    loadProjects();
    return true;
  }

  function close(restore) {
    var dlg = dialog();
    restoreOnClose = restore !== false;
    if (dlg && dlg.open) {
      dlg.close();
    }
  }

  tix.openPalette = open;

  // -- what choosing does -------------------------------------------------------

  function csrf() {
    var f = doc.querySelector('input[name="csrf_token"]');
    return f ? f.value : "";
  }

  function postTo(action, fields) {
    var form = doc.createElement("form");
    form.method = "post";
    form.action = action;
    form.hidden = true;
    var all = { csrf_token: csrf() };
    for (var k in fields) {
      if (Object.prototype.hasOwnProperty.call(fields, k)) {
        all[k] = fields[k];
      }
    }
    for (var name in all) {
      if (Object.prototype.hasOwnProperty.call(all, name)) {
        var input = doc.createElement("input");
        input.type = "hidden";
        input.name = name;
        input.value = all[name];
        form.appendChild(input);
      }
    }
    doc.body.appendChild(form);
    form.submit();
  }

  function submitForm(form) {
    if (!form) {
      return;
    }
    if (typeof form.requestSubmit === "function") {
      form.requestSubmit();
    } else {
      form.submit();
    }
  }

  // reveal focuses a control, opening the menu it sits in first when that
  // menu is a closed popover (the filter on a phone).
  function reveal(target) {
    var pop = target.closest ? target.closest("[popover]") : null;
    if (pop && typeof pop.showPopover === "function" && global.getComputedStyle &&
        global.getComputedStyle(pop).display === "none") {
      try {
        pop.showPopover();
      } catch (err) {
        return;
      }
    }
    target.focus();
  }

  function later(action, project, path) {
    var s = store("session");
    if (s) {
      try {
        s.setItem(PENDING_KEY, JSON.stringify({ action: action, project: project || "" }));
      } catch (err) {
        return;
      }
    }
    global.location.assign(path);
  }

  // act runs one action here when this page has the control it drives, and
  // returns whether it could.
  function act(action, project) {
    var target;
    if (action === "new-task") {
      target = doc.querySelector('[data-shortcut-target="new"]');
      if (!target) {
        return false;
      }
      var select = target.form && target.form.querySelector('select[name="project_ref"]');
      if (select && project) {
        for (var i = 0; i < select.options.length; i++) {
          if (select.options[i].value === project) {
            select.value = project;
          }
        }
      }
      reveal(target);
      return true;
    }
    if (action === "filter") {
      target = doc.querySelector('[data-shortcut-target="filter"]');
      if (!target) {
        return false;
      }
      reveal(target);
      return true;
    }
    if (action === "new-project") {
      target = doc.getElementById("newproject");
      if (!target || typeof target.showPopover !== "function") {
        return false;
      }
      try {
        target.showPopover();
      } catch (err) {
        return false;
      }
      var first = target.querySelector("input:not([type=hidden])");
      if (first) {
        first.focus();
      }
      return true;
    }
    return false;
  }

  function run(item) {
    var action = item.action;
    if (action === "new-task" || action === "filter" || action === "new-project") {
      close(false);
      if (!act(action, item.project)) {
        later(action, item.project, action === "new-project" ? "/projects" : "/tasks");
      }
      return;
    }
    if (action === "view-list" || action === "view-board" || action === "view-lanes") {
      var view = action.slice(5);
      close(false);
      var segment = doc.querySelector('form.viewswitch button[name="view"][value="' + view + '"]');
      if (segment && global.location.pathname === "/tasks") {
        segment.click();
        return;
      }
      postTo("/taskview", { view: view, next: "/tasks" });
      return;
    }
    if (action === "theme") {
      close(false);
      submitForm(doc.querySelector("form.themer"));
      return;
    }
    if (action === "signout") {
      close(false);
      submitForm(doc.querySelector('form[action="/logout"]'));
      return;
    }
    if (action === "shortcuts") {
      close(true);
      if (typeof tix.openShortcutsHelp === "function") {
        tix.openShortcutsHelp();
      }
    }
  }

  function choose(item) {
    if (!item) {
      return;
    }
    remember(item);
    if (item.kind === "action") {
      run(item);
      return;
    }
    var from = returnFocus;
    close(false);
    if (item.kind === "task" && typeof tix.openTaskHref === "function" && tix.openTaskHref(item.href, from)) {
      return;
    }
    global.location.assign(item.href);
  }

  function runPending() {
    var s = store("session");
    if (!s) {
      return;
    }
    var raw = null;
    try {
      raw = s.getItem(PENDING_KEY);
      s.removeItem(PENDING_KEY);
    } catch (err) {
      return;
    }
    if (!raw) {
      return;
    }
    try {
      var pending = JSON.parse(raw);
      act(pending.action, pending.project);
    } catch (err) {
      return;
    }
  }

  function revealOpeners() {
    var mac = /Mac|iPhone|iPad/.test((global.navigator && (global.navigator.platform || global.navigator.userAgent)) || "");
    var openers = doc.querySelectorAll("[data-palette-open][hidden]");
    for (var i = 0; i < openers.length; i++) {
      openers[i].removeAttribute("hidden");
    }
    if (mac) {
      var kbds = doc.querySelectorAll("[data-palette-kbd]");
      for (var k = 0; k < kbds.length; k++) {
        kbds[k].textContent = "⌘K";
      }
    }
  }

  function chord(event) {
    return (event.ctrlKey || event.metaKey) && !event.altKey && !event.shiftKey &&
      typeof event.key === "string" && event.key.toLowerCase() === "k";
  }

  function focusable() {
    var dlg = dialog();
    if (!dlg) {
      return [];
    }
    return Array.prototype.filter.call(dlg.querySelectorAll("input, button, [href]"), function (x) {
      return !x.disabled && !x.hidden;
    });
  }

  doc.addEventListener("keydown", function (event) {
    if (chord(event)) {
      if (event.defaultPrevented) {
        return;
      }
      if (isOpen()) {
        event.preventDefault();
        close(true);
        return;
      }
      if (open()) {
        event.preventDefault();
      }
      return;
    }
    if (!isOpen()) {
      return;
    }
    var key = event.key;
    if (key === "ArrowDown" || key === "ArrowUp") {
      event.preventDefault();
      setActive(active + (key === "ArrowDown" ? 1 : -1), true);
    } else if (key === "Enter" && event.target === el("palette-input")) {
      event.preventDefault();
      choose(shown[active]);
    } else if (key === "Tab") {
      var list = focusable();
      if (!list.length) {
        return;
      }
      var at = list.indexOf(doc.activeElement);
      var next = event.shiftKey ? (at <= 0 ? list.length - 1 : at - 1) : (at === list.length - 1 ? 0 : at + 1);
      event.preventDefault();
      list[next].focus();
    }
  });

  doc.addEventListener("input", function (event) {
    if (event.target && event.target.id === "palette-input") {
      typed();
    }
  });

  doc.addEventListener("mousemove", function (event) {
    var opt = event.target && event.target.closest ? event.target.closest("#palette-list [role=option]") : null;
    if (opt) {
      var index = parseInt(opt.getAttribute("data-palette-index"), 10);
      if (index !== active) {
        setActive(index, false);
      }
    }
  });

  doc.addEventListener("click", function (event) {
    var target = event.target;
    if (!target || typeof target.closest !== "function") {
      return;
    }
    if (target.closest("[data-palette-open]")) {
      event.preventDefault();
      open();
      return;
    }
    var dlg = dialog();
    if (!dlg || !dlg.open) {
      return;
    }
    if (target === dlg || target.closest("[data-palette-close]")) {
      close(true);
      return;
    }
    var opt = target.closest("#palette-list [role=option]");
    if (opt) {
      choose(shown[parseInt(opt.getAttribute("data-palette-index"), 10)]);
    }
  });

  doc.addEventListener("close", function (event) {
    if (event.target !== dialog()) {
      return;
    }
    if (timer !== null) {
      clearTimeout(timer);
      timer = null;
    }
    if (inflight) {
      inflight.abort();
      inflight = null;
    }
    seq++;
    var target = returnFocus;
    returnFocus = null;
    if (restoreOnClose && target && doc.contains(target) && typeof target.focus === "function") {
      target.focus();
    }
    restoreOnClose = true;
  }, true);

  revealOpeners();
  runPending();
  doc.addEventListener("htmx:load", revealOpeners);
})(typeof globalThis !== "undefined" ? globalThis : this);
