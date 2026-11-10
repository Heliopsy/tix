// SPDX-License-Identifier: AGPL-3.0-or-later

// assets/deploy.js: when a tab reloads into a new release by itself, when it
// only offers to, and what it says afterwards. The DOM is a stub.
import test from "node:test";
import assert from "node:assert/strict";
import { newContext, run, fakeDocument, fakeTimers } from "./harness.mjs";

const settle = () => new Promise((resolve) => setImmediate(resolve));

function field(tagName, value, defaultValue, type = "text") {
  return { tagName, type, value, defaultValue };
}

function fakeStore(initial = {}) {
  const data = { ...initial };
  return {
    data,
    getItem: (k) => (k in data ? data[k] : null),
    setItem: (k, v) => { data[k] = String(v); },
    removeItem: (k) => { delete data[k]; },
  };
}

function page({ build = "v0.55.0", server = "v0.56.0", selector = {}, fields = [], store = fakeStore() } = {}) {
  const doc = fakeDocument({ bySelector: selector, bySelectorAll: { "input, textarea, select": fields } });
  const appended = [];
  doc.readyState = "complete";
  doc.hidden = false;
  doc.body.getAttribute = (name) => (name === "data-build" ? build : null);
  doc.body.appendChild = (el) => { appended.push(el); el.parentNode = doc.body; el.isConnected = true; };
  doc.body.removeChild = (el) => { appended.splice(appended.indexOf(el), 1); };
  doc.createElement = (tagName) => ({
    tagName: tagName.toUpperCase(),
    attrs: {},
    kids: [],
    listeners: {},
    setAttribute(n, v) { this.attrs[n] = v; },
    appendChild(el) { this.kids.push(el); },
    addEventListener(type, fn) { this.listeners[type] = fn; },
  });
  const timers = fakeTimers();
  const reloads = [];
  const fetched = [];
  const ctx = newContext({
    setTimeout: timers.setTimeout,
    clearTimeout: timers.clearTimeout,
    setInterval() {},
    scrollY: 120,
    scrollTo() {},
    sessionStorage: store,
    location: { pathname: "/tasks", search: "?view=board", reload: () => reloads.push(1) },
    fetch: (url) => {
      fetched.push(url);
      return Promise.resolve({ ok: true, json: () => Promise.resolve({ status: "ok", version: server }) });
    },
  });
  ctx.document = doc;
  run(ctx, "deploy.js");
  return { ctx, doc, appended, timers, reloads, fetched, store };
}

test("only a known, different version counts as a new build", () => {
  const { ctx } = page();
  assert.equal(ctx.tix.buildChanged("v0.55.0", "v0.56.0"), true);
  assert.equal(ctx.tix.buildChanged("v0.55.0", "v0.55.0"), false);
  assert.equal(ctx.tix.buildChanged("", "v0.56.0"), false);
  assert.equal(ctx.tix.buildChanged("v0.55.0", undefined), false);
});

test("each thing a reload would lose blocks it", () => {
  const { ctx } = page();
  const doc = (selector = {}, fields = [], active = null) => {
    const d = fakeDocument({ bySelector: selector, bySelectorAll: { "input, textarea, select": fields } });
    d.activeElement = active;
    return d;
  };
  const block = ctx.tix.reloadBlocked;
  assert.equal(block(doc(), 0), "");
  assert.equal(block(doc({}, [], { tagName: "TEXTAREA" }), 0), "focus");
  assert.equal(block(doc({}, [], { tagName: "DIV", isContentEditable: true }), 0), "focus");
  assert.equal(block(doc({ "dialog[open]": {} }), 0), "dialog");
  assert.equal(block(doc({ ":popover-open": {} }), 0), "dialog");
  assert.equal(block(doc(), 1), "request");
  assert.equal(block(doc({ '.htmx-request, form[aria-busy="true"], .is-pending': {} }), 0), "request");
  assert.equal(block(doc({
    '[data-select-toggle][aria-pressed="true"]': {},
    "input[data-pick]:checked": {},
  }), 0), "selection");
  assert.equal(block(doc({}, [field("TEXTAREA", "half a comment", "")]), 0), "edited");
  assert.equal(block(doc({}, [field("INPUT", "C:\\fakepath\\a.png", "", "file")]), 0), "edited");
  assert.equal(block(doc({}, [field("INPUT", "set by a script", "", "hidden")]), 0), "",
    "a hidden input a script filled in is not the reader's work");
  assert.equal(block(doc({}, [field("INPUT", "same", "same")]), 0), "");
});

test("a new build on a quiet page reloads it and leaves a note for after", async () => {
  const { doc, reloads, fetched, store, appended } = page();
  doc.dispatch("tix:live-reconnected", {});
  await settle();
  assert.deepEqual(fetched, ["/healthz"]);
  assert.equal(reloads.length, 1, "the quiet page was not reloaded");
  assert.equal(appended.length, 0, "a banner was shown on a page that could just reload");
  assert.equal(store.data["tix.updatedTo"], "v0.56.0");
  assert.deepEqual(JSON.parse(store.data["tix.updatedScroll"]), { at: "/tasks?view=board", y: 120 });
});

test("the same build changes nothing", async () => {
  const { doc, reloads, appended } = page({ server: "v0.55.0" });
  doc.dispatch("tix:live-reconnected", {});
  await settle();
  assert.equal(reloads.length, 0);
  assert.equal(appended.length, 0);
});

test("a page with typing in it gets a banner, and reloads once the text is sent", async () => {
  const comment = field("TEXTAREA", "half a comment", "");
  const { doc, reloads, appended, timers } = page({ fields: [comment] });
  doc.dispatch("visibilitychange", {});
  await settle();
  assert.equal(reloads.length, 0, "the page reloaded over unsent text");
  assert.equal(appended.length, 1, "no banner offered the reload");
  assert.equal(appended[0].className, "deploybar");
  assert.equal(appended[0].hidden, false);

  doc.dispatch("focusout", {});
  timers.fire();
  assert.equal(reloads.length, 0, "leaving a field that still holds text reloaded the page");

  comment.defaultValue = comment.value;
  doc.dispatch("htmx:afterSettle", {});
  timers.fire();
  assert.equal(reloads.length, 1, "the page did not reload once nothing was left to lose");
});

test("the banner's Reload button reloads whatever the page holds", async () => {
  const { doc, reloads, appended } = page({ fields: [field("INPUT", "x", "")] });
  doc.dispatch("tix:live-reconnected", {});
  await settle();
  const reload = appended[0].kids.find((k) => k.className === "deploybar-reload");
  reload.listeners.click();
  assert.equal(reloads.length, 1);
});

test("after the reload the page names the version it now runs, once", () => {
  const store = fakeStore({ "tix.updatedTo": "v0.56.0" });
  const { appended } = page({ build: "v0.56.0", server: "v0.56.0", store });
  assert.equal(appended.length, 1);
  assert.equal(appended[0].className, "deploynote");
  assert.equal(appended[0].attrs.role, "status");
  assert.equal(appended[0].textContent, "Updated to v0.56.0");
  assert.equal(store.getItem("tix.updatedTo"), null, "the note would show again on the next load");
});

test("a page drawn without a build never asks", async () => {
  const { doc, fetched } = page({ build: null });
  doc.dispatch("tix:live-reconnected", {});
  await settle();
  assert.equal(fetched.length, 0);
});
