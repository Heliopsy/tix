// SPDX-License-Identifier: AGPL-3.0-or-later

// The scope picker: assets/scopes.js.
import test from "node:test";
import assert from "node:assert/strict";
import { bareContext, run } from "./harness.mjs";

function loaded() {
  const ctx = bareContext();
  run(ctx, "scopes.js");
  return ctx.tix;
}

function boxes(spec) {
  return spec.map(([value, checked, disabled]) => ({ value, checked: !!checked, disabled: !!disabled }));
}

const ticked = (list) => list.filter((b) => b.checked).map((b) => b.value);

test("select all ticks every scope the reader holds and nothing disabled", () => {
  const tix = loaded();
  const list = boxes([["task:read"], ["task:write", false, true], ["comment:write"]]);
  tix.scopeSet(list, null);
  assert.deepEqual(ticked(list), ["task:read", "comment:write"]);
});

test("a preset ticks exactly its scopes and clears the rest", () => {
  const tix = loaded();
  const list = boxes([["task:read", true], ["task:write"], ["audit:read", true], ["export", true, true]]);
  tix.scopeSet(list, ["task:write"]);
  assert.deepEqual(ticked(list), ["task:write", "export"], "a disabled box keeps its state, which the browser never posts");
});

test("clear unticks every enabled box", () => {
  const tix = loaded();
  const list = boxes([["task:read", true], ["task:write", true]]);
  tix.scopeSet(list, []);
  assert.deepEqual(ticked(list), []);
});

test("a group reads all, some or none, counting only boxes the reader may tick", () => {
  const tix = loaded();
  assert.equal(tix.scopeGroupState(boxes([["a", true], ["b", false, true]])), "all");
  assert.equal(tix.scopeGroupState(boxes([["a", true], ["b"]])), "some");
  assert.equal(tix.scopeGroupState(boxes([["a"], ["b"]])), "none");
  assert.equal(tix.scopeGroupState(boxes([["a", false, true]])), "none");
});

test("the summary names what is ticked, or says a scope is needed", () => {
  const tix = loaded();
  assert.equal(tix.scopeSummary(["task:read", "task:claim"]), "This token can use: task:read, task:claim");
  assert.match(tix.scopeSummary([]), /at least one scope/);
});
