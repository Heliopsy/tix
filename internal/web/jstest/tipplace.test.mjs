// SPDX-License-Identifier: AGPL-3.0-or-later

// assets/tipplace.js: where an open (i) note goes so it stays inside the
// window. That the page stops scrolling sideways with a note open is the
// screenshot gate's (just ui-check), which reads real layout.
import test from "node:test";
import assert from "node:assert/strict";
import { newContext, run } from "./harness.mjs";

function load() {
  const ctx = newContext();
  run(ctx, "tipplace.js");
  return ctx.tix;
}

test("tipFit moves a note only as far as it has to", () => {
  const tix = load();
  for (const [name, left, right, width, want] of [
    ["fits", 20, 300, 390, { shift: 0, width: 0 }],
    ["runs past the right edge", 200, 520, 390, { shift: -138, width: 0 }],
    ["starts left of the gutter", -30, 200, 390, { shift: 38, width: 0 }],
    ["touches the gutter exactly", 62, 382, 390, { shift: 0, width: 0 }],
    ["wider than the window", 100, 600, 390, { shift: -92, width: 374 }],
  ]) {
    assert.deepEqual({ ...tix.tipFit(left, right, width, 8) }, want, name);
  }
});

test("tipFit never moves a note off the left edge to fit the right", () => {
  const tix = load();
  const fit = tix.tipFit(300, 800, 390, 8);
  assert.equal(300 + fit.shift, 8);
  assert.equal(fit.width, 374);
});
