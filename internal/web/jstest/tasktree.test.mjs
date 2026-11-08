// SPDX-License-Identifier: AGPL-3.0-or-later

// The subtask tree's fold buttons: assets/tasktree.js.
import test from "node:test";
import assert from "node:assert/strict";
import { bareContext, run } from "./harness.mjs";

function loaded() {
  const ctx = bareContext();
  run(ctx, "tasktree.js");
  return ctx.tix;
}

test("an open node folds and a folded one opens", () => {
  const tix = loaded();
  assert.equal(tix.treeFold(true, true), "collapse");
  assert.equal(tix.treeFold(false, true), "expand");
});

test("a node whose subtasks were never drawn loads them", () => {
  const tix = loaded();
  assert.equal(tix.treeFold(false, false), "load");
  assert.equal(tix.treeFold(true, false), "load");
});
