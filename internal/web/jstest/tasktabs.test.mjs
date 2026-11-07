// SPDX-License-Identifier: AGPL-3.0-or-later

// The activity tabs on a task screen: assets/tasktabs.js. The switching
// itself, focus and the live update are driven in Chrome by
// TestTaskTabsSwitchRememberAndSurviveALiveUpdateInABrowser.
import test from "node:test";
import assert from "node:assert/strict";
import { bareContext, run } from "./harness.mjs";

function loaded() {
  const ctx = bareContext();
  run(ctx, "tasktabs.js");
  return ctx.tix;
}

const names = ["comments", "history", "artifacts"];

test("a fragment beats a forced tab, which beats the remembered one and the default", () => {
  const tix = loaded();
  assert.equal(tix.taskTabPick(names, "artifacts", "history", "comments", "comments"), "artifacts");
  assert.equal(tix.taskTabPick(names, "", "history", "artifacts", "comments"), "history");
  assert.equal(tix.taskTabPick(names, "", "", "artifacts", "comments"), "artifacts");
  assert.equal(tix.taskTabPick(names, "", "", "", "comments"), "comments");
});

test("a remembered tab that no longer exists falls back to the default", () => {
  const tix = loaded();
  assert.equal(tix.taskTabPick(names, "", "", "attachments", "comments"), "comments");
  assert.equal(tix.taskTabPick(names, "", "", "", ""), "comments");
});

test("the arrow keys wrap, Home and End go to the ends, other keys do nothing", () => {
  const tix = loaded();
  assert.equal(tix.taskTabStep("ArrowRight", 2, 3), 0);
  assert.equal(tix.taskTabStep("ArrowLeft", 0, 3), 2);
  assert.equal(tix.taskTabStep("ArrowRight", 0, 3), 1);
  assert.equal(tix.taskTabStep("Home", 2, 3), 0);
  assert.equal(tix.taskTabStep("End", 0, 3), 2);
  assert.equal(tix.taskTabStep("Enter", 1, 3), -1);
  assert.equal(tix.taskTabStep("ArrowDown", 1, 3), -1);
});
