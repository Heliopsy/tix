// SPDX-License-Identifier: AGPL-3.0-or-later

// Recently viewed tasks in the sidebar: assets/recent.js.
import test from "node:test";
import assert from "node:assert/strict";
import { bareContext, run } from "./harness.mjs";

function memoryStorage(seed = {}) {
  const data = new Map(Object.entries(seed));
  return {
    getItem: (k) => (data.has(k) ? data.get(k) : null),
    setItem: (k, v) => data.set(k, String(v)),
    removeItem: (k) => data.delete(k),
    data,
  };
}

function loaded(extra = {}) {
  const ctx = bareContext(extra);
  run(ctx, "recent.js");
  return ctx;
}

test("a task opened again moves to the front, once", () => {
  const { tix } = loaded();
  let list = [];
  for (const ref of ["a-1", "a-2", "a-3"]) list = tix.recentAdd(list, { ref, title: ref, at: "" });
  list = tix.recentAdd(list, { ref: "a-1", title: "renamed", at: "" });
  assert.deepEqual(JSON.parse(JSON.stringify(list.map((e) => e.ref))), ["a-1", "a-3", "a-2"]);
  assert.equal(list[0].title, "renamed");
});

test("the list keeps the last ten", () => {
  const { tix } = loaded();
  let list = [];
  for (let i = 1; i <= 12; i++) list = tix.recentAdd(list, { ref: "a-" + i, title: "t", at: "" });
  assert.equal(list.length, 10);
  assert.equal(list[0].ref, "a-12");
  assert.equal(list[9].ref, "a-3");
});

test("a damaged store reads as empty, and bad entries are dropped", () => {
  const { tix } = loaded();
  assert.equal(tix.recentParse("not json").length, 0);
  assert.equal(tix.recentParse('{"ref":"a-1"}').length, 0);
  const got = tix.recentParse('[{"ref":"a-1","title":"x"},{"title":"no ref"},null,{"ref":"a-2","title":"y","at":"2026-11-01T10:00:00Z"}]');
  assert.deepEqual(JSON.parse(JSON.stringify(got)), [
    { ref: "a-1", title: "x", at: "" },
    { ref: "a-2", title: "y", at: "2026-11-01T10:00:00Z" },
  ]);
});

test("recentTasks reads the signed-in reader's own list, newest first", () => {
  const localStorage = memoryStorage({
    "tix.recent.t1:alice": '[{"ref":"infra-2","title":"Deploy","at":"x"},{"ref":"infra-1","title":"Plan","at":"y"}]',
    "tix.recent.t1:bob": '[{"ref":"ops-9","title":"Not alice","at":"z"}]',
  });
  const document = { querySelector: (sel) => (sel === "[data-recent-for]" ? { getAttribute: () => "t1:alice" } : null) };
  const { tix } = loaded({ localStorage, document });
  const got = JSON.parse(JSON.stringify(tix.recentTasks()));
  assert.deepEqual(got, [
    { ref: "infra-2", title: "Deploy", href: "/tasks/infra-2", at: "x" },
    { ref: "infra-1", title: "Plan", href: "/tasks/infra-1", at: "y" },
  ]);
});

test("each reader has a key of their own", () => {
  const { tix } = loaded();
  assert.notEqual(tix.recentKey("t1:alice"), tix.recentKey("t1:bob"));
});

// sidebarPage is as much of a page as recent.js draws into: the preserved
// recent block, its list, and whatever task marks the swapped content holds.
function sidebarPage(pathname) {
  const listeners = {};
  const node = (tag) => {
    const attrs = {};
    return {
      tag, attrs, children: [], className: "", textContent: "", href: "",
      setAttribute(k, v) { attrs[k] = String(v); },
      getAttribute(k) { return k in attrs ? attrs[k] : null; },
      hasAttribute(k) { return k in attrs; },
      appendChild(c) { this.children.push(c); return c; },
      removeChild(c) { this.children.splice(this.children.indexOf(c), 1); return c; },
      get firstChild() { return this.children[0] || null; },
    };
  };
  const ul = node("ul");
  const box = node("div");
  box.hidden = true;
  box.className = "navgroup side-recent is-folded";
  box.setAttribute("data-recent-for", "t1:alice");
  box.querySelector = (sel) => (sel === "[data-recent-list]" ? ul : null);
  let marks = [];
  const document = {
    readyState: "complete",
    querySelector: (sel) => (sel === "[data-recent]" || sel === "[data-recent-for]" ? box : null),
    querySelectorAll: (sel) => (sel === "[data-recent-task]" ? marks : []),
    createElement: node,
    addEventListener(type, fn) { (listeners[type] ||= []).push(fn); },
    dispatchEvent() {},
  };
  const location = { pathname };
  return {
    box, ul, document, location,
    show(ref, title) {
      const m = node("h1");
      m.setAttribute("data-recent-task", ref);
      m.setAttribute("data-recent-title", title);
      marks = [m];
    },
    navigate(path) {
      location.pathname = path;
      for (const fn of listeners["htmx:afterSettle"] || []) fn({ detail: { elt: document } });
    },
  };
}

test("a boosted navigation redraws the preserved list in place and moves only the current mark", () => {
  const localStorage = memoryStorage({
    "tix.recent.t1:alice": '[{"ref":"infra-2","title":"Deploy","at":"x"},{"ref":"infra-1","title":"Plan","at":"y"}]',
  });
  const p = sidebarPage("/tasks/infra-2");
  loaded({ localStorage, document: p.document, location: p.location });
  assert.equal(p.box.hidden, false, "a list with entries is shown");
  assert.equal(p.ul.children.length, 2);
  assert.equal(p.ul.children[0].children[0].getAttribute("aria-current"), "page");
  const first = p.ul.children[0].children[0];
  assert.equal(first.children[0].textContent, "INFRA-2", "the ref is drawn upper case");
  assert.equal(first.getAttribute("data-rail-name"), "INFRA-2 Deploy", "the folded rail names it upper case too");
  assert.equal(first.href, "/tasks/infra-2", "the link keeps the stored ref");

  p.navigate("/tasks/infra-1");
  assert.equal(p.ul.children.length, 2, "the same two entries, no row comes or goes");
  assert.equal(p.ul.children[0].children[0].getAttribute("aria-current"), null);
  assert.equal(p.ul.children[1].children[0].getAttribute("aria-current"), "page");
  assert.equal(p.box.className, "navgroup side-recent is-folded", "folding is the sidebar's, never redrawn here");
  assert.equal(p.box.open, undefined, "nothing re-opens a details element on each swap");
});

test("the drawn links are handed to htmx, so opening one is a boosted navigation", () => {
  const localStorage = memoryStorage({ "tix.recent.t1:alice": '[{"ref":"infra-2","title":"Deploy","at":"x"}]' });
  const p = sidebarPage("/tasks");
  const processed = [];
  loaded({ localStorage, document: p.document, location: p.location, htmx: { process: (el) => processed.push(el) } });
  assert.equal(processed.at(-1), p.ul);
  p.navigate("/projects");
  assert.equal(processed.length, 2, "and again after each redraw");
});
