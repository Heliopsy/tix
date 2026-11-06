// SPDX-License-Identifier: AGPL-3.0-or-later

// The hand to an agent dialog's client tabs: assets/handoff.js.
import test from "node:test";
import assert from "node:assert/strict";
import { bareContext, run } from "./harness.mjs";

function loaded() {
  const ctx = bareContext();
  run(ctx, "handoff.js");
  return ctx.tix;
}

function memoryStorage() {
  const data = new Map();
  return { getItem: (k) => (data.has(k) ? data.get(k) : null), setItem: (k, v) => data.set(k, String(v)) };
}

const refusing = {
  getItem() { throw new Error("SecurityError"); },
  setItem() { throw new Error("QuotaExceededError"); },
};

// fakeNode is an element with attributes and a class list, and nothing else.
function fakeNode(attrs) {
  const a = { ...attrs };
  const classes = new Set();
  return {
    getAttribute: (k) => (k in a ? a[k] : null),
    setAttribute: (k, v) => { a[k] = String(v); },
    classList: { add: (c) => classes.add(c), remove: (c) => classes.delete(c), contains: (c) => classes.has(c) },
    attrs: a,
  };
}

function fakeDialog(ids) {
  const tabs = ids.map((id) => fakeNode({ "data-handoff-tab": id }));
  const panels = ids.map((id) => fakeNode({ "data-handoff-panel": id }));
  return {
    tabs, panels,
    querySelectorAll: (sel) => (sel === "[data-handoff-tab]" ? tabs : sel === "[data-handoff-panel]" ? panels : []),
  };
}

test("the chosen client is remembered and read back", () => {
  const tix = loaded();
  const store = memoryStorage();
  assert.equal(tix.handoffRemembered(store), "");
  tix.handoffRemember(store, "codex");
  assert.equal(tix.handoffRemembered(store), "codex");
});

test("storage that throws forgets instead of failing", () => {
  const tix = loaded();
  assert.doesNotThrow(() => tix.handoffRemember(refusing, "codex"));
  assert.equal(tix.handoffRemembered(refusing), "");
  assert.equal(tix.handoffRemembered(null), "");
});

test("selecting a client marks its tab and shows only its panel", () => {
  const tix = loaded();
  const dlg = fakeDialog(["claude-code", "codex", "zed"]);
  assert.equal(tix.handoffSelect(dlg, "codex"), true);
  assert.deepEqual(dlg.tabs.map((t) => t.attrs["aria-selected"]), ["false", "true", "false"]);
  assert.deepEqual(dlg.tabs.map((t) => t.attrs.tabindex), ["-1", "0", "-1"]);
  assert.deepEqual(dlg.panels.map((p) => p.classList.contains("is-current")), [false, true, false]);
});

test("a remembered client this dialog does not have changes nothing", () => {
  const tix = loaded();
  const dlg = fakeDialog(["claude-code", "codex"]);
  tix.handoffSelect(dlg, "claude-code");
  assert.equal(tix.handoffSelect(dlg, "gone"), false);
  assert.equal(dlg.tabs[0].attrs["aria-selected"], "true");
  assert.equal(dlg.panels[0].classList.contains("is-current"), true);
});

test("restore puts every dialog in scope on the remembered client", () => {
  const tix = loaded();
  const store = memoryStorage();
  tix.handoffRemember(store, "zed");
  const dlg = fakeDialog(["claude-code", "zed"]);
  tix.handoffRestore({ querySelectorAll: () => [dlg] }, store);
  assert.equal(dlg.tabs[1].attrs["aria-selected"], "true");
});

test("arrow keys wrap, Home and End go to the ends, other keys do nothing", () => {
  const tix = loaded();
  assert.equal(tix.handoffStep("ArrowRight", 7, 8), 0);
  assert.equal(tix.handoffStep("ArrowLeft", 0, 8), 7);
  assert.equal(tix.handoffStep("Home", 4, 8), 0);
  assert.equal(tix.handoffStep("End", 4, 8), 7);
  assert.equal(tix.handoffStep("a", 4, 8), -1);
});
