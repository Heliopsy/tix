// SPDX-License-Identifier: AGPL-3.0-or-later

// Recently viewed tasks in the sidebar: assets/recent.js.
import test from "node:test";
import assert from "node:assert/strict";
import { bareContext, run } from "./harness.mjs";

function memoryStorage(seed = {}) {
  const data = new Map(Object.entries(seed));
  return {
    getItem: (k) => (data.has(k) ? data.get(k) : null),
    setItem: (k, v) => data.set(k, String(v)),
    removeItem: (k) => data.delete(k),
    data,
  };
}

function loaded(extra = {}) {
  const ctx = bareContext(extra);
  run(ctx, "recent.js");
  return ctx;
}

test("a task opened again moves to the front, once", () => {
  const { tix } = loaded();
  let list = [];
  for (const ref of ["a-1", "a-2", "a-3"]) list = tix.recentAdd(list, { ref, title: ref, at: "" });
  list = tix.recentAdd(list, { ref: "a-1", title: "renamed", at: "" });
  assert.deepEqual(JSON.parse(JSON.stringify(list.map((e) => e.ref))), ["a-1", "a-3", "a-2"]);
  assert.equal(list[0].title, "renamed");
});

test("the list keeps the last ten", () => {
  const { tix } = loaded();
  let list = [];
  for (let i = 1; i <= 12; i++) list = tix.recentAdd(list, { ref: "a-" + i, title: "t", at: "" });
  assert.equal(list.length, 10);
  assert.equal(list[0].ref, "a-12");
  assert.equal(list[9].ref, "a-3");
});

test("a damaged store reads as empty, and bad entries are dropped", () => {
  const { tix } = loaded();
  assert.equal(tix.recentParse("not json").length, 0);
  assert.equal(tix.recentParse('{"ref":"a-1"}').length, 0);
  const got = tix.recentParse('[{"ref":"a-1","title":"x"},{"title":"no ref"},null,{"ref":"a-2","title":"y","at":"2026-11-01T10:00:00Z"}]');
  assert.deepEqual(JSON.parse(JSON.stringify(got)), [
    { ref: "a-1", title: "x", at: "" },
    { ref: "a-2", title: "y", at: "2026-11-01T10:00:00Z" },
  ]);
});

test("recentTasks reads the signed-in reader's own list, newest first", () => {
  const localStorage = memoryStorage({
    "tix.recent.t1:alice": '[{"ref":"infra-2","title":"Deploy","at":"x"},{"ref":"infra-1","title":"Plan","at":"y"}]',
    "tix.recent.t1:bob": '[{"ref":"ops-9","title":"Not alice","at":"z"}]',
  });
  const document = { querySelector: (sel) => (sel === "[data-recent-for]" ? { getAttribute: () => "t1:alice" } : null) };
  const { tix } = loaded({ localStorage, document });
  const got = JSON.parse(JSON.stringify(tix.recentTasks()));
  assert.deepEqual(got, [
    { ref: "infra-2", title: "Deploy", href: "/tasks/infra-2", at: "x" },
    { ref: "infra-1", title: "Plan", href: "/tasks/infra-1", at: "y" },
  ]);
});

test("each reader has a key of their own", () => {
  const { tix } = loaded();
  assert.notEqual(tix.recentKey("t1:alice"), tix.recentKey("t1:bob"));
});
