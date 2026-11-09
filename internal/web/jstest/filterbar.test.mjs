// SPDX-License-Identifier: AGPL-3.0-or-later

// The filter box's suggestions: which word the caret is in, what it
// completes to, what the box becomes, and what each key does. assets/
// filterbar.js only draws what these decide.
import test from "node:test";
import assert from "node:assert/strict";
import { newContext, bareContext, run } from "./harness.mjs";

const ctx = newContext();
const tix = ctx.tix;
const plain = (value) => JSON.parse(JSON.stringify(value));

const data = {
  keys: ["project", "tag", "assignee", "status", "priority", "due", "is"],
  project: [{ k: "fidaro", n: "Fidaro rollout" }, { k: "infra", n: "Infrastructure" }],
  tag: ["ops", "chore", "two words"],
  person: ["raj", "rajesh", "lena"],
  status: ["todo", "doing", "done"],
  priority: ["highest", "high", "normal", "low", "lowest"],
  due: ["overdue", "today", "week", "month"],
  is: ["starred", "blocked"],
};

test("filterTokenAt finds the word under the caret and its key", () => {
  assert.deepEqual(plain(tix.filterTokenAt("tag:ops pro", 11)), { start: 8, end: 11, neg: false, key: "", typed: "pro" });
  assert.deepEqual(plain(tix.filterTokenAt("-@ra", 4)), { start: 0, end: 4, neg: true, key: "@", typed: "ra" });
  assert.deepEqual(plain(tix.filterTokenAt("p:fi tag:ops", 2)), { start: 0, end: 4, neg: false, key: "project", typed: "fi" });
  assert.deepEqual(plain(tix.filterTokenAt("Status:do", 9)), { start: 0, end: 9, neg: false, key: "status", typed: "do" });
  assert.equal(tix.filterTokenAt('title:"two wo', 13), null, "inside an open quote is free text");
});

test("a bare word completes to a key, not to nothing", () => {
  const got = plain(tix.filterSuggest(data, tix.filterTokenAt("pr", 2)));
  assert.deepEqual(got.map((s) => s.insert), ["project:", "priority:"]);
  assert.deepEqual(plain(tix.filterSuggest(data, tix.filterTokenAt("", 0))), [], "an empty box suggests nothing");
});

test("values come from the tenant's data", () => {
  const people = plain(tix.filterSuggest(data, tix.filterTokenAt("@ra", 3))).map((s) => s.insert);
  assert.deepEqual(people, ["@raj", "@rajesh"]);
  const projects = plain(tix.filterSuggest(data, tix.filterTokenAt("project:roll", 12)));
  assert.deepEqual(projects.map((s) => s.insert), ["project:fidaro"], "a project matches by a word of its name");
  assert.equal(projects[0].note, "Fidaro rollout");
  const tags = plain(tix.filterSuggest(data, tix.filterTokenAt("tag:two", 7))).map((s) => s.insert);
  assert.deepEqual(tags, ['tag:"two words"'], "a value with a space is quoted");
  const due = plain(tix.filterSuggest(data, tix.filterTokenAt("due:w", 5))).map((s) => s.insert);
  assert.deepEqual(due, ["due:week"]);
  const me = plain(tix.filterSuggest(data, tix.filterTokenAt("assignee:m", 10))).map((s) => s.insert);
  assert.deepEqual(me, ["assignee:me"]);
  assert.deepEqual(plain(tix.filterSuggest(data, tix.filterTokenAt("title:x", 7))), [], "a key with no list suggests nothing");
});

test("a word that is already its only suggestion gets none, so Enter applies it", () => {
  assert.deepEqual(plain(tix.filterSuggest(data, tix.filterTokenAt("due:week", 8))), []);
  assert.deepEqual(plain(tix.filterSuggest(data, tix.filterTokenAt("@lena", 5))), []);
});

test("filterInsert replaces the word, keeps its minus and places the caret", () => {
  const tok = tix.filterTokenAt("tag:ops -@ra due:week", 12);
  assert.deepEqual(plain(tix.filterInsert("tag:ops -@ra due:week", tok, "@raj")), { value: "tag:ops -@raj due:week", caret: 14 });
  const key = tix.filterTokenAt("pro", 3);
  assert.deepEqual(plain(tix.filterInsert("pro", key, "project:")), { value: "project:", caret: 8 }, "no space after a bare key");
  const end = tix.filterTokenAt("project:fi", 10);
  assert.deepEqual(plain(tix.filterInsert("project:fi", end, "project:fidaro")), { value: "project:fidaro ", caret: 15 });
});

test("the keys: arrows move, Enter takes only a reached option, Tab takes, Escape closes", () => {
  assert.deepEqual(plain(tix.filterKey("Enter", true, 3, -1)), { act: "none" }, "Enter with nothing reached applies the filter");
  assert.deepEqual(plain(tix.filterKey("ArrowDown", true, 3, -1)), { act: "move", index: 0 });
  assert.deepEqual(plain(tix.filterKey("ArrowDown", true, 3, 2)), { act: "move", index: 0 }, "Down wraps");
  assert.deepEqual(plain(tix.filterKey("ArrowUp", true, 3, 0)), { act: "move", index: 2 }, "Up wraps");
  assert.deepEqual(plain(tix.filterKey("Enter", true, 3, 1)), { act: "take", index: 1 });
  assert.deepEqual(plain(tix.filterKey("Tab", true, 3, -1)), { act: "take", index: 0 });
  assert.deepEqual(plain(tix.filterKey("Escape", true, 3, 1)), { act: "close" });
  assert.deepEqual(plain(tix.filterKey("Escape", false, 0, -1)), { act: "none" }, "Escape with the list shut is not ours");
  assert.deepEqual(plain(tix.filterKey("Tab", false, 0, -1)), { act: "none" }, "Tab with the list shut moves focus");
  assert.deepEqual(plain(tix.filterKey("a", true, 3, 1)), { act: "none" });
});

test("filterParams leaves out empty fields and pickers nobody touched", () => {
  const pairs = [
    ["q", "tag:ops"], ["due", ""], ["pick_status", "todo"], ["was_status", "todo"],
    ["pick_priority", "high"], ["was_priority", "low"], ["pick_assignee", ""], ["was_assignee", "raj"],
    ["pick_tag", ""], ["sort", "title"], ["limit", ""],
  ];
  assert.deepEqual(plain(tix.filterParams(pairs)), [
    ["q", "tag:ops"], ["pick_priority", "high"], ["was_priority", "low"],
    ["pick_assignee", ""], ["was_assignee", "raj"], ["sort", "title"],
  ]);
  assert.deepEqual(plain(tix.filterParams([["pick_tag", "ops"]])), [["pick_tag", "ops"]], "a typed tag is a change");
});

test("filterbar.js stands down without its decisions", () => {
  const listeners = [];
  const bare = bareContext({ document: { addEventListener: (...a) => listeners.push(a) } });
  run(bare, "filterbar.js");
  assert.equal(listeners.length, 0);
});
