// SPDX-License-Identifier: AGPL-3.0-or-later

// Sharing a task: assets/share.js. The form cases are the ones in
// internal/tasklink/tasklink_test.go, so the browser and the command line
// never disagree about what a shared link says.
import test from "node:test";
import assert from "node:assert/strict";
import { bareContext, run } from "./harness.mjs";

function loaded() {
  const ctx = bareContext();
  run(ctx, "share.js");
  return ctx.tix;
}

const forms = [
  ["plain", "INFRA-12", "Fix the build", "https://tix.example.com", "/tasks/INFRA-12",
    "https://tix.example.com/tasks/INFRA-12", "INFRA-12 Fix the build", "[INFRA-12 Fix the build](https://tix.example.com/tasks/INFRA-12)"],
  ["trailing slash", "infra-12", "x", "https://tix.example.com/", "/tasks/infra-12",
    "https://tix.example.com/tasks/infra-12", "infra-12 x", "[infra-12 x](https://tix.example.com/tasks/infra-12)"],
  ["brackets", "INFRA-3", "see [docs] \\ here", "http://h:8080", "/tasks/INFRA-3",
    "http://h:8080/tasks/INFRA-3", "INFRA-3 see [docs] \\ here", "[INFRA-3 see \\[docs\\] \\\\ here](http://h:8080/tasks/INFRA-3)"],
  ["lines", "INFRA-4", "one\n  two\tthree ", "https://h", "/tasks/INFRA-4",
    "https://h/tasks/INFRA-4", "INFRA-4 one two three", "[INFRA-4 one two three](https://h/tasks/INFRA-4)"],
  ["no title", "INFRA-5", "  ", "https://h", "/tasks/INFRA-5", "https://h/tasks/INFRA-5", "INFRA-5", "[INFRA-5](https://h/tasks/INFRA-5)"],
];

for (const [name, ref, title, origin, path, url, text, markdown] of forms) {
  test(`share forms: ${name}`, () => {
    const tix = loaded();
    const task = { ref, title, path };
    assert.equal(tix.shareValue("link", task, origin), url);
    assert.equal(tix.shareValue("text", task, origin), text);
    assert.equal(tix.shareValue("markdown", task, origin), markdown);
  });
}

test("shareURL widens only a path of this origin", () => {
  const tix = loaded();
  assert.equal(tix.shareURL("//evil.example/x", "https://h"), "//evil.example/x");
  assert.equal(tix.shareURL("https://other/x", "https://h"), "https://other/x");
  assert.equal(tix.shareURL("/tasks/a-1", ""), "/tasks/a-1");
  assert.equal(tix.shareURL(undefined, "https://h"), "");
});

test("shareValue copies nothing for a kind it does not know", () => {
  const tix = loaded();
  assert.equal(tix.shareValue("native", { ref: "A-1", path: "/tasks/a-1" }, "https://h"), "");
  assert.equal(tix.shareValue("link", null, "https://h"), "");
});

test("shareMessage names what was copied, or that it failed", () => {
  const tix = loaded();
  assert.equal(tix.shareMessage("link", true), "Link copied");
  assert.equal(tix.shareMessage("markdown", true), "Markdown link copied");
  assert.equal(tix.shareMessage("text", true), "Key and title copied");
  assert.equal(tix.shareMessage("markdown", false), "Copy failed");
});

test("shareCanNative is true only for a navigator with share", () => {
  const tix = loaded();
  assert.equal(tix.shareCanNative({ share() {} }), true);
  assert.equal(tix.shareCanNative({}), false);
  assert.equal(tix.shareCanNative({ share: "no" }), false);
  assert.equal(tix.shareCanNative(undefined), false);
});

const task = { ref: "INFRA-12", title: "Fix the build", path: "/tasks/infra-12" };

test("shareNative hands the sheet the title and the absolute address", async () => {
  const tix = loaded();
  let shared = null;
  let fell = 0;
  tix.shareNative({ share: (d) => { shared = d; return Promise.resolve(); } }, task, "https://h", () => fell++);
  await new Promise((r) => setImmediate(r));
  assert.deepEqual({ ...shared }, { title: "INFRA-12 Fix the build", url: "https://h/tasks/infra-12" });
  assert.equal(fell, 0);
});

test("a dismissed sheet does nothing, a refused one copies the link", async () => {
  const tix = loaded();
  const outcomes = [];
  const abort = Object.assign(new Error("cancelled"), { name: "AbortError" });
  const denied = Object.assign(new Error("denied"), { name: "NotAllowedError" });
  tix.shareNative({ share: () => Promise.reject(abort) }, task, "https://h", () => outcomes.push("abort fell back"));
  tix.shareNative({ share: () => Promise.reject(denied) }, task, "https://h", () => outcomes.push("denied"));
  tix.shareNative({ share: () => { throw denied; } }, task, "https://h", () => outcomes.push("threw"));
  await new Promise((r) => setImmediate(r));
  assert.deepEqual(outcomes.sort(), ["denied", "threw"]);
});

test("shareOrigin prefers the public address the server drew", () => {
  const tix = loaded();
  const root = (value) => ({ getAttribute: (name) => (name === "data-share-origin" ? value : null) });
  assert.equal(tix.shareOrigin(root("https://tix.example.com"), "http://tix.lan:8080"), "https://tix.example.com");
  assert.equal(tix.shareOrigin(root(null), "http://tix.lan:8080"), "http://tix.lan:8080");
  assert.equal(tix.shareOrigin(root(""), "http://tix.lan:8080"), "http://tix.lan:8080");
  assert.equal(tix.shareOrigin(null, "http://tix.lan:8080"), "http://tix.lan:8080");
});
