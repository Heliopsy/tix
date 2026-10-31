// SPDX-License-Identifier: AGPL-3.0-or-later

// Show completed on the lanes view: assets/lanes.js.
import test from "node:test";
import assert from "node:assert/strict";
import { bareContext, run } from "./harness.mjs";

const KEY = "tix.lanes.showCompleted";
const HIDE = "lanes-hide-finished";

// memoryStorage is localStorage's surface over a plain map.
function memoryStorage(initial = {}) {
  const data = new Map(Object.entries(initial));
  return {
    data,
    getItem: (k) => (data.has(k) ? data.get(k) : null),
    setItem: (k, v) => data.set(k, String(v)),
    removeItem: (k) => data.delete(k),
  };
}

// throwingStorage is a browser that refuses storage outright.
const throwingStorage = {
  getItem() { throw new Error("denied"); },
  setItem() { throw new Error("denied"); },
  removeItem() { throw new Error("denied"); },
};

// page is as much of a document as lanes.js touches: the root's class list,
// the toggles, and listeners it registers.
function page(toggles = 1) {
  const classes = new Set();
  const listeners = {};
  const buttons = Array.from({ length: toggles }, () => {
    const attrs = { "aria-pressed": "true" };
    return {
      hidden: true,
      attrs,
      setAttribute(k, v) { attrs[k] = v; },
      closest(sel) { return sel === "[data-lanes-completed]" ? this : null; },
    };
  });
  return {
    buttons,
    listeners,
    classes,
    document: {
      documentElement: {
        classList: {
          toggle(name, on) { if (on) classes.add(name); else classes.delete(name); },
          contains: (name) => classes.has(name),
        },
      },
      querySelectorAll: (sel) => (sel === "[data-lanes-completed]" ? buttons : []),
      addEventListener(type, fn) { (listeners[type] ||= []).push(fn); },
    },
  };
}

function load(storage, p = page()) {
  const ctx = bareContext({ localStorage: storage, document: p.document });
  run(ctx, "lanes.js");
  return { ctx, p };
}

test("an untouched browser shows finished cards", () => {
  const { p } = load(memoryStorage());
  assert.equal(p.classes.has(HIDE), false);
  assert.equal(p.buttons[0].hidden, false, "the toggle stays hidden with scripting on");
  assert.equal(p.buttons[0].attrs["aria-pressed"], "true");
});

test("a stored choice is applied as the script loads, before the page is parsed", () => {
  const { p } = load(memoryStorage({ [KEY]: "false" }));
  assert.equal(p.classes.has(HIDE), true);
  assert.equal(p.buttons[0].attrs["aria-pressed"], "false");
});

test("pressing the toggle hides, then shows, and only the one key is ever stored", () => {
  const storage = memoryStorage();
  const { p } = load(storage);
  const click = p.listeners.click[0];

  click({ target: p.buttons[0] });
  assert.equal(p.classes.has(HIDE), true);
  assert.deepEqual([...storage.data.entries()], [[KEY, "false"]]);
  assert.equal(p.buttons[0].attrs["aria-pressed"], "false");

  click({ target: p.buttons[0] });
  assert.equal(p.classes.has(HIDE), false);
  assert.equal(storage.data.size, 0, "showing them, the default, leaves nothing behind");
  assert.equal(p.buttons[0].attrs["aria-pressed"], "true");
});

test("a click elsewhere changes nothing", () => {
  const storage = memoryStorage();
  const { p } = load(storage);
  p.listeners.click[0]({ target: { closest: () => null } });
  assert.equal(p.classes.has(HIDE), false);
  assert.equal(storage.data.size, 0);
});

test("a refreshed listing gets its toggle state back", () => {
  const storage = memoryStorage({ [KEY]: "false" });
  const p = page();
  load(storage, p);
  p.buttons[0].attrs["aria-pressed"] = "true";
  p.buttons[0].hidden = true;
  for (const fn of p.listeners["htmx:load"]) fn({});
  assert.equal(p.buttons[0].attrs["aria-pressed"], "false");
  assert.equal(p.buttons[0].hidden, false);
});

test("storage that throws reads as shown and pressing does not throw", () => {
  const { p } = load(throwingStorage);
  assert.equal(p.classes.has(HIDE), false);
  assert.doesNotThrow(() => p.listeners.click[0]({ target: p.buttons[0] }));
});

test("anything but false reads as shown", () => {
  for (const value of ["true", "", "0", "no"]) {
    const { p } = load(memoryStorage({ [KEY]: value }));
    assert.equal(p.classes.has(HIDE), false, JSON.stringify(value));
  }
});

// ---------------------------------------------------------------------------
// A lane card's own forms under htmx.

// laneDoc is a document with one lane card holding a form of the named
// control, and the elements lanes.js creates or focuses.
function laneDoc(control = "status") {
  const listeners = {};
  const focused = [];
  const appended = [];
  const pressed = { focus: (opts) => focused.push(["control", opts]) };
  const body = { appendChild: (el) => appended.push(el) };
  const card = {
    id: "t-infra-1",
    focus: () => focused.push(["card"]),
    querySelector: (sel) => {
      if (sel === `[data-focus="${control}"]`) return pressed;
      if (sel === ".lane-body") return body;
      return null;
    },
    querySelectorAll: (sel) => (sel === ".is-busy" ? busyButtons : sel === 'form[aria-busy="true"]' ? busyForms : []),
  };
  const busyButtons = [{
    disabled: true,
    classes: new Set(["check", "is-busy"]),
    get classList() { const c = this.classes; return { remove: (n) => c.delete(n) }; },
  }];
  const busyForms = [{ attrs: { "aria-busy": "true" }, removeAttribute(n) { delete this.attrs[n]; } }];
  const form = {
    getAttribute: (name) => (name === "data-lane-control" ? control : null),
    closest: (sel) => (sel === ".lane-card" ? card : null),
  };
  const button = { closest: (sel) => (sel === "[data-lane-control]" ? form : sel === ".lane-card" ? card : null) };
  const document = {
    documentElement: { classList: { toggle() {}, contains: () => false } },
    querySelectorAll: () => [],
    addEventListener(type, fn) { (listeners[type] ||= []).push(fn); },
    getElementById: (id) => (id === card.id ? card : null),
    createElement: () => ({ setAttribute(k, v) { this[k] = v; } }),
  };
  return { document, listeners, focused, appended, button, card, busyButtons, busyForms };
}

function fire(listeners, type, detail) {
  for (const fn of listeners[type] || []) fn({ detail });
}

test("laneControl names a lane form's control and nothing outside a card", () => {
  const d = laneDoc("prio");
  const ctx = bareContext({ document: d.document });
  run(ctx, "lanes.js");
  assert.equal(ctx.tix.laneControl(d.button), "prio");
  assert.equal(ctx.tix.laneControl({ closest: () => null }), "");
  assert.equal(ctx.tix.laneControl(null), "");
});

test("after a lane card is swapped, focus is on the same control of the new card", () => {
  const d = laneDoc("tick");
  const ctx = bareContext({ document: d.document });
  run(ctx, "lanes.js");
  fire(d.listeners, "htmx:beforeRequest", { elt: d.button });
  fire(d.listeners, "htmx:afterSettle", {});
  assert.deepEqual(JSON.parse(JSON.stringify(d.focused)), [["control", { preventScroll: true }]]);
  fire(d.listeners, "htmx:afterSettle", {});
  assert.equal(d.focused.length, 1, "a later swap elsewhere stole focus back to the card");
});

test("a refused lane action stays on the card and says why", () => {
  const d = laneDoc("status");
  const errorPage = {
    querySelector: (sel) => (sel.includes(".failure-message") ? { textContent: "  no route from \"done\"  " } : null),
  };
  const ctx = bareContext({
    document: d.document,
    DOMParser: function () { return { parseFromString: () => errorPage }; },
  });
  run(ctx, "lanes.js");
  fire(d.listeners, "htmx:beforeRequest", { elt: d.button });
  const detail = { elt: d.card, requestConfig: { elt: d.button }, xhr: { status: 412, responseText: "<html>" }, shouldSwap: true };
  fire(d.listeners, "htmx:beforeSwap", detail);
  assert.equal(detail.shouldSwap, false, "the error page replaced the card");
  assert.equal(d.appended.length, 1, "nothing was said on the card");
  assert.equal(d.appended[0].className, "lane-error");
  assert.equal(d.appended[0].role, "alert");
  assert.equal(d.appended[0].textContent, 'Not saved: no route from "done"');
  assert.equal(d.focused.length, 1, "focus was not kept on the pressed control");
  assert.equal(d.busyButtons[0].disabled, false, "the pressed button stayed disabled after the refusal");
  assert.equal(d.busyButtons[0].classes.has("is-busy"), false, "the pressed button still says it is busy");
  assert.deepEqual(Object.keys(d.busyForms[0].attrs), [], "a form left aria-busy would hold every live refresh back");

  const ok = { elt: d.card, requestConfig: { elt: d.button }, xhr: { status: 200 }, shouldSwap: true };
  fire(d.listeners, "htmx:beforeSwap", ok);
  assert.equal(ok.shouldSwap, true, "a successful answer was held back");
  const elsewhere = { elt: { closest: () => null }, xhr: { status: 500 }, shouldSwap: true };
  fire(d.listeners, "htmx:beforeSwap", elsewhere);
  assert.equal(elsewhere.shouldSwap, true, "a refusal outside the lanes was taken over");
});

// ---------------------------------------------------------------------------
// The more actions menu.

function lanesCtx(extra = {}) {
  const listeners = {};
  const document = {
    documentElement: { classList: { toggle() {}, contains: () => false } },
    querySelectorAll: () => [],
    addEventListener(type, fn) { (listeners[type] ||= []).push(fn); },
    getElementById: () => null,
    ...extra,
  };
  const ctx = bareContext({ document });
  run(ctx, "lanes.js");
  return { ctx, listeners, document };
}

test("the arrow keys walk a menu and wrap, Home and End go to its ends", () => {
  const { ctx } = lanesCtx();
  const step = ctx.tix.laneMenuStep;
  const cases = [
    [5, 0, "ArrowDown", 1],
    [5, 4, "ArrowDown", 0],
    [5, 0, "ArrowUp", 4],
    [5, 2, "ArrowUp", 1],
    [5, -1, "ArrowDown", 0],
    [5, -1, "ArrowUp", 4],
    [5, 3, "Home", 0],
    [5, 1, "End", 4],
    [5, 1, "Enter", -1],
    [5, 1, "a", -1],
    [0, -1, "ArrowDown", -1],
  ];
  for (const [count, index, key, want] of cases) {
    assert.equal(step(count, index, key), want, `${key} from ${index} of ${count}`);
  }
});

// menuDoc is a lane menu of three items with focus on one of them.
function menuDoc(on = 0) {
  const focused = [];
  const items = ["a", "b", "c"].map((name) => ({
    name,
    tagName: "BUTTON",
    hidden: false,
    closest(sel) { return sel === ".lanepop" ? menu : null; },
    focus() { focused.push(name); },
  }));
  const menu = { querySelectorAll: () => items };
  return { items, focused, target: items[on] };
}

test("an arrow key in a lane menu moves focus to the next item and is not scrolled by", () => {
  const { listeners } = lanesCtx();
  const d = menuDoc(2);
  let prevented = false;
  for (const fn of listeners.keydown) fn({ target: d.target, key: "ArrowDown", preventDefault() { prevented = true; } });
  assert.deepEqual(d.focused, ["a"]);
  assert.equal(prevented, true, "the page scrolled under the menu as well");

  const field = { ...menuDoc(0).target, tagName: "INPUT" };
  let taken = false;
  for (const fn of listeners.keydown) fn({ target: field, key: "ArrowDown", preventDefault() { taken = true; } });
  assert.equal(taken, false, "a text field lost its own arrows, and the assignee suggestions with them");

  const outside = { tagName: "BUTTON", closest: () => null };
  let stolen = false;
  for (const fn of listeners.keydown) fn({ target: outside, key: "ArrowDown", preventDefault() { stolen = true; } });
  assert.equal(stolen, false, "an arrow key outside any menu was taken");
});

test("opening the more actions menu puts focus on its first item, other menus are left alone", () => {
  const { listeners } = lanesCtx();
  const d = menuDoc();
  const more = { hasAttribute: (n) => n === "data-lane-menu", querySelectorAll: () => d.items };
  for (const fn of listeners.toggle) fn({ target: more, newState: "open" });
  assert.deepEqual(d.focused, ["a"]);
  for (const fn of listeners.toggle) fn({ target: more, newState: "closed" });
  const badge = { hasAttribute: () => false, querySelectorAll: () => d.items };
  for (const fn of listeners.toggle) fn({ target: badge, newState: "open" });
  assert.deepEqual(d.focused, ["a"], "closing it, or opening another menu, moved focus");
});

test("a link in a lane menu shuts the menu before it is followed", () => {
  const { listeners } = lanesCtx();
  let hidden = 0;
  const menu = { hidePopover() { hidden++; } };
  const link = { closest: (sel) => (sel === "[popover]" ? menu : null) };
  const target = { closest: (sel) => (sel === ".lanepop a[href]" ? link : null) };
  let stopped = false;
  for (const fn of listeners.click) fn({ target, stopPropagation() { stopped = true; } });
  assert.equal(hidden, 1, "the menu stayed open over the task, holding live refreshes back");
  assert.equal(stopped, false, "the link itself was stopped");
});

// card is a lane card among siblings, for laneNeighbour.
function cards(...ids) {
  const els = ids.map((id) => ({ id, classList: { contains: (c) => c === "lane-card" && id.startsWith("t-") } }));
  els.forEach((el, i) => {
    el.nextElementSibling = els[i + 1] || null;
    el.previousElementSibling = els[i - 1] || null;
  });
  return els;
}

test("a deleted card hands focus to the next card in its lane, or the one before", () => {
  const { ctx } = lanesCtx();
  const [a, , c] = cards("t-a", "lane-empty", "t-c");
  assert.equal(ctx.tix.laneNeighbour(a), "t-c", "a paragraph between cards was taken for one");
  assert.equal(ctx.tix.laneNeighbour(c), "t-a");
  assert.equal(ctx.tix.laneNeighbour(cards("t-only")[0]), "");
  assert.equal(ctx.tix.laneNeighbour(null), "");
});

test("after a delete swaps the card away, focus lands on its neighbour", () => {
  const focused = [];
  const [gone, next] = cards("t-gone", "t-next");
  next.focus = () => focused.push("t-next");
  next.querySelector = () => null;
  const form = {
    getAttribute: (n) => (n === "data-lane-control" ? "more" : null),
    closest: (sel) => (sel === ".lane-card" ? gone : null),
  };
  gone.querySelector = () => null;
  const button = { closest: (sel) => (sel === "[data-lane-control]" ? form : sel === ".lane-card" ? gone : null) };
  const { listeners } = lanesCtx({ getElementById: (id) => (id === "t-next" ? next : null) });
  fire(listeners, "htmx:beforeRequest", { elt: button });
  fire(listeners, "htmx:afterSettle", {});
  assert.deepEqual(focused, ["t-next"], "focus fell to the page when the card it was on went away");
});
