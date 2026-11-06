// SPDX-License-Identifier: AGPL-3.0-or-later

// Where a timestamp's tip goes: assets/timetip.js.
import test from "node:test";
import assert from "node:assert/strict";
import { bareContext, run } from "./harness.mjs";

function place(rect, width) {
  const ctx = bareContext();
  run(ctx, "timetip.js");
  return ctx.tix.timeTipPlace(rect, width, 352, 16);
}

test("a tip under a timestamp at the left starts where the timestamp does", () => {
  const p = place({ left: 300, bottom: 100 }, 1920);
  assert.equal(p.left, 300);
  assert.equal(p.top, 104);
  assert.equal(p.width, 352);
});

test("a tip for a timestamp at the right edge moves left to stay on screen", () => {
  const p = place({ left: 330, bottom: 50 }, 390);
  assert.equal(p.width, 352);
  assert.equal(p.left, 22);
  assert.ok(p.left + p.width <= 390 - 16);
});

test("on a screen narrower than the widest tip, the tip fills it between the gutters", () => {
  const p = place({ left: 200, bottom: 10 }, 320);
  assert.equal(p.width, 288);
  assert.equal(p.left, 16);
});

test("a tip never starts inside the gutter", () => {
  const p = place({ left: 2, bottom: 10 }, 1000);
  assert.equal(p.left, 16);
});
