// SPDX-License-Identifier: AGPL-3.0-or-later

// The Connect an agent screen: assets/connect.js.
import test from "node:test";
import assert from "node:assert/strict";
import { bareContext, run } from "./harness.mjs";

function loaded() {
  const ctx = bareContext();
  run(ctx, "connect.js");
  return ctx.tix;
}

test("an issued token is written into every slot", () => {
  const tix = loaded();
  const slots = [{ textContent: "$TIX_TOKEN" }, { textContent: "$TIX_TOKEN" }];
  assert.equal(tix.fillTokenSlots(slots, "tix_abc"), 2);
  assert.deepEqual(slots.map((s) => s.textContent), ["tix_abc", "tix_abc"]);
});

test("with no token the slots keep $TIX_TOKEN", () => {
  const tix = loaded();
  const slots = [{ textContent: "$TIX_TOKEN" }];
  assert.equal(tix.fillTokenSlots(slots, ""), 0);
  assert.equal(slots[0].textContent, "$TIX_TOKEN");
});
