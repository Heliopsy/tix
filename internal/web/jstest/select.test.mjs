// SPDX-License-Identifier: AGPL-3.0-or-later

// Bulk selection on the task list: assets/select.js.
import test from "node:test";
import assert from "node:assert/strict";
import { newContext, run } from "./harness.mjs";

function loaded() {
  const ctx = newContext();
  run(ctx, "select.js");
  return ctx;
}

test("shift-click ticks every box between the anchor and the click", () => {
  const { tix } = loaded();
  assert.deepEqual([...tix.pickRange(1, 4, 6)], [1, 2, 3, 4]);
  assert.deepEqual([...tix.pickRange(4, 1, 6)], [1, 2, 3, 4]);
});

test("no anchor, or one that left the list, is the clicked box alone", () => {
  const { tix } = loaded();
  assert.deepEqual([...tix.pickRange(-1, 2, 6)], [2]);
  assert.deepEqual([...tix.pickRange(9, 2, 6)], [2]);
  assert.deepEqual([...tix.pickRange(0, 7, 6)], []);
});

test("the count says one task or many", () => {
  const { tix } = loaded();
  assert.equal(tix.pickCountText(1), "1 task selected");
  assert.equal(tix.pickCountText(4), "4 tasks selected");
  const bg = { "%d task selected": "%d избрана задача", "%d tasks selected": "%d избрани задачи" };
  assert.equal(tix.pickCountText(3, (s) => bg[s]), "3 избрани задачи");
});

function within(selector, tagName = "LI") {
  return {
    tagName,
    closest(sel) {
      return sel.split(",").some((s) => s.trim() === selector) ? this : null;
    },
  };
}

test("select all belongs to the list only while focus is in it", () => {
  const { tix } = loaded();
  assert.equal(tix.pickAllApplies(within("[data-shortcut-row]")), true);
  assert.equal(tix.pickAllApplies(within(".tasklist", "UL")), true);
  assert.equal(tix.pickAllApplies(within(".elsewhere", "DIV")), false);
  assert.equal(tix.pickAllApplies({ tagName: "INPUT", type: "text", closest: () => ({}) }), false);
  assert.equal(tix.pickAllApplies(null), false);
});

test("Cmd+A is its own chord and never reaches the page as a bare a", () => {
  const { tix } = loaded();
  assert.equal(tix.comboOf({ key: "a", metaKey: true }), "meta+a");
  assert.equal(tix.comboOf({ key: "a", ctrlKey: true }), "ctrl+a");
  assert.equal(tix.comboOf({ key: "a" }), "a");
});
