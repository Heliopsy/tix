// SPDX-License-Identifier: AGPL-3.0-or-later

// The settings navigation's scroll-spy: assets/tocspy.js. The observers, the
// smooth scroll and the focus move are driven in Chrome by
// TestSettingsNavigationFollowsTheScrollInABrowser.
import test from "node:test";
import assert from "node:assert/strict";
import { bareContext, run } from "./harness.mjs";

function loaded() {
  const ctx = bareContext();
  run(ctx, "tocspy.js");
  return ctx.tix;
}

test("the last section whose top has reached the band is the one in view", () => {
  const tix = loaded();
  assert.equal(tix.tocActive([-900, -300, 120, 700], 250, false), 2);
  assert.equal(tix.tocActive([-900, -300, 260, 700], 250, false), 1);
  assert.equal(tix.tocActive([-900, -300, 250, 700], 250, false), 2);
});

test("above the first section the first is in view", () => {
  const tix = loaded();
  assert.equal(tix.tocActive([300, 900, 1500], 250, false), 0);
});

test("at the end of the page the last section is in view, however short", () => {
  const tix = loaded();
  assert.equal(tix.tocActive([-900, -300, 120, 700], 250, true), 3);
});

test("no sections, no mark", () => {
  const tix = loaded();
  assert.equal(tix.tocActive([], 250, false), -1);
  assert.equal(tix.tocActive([], 250, true), -1);
});

test("only a plain main-button click is taken; a new tab or window is the browser's", () => {
  const tix = loaded();
  assert.equal(tix.tocTakesClick({ button: 0 }), true);
  assert.equal(tix.tocTakesClick({}), true);
  assert.equal(tix.tocTakesClick({ button: 1 }), false);
  for (const mod of ["metaKey", "ctrlKey", "shiftKey", "altKey"]) {
    assert.equal(tix.tocTakesClick({ button: 0, [mod]: true }), false, mod);
  }
  assert.equal(tix.tocTakesClick({ button: 0, defaultPrevented: true }), false);
});

test("without a document or IntersectionObserver it defines its helpers and does nothing else", () => {
  const ctx = bareContext({ document: { addEventListener() { throw new Error("must not listen"); } } });
  run(ctx, "tocspy.js");
  assert.equal(typeof ctx.tix.tocActive, "function");
});
