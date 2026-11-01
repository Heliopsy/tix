// SPDX-License-Identifier: AGPL-3.0-or-later

// assets/markdown.js: the Preview toggle, and decide.js's tick answer for a
// checklist item feel.js draws before the server replies. The DOM is a stub.
import test from "node:test";
import assert from "node:assert/strict";
import { bareContext, newContext, run } from "./harness.mjs";

function load() {
  const ctx = bareContext({});
  run(ctx, "markdown.js");
  return ctx.tixMarkdown;
}

test("the toggle swaps between Preview and Write", () => {
  const md = load();
  assert.deepEqual({ ...md.nextMode(false) }, { previewing: true, label: "Write" });
  assert.deepEqual({ ...md.nextMode(true) }, { previewing: false, label: "Preview" });
});

test("reveal unhides every toggle, which a page without a script never shows", () => {
  const md = load();
  const buttons = [{ hidden: true }, { hidden: true }];
  md.reveal({ querySelectorAll: (sel) => (sel === "[data-md-preview]" ? buttons : []) });
  assert.deepEqual(buttons.map((b) => b.hidden), [false, false]);
});

test("a checklist item says the opposite of what it said while its post is out", () => {
  const ctx = newContext();
  assert.equal(ctx.tix.tickedNext("false"), "true");
  assert.equal(ctx.tix.tickedNext("true"), "false");
  assert.equal(ctx.tix.tickedNext(null), "true");
});
