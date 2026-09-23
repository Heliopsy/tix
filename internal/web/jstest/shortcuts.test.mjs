// SPDX-License-Identifier: AGPL-3.0-or-later

// assets/shortcuts.js at its own call sites: that the keydown handler really
// is gated on the typing guard, and really routes a combo to one action.
// The DOM here is a stub, so this covers dispatch and nothing about layout,
// focus order or real key events.
import test from "node:test";
import assert from "node:assert/strict";
import { newContext, bareContext, run, fakeDocument, element, keydown } from "./harness.mjs";

const bindings = {
  default: {
    up: ["ArrowUp", "k"],
    down: ["ArrowDown", "j"],
    open: ["Enter"],
    back: ["Escape"],
    filter: ["/"],
    help: ["?"],
  },
  emacs: { down: ["ctrl+n"], help: ["?"] },
};

const actions = [
  { name: "up", label: "Move selection up" },
  { name: "down", label: "Move selection down" },
  { name: "help", label: "Show this help" },
];

// page wires a document carrying the shortcut table and runs shortcuts.js
// against it, returning what a test needs to drive and observe.
function page({ scheme = "default", overlay = null, rows = [], data = { scheme, bindings, actions } } = {}) {
  const history = { length: 2, backs: 0, back() { this.backs++; } };
  const location = { assigned: [], assign(href) { this.assigned.push(href); } };
  const ctx = newContext({ history, location });
  const doc = fakeDocument({
    byId: {
      "tix-shortcuts-data": data === null ? null : { textContent: JSON.stringify(data) },
      "shortcuts-help": overlay,
      "shortcuts-help-list": null,
    },
    bySelectorAll: { "[data-shortcut-row]": rows },
  });
  ctx.document = doc;
  run(ctx, "shortcuts.js");
  return { ctx, doc, history, location, overlay };
}

test("a bound key is claimed", () => {
  const { doc } = page();
  for (const key of ["j", "ArrowDown", "k", "/", "?"]) {
    const event = doc.dispatch("keydown", keydown(key));
    assert.equal(event.prevented, 1, `${key} was not claimed`);
  }
});

test("an unbound key is left alone", () => {
  const { doc } = page();
  for (const key of ["x", "F5", "Tab", "PageDown", "ArrowLeft"]) {
    const event = doc.dispatch("keydown", keydown(key));
    assert.equal(event.prevented, 0, `${key} was claimed`);
  }
});

test("a bound key is not stolen out of text entry", () => {
  const { doc } = page();
  const targets = [
    element("INPUT"),
    element("TEXTAREA"),
    element("SELECT"),
    element("DIV", { isContentEditable: true }),
  ];
  for (const target of targets) {
    for (const key of ["j", "k", "/", "?", "ArrowDown"]) {
      const event = doc.dispatch("keydown", keydown(key, { target }));
      assert.equal(event.prevented, 0, `${key} was stolen out of ${target.tagName}`);
    }
  }
});

test("a chord resolves through the active scheme only", () => {
  const emacs = page({ scheme: "emacs" });
  const chord = emacs.doc.dispatch("keydown", keydown("n", { ctrlKey: true }));
  assert.equal(chord.prevented, 1, "ctrl+n did not reach the emacs binding");

  const plain = page({ scheme: "default" });
  const same = plain.doc.dispatch("keydown", keydown("n", { ctrlKey: true }));
  assert.equal(same.prevented, 0, "ctrl+n fired under a scheme that does not bind it");
});

test("back goes back only when there is somewhere to go", () => {
  const withHistory = page();
  withHistory.doc.dispatch("keydown", keydown("Escape"));
  assert.equal(withHistory.history.backs, 1);

  const fresh = page();
  fresh.history.length = 1;
  fresh.doc.dispatch("keydown", keydown("Escape"));
  assert.equal(fresh.history.backs, 0, "it navigated back off the first page in the history");
});

test("open follows the selected row and nothing else", () => {
  const row = element("A", { "data-shortcut-row": "", dataset: { shortcutHref: "/tasks/TIX-1" } });
  const { doc, location } = page({ rows: [row] });

  doc.dispatch("keydown", keydown("Enter"));
  assert.deepEqual(location.assigned, [], "it opened something with no row selected");

  doc.activeElement = row;
  doc.dispatch("keydown", keydown("Enter"));
  assert.deepEqual(location.assigned, ["/tasks/TIX-1"]);
});

test("selection moves over the rows the page has", () => {
  const rows = [element("A", { "data-shortcut-row": "" }), element("A", { "data-shortcut-row": "" })];
  const { doc } = page({ rows });

  doc.dispatch("keydown", keydown("j"));
  assert.equal(rows[0].focused, 1, "the first row was not focused");

  doc.activeElement = rows[0];
  doc.dispatch("keydown", keydown("j"));
  assert.equal(rows[1].focused, 1);

  doc.activeElement = rows[1];
  doc.dispatch("keydown", keydown("j"));
  assert.equal(rows[1].focused, 2, "selection ran off the end of the list");
});

test("while the help overlay is open the global shortcuts stand down", () => {
  const overlay = element("DIV", { hidden: "" });
  const { doc } = page({ overlay });

  doc.dispatch("keydown", keydown("?"));
  assert.equal(overlay.hasAttribute("hidden"), false, "the help key did not open the overlay");

  const event = doc.dispatch("keydown", keydown("Escape"));
  assert.equal(event.prevented, 0, "a global shortcut fired underneath the open overlay");
});

test("a page carrying no shortcut table binds no keys at all", () => {
  for (const data of [null, { scheme: "nope", bindings }, { bindings }, {}]) {
    const { doc } = page({ data });
    assert.equal(doc.listenerCount("keydown"), 0, `a keydown listener was bound for ${JSON.stringify(data)}`);
  }
});

test("a page whose shortcut table is not JSON binds no keys", () => {
  const ctx = newContext({ history: {}, location: {} });
  const doc = fakeDocument({ byId: { "tix-shortcuts-data": { textContent: "{not json" } } });
  ctx.document = doc;
  assert.doesNotThrow(() => run(ctx, "shortcuts.js"));
  assert.equal(doc.listenerCount("keydown"), 0);
});

test("shortcuts.js stands down entirely without decide.js", () => {
  const ctx = bareContext();
  const doc = fakeDocument({ byId: { "tix-shortcuts-data": { textContent: JSON.stringify({ scheme: "default", bindings, actions }) } } });
  ctx.document = doc;
  assert.doesNotThrow(() => run(ctx, "shortcuts.js"));
  assert.equal(doc.listenerCount("keydown"), 0);
});
