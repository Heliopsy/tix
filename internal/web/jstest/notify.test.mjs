// SPDX-License-Identifier: AGPL-3.0-or-later

// Browser notifications and settings search: the pure parts of
// assets/notify.js and assets/settingsfind.js.
import test from "node:test";
import assert from "node:assert/strict";
import { newContext, run } from "./harness.mjs";

const ctx = newContext();
run(ctx, "notify.js");
run(ctx, "settingsfind.js");
const { tix } = ctx;
const plain = (value) => JSON.parse(JSON.stringify(value));

test("a notification names who did what to which task", () => {
  const item = { id: "1", kind: "status", actor_handle: "carol", from_status: "todo", to_status: "doing",
    task_ref: "infra-4", task_title: "cut over" };
  assert.deepEqual(plain(tix.notifyContent(item, false)), { title: "carol moved it from todo to doing", body: "INFRA-4 cut over" });
  assert.equal(tix.notifyContent({ id: "2", kind: "mention" }, false).title, "somebody mentioned you");
});

test("a hidden notification says only that something arrived", () => {
  const item = { id: "1", kind: "mention", actor_handle: "carol", task_ref: "infra-4", task_title: "secret plan" };
  const c = plain(tix.notifyContent(item, true));
  assert.deepEqual(c, { title: "tix", body: "Something new reached your inbox." });
});

test("only items not seen before are shown, oldest first", () => {
  const items = [{ id: "c" }, { id: "b" }, { id: "a" }];
  assert.deepEqual(plain(tix.notifyFresh(items, ["a"])).map((i) => i.id), ["b", "c"]);
  assert.deepEqual(plain(tix.notifyFresh(items, ["a", "b", "c"])), []);
});

test("a setting matches every word typed, in any order and case", () => {
  assert.equal(tix.settingMatches("Reduce motion Keep tix still", "motion reduce"), true);
  assert.equal(tix.settingMatches("Reduce motion", "MOTION"), true);
  assert.equal(tix.settingMatches("Reduce motion", "motion hover"), false);
  assert.equal(tix.settingMatches("Café", "cafe"), true);
  assert.equal(tix.settingMatches("anything", "  "), true);
});
