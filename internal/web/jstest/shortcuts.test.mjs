// SPDX-License-Identifier: AGPL-3.0-or-later

// assets/shortcuts.js at its own call sites: that the keydown handler really
// is gated on the typing guard, and really routes a combo to one action.
// The DOM here is a stub, so this covers dispatch and nothing about layout,
// focus order or real key events.
import test from "node:test";
import assert from "node:assert/strict";
import { newContext, bareContext, run, fakeDocument, element, keydown, fakeTimers } from "./harness.mjs";

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
  const { doc, history } = page({ overlay });

  doc.dispatch("keydown", keydown("?"));
  assert.equal(overlay.hasAttribute("hidden"), false, "the help key did not open the overlay");

  const event = doc.dispatch("keydown", keydown("Escape"));
  assert.equal(overlay.hasAttribute("hidden"), true, "Escape did not close the overlay");
  assert.equal(history.backs, 0, "Escape also fired the global back shortcut underneath the overlay");
  assert.equal(event.immediateStopped, true, "the overlay let Escape through to the global dispatch");
});

// The body, overlay included, is replaced by every boosted navigation. The
// overlay is looked up when "?" is pressed, so the one on screen is the one
// that opens, not the copy that was there when the script first ran.
test("the help key opens the overlay that is on the page now", () => {
  const first = element("DIV", { hidden: "" });
  const { doc } = page({ overlay: first });
  doc.dispatch("keydown", keydown("?"));
  doc.dispatch("keydown", keydown("Escape"));
  assert.equal(first.hasAttribute("hidden"), true, "the first overlay did not open and close");
  const swapped = element("DIV", { hidden: "" });
  const byId = doc.getElementById;
  doc.getElementById = (id) => (id === "shortcuts-help" ? swapped : byId(id));

  doc.dispatch("keydown", keydown("?"));
  assert.equal(swapped.hasAttribute("hidden"), false, "the overlay on the page did not open");
  assert.equal(first.hasAttribute("hidden"), true, "the detached overlay opened instead");
});

test("a control marked data-shortcuts-open opens the overlay", () => {
  const overlay = element("DIV", { hidden: "" });
  const { doc } = page({ overlay });
  const button = element("BUTTON", { "data-shortcuts-open": "" });
  button.closest = (sel) => (sel === "[data-shortcuts-open]" ? button : null);
  const click = { target: button, prevented: 0, preventDefault() { this.prevented++; } };

  doc.dispatch("click", click);
  assert.equal(overlay.hasAttribute("hidden"), false, "the button did not open the overlay");
  assert.equal(click.prevented, 1);
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

// A task open in the dialog owns the keyboard. Escape there is the dialog's
// own close, which goes Back once; the page's back shortcut firing as well
// would go Back twice and leave the page the dialog was opened over.
test("while a task dialog is open the global shortcuts stand down", () => {
  const p = page();
  p.doc.querySelector = (sel) => (sel === "dialog[open]" ? {} : null);
  const event = p.doc.dispatch("keydown", keydown("Escape"));
  assert.equal(event.prevented, 0, "a shortcut fired underneath the open dialog");
  assert.equal(p.history.backs, 0, "Escape in the dialog also went Back from the page");
});

// The task panel sits beside the list rather than over it: a key pressed in
// the list is the list's, and only a key pressed in the panel is the panel's.
test("while the task panel is open the shortcuts act outside it and stand down inside", () => {
  const inside = element("BUTTON");
  const pane = { getAttribute: (k) => (k === "data-shown" ? "pane" : null), contains: (el) => el === inside };
  const p = page();
  p.doc.querySelector = (sel) => (sel === "dialog[open]" ? pane : null);
  p.doc.dispatch("keydown", keydown("Escape"));
  assert.equal(p.history.backs, 1, "the list's back shortcut stood down beside the panel");
  p.doc.dispatch("keydown", keydown("Escape", { target: inside }));
  assert.equal(p.history.backs, 1, "a key pressed in the panel acted on the list");

  const modal = { getAttribute: () => null, contains: () => false };
  const q = page();
  q.doc.querySelector = (sel) => (sel === "dialog[open]" ? modal : null);
  q.doc.dispatch("keydown", keydown("Escape"));
  assert.equal(q.history.backs, 0, "a key acted on the page under a modal dialog");
});

// In Dialog mode the open shortcut hands the row to the dialog first.
test("open hands the selected row to the dialog when the page has one", () => {
  const row = element("A", { "data-shortcut-row": "", dataset: { shortcutHref: "/tasks/TIX-1" } });
  const { ctx, doc, location } = page({ rows: [row] });
  const asked = [];
  ctx.tix.openRow = (el) => { asked.push(el); return true; };
  doc.activeElement = row;
  doc.dispatch("keydown", keydown("Enter"));
  assert.deepEqual(asked, [row], "the dialog was not asked");
  assert.deepEqual(location.assigned, [], "the page navigated as well");

  ctx.tix.openRow = () => false;
  doc.dispatch("keydown", keydown("Enter"));
  assert.deepEqual(location.assigned, ["/tasks/TIX-1"], "Page mode no longer opens the row");
});

// Enter on a focused button presses the button. The open shortcut is bound
// to Enter, and it used to claim the key from every button on the page, so a
// tick or a menu could not be pressed from the keyboard at all.
test("Enter on a focused button or link is left to the control", () => {
  const { doc } = page();
  for (const tagName of ["BUTTON", "A", "SUMMARY"]) {
    const event = doc.dispatch("keydown", keydown("Enter", { target: element(tagName) }));
    assert.equal(event.prevented, 0, `Enter was taken from a focused ${tagName}`);
  }
  const row = doc.dispatch("keydown", keydown("Enter", { target: element("ARTICLE") }));
  assert.equal(row.prevented, 1, "Enter on a row no longer opens it");
});

// Escape in an open menu closes the menu and nothing else. The back shortcut
// took it as well, so dismissing a status menu left the page.
test("while a menu is open the global shortcuts stand down", () => {
  const p = page();
  p.doc.querySelector = (sel) => (sel === ":popover-open" ? {} : null);
  const event = p.doc.dispatch("keydown", keydown("Escape"));
  assert.equal(event.prevented, 0, "a shortcut fired underneath the open menu");
  assert.equal(p.history.backs, 0, "Escape in a menu also went Back from the page");

  const old = page();
  old.doc.querySelector = (sel) => { if (sel === ":popover-open") throw new Error("unsupported"); return null; };
  assert.doesNotThrow(() => old.doc.dispatch("keydown", keydown("j")), "a browser without popovers broke the shortcuts");
});

test("a shortcut target inside a closed disclosure opens it before taking focus", () => {
  const fold = { open: false };
  const title = element("INPUT");
  title.closest = (sel) => (sel === "details" ? fold : null);
  const data = { scheme: "default", bindings: { default: { editTitle: ["e"] } }, actions: [{ name: "editTitle", label: "Edit the title", group: "Task" }] };
  const ctx = newContext({ history: { length: 1, back() {} }, location: { assign() {} } });
  const doc = fakeDocument({
    byId: { "tix-shortcuts-data": { textContent: JSON.stringify(data) }, "shortcuts-help": null, "shortcuts-help-list": null },
    bySelector: { '[data-shortcut-target="editTitle"]': title },
  });
  ctx.document = doc;
  run(ctx, "shortcuts.js");
  doc.dispatch("keydown", keydown("e"));
  assert.equal(fold.open, true, "the disclosure holding the title stayed shut");
  assert.equal(title.focused, 1, "the title field was not focused");
});

// -- the shared key table: sequences, go-to keys and the task keys ----------

const shared = {
  goInbox: ["g i"],
  goStats: ["g S"],
  top: ["g", "Home"],
  down: ["ArrowDown", "j"],
  star: ["s", "*"],
  priority: ["p"],
  assign: ["a"],
  copyRef: ["y"],
  refresh: ["r"],
  switchView: ["v"],
  help: ["?"],
};
const sharedActions = [
  { name: "goInbox", label: "Go to Inbox", group: "Navigate" },
  { name: "goStats", label: "Go to Statistics", group: "Navigate" },
  { name: "top", label: "Jump to the first task", group: "Lists" },
  { name: "down", label: "Move selection down", group: "Lists" },
  { name: "star", label: "Star or unstar", group: "Task" },
  { name: "help", label: "Show this help", group: "General" },
];
const groups = [
  { value: "Navigate", label: "Navigate" },
  { value: "Lists", label: "Lists" },
  { value: "Task", label: "Task" },
  { value: "General", label: "General" },
];

// row is a focusable list row holding the controls it is given, each found
// by its data-shortcut-target the way the page's markup is.
function row(href, controls = {}) {
  const el = element("LI", { "data-shortcut-row": "" });
  el.getAttribute = (name) => (name === "data-shortcut-href" ? href : null);
  el.closest = (sel) => (sel === "[data-shortcut-row]" ? el : null);
  el.querySelector = (sel) => {
    const m = /data-shortcut-target="([^"]+)"/.exec(sel);
    return (m && controls[m[1]]) || null;
  };
  for (const c of Object.values(controls)) {
    c.closest = (sel) => (sel === "[data-shortcut-row]" ? el : null);
  }
  return el;
}

function button() {
  const el = element("BUTTON");
  el.clicked = 0;
  el.click = () => { el.clicked++; };
  el.closest = () => null;
  return el;
}

function sharedPage({ rows = [], byTarget = {}, extra = {}, scheme = "default", bindings = { default: shared } } = {}) {
  const timers = fakeTimers();
  const location = { assigned: [], reloads: 0, assign(href) { this.assigned.push(href); }, reload() { this.reloads++; } };
  const ctx = newContext({ history: { length: 1, back() {} }, location, setTimeout: timers.setTimeout, clearTimeout: timers.clearTimeout, ...extra });
  const all = {};
  for (const [name, list] of Object.entries(byTarget)) {
    all[`[data-shortcut-target="${name}"]`] = list;
  }
  const data = { scheme, bindings, actions: sharedActions, groups, go: { goInbox: "/inbox", goStats: "/stats" } };
  const doc = fakeDocument({
    byId: { "tix-shortcuts-data": { textContent: JSON.stringify(data) }, "shortcuts-help": null, "shortcuts-help-list": null },
    bySelector: Object.fromEntries(Object.entries(all).map(([k, v]) => [k, v[0] || null])),
    bySelectorAll: { "[data-shortcut-row]": rows, ...all },
  });
  ctx.document = doc;
  run(ctx, "shortcuts.js");
  return { ctx, doc, timers, location };
}

test("g then i goes to the inbox, and g then S to the statistics", () => {
  const p = sharedPage();
  const first = p.doc.dispatch("keydown", keydown("g"));
  assert.equal(first.prevented, 1, "g was not held for a second key");
  assert.deepEqual(p.location.assigned, [], "it navigated before the second key");
  p.doc.dispatch("keydown", keydown("i"));
  assert.deepEqual(p.location.assigned, ["/inbox"]);
  assert.equal(p.timers.count, 0, "the wait outlived the sequence it was for");

  p.doc.dispatch("keydown", keydown("g"));
  p.doc.dispatch("keydown", keydown("Shift", { shiftKey: true }));
  p.doc.dispatch("keydown", keydown("S", { shiftKey: true }));
  assert.deepEqual(p.location.assigned, ["/inbox", "/stats"], "holding Shift for the S broke the sequence");
});

test("a lone g jumps to the top once the wait is over, not before", () => {
  const rows = [element("LI", { "data-shortcut-row": "" }), element("LI", { "data-shortcut-row": "" })];
  const p = sharedPage({ rows });
  p.doc.dispatch("keydown", keydown("g"));
  assert.equal(rows[0].focused, 0, "g jumped before the wait was over");
  assert.deepEqual(p.timers.delays(), [1000]);
  p.timers.fire();
  assert.equal(rows[0].focused, 1, "a lone g no longer jumps to the top");
});

test("a key that completes no sequence drops the wait and acts on its own", () => {
  const rows = [element("LI", { "data-shortcut-row": "" }), element("LI", { "data-shortcut-row": "" })];
  const p = sharedPage({ rows });
  p.doc.dispatch("keydown", keydown("g"));
  p.doc.dispatch("keydown", keydown("j"));
  assert.equal(rows[0].focused, 1, "j after g did not move down");
  assert.equal(p.timers.count, 0, "the dropped wait would still jump to the top");
});

test("typing into a field drops a wait without running it", () => {
  const rows = [element("LI", { "data-shortcut-row": "" })];
  const p = sharedPage({ rows });
  p.doc.dispatch("keydown", keydown("g"));
  p.doc.dispatch("keydown", keydown("x", { target: element("INPUT") }));
  assert.equal(p.timers.count, 0);
  assert.equal(rows[0].focused, 0);
});

test("a task key presses the focused row's own control", () => {
  const star = button();
  const r = row("/tasks/infra-1", { star });
  const p = sharedPage({ rows: [r], byTarget: { star: [star] } });
  p.doc.activeElement = r;
  const event = p.doc.dispatch("keydown", keydown("s"));
  assert.equal(star.clicked, 1, "s did not press the row's star");
  assert.equal(event.prevented, 1);
  p.doc.dispatch("keydown", keydown("*"));
  assert.equal(star.clicked, 2, "* is not star as well");
});

test("a task key with no row focused leaves the stars inside rows alone", () => {
  const star = button();
  const r = row("/tasks/infra-1", { star });
  const p = sharedPage({ rows: [r], byTarget: { star: [star] } });
  const event = p.doc.dispatch("keydown", keydown("s"));
  assert.equal(star.clicked, 0, "s starred some row nobody had selected");
  assert.equal(event.prevented, 0, "s was claimed with nothing to press");
});

test("a task key on a row without the control falls through", () => {
  const r = row("/tasks/infra-1", {});
  const page = sharedPage({ rows: [r], byTarget: { priority: [button()] } });
  page.doc.activeElement = r;
  const event = page.doc.dispatch("keydown", keydown("p"));
  assert.equal(event.prevented, 0, "p was claimed on a row that has no priority control");
});

test("a task key focuses the task page's field rather than pressing it", () => {
  const field = element("INPUT");
  field.closest = () => null;
  field.click = () => { throw new Error("a field was clicked"); };
  const p = sharedPage({ byTarget: { assign: [field] } });
  p.doc.dispatch("keydown", keydown("a"));
  assert.equal(field.focused, 1, "a did not focus the assignee field");
});

test("y copies the focused row's reference", () => {
  const copied = [];
  const clipboard = { writeText(text) { copied.push(text); return { then(ok) { ok(); } }; } };
  const r = row("/tasks/infra-7", {});
  const p = sharedPage({ rows: [r], extra: { navigator: { clipboard } } });
  p.doc.activeElement = r;
  const event = p.doc.dispatch("keydown", keydown("y"));
  assert.deepEqual(copied, ["INFRA-7"]);
  assert.equal(event.prevented, 1);
});

test("y on a task page copies that task's reference", () => {
  const copied = [];
  const clipboard = { writeText(text) { copied.push(text); return { then(ok) { ok(); } }; } };
  const ref = element("SPAN", { "data-shortcut-ref": "INFRA-9" });
  ref.closest = () => null;
  const p = sharedPage({ byTarget: { copyRef: [ref] }, extra: { navigator: { clipboard } } });
  p.doc.dispatch("keydown", keydown("y"));
  assert.deepEqual(copied, ["INFRA-9"]);
});

test("y with nothing to copy falls through", () => {
  const p = sharedPage({ extra: { navigator: { clipboard: { writeText() { throw new Error("copied"); } } } } });
  const event = p.doc.dispatch("keydown", keydown("y"));
  assert.equal(event.prevented, 0);
});

test("r redraws the live regions, and reloads a page without them", () => {
  const live = sharedPage();
  let redrawn = 0;
  live.ctx.tix.liveRefresh = () => { redrawn++; return true; };
  live.doc.dispatch("keydown", keydown("r"));
  assert.equal(redrawn, 1);
  assert.equal(live.location.reloads, 0, "it reloaded a page it had just redrawn");

  const still = sharedPage();
  still.ctx.tix.liveRefresh = () => false;
  still.doc.dispatch("keydown", keydown("r"));
  assert.equal(still.location.reloads, 1);
});

test("v presses the view after the current one, and falls through without a switch", () => {
  const links = ["list", "board", "lanes"].map((view, i) => {
    const a = button();
    a.getAttribute = (name) => (name === "aria-current" && i === 1 ? "page" : null);
    return a;
  });
  const nav = element("NAV");
  nav.querySelectorAll = (sel) => (sel === "a[data-view]" ? links : []);
  const p = sharedPage({ byTarget: { switchView: [nav] } });
  p.doc.dispatch("keydown", keydown("v"));
  assert.deepEqual(links.map((l) => l.clicked), [0, 0, 1], "v did not move from board to lanes");

  const bare = sharedPage();
  assert.equal(bare.doc.dispatch("keydown", keydown("v")).prevented, 0);
});

test("the help overlay heads the active scheme's keys by group and leaves empty groups out", () => {
  const made = [];
  const node = (tag) => {
    const n = { tag, children: [], textContent: "", appendChild(c) { this.children.push(c); return c; } };
    made.push(n);
    return n;
  };
  const list = node("list");
  const overlay = element("DIV", { hidden: "" });
  overlay.querySelector = () => null;
  const ctx = newContext({ history: { length: 1, back() {} }, location: { assign() {} } });
  const data = { scheme: "default", bindings: { default: { goInbox: ["g i"], help: ["?"] } }, actions: sharedActions, groups };
  const doc = fakeDocument({
    byId: { "tix-shortcuts-data": { textContent: JSON.stringify(data) }, "shortcuts-help": overlay, "shortcuts-help-list": list },
  });
  doc.createElement = node;
  doc.createTextNode = (text) => ({ tag: "#text", textContent: text });
  ctx.document = doc;
  run(ctx, "shortcuts.js");
  doc.dispatch("keydown", keydown("?"));
  const heads = list.children.filter((c) => c.tag === "h3").map((c) => c.textContent);
  assert.deepEqual(heads, ["Navigate", "General"], "the groups are wrong, or an empty one was drawn");
  const firstKeys = list.children[1].children[0].children[0].children.filter((c) => c.tag === "kbd").map((c) => c.textContent);
  assert.deepEqual(firstKeys, ["g", "i"], "a sequence is not drawn as its two keys");
});
