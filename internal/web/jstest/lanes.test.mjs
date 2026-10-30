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
