// SPDX-License-Identifier: AGPL-3.0-or-later

// Loads the shipped assets into a fresh vm context, with only as much of a
// browser as the script under test actually touches. Nothing here is a DOM
// implementation: a test that needs real layout, real events or real
// clipboard behaviour does not belong in this suite.
import { readFileSync } from "node:fs";
import { createContext, runInContext } from "node:vm";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const assets = join(dirname(fileURLToPath(import.meta.url)), "..", "assets");

export function source(name) {
  return readFileSync(join(assets, name), "utf8");
}

// newContext returns a context with decide.js loaded and window bound, which
// is the state every other asset assumes when the browser runs it.
export function newContext(extra = {}) {
  const ctx = bareContext(extra);
  run(ctx, "decide.js");
  return ctx;
}

// bareContext is the same browser stand-in without decide.js, so a test can
// check that a script stands down when its decisions are not there.
export function bareContext(extra = {}) {
  const ctx = createContext({ ...extra });
  runInContext("var window = globalThis;", ctx);
  return ctx;
}

export function run(ctx, name) {
  runInContext(source(name), ctx, { filename: name });
}

// fakeTimers is a hand-driven setTimeout/clearTimeout pair. Time only moves
// when a test says so, so a debounce is asserted rather than waited out.
export function fakeTimers() {
  let next = 1;
  const pending = new Map();
  return {
    setTimeout(fn, ms) {
      const id = next++;
      pending.set(id, { fn, ms });
      return id;
    },
    clearTimeout(id) {
      pending.delete(id);
    },
    get count() {
      return pending.size;
    },
    delays() {
      return [...pending.values()].map((t) => t.ms);
    },
    // fire runs every timer currently due, the way a real clock would.
    fire() {
      const due = [...pending.entries()];
      pending.clear();
      for (const [, t] of due) {
        t.fn();
      }
      return due.length;
    },
  };
}

// element is the smallest thing shortcuts.js will accept as a node.
export function element(tagName, attrs = {}) {
  const own = { ...attrs };
  return {
    tagName,
    isContentEditable: !!attrs.isContentEditable,
    dataset: attrs.dataset || {},
    focused: 0,
    focus() {
      this.focused++;
    },
    hasAttribute(name) {
      return Object.prototype.hasOwnProperty.call(own, name);
    },
    getAttribute(name) {
      return Object.prototype.hasOwnProperty.call(own, name) ? own[name] : null;
    },
    setAttribute(name, value) {
      own[name] = value;
    },
    removeAttribute(name) {
      delete own[name];
    },
    querySelector() {
      return null;
    },
    addEventListener() {},
  };
}

// domNode is the smallest thing the delegated handlers in live.js will accept
// as an element: it answers closest() by walking real parents, and carries
// attributes and a classList. Unlike element() above it has a place in a tree,
// which is the whole point -- a delegated handler's first move is to ask the
// event target what it is inside of.
export function domNode(selectors = [], attrs = {}, children = []) {
  const own = { ...attrs };
  const classes = new Set((own.class || "").split(" ").filter(Boolean));
  const node = {
    selectors: [...selectors],
    parentNode: null,
    children,
    tagName: own.tagName || "DIV",
    innerHTML: "",
    textContent: own.textContent || "",
    options: own.options || [],
    value: own.value,
    fields: own.fields || {},
    matches(sel) {
      return this.selectors.indexOf(sel) !== -1;
    },
    closest(sel) {
      let at = this;
      while (at) {
        if (at.matches(sel)) {
          return at;
        }
        at = at.parentNode;
      }
      return null;
    },
    getAttribute(name) {
      return Object.prototype.hasOwnProperty.call(own, name) ? own[name] : null;
    },
    setAttribute(name, value) {
      own[name] = value;
    },
    hasAttribute(name) {
      return Object.prototype.hasOwnProperty.call(own, name);
    },
    classList: {
      add: (c) => classes.add(c),
      remove: (c) => classes.delete(c),
      contains: (c) => classes.has(c),
    },
    // descendants walks the subtree, which is all querySelector needs here:
    // these trees are a handful of nodes, and a real engine's ordering rules
    // are not what any of these tests are about.
    descendants() {
      return this.children.flatMap((c) => [c, ...c.descendants()]);
    },
    querySelector(sel) {
      return this.descendants().find((c) => c.matches(sel)) || null;
    },
    querySelectorAll(sel) {
      return this.descendants().filter((c) => c.matches(sel));
    },
  };
  for (const child of children) {
    child.parentNode = node;
  }
  return node;
}

// fakeDocument dispatches to whatever the script registered, and serves the
// handful of lookups the scripts make by id or selector.
export function fakeDocument({ byId = {}, bySelector = {}, bySelectorAll = {} } = {}) {
  const listeners = new Map();
  return {
    activeElement: null,
    // body is a node in its own right: live.js binds its htmx:beforeSwap
    // handler there, because hx-boost replaces the body's contents and leaves
    // the body itself in place.
    body: {
      addEventListener() {},
    },
    addEventListener(type, fn) {
      if (!listeners.has(type)) {
        listeners.set(type, []);
      }
      listeners.get(type).push(fn);
    },
    getElementById(id) {
      return Object.prototype.hasOwnProperty.call(byId, id) ? byId[id] : null;
    },
    querySelector(sel) {
      return Object.prototype.hasOwnProperty.call(bySelector, sel) ? bySelector[sel] : null;
    },
    querySelectorAll(sel) {
      return Object.prototype.hasOwnProperty.call(bySelectorAll, sel) ? bySelectorAll[sel] : [];
    },
    contains() {
      return true;
    },
    dispatch(type, event) {
      for (const fn of listeners.get(type) || []) {
        fn(event);
      }
      return event;
    },
    listenerCount(type) {
      return (listeners.get(type) || []).length;
    },
  };
}

// keydown builds an event that records whether the handler claimed it.
export function keydown(key, { target = element("DIV"), ctrlKey = false, shiftKey = false } = {}) {
  return {
    key,
    ctrlKey,
    shiftKey,
    target,
    prevented: 0,
    stopped: 0,
    preventDefault() {
      this.prevented++;
    },
    stopPropagation() {
      this.stopped++;
    },
  };
}
