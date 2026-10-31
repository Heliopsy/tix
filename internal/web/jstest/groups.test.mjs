// SPDX-License-Identifier: AGPL-3.0-or-later

// Heading menus on columns, lanes and list groups: assets/groups.js.
import test from "node:test";
import assert from "node:assert/strict";
import { bareContext, run } from "./harness.mjs";

function loaded() {
  const ctx = bareContext();
  run(ctx, "groups.js");
  return ctx;
}

test("focus returns to the same group's dots after a swap", () => {
  const { tix } = loaded();
  assert.equal(tix.groupFocusSelector({ key: "todo", hides: false }), '[data-group-btn="todo"]');
});

test("focus goes to the Hidden control after a hide", () => {
  const { tix } = loaded();
  assert.equal(tix.groupFocusSelector({ key: "todo", hides: true }), "[data-group-hidden]");
});

test("a key is escaped before it reaches a selector", () => {
  const { tix } = loaded();
  const escape = (v) => v.replace(/"/g, '\\"');
  assert.equal(tix.groupFocusSelector({ key: 'a"b', hides: false }, escape), '[data-group-btn="a\\"b"]');
});

test("nothing remembered is nothing to focus", () => {
  const { tix } = loaded();
  assert.equal(tix.groupFocusSelector(null), "");
});

test("the arrow keys walk a menu and wrap", () => {
  const { tix } = loaded();
  const cases = [
    [4, -1, "ArrowDown", 0],
    [4, -1, "ArrowUp", 3],
    [4, 3, "ArrowDown", 0],
    [4, 0, "ArrowUp", 3],
    [4, 1, "Home", 0],
    [4, 1, "End", 3],
    [4, 1, "Enter", -1],
    [0, -1, "ArrowDown", -1],
  ];
  for (const [count, index, key, want] of cases) {
    assert.equal(tix.groupMenuStep(count, index, key), want, `${key} from ${index} of ${count}`);
  }
});
