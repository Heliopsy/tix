// SPDX-License-Identifier: AGPL-3.0-or-later

// assets/palette.js: the ranking it does without a server, and the wiring
// around it. The DOM is a stub, so this covers what is asked for, when, and
// what choosing does; nothing about layout or real focus.
import test from "node:test";
import assert from "node:assert/strict";
import { newContext, run, fakeTimers } from "./harness.mjs";

const plain = (v) => JSON.parse(JSON.stringify(v));

function bare() {
  const ctx = newContext();
  run(ctx, "palette.js");
  return ctx.tix;
}

test("a prefix outranks a word start, which outranks a substring, which outranks scattered letters", () => {
  const tix = bare();
  const prefix = tix.paletteScore("set", "Settings");
  const word = tix.paletteScore("set", "My settings");
  const inside = tix.paletteScore("set", "Asset");
  const scattered = tix.paletteScore("stg", "Settings");
  assert.ok(prefix > word, `${prefix} <= ${word}`);
  assert.ok(word > inside, `${word} <= ${inside}`);
  assert.ok(inside > scattered, `${inside} <= ${scattered}`);
  assert.ok(scattered > 0);
});

test("every word of the query must match, and a miss is -1", () => {
  const tix = bare();
  assert.equal(tix.paletteScore("zzz", "Settings"), -1);
  assert.equal(tix.paletteScore("set zzz", "Settings"), -1);
  assert.ok(tix.paletteScore("show board", "Show tasks as a board") > 0);
  assert.equal(tix.paletteScore("", "anything"), 0);
});

test("ranking filters, orders best first and keeps ties in their order", () => {
  const tix = bare();
  const items = [
    { key: "a", label: "Activity" },
    { key: "s", label: "Statistics" },
    { key: "t", label: "Tasks" },
    { key: "st", label: "Settings", hint: "Keyboard" },
  ];
  assert.deepEqual(plain(tix.paletteRank(items, "", []).map((x) => x.key)), ["a", "s", "t", "st"]);
  assert.deepEqual(plain(tix.paletteRank(items, "ta", []).map((x) => x.key))[0], "t");
  assert.deepEqual(plain(tix.paletteRank(items, "keyb", []).map((x) => x.key)), ["st"]);
});

test("what was used recently comes first with nothing typed, and is lifted when typing", () => {
  const tix = bare();
  const items = [
    { key: "a", label: "Statistics" },
    { key: "b", label: "Status" },
    { key: "c", label: "Settings" },
  ];
  assert.deepEqual(plain(tix.paletteRank(items, "", ["c", "b"]).map((x) => x.key)), ["c", "b", "a"]);
  const typed = plain(tix.paletteRank(items, "stat", ["b"]).map((x) => x.key));
  assert.equal(typed[0], "b");
});

test("the group holding the best match comes first", () => {
  const tix = bare();
  const groups = [
    { name: "Tasks", items: [{ label: "Live updates", hint: "web-1" }] },
    { name: "Screens", items: [{ label: "Keyboard", hint: "Settings" }] },
    { name: "Actions", items: [{ label: "Show tasks as a board" }] },
    { name: "Projects", items: [] },
  ];
  const order = plain(tix.paletteOrderGroups(groups, "board").map((g) => g.name));
  assert.deepEqual(order, ["Actions", "Screens", "Tasks", "Projects"]);
});

test("remembering puts an item first once and keeps at most the cap", () => {
  const tix = bare();
  const list = [{ key: "a" }, { key: "b" }, { key: "c" }];
  assert.deepEqual(plain(tix.paletteRemember(list, { key: "b" }, 3)), [{ key: "b" }, { key: "a" }, { key: "c" }]);
  assert.deepEqual(plain(tix.paletteRemember(list, { key: "d" }, 2)), [{ key: "d" }, { key: "a" }]);
});

test("recent tasks are read from refs or objects and anything else is dropped", () => {
  const tix = bare();
  const got = plain(tix.paletteRecentTasks([
    "infra-1",
    { ref: "web-2", title: "Fix login" },
    { ref: "ops-3", href: "https://elsewhere.example/x" },
    null,
    42,
    { title: "no ref" },
  ]));
  assert.deepEqual(got, [
    { key: "task:infra-1", kind: "task", label: "infra-1", hint: "", href: "/tasks/infra-1" },
    { key: "task:web-2", kind: "task", label: "Fix login", hint: "web-2", href: "/tasks/web-2" },
    { key: "task:ops-3", kind: "task", label: "ops-3", hint: "", href: "/tasks/ops-3" },
  ]);
  assert.deepEqual(plain(tix.paletteRecentTasks(undefined)), []);
});

test("nothing typed asks for no task search", () => {
  const tix = bare();
  assert.equal(tix.paletteSearchFor(""), "");
  assert.equal(tix.paletteSearchFor("   "), "");
  assert.equal(tix.paletteSearchFor(" deploy api "), "/palette/tasks?q=deploy%20api");
});

// -- wiring -------------------------------------------------------------------

// node is just enough of an element for the palette to build its list in.
function node(tag, attrs = {}) {
  const own = { ...attrs };
  const classes = new Set();
  const n = {
    tagName: tag.toUpperCase(),
    children: [],
    parentNode: null,
    id: own.id || "",
    value: "",
    focused: 0,
    get textContent() {
      return this._text || this.children.map((c) => c.textContent).join("");
    },
    set textContent(v) {
      this._text = v;
      this.children = [];
    },
    appendChild(c) {
      c.parentNode = this;
      this.children.push(c);
      return c;
    },
    setAttribute(k, v) {
      own[k] = String(v);
      if (k === "id") this.id = String(v);
    },
    getAttribute(k) {
      return Object.prototype.hasOwnProperty.call(own, k) ? own[k] : null;
    },
    removeAttribute(k) {
      delete own[k];
    },
    hasAttribute(k) {
      return Object.prototype.hasOwnProperty.call(own, k);
    },
    classList: {
      toggle: (c, on) => (on ? classes.add(c) : classes.delete(c)),
      contains: (c) => classes.has(c),
    },
    descendants() {
      return this.children.flatMap((c) => [c, ...c.descendants()]);
    },
    querySelectorAll(sel) {
      if (sel === "[role=option]") {
        return this.descendants().filter((c) => c.getAttribute("role") === "option");
      }
      if (sel === "input, button, [href]") {
        return this.descendants().filter((c) => c.tagName === "INPUT" || c.tagName === "BUTTON");
      }
      return [];
    },
    querySelector() {
      return null;
    },
    focus() {
      this.focused++;
      doc.activeElement = this;
    },
    closest(sel) {
      let at = this;
      while (at) {
        if (sel === "#palette-list [role=option]" && at.getAttribute && at.getAttribute("role") === "option") return at;
        if (sel === "[data-palette-open]" && at.hasAttribute && at.hasAttribute("data-palette-open")) return at;
        if (sel === "[data-palette-close]" && at.hasAttribute && at.hasAttribute("data-palette-close")) return at;
        at = at.parentNode;
      }
      return null;
    },
  };
  return n;
}

let doc;

function page({ data, recentTasks, openTaskHref } = {}) {
  const timers = fakeTimers();
  const requests = [];
  const answers = {};
  const location = { assigned: [], pathname: "/tasks", assign(h) { this.assigned.push(h); } };
  const listeners = new Map();
  const input = node("input", { id: "palette-input" });
  const list = node("div", { id: "palette-list" });
  const status = node("p", { id: "palette-status" });
  const close = node("button", { "data-palette-close": "" });
  const dlg = node("dialog", { id: "palette" });
  dlg.open = false;
  dlg.showModal = function () { this.open = true; };
  dlg.close = function () {
    this.open = false;
    for (const fn of listeners.get("close") || []) fn({ target: dlg });
  };
  dlg.appendChild(input);
  dlg.appendChild(close);
  dlg.appendChild(list);
  const opener = node("button", { "data-palette-open": "" });
  const embedded = data || {
    screens: [{ id: "screen:/tasks", label: "Tasks", href: "/tasks" }, { id: "screen:/settings", label: "Settings", href: "/settings" }],
    actions: [{ id: "new-task", label: "New task" }, { id: "theme", label: "Switch the colour scheme" }],
  };
  const byId = {
    palette: dlg,
    "palette-input": input,
    "palette-list": list,
    "palette-status": status,
    "tix-palette-data": { textContent: JSON.stringify(embedded) },
  };
  doc = {
    activeElement: opener,
    body: node("body"),
    getElementById: (id) => byId[id] || null,
    querySelector: () => null,
    querySelectorAll: () => [],
    createElement: (tag) => node(tag),
    contains: () => true,
    addEventListener(type, fn) {
      if (!listeners.has(type)) listeners.set(type, []);
      listeners.get(type).push(fn);
    },
    dispatch(type, event) {
      for (const fn of listeners.get(type) || []) fn(event);
      return event;
    },
  };
  const fetch = (url) => {
    requests.push(url);
    const body = answers[url.split("?")[0]] || [];
    return Promise.resolve({ ok: true, json: () => Promise.resolve(body) });
  };
  const ctx = newContext({
    document: doc,
    location,
    fetch,
    setTimeout: timers.setTimeout,
    clearTimeout: timers.clearTimeout,
    localStorage: memoryStore(),
    sessionStorage: memoryStore(),
    Date,
    Promise,
    JSON,
  });
  if (recentTasks) ctx.tix.recentTasks = recentTasks;
  if (openTaskHref) ctx.tix.openTaskHref = openTaskHref;
  run(ctx, "palette.js");
  return { ctx, dlg, input, list, status, timers, requests, answers, location, opener };
}

function memoryStore() {
  const m = new Map();
  return {
    getItem: (k) => (m.has(k) ? m.get(k) : null),
    setItem: (k, v) => m.set(k, String(v)),
    removeItem: (k) => m.delete(k),
  };
}

function key(k, extra = {}) {
  return {
    key: k,
    ctrlKey: false,
    metaKey: false,
    altKey: false,
    shiftKey: false,
    target: doc.activeElement,
    defaultPrevented: false,
    prevented: 0,
    preventDefault() {
      this.prevented++;
      this.defaultPrevented = true;
    },
    ...extra,
  };
}

function labels(list) {
  return list.querySelectorAll("[role=option]").map((o) => o.children[0].textContent);
}

const settle = () => new Promise((r) => setImmediate(r));

test("ctrl+k and command+k open the palette with every screen and action drawn and no task request", () => {
  for (const mod of [{ ctrlKey: true }, { metaKey: true }]) {
    const p = page();
    const ev = doc.dispatch("keydown", key("k", mod));
    assert.equal(ev.prevented, 1);
    assert.equal(p.dlg.open, true);
    assert.equal(p.input.focused, 1);
    assert.deepEqual(labels(p.list), ["New task", "Switch the colour scheme", "Tasks", "Settings"]);
    assert.deepEqual(p.requests, ["/palette/projects"], "opening asked for something besides the projects");
  }
});

test("a chord the shortcut layer already claimed is left alone, open or closed", () => {
  const p = page();
  const ev = key("k", { ctrlKey: true });
  ev.defaultPrevented = true;
  doc.dispatch("keydown", ev);
  assert.equal(p.dlg.open, false);
  p.ctx.tix.openPalette();
  const again = key("k", { ctrlKey: true });
  again.defaultPrevented = true;
  doc.dispatch("keydown", again);
  assert.equal(p.dlg.open, true, "the palette the shortcut layer just opened was closed again");
});

test("ctrl+k again closes it and focus returns to what had it", () => {
  const p = page();
  doc.dispatch("keydown", key("k", { ctrlKey: true }));
  doc.dispatch("keydown", key("k", { ctrlKey: true }));
  assert.equal(p.dlg.open, false);
  assert.equal(p.opener.focused, 1);
});

test("typing searches tasks once, after the pause, and draws what comes back", async () => {
  const p = page();
  p.ctx.tix.openPalette();
  p.answers["/palette/tasks"] = [{ ref: "infra-4", title: "Deploy the api", status: "todo", href: "/tasks/infra-4" }];
  p.input.value = "de";
  doc.dispatch("input", { target: p.input });
  p.input.value = "dep";
  doc.dispatch("input", { target: p.input });
  assert.deepEqual(p.requests, ["/palette/projects"], "a task search went out before the pause");
  assert.equal(p.timers.delays()[0], 150);
  p.timers.fire();
  assert.deepEqual(p.requests, ["/palette/projects", "/palette/tasks?q=dep"]);
  await settle();
  assert.equal(labels(p.list)[0], "Deploy the api");
  assert.equal(p.status.textContent, "");
});

test("clearing the box sends nothing", () => {
  const p = page();
  p.ctx.tix.openPalette();
  p.input.value = "  ";
  doc.dispatch("input", { target: p.input });
  p.timers.fire();
  assert.deepEqual(p.requests, ["/palette/projects"]);
});

test("Enter on a task opens it in the dialog when the page has one", async () => {
  const opened = [];
  const p = page({ openTaskHref: (href, from) => { opened.push([href, from]); return true; } });
  p.ctx.tix.openPalette();
  p.answers["/palette/tasks"] = [{ ref: "infra-4", title: "Deploy", href: "/tasks/infra-4" }];
  p.input.value = "deploy";
  doc.dispatch("input", { target: p.input });
  p.timers.fire();
  await settle();
  doc.dispatch("keydown", key("Enter", { target: p.input }));
  assert.equal(opened.length, 1);
  assert.equal(opened[0][0], "/tasks/infra-4");
  assert.equal(opened[0][1], p.opener, "the dialog was not told where focus goes back to");
  assert.deepEqual(p.location.assigned, []);
  assert.equal(p.dlg.open, false);
});

test("Enter on a task opens its page when tasks open as pages", async () => {
  const p = page({ openTaskHref: () => false });
  p.ctx.tix.openPalette();
  p.answers["/palette/tasks"] = [{ ref: "infra-4", title: "Deploy", href: "/tasks/infra-4" }];
  p.input.value = "deploy";
  doc.dispatch("input", { target: p.input });
  p.timers.fire();
  await settle();
  doc.dispatch("keydown", key("Enter", { target: p.input }));
  assert.deepEqual(p.location.assigned, ["/tasks/infra-4"]);
});

test("arrows move the selection and wrap", () => {
  const p = page();
  p.ctx.tix.openPalette();
  doc.dispatch("keydown", key("ArrowDown", { target: p.input }));
  assert.equal(p.input.getAttribute("aria-activedescendant"), "palette-opt-1");
  doc.dispatch("keydown", key("ArrowUp", { target: p.input }));
  doc.dispatch("keydown", key("ArrowUp", { target: p.input }));
  assert.equal(p.input.getAttribute("aria-activedescendant"), "palette-opt-3");
  doc.dispatch("keydown", key("Enter", { target: p.input }));
  assert.deepEqual(p.location.assigned, ["/settings"]);
});

test("recent tasks from the sidebar come first, and their absence changes nothing else", () => {
  const p = page({ recentTasks: () => [{ ref: "web-9", title: "Looked at lately" }] });
  p.ctx.tix.openPalette();
  assert.equal(labels(p.list)[0], "Looked at lately");
  const q = page();
  q.ctx.tix.openPalette();
  assert.equal(labels(q.list)[0], "New task");
});

test("a chosen screen is remembered and offered first next time", () => {
  const p = page();
  p.ctx.tix.openPalette();
  doc.dispatch("keydown", key("ArrowUp", { target: p.input }));
  doc.dispatch("keydown", key("Enter", { target: p.input }));
  p.ctx.tix.openPalette();
  assert.equal(labels(p.list)[0], "Settings");
});
