// SPDX-License-Identifier: AGPL-3.0-or-later

// The sidebar's width and collapsed rail: assets/sidebar.js.
import test from "node:test";
import assert from "node:assert/strict";
import { bareContext, run } from "./harness.mjs";

const WIDTH = "tix.sidebar.width";
const COLLAPSED = "tix.sidebar.collapsed";
const CLASS = "side-collapsed";

function memoryStorage(initial = {}) {
  const data = new Map(Object.entries(initial));
  return {
    data,
    getItem: (k) => (data.has(k) ? data.get(k) : null),
    setItem: (k, v) => data.set(k, String(v)),
    removeItem: (k) => data.delete(k),
  };
}

const throwingStorage = {
  getItem() { throw new Error("denied"); },
  setItem() { throw new Error("denied"); },
  removeItem() { throw new Error("denied"); },
};

// element is an attribute bag that answers closest for its own marker.
function element(marker) {
  const attrs = {};
  return {
    hidden: true,
    attrs,
    setAttribute(k, v) { attrs[k] = v; },
    removeAttribute(k) { delete attrs[k]; },
    getAttribute(k) { return k in attrs ? attrs[k] : null; },
    closest(sel) { return sel === marker ? this : null; },
  };
}

// page is as much of a document as sidebar.js touches.
function page() {
  const classes = new Set();
  const props = {};
  const listeners = {};
  const handle = element("[data-side-resize]");
  const toggle = element("[data-side-toggle]");
  const link = element("[data-rail-name]");
  link.attrs["data-rail-name"] = "Tasks";
  const side = { getBoundingClientRect: () => ({ left: 0, width: 224 }) };
  const lists = { "[data-side-resize]": [handle], "[data-side-toggle]": [toggle], "[data-rail-name]": [link] };
  return {
    classes, props, listeners, handle, toggle, link,
    fire(type, event) { for (const fn of listeners[type] || []) fn(event); },
    document: {
      documentElement: {
        classList: {
          toggle(name, on) { if (on) classes.add(name); else classes.delete(name); },
          add: (name) => classes.add(name),
          remove: (name) => classes.delete(name),
        },
        style: {
          setProperty: (k, v) => { props[k] = v; },
          removeProperty: (k) => { delete props[k]; },
        },
      },
      querySelector: (sel) => (sel === "aside.side" ? side : null),
      querySelectorAll: (sel) => lists[sel] || [],
      addEventListener(type, fn) { (listeners[type] ||= []).push(fn); },
    },
  };
}

function load(storage, p = page()) {
  const ctx = bareContext({ localStorage: storage, document: p.document });
  run(ctx, "sidebar.js");
  return { tix: ctx.tix, p };
}

test("an untouched browser gets the stylesheet's width, expanded, and nothing stored", () => {
  const storage = memoryStorage();
  const { p } = load(storage);
  assert.equal(p.props["--side-w"], undefined);
  assert.equal(p.classes.has(CLASS), false);
  assert.equal(storage.data.size, 0);
  assert.equal(p.handle.hidden, false, "the handle is shown once the script runs");
  assert.equal(p.toggle.hidden, false, "the toggle is shown once the script runs");
  assert.equal(p.handle.attrs["aria-valuenow"], "224");
  assert.equal(p.toggle.attrs["aria-expanded"], "true");
});

test("a stored width and rail are applied as the script loads, before the page is parsed", () => {
  const { p } = load(memoryStorage({ [WIDTH]: "320", [COLLAPSED]: "true" }));
  assert.equal(p.props["--side-w"], "320px");
  assert.equal(p.classes.has(CLASS), true);
  assert.equal(p.toggle.attrs["aria-expanded"], "false");
  assert.equal(p.toggle.attrs["aria-label"], "Expand sidebar");
  assert.equal(p.link.attrs.title, "Tasks", "a rail entry gets a tooltip");
});

test("a stored width outside the limits is clamped, and garbage is the default", () => {
  assert.equal(load(memoryStorage({ [WIDTH]: "9000" })).p.props["--side-w"], "480px");
  assert.equal(load(memoryStorage({ [WIDTH]: "10" })).p.props["--side-w"], "180px");
  for (const v of ["", "wide", "-5", "3e2", "200px"]) {
    assert.equal(load(memoryStorage({ [WIDTH]: v })).p.props["--side-w"], undefined, JSON.stringify(v));
  }
});

test("the toggle collapses and expands, keeping the width it had", () => {
  const storage = memoryStorage({ [WIDTH]: "300" });
  const { p } = load(storage);
  p.fire("click", { target: p.toggle });
  assert.equal(p.classes.has(CLASS), true);
  assert.equal(storage.data.get(COLLAPSED), "true");
  assert.equal(storage.data.get(WIDTH), "300");
  p.fire("click", { target: p.toggle });
  assert.equal(p.classes.has(CLASS), false);
  assert.equal(storage.data.has(COLLAPSED), false, "expanded, the default, leaves no key");
  assert.equal(p.props["--side-w"], "300px");
  assert.equal(p.link.attrs.title, undefined, "the tooltip goes when the label is back");
});

test("letting go of a drag below the threshold collapses, above it sets a clamped width", () => {
  const { tix } = load(memoryStorage());
  assert.equal(tix.sidebarDrop(60), "collapse");
  assert.equal(tix.sidebarDrop(119), "collapse");
  assert.equal(tix.sidebarDrop(120), 180);
  assert.equal(tix.sidebarDrop(333.4), 333);
  assert.equal(tix.sidebarDrop(2000), 480);
});

test("a drag is applied while it moves and stored when it ends", () => {
  const storage = memoryStorage();
  const { p } = load(storage);
  p.fire("pointerdown", { target: p.handle, button: 0, pointerId: 1, preventDefault() {} });
  assert.equal(p.classes.has("side-dragging"), true);
  p.fire("pointermove", { pointerId: 1, clientX: 300 });
  assert.equal(p.props["--side-w"], "300px");
  assert.equal(storage.data.size, 0, "nothing is written until the pointer is let go");
  p.fire("pointerup", { pointerId: 1, clientX: 310 });
  assert.equal(storage.data.get(WIDTH), "310");
  assert.equal(p.classes.has("side-dragging"), false);

  p.fire("pointerdown", { target: p.handle, button: 0, pointerId: 2, preventDefault() {} });
  p.fire("pointerup", { pointerId: 2, clientX: 40 });
  assert.equal(p.classes.has(CLASS), true);
  assert.equal(storage.data.get(COLLAPSED), "true");
  assert.equal(storage.data.get(WIDTH), "310", "collapsing keeps the width to come back to");
});

test("the keyboard moves the separator and reports the new value", () => {
  const storage = memoryStorage({ [WIDTH]: "240" });
  const { p } = load(storage);
  const key = (k, shiftKey = false) => p.fire("keydown", { target: p.handle, key: k, shiftKey, preventDefault() {} });
  key("ArrowRight");
  assert.equal(storage.data.get(WIDTH), "256");
  assert.equal(p.handle.attrs["aria-valuenow"], "256");
  key("ArrowLeft", true);
  assert.equal(storage.data.get(WIDTH), "192");
  key("End");
  assert.equal(storage.data.get(WIDTH), "480");
  key("Home");
  assert.equal(storage.data.get(WIDTH), "180");
  key("ArrowLeft");
  assert.equal(p.classes.has(CLASS), true, "left at the minimum collapses");
  key("ArrowRight");
  assert.equal(p.classes.has(CLASS), false, "right from the rail expands");
  assert.equal(storage.data.get(WIDTH), "180");
  key("Enter");
  assert.equal(storage.data.size, 0, "Enter restores the default and forgets the width");
  assert.equal(p.props["--side-w"], undefined);
});

test("a boosted swap gets its controls back", () => {
  const p = page();
  load(memoryStorage({ [COLLAPSED]: "true" }), p);
  p.toggle.hidden = true;
  p.toggle.attrs["aria-expanded"] = "true";
  p.fire("htmx:load", {});
  assert.equal(p.toggle.hidden, false);
  assert.equal(p.toggle.attrs["aria-expanded"], "false");
});

test("storage that throws reads as the default and pressing does not throw", () => {
  const { p } = load(throwingStorage);
  assert.equal(p.classes.has(CLASS), false);
  assert.doesNotThrow(() => p.fire("click", { target: p.toggle }));
});
