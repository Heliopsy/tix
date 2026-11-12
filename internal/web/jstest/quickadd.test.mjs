// SPDX-License-Identifier: AGPL-3.0-or-later

// The quick-add box: which word completes, to what, how a value is written so
// the quick-add parser reads it, the clipboard's first line, and the
// ctrl+shift chord its shortcut is bound to. assets/filterbar.js and
// assets/quickadd.js only draw and wire what these decide.
import test from "node:test";
import assert from "node:assert/strict";
import { newContext, bareContext, run } from "./harness.mjs";

const ctx = newContext();
const tix = ctx.tix;
const plain = (value) => JSON.parse(JSON.stringify(value));

const data = {
  project: [{ k: "infra", n: "Infrastructure" }, { k: "web", n: "Website" }],
  tag: ["backend", "backlog", "c++", "two words", "ops"],
  person: ["raj", "rajesh", "lena", "ann-"],
};

const inserts = (line, caret = line.length) =>
  plain(tix.quickSuggest(data, tix.quickTokenAt(line, caret))).map((s) => s.insert);

test("quickTokenAt finds a word opening with #, @ or +", () => {
  assert.deepEqual(plain(tix.quickTokenAt("Fix login #ba", 13)), { start: 10, end: 13, neg: false, key: "#", typed: "ba" });
  assert.deepEqual(plain(tix.quickTokenAt("@ra fix", 2)), { start: 0, end: 3, neg: false, key: "@", typed: "ra" });
  assert.deepEqual(plain(tix.quickTokenAt("Fix +in", 7)), { start: 4, end: 7, neg: false, key: "+", typed: "in" });
  assert.deepEqual(plain(tix.quickTokenAt("Fix #", 5)), { start: 4, end: 5, neg: false, key: "#", typed: "" });
});

test("a bare word, a keyed word, an escaped or quoted one gets nothing", () => {
  assert.equal(tix.quickTokenAt("Fix lo", 6), null, "a bare word is the title");
  assert.equal(tix.quickTokenAt("tag:ba", 6), null, "only the sigils complete");
  assert.equal(tix.quickTokenAt("Fix \\#ba", 8), null, "an escaped sigil is title text");
  assert.equal(tix.quickTokenAt('"Fix #ba', 8), null, "inside an open quote is title text");
  assert.equal(tix.quickTokenAt("„Fix #ba", 8), null, "the Bulgarian opening quote too");
  assert.equal(tix.quickTokenAt("", 0), null);
  assert.deepEqual(inserts("Fix lo"), []);
});

test("each sigil completes from its own list", () => {
  assert.deepEqual(inserts("Fix #back"), ["#backend", "#backlog"]);
  assert.deepEqual(inserts("Fix @ra"), ["@raj", "@rajesh"]);
  const projects = plain(tix.quickSuggest(data, tix.quickTokenAt("Fix +we", 7)));
  assert.deepEqual(projects, [{ label: "+web", insert: "+web", note: "Website" }]);
  assert.deepEqual(inserts("Fix +infra"), [], "a word that is already its only suggestion");
  assert.deepEqual(inserts("Fix +Infrastr"), ["+infra"], "a project is found by its name");
  assert.deepEqual(inserts("Fix #"), ["#backend", "#backlog", "tag:c++", "#ops"], "the sigil alone lists them all");
});

test("a value the sigil cannot carry is keyed, and one with a space is left out", () => {
  assert.deepEqual(inserts("Fix #c"), ["tag:c++", "#backend", "#backlog"]);
  assert.deepEqual(inserts("Fix #two"), [], "the parser has no way to write a space into a tag");
  assert.deepEqual(inserts("Fix @ann"), ["assignee:ann-"], "a handle ending in a hyphen is not an @ word");
});

test("taking a suggestion keeps the sigil and adds a space", () => {
  const line = "Fix login #ba now";
  const tok = tix.quickTokenAt(line, 13);
  assert.deepEqual(plain(tix.filterInsert(line, tok, "#backend")), { value: "Fix login #backend now", caret: 19 });
  const end = "Fix @ra";
  assert.deepEqual(plain(tix.filterInsert(end, tix.quickTokenAt(end, 7), "@raj")), { value: "Fix @raj ", caret: 9 });
});

test("Enter with nothing reached still adds", () => {
  assert.deepEqual(plain(tix.filterKey("Enter", true, 2, -1)), { act: "none" });
});

test("firstLine is the clipboard's first line with anything on it", () => {
  assert.equal(tix.firstLine("\n  \r\n  Renew the certificate  \nmore notes"), "Renew the certificate");
  assert.equal(tix.firstLine("one line"), "one line");
  assert.equal(tix.firstLine(" \n\t\n"), "");
  assert.equal(tix.firstLine(""), "");
  assert.equal(tix.firstLine(null), "");
});

test("comboOf names shift on a ctrl or command chord", () => {
  assert.equal(tix.comboOf({ key: "V", ctrlKey: true, shiftKey: true }), "ctrl+shift+v");
  assert.equal(tix.comboOf({ key: "v", metaKey: true, shiftKey: true }), "meta+shift+v");
  assert.equal(tix.comboOf({ key: "v", ctrlKey: true }), "ctrl+v");
});

test("quickadd.js stands down without its decisions", () => {
  const listeners = [];
  const bare = bareContext({ document: { addEventListener: (...a) => listeners.push(a) } });
  run(bare, "quickadd.js");
  assert.equal(listeners.length, 0);
});

test("quickPaste answers false where the page has no quick-add box", () => {
  const listeners = [];
  const page = newContext({
    navigator: { clipboard: { readText: () => Promise.resolve("x") } },
    document: {
      addEventListener: (...a) => listeners.push(a),
      querySelector: () => null,
      querySelectorAll: () => [],
    },
  });
  run(page, "quickadd.js");
  assert.equal(page.tix.quickPaste(), false);
});

test("quickPaste fills the box with the first line and does not submit", async () => {
  const events = [];
  const box = {
    value: "",
    focused: false,
    focus() { this.focused = true; },
    setSelectionRange() {},
    dispatchEvent(e) { events.push(e.type); },
  };
  const page = newContext({
    Event: class { constructor(type) { this.type = type; } },
    navigator: { clipboard: { readText: () => Promise.resolve("\nRenew the certificate\nmore") } },
    document: {
      addEventListener: () => {},
      querySelector: (sel) => (sel === "input[data-quickadd]" ? box : null),
      querySelectorAll: () => [],
    },
  });
  run(page, "quickadd.js");
  assert.equal(page.tix.quickPaste(), true);
  await new Promise((r) => setImmediate(r));
  assert.equal(box.value, "Renew the certificate");
  assert.equal(box.focused, true);
  assert.deepEqual(events, ["input"], "only the preview's input event, no submit");
});
