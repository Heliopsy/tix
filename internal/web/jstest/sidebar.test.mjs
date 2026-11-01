// SPDX-License-Identifier: AGPL-3.0-or-later

// The sidebar's width and collapsed rail: assets/sidebar.js.
import test from "node:test";
import assert from "node:assert/strict";
import { bareContext, run } from "./harness.mjs";

const COOKIE = "tix_sidebar";
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
// cookieJar is document.cookie as a browser keeps it: assigning sets or
// removes one cookie, reading lists them all.
function cookieJar(initial = "") {
  const jar = new Map();
  if (initial) jar.set(COOKIE, initial);
  const writes = [];
  return {
    jar,
    writes,
    get cookie() {
      return [...jar].map(([k, v]) => k + "=" + v).join("; ");
    },
    set cookie(line) {
      writes.push(line);
      const [pair, ...attrs] = line.split(";").map((p) => p.trim());
      const at = pair.indexOf("=");
      const name = pair.slice(0, at);
      if (attrs.includes("Max-Age=0")) jar.delete(name);
      else jar.set(name, pair.slice(at + 1));
    },
  };
}

// group is a foldable sidebar group and its heading button.
function group(name) {
  const classes = new Set();
  const box = element('[data-navgroup="' + name + '"]');
  box.classList = { toggle(c, on) { if (on) classes.add(c); else classes.delete(c); }, contains: (c) => classes.has(c) };
  box.classes = classes;
  const button = element("[data-navgroup-toggle]");
  button.attrs["data-navgroup-toggle"] = name;
  button.attrs["aria-expanded"] = "true";
  return { box, button };
}

function page(cookie = "") {
  const jar = cookieJar(cookie);
  const configure = group("configure");
  const recent = group("recent");
  const classes = new Set();
  const props = {};
  const listeners = {};
  const handle = element("[data-side-resize]");
  const toggle = element("[data-side-toggle]");
  const link = element("[data-rail-name]");
  link.attrs["data-rail-name"] = "Tasks";
  const side = { getBoundingClientRect: () => ({ left: 0, width: 224 }) };
  const lists = {
    "[data-side-resize]": [handle], "[data-side-toggle]": [toggle], "[data-rail-name]": [link],
    '[data-navgroup="configure"]': [configure.box], '[data-navgroup-toggle="configure"]': [configure.button],
    '[data-navgroup="recent"]': [recent.box], '[data-navgroup-toggle="recent"]': [recent.button],
  };
  return {
    classes, props, listeners, handle, toggle, link, jar, configure, recent,
    fire(type, event) { for (const fn of listeners[type] || []) fn(event); },
    document: {
      get cookie() { return jar.cookie; },
      set cookie(v) { jar.cookie = v; },
      documentElement: {
        classList: {
          toggle(name, on) { if (on) classes.add(name); else classes.delete(name); },
          contains: (name) => classes.has(name),
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

function load(cookie = "", { storage = memoryStorage(), p = page(cookie) } = {}) {
  const ctx = bareContext({ localStorage: storage, document: p.document });
  run(ctx, "sidebar.js");
  return { tix: ctx.tix, p, storage };
}

const stored = (p) => p.jar.jar.get(COOKIE);

test("the root is marked as scripted before the body is parsed", () => {
  const { p } = load();
  assert.equal(p.classes.has("js"), true, "without it every [data-needs-js] control stays hidden");
});

test("an untouched browser gets the stylesheet's width, expanded, and nothing stored", () => {
  const { p } = load();
  assert.equal(p.props["--side-w"], undefined);
  assert.equal(p.classes.has(CLASS), false);
  assert.equal(p.jar.writes.length, 0);
  assert.equal(p.handle.attrs["aria-valuenow"], "224");
  assert.equal(p.toggle.attrs["aria-expanded"], "true");
});

test("a stored width and rail are applied as the script loads, before the page is parsed", () => {
  const { p } = load("w=320&rail=1");
  assert.equal(p.props["--side-w"], "320px");
  assert.equal(p.classes.has(CLASS), true);
  assert.equal(p.toggle.attrs["aria-expanded"], "false");
  assert.equal(p.toggle.attrs["aria-label"], "Expand sidebar");
  assert.equal(p.link.attrs.title, "Tasks", "a rail entry gets a tooltip");
});

test("a stored width outside the limits is clamped, and garbage is the default", () => {
  assert.equal(load("w=9000").p.props["--side-w"], "480px");
  assert.equal(load("w=10").p.props["--side-w"], "180px");
  for (const v of ["w=", "w=wide", "w=-5", "w=3e2", "w=200px", "w=0", "nonsense"]) {
    assert.equal(load(v).p.props["--side-w"], undefined, JSON.stringify(v));
  }
});

test("the cookie value reads and writes the same state, leaving defaults out", () => {
  const { tix } = load();
  const state = JSON.parse(JSON.stringify(tix.sidebarParse("fold=data.configure.bogus&w=300&rail=1")));
  assert.deepEqual(state, { width: 300, collapsed: true, fold: ["data", "configure"] });
  assert.equal(tix.sidebarFormat(state), "w=300&rail=1&fold=configure.data");
  assert.equal(tix.sidebarFormat({ width: null, collapsed: false, fold: [] }), "");
});

test("the toggle collapses and expands, keeping the width it had", () => {
  const { p } = load("w=300");
  p.fire("click", { target: p.toggle });
  assert.equal(p.classes.has(CLASS), true);
  assert.equal(stored(p), "w=300&rail=1");
  p.fire("click", { target: p.toggle });
  assert.equal(p.classes.has(CLASS), false);
  assert.equal(stored(p), "w=300", "expanded, the default, is left out");
  assert.equal(p.props["--side-w"], "300px");
  assert.equal(p.link.attrs.title, undefined, "the tooltip goes when the label is back");
});

test("the cookie is written for the whole site, a year, same-site", () => {
  const { p } = load();
  p.fire("click", { target: p.toggle });
  assert.match(p.jar.writes.at(-1), /^tix_sidebar=rail=1; Path=\/; Max-Age=31536000; SameSite=Lax$/);
  p.fire("click", { target: p.toggle });
  assert.match(p.jar.writes.at(-1), /^tix_sidebar=; Path=\/; Max-Age=0;/, "nothing but defaults removes it");
  assert.equal(p.jar.jar.has(COOKIE), false);
});

test("letting go of a drag below the threshold collapses, above it sets a clamped width", () => {
  const { tix } = load();
  assert.equal(tix.sidebarDrop(60), "collapse");
  assert.equal(tix.sidebarDrop(119), "collapse");
  assert.equal(tix.sidebarDrop(120), 180);
  assert.equal(tix.sidebarDrop(333.4), 333);
  assert.equal(tix.sidebarDrop(2000), 480);
});

test("a drag is applied while it moves and stored when it ends", () => {
  const { p } = load();
  p.fire("pointerdown", { target: p.handle, button: 0, pointerId: 1, preventDefault() {} });
  assert.equal(p.classes.has("side-dragging"), true);
  p.fire("pointermove", { pointerId: 1, clientX: 300 });
  assert.equal(p.props["--side-w"], "300px");
  assert.equal(p.jar.writes.length, 0, "nothing is written until the pointer is let go");
  p.fire("pointerup", { pointerId: 1, clientX: 310 });
  assert.equal(stored(p), "w=310");
  assert.equal(p.classes.has("side-dragging"), false);

  p.fire("pointerdown", { target: p.handle, button: 0, pointerId: 2, preventDefault() {} });
  p.fire("pointerup", { pointerId: 2, clientX: 40 });
  assert.equal(p.classes.has(CLASS), true);
  assert.equal(stored(p), "w=310&rail=1", "collapsing keeps the width to come back to");
});

test("the keyboard moves the separator and reports the new value", () => {
  const { p } = load("w=240");
  const key = (k, shiftKey = false) => p.fire("keydown", { target: p.handle, key: k, shiftKey, preventDefault() {} });
  key("ArrowRight");
  assert.equal(stored(p), "w=256");
  assert.equal(p.handle.attrs["aria-valuenow"], "256");
  key("ArrowLeft", true);
  assert.equal(stored(p), "w=192");
  key("End");
  assert.equal(stored(p), "w=480");
  key("Home");
  assert.equal(stored(p), "w=180");
  key("ArrowLeft");
  assert.equal(p.classes.has(CLASS), true, "left at the minimum collapses");
  key("ArrowRight");
  assert.equal(p.classes.has(CLASS), false, "right from the rail expands");
  assert.equal(stored(p), "w=180");
  key("Enter");
  assert.equal(p.jar.jar.has(COOKIE), false, "Enter restores the default and forgets the width");
  assert.equal(p.props["--side-w"], undefined);
});

test("a boosted swap brings the server's drawing and the script leaves it in place", () => {
  const p = page("rail=1");
  load("rail=1", { p });
  p.toggle.attrs["aria-expanded"] = "false";
  p.toggle.hidden = false;
  p.fire("htmx:load", {});
  assert.equal(p.toggle.attrs["aria-expanded"], "false");
  assert.equal(p.toggle.hidden, false, "the script never hides or reveals a control on a swap");
  assert.equal(p.handle.hidden, true, "nor reveals one: [data-needs-js] does, from the root's class");
});

test("a group heading folds and unfolds its group and remembers it", () => {
  const { p } = load("w=300");
  p.fire("click", { target: p.configure.button });
  assert.equal(p.configure.box.classes.has("is-folded"), true);
  assert.equal(p.configure.button.attrs["aria-expanded"], "false");
  assert.equal(stored(p), "w=300&fold=configure");
  p.fire("click", { target: p.recent.button });
  assert.equal(stored(p), "w=300&fold=recent.configure", "groups are written in drawing order");
  p.fire("click", { target: p.configure.button });
  assert.equal(p.configure.box.classes.has("is-folded"), false);
  assert.equal(p.configure.button.attrs["aria-expanded"], "true");
  assert.equal(stored(p), "w=300&fold=recent");
});

test("on the rail a group heading opens the sidebar with the group unfolded", () => {
  const { p } = load("rail=1&fold=recent");
  p.recent.box.classes.add("is-folded");
  p.recent.button.attrs["aria-expanded"] = "false";
  p.fire("click", { target: p.recent.button });
  assert.equal(p.classes.has(CLASS), false);
  assert.equal(p.recent.box.classes.has("is-folded"), false);
  assert.equal(p.recent.button.attrs["aria-expanded"], "true");
  assert.equal(p.jar.jar.has(COOKIE), false);
});

test("what an earlier build kept in localStorage moves into the cookie once", () => {
  const storage = memoryStorage({ "tix.sidebar.width": "320", "tix.sidebar.collapsed": "true", "tix.recent.closed": "true" });
  const { p } = load("", { storage });
  assert.equal(stored(p), "w=320&rail=1&fold=recent");
  assert.equal(storage.data.size, 0, "the old keys are forgotten");
  assert.equal(p.classes.has(CLASS), true);

  const kept = memoryStorage({ "tix.sidebar.collapsed": "true" });
  const second = load("w=200", { storage: kept });
  assert.equal(stored(second.p), "w=200", "a cookie already there wins");
  assert.equal(kept.data.size, 0);
});

test("storage and cookies that throw read as the default and pressing does not throw", () => {
  const p = page();
  Object.defineProperty(p.document, "cookie", { get() { throw new Error("denied"); }, set() { throw new Error("denied"); } });
  const { p: got } = load("", { storage: throwingStorage, p });
  assert.equal(got.classes.has(CLASS), false);
  assert.doesNotThrow(() => got.fire("click", { target: got.toggle }));
  assert.doesNotThrow(() => got.fire("click", { target: got.configure.button }));
});
