// SPDX-License-Identifier: AGPL-3.0-or-later

// What the project icon picker previews: assets/iconpick.js.
import test from "node:test";
import assert from "node:assert/strict";
import { newContext, bareContext, run } from "./harness.mjs";

const plain = (value) => JSON.parse(JSON.stringify(value));

function preview(state) {
  const ctx = newContext();
  run(ctx, "iconpick.js");
  return plain(ctx.tix.iconPreview(state));
}

test("iconpick.js stands down without decide.js", () => {
  const ctx = bareContext();
  run(ctx, "iconpick.js");
  assert.equal(ctx.tix, undefined);
});

test("iconPreview shows the kind that is checked", () => {
  const cases = [
    [{ kind: "text", text: " W " }, { kind: "text", value: "W" }],
    [{ kind: "builtin", name: "rocket", text: "W" }, { kind: "builtin", value: "rocket" }],
    [{ kind: "image", fileURL: "blob:new", current: "/projects/a/icon" }, { kind: "image", value: "blob:new" }],
    [{ kind: "image", current: "/projects/a/icon" }, { kind: "image", value: "/projects/a/icon" }],
    [{ kind: "none", text: "W", name: "rocket", current: "/x" }, { kind: "none", value: "" }],
  ];
  for (const [state, want] of cases) {
    assert.deepEqual(preview(state), want, JSON.stringify(state));
  }
});

test("iconPreview shows nothing for a kind with nothing chosen", () => {
  for (const state of [{ kind: "text", text: "  " }, { kind: "builtin" }, { kind: "image" }, {}, undefined]) {
    assert.deepEqual(preview(state), { kind: "none", value: "" }, JSON.stringify(state));
  }
});
