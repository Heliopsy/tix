// SPDX-License-Identifier: AGPL-3.0-or-later

// The inbox badge and @handle completion: the decisions in assets/decide.js
// that assets/live.js and assets/mention.js act on.
import test from "node:test";
import assert from "node:assert/strict";
import { newContext } from "./harness.mjs";

const { tix } = newContext();
const plain = (value) => JSON.parse(JSON.stringify(value));

test("the @word at the caret is found, an address is not", () => {
  assert.deepEqual(plain(tix.mentionAt("hi @Bo", 6)), { start: 3, query: "bo" });
  assert.deepEqual(plain(tix.mentionAt("@", 1)), { start: 0, query: "" });
  assert.equal(tix.mentionAt("mail me@exa", 11), null);
  assert.equal(tix.mentionAt("@bob done", 9), null);
  assert.deepEqual(plain(tix.mentionAt("(@ca", 4)), { start: 1, query: "ca" });
});

test("handles starting with the query come before those containing it", () => {
  const handles = ["carol", "bob", "abbot", "Bobby"];
  assert.deepEqual(plain(tix.mentionMatches(handles, "bo", 8)), ["bob", "Bobby", "abbot"]);
  assert.deepEqual(plain(tix.mentionMatches(handles, "", 2)), ["carol", "bob"]);
  assert.deepEqual(plain(tix.mentionMatches(handles, "zz", 8)), []);
});

test("choosing a handle replaces the @word and leaves the caret after a space", () => {
  const at = tix.mentionAt("ask @bo now", 7);
  assert.deepEqual(plain(tix.mentionInsert("ask @bo now", 7, at, "bob")), { value: "ask @bob now", caret: 9 });
  const end = tix.mentionAt("@c", 2);
  assert.deepEqual(plain(tix.mentionInsert("@c", 2, end, "carol")), { value: "@carol ", caret: 7 });
});

function mark(attrs) {
  const node = {
    attrs: { ...attrs },
    innerHTML: "",
    getAttribute(n) { return n in this.attrs ? this.attrs[n] : null; },
    setAttribute(n, v) { this.attrs[n] = String(v); },
    removeAttribute(n) { delete this.attrs[n]; },
  };
  return node;
}

test("a fresh badge carries its count and shows or hides every mark", () => {
  const here = mark({ "data-inbox-count": "0", hidden: "hidden" });
  const dot = mark({ hidden: "hidden" });
  const fresh = mark({ "data-inbox-count": "3" });
  fresh.innerHTML = "3";
  tix.applyBadge(here, fresh, [dot]);
  assert.equal(here.innerHTML, "3");
  assert.equal(here.getAttribute("data-inbox-count"), "3");
  assert.equal(here.getAttribute("hidden"), null);
  assert.equal(dot.getAttribute("hidden"), null);

  const none = mark({ "data-inbox-count": "0" });
  tix.applyBadge(here, none, [dot]);
  assert.equal(here.getAttribute("hidden"), "hidden");
  assert.equal(dot.getAttribute("hidden"), "hidden");
});
