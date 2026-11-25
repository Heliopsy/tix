// SPDX-License-Identifier: AGPL-3.0-or-later

// Bulk selection on the task list: assets/select.js.
import test from "node:test";
import assert from "node:assert/strict";
import { newContext, run, domNode, fakeDocument, fakeTimers, keydown, element } from "./harness.mjs";

function loaded() {
  const ctx = newContext();
  run(ctx, "select.js");
  return ctx;
}

test("shift-click ticks every box between the anchor and the click", () => {
  const { tix } = loaded();
  assert.deepEqual([...tix.pickRange(1, 4, 6)], [1, 2, 3, 4]);
  assert.deepEqual([...tix.pickRange(4, 1, 6)], [1, 2, 3, 4]);
});

test("no anchor, or one that left the list, is the clicked box alone", () => {
  const { tix } = loaded();
  assert.deepEqual([...tix.pickRange(-1, 2, 6)], [2]);
  assert.deepEqual([...tix.pickRange(9, 2, 6)], [2]);
  assert.deepEqual([...tix.pickRange(0, 7, 6)], []);
});

test("the count says one task or many", () => {
  const { tix } = loaded();
  assert.equal(tix.pickCountText(1), "1 task selected");
  assert.equal(tix.pickCountText(4), "4 tasks selected");
  const bg = { "%d task selected": "%d избрана задача", "%d tasks selected": "%d избрани задачи" };
  assert.equal(tix.pickCountText(3, (s) => bg[s]), "3 избрани задачи");
});

function within(selector, tagName = "LI") {
  return {
    tagName,
    closest(sel) {
      return sel.split(",").some((s) => s.trim() === selector) ? this : null;
    },
  };
}

test("select all belongs to the list only while focus is in it", () => {
  const { tix } = loaded();
  assert.equal(tix.pickAllApplies(within("[data-shortcut-row]")), true);
  assert.equal(tix.pickAllApplies(within(".tasklist", "UL")), true);
  assert.equal(tix.pickAllApplies(within(".elsewhere", "DIV")), false);
  assert.equal(tix.pickAllApplies({ tagName: "INPUT", type: "text", closest: () => ({}) }), false);
  assert.equal(tix.pickAllApplies(null), false);
});

test("Cmd+A is its own chord and never reaches the page as a bare a", () => {
  const { tix } = loaded();
  assert.equal(tix.comboOf({ key: "a", metaKey: true }), "meta+a");
  assert.equal(tix.comboOf({ key: "a", ctrlKey: true }), "ctrl+a");
  assert.equal(tix.comboOf({ key: "a" }), "a");
});

// -- select mode over a stub page ------------------------------------------


const CONTROL = "a, button, input, select, textarea, label, summary, [popover]";
const NO_HOLD = "button, input, select, textarea, label, summary, [popover]";

// row is one task row: its box, a title link and a stretch of empty space.
function row(ref) {
  const box = domNode(["input[data-pick]"], { value: ref, tagName: "INPUT" });
  box.value = ref;
  box.checked = false;
  const link = domNode([CONTROL], { tagName: "A" });
  const space = domNode([], { tagName: "SPAN" });
  const check = domNode(["button.check", CONTROL, NO_HOLD], { tagName: "BUTTON", class: "check" });
  check.clicks = 0;
  check.click = function () { this.clicks++; };
  const r = domNode(["[data-shortcut-row]"], { tagName: "LI" }, [box, check, link, space]);
  r.focused = 0;
  r.focus = function () { this.focused++; };
  return { row: r, box, link, space, check };
}

// screen runs select.js against a toggle, a bar and the given rows.
function screen(refs, { pressed = false } = {}) {
  const timers = fakeTimers();
  const ctx = newContext({ setTimeout: timers.setTimeout, clearTimeout: timers.clearTimeout });
  const rows = refs.map(row);
  const toggle = domNode(["[data-select-toggle]"], { "aria-pressed": pressed ? "true" : "false" });
  const label = domNode(["[data-bulk-count]"]);
  const bar = domNode(["form[data-bulkbar]"], {}, [label]);
  const page = { rows };
  const doc = fakeDocument({
    bySelector: { "[data-select-toggle]": toggle, "form[data-bulkbar]": bar },
  });
  doc.querySelectorAll = (sel) => (sel === "input[data-pick]" ? page.rows.map((r) => r.box) : []);
  ctx.document = doc;
  run(ctx, "select.js");
  const mode = () => toggle.getAttribute("aria-pressed") === "true";
  const picked = () => page.rows.filter((r) => r.box.checked).map((r) => r.box.value);
  return { ctx, tix: ctx.tix, doc, toggle, label, page, rows, timers, mode, picked };
}

function click(target, mods = {}) {
  return {
    target,
    shiftKey: false,
    ctrlKey: false,
    metaKey: false,
    prevented: 0,
    preventDefault() { this.prevented++; },
    stopPropagation() {},
    stopImmediatePropagation() { this.immediateStopped = true; },
    ...mods,
  };
}

function tick(s, i, mods = {}) {
  const box = s.rows[i].box;
  box.checked = !box.checked;
  return s.doc.dispatch("click", click(box, mods));
}

test("rows show no selection until the mode is asked for", () => {
  const s = screen(["a-1", "a-2"]);
  assert.equal(s.mode(), false);
  s.doc.dispatch("click", click(s.rows[0].space));
  assert.equal(s.mode(), false, "a plain click on a row entered select mode");
  assert.deepEqual(s.picked(), []);
});

test("the toggle presses in place and leaving clears the selection", () => {
  const s = screen(["a-1", "a-2"]);
  const ev = s.doc.dispatch("click", click(s.toggle));
  assert.equal(ev.prevented, 1, "the toggle's link navigated");
  assert.equal(s.mode(), true);
  assert.equal(s.label.textContent, "0 tasks selected");
  tick(s, 1);
  s.doc.dispatch("click", click(s.toggle));
  assert.equal(s.mode(), false);
  assert.deepEqual(s.picked(), []);
});

test("x on a focused row enters the mode and selects it", () => {
  const s = screen(["a-1", "a-2"]);
  s.doc.activeElement = s.rows[1].row;
  assert.equal(s.tix.togglePick(), true);
  assert.equal(s.mode(), true);
  assert.deepEqual(s.picked(), ["a-2"]);
  assert.equal(s.label.textContent, "1 task selected");
});

test("unselecting the last selected row leaves the mode", () => {
  const s = screen(["a-1", "a-2"]);
  s.doc.activeElement = s.rows[0].row;
  s.tix.togglePick();
  s.tix.togglePick();
  assert.equal(s.mode(), false, "the mode stayed with nothing selected");

  const box = screen(["a-1"]);
  box.doc.dispatch("click", click(box.toggle));
  tick(box, 0);
  assert.equal(box.mode(), true);
  tick(box, 0);
  assert.equal(box.mode(), false, "unticking the last box kept the mode");
});

test("entering with the toggle and selecting nothing stays in the mode", () => {
  const s = screen(["a-1"]);
  s.doc.dispatch("click", click(s.toggle));
  s.doc.dispatch("htmx:load", {});
  assert.equal(s.mode(), true);
});

test("Shift+arrows extend the selection from the focused row", () => {
  const s = screen(["a-1", "a-2", "a-3", "a-4"]);
  s.doc.activeElement = s.rows[1].row;
  assert.equal(s.tix.extendPick(1), true);
  s.doc.activeElement = s.rows[2].row;
  s.tix.extendPick(1);
  assert.equal(s.mode(), true);
  assert.deepEqual(s.picked(), ["a-2", "a-3", "a-4"]);
  assert.equal(s.rows[3].row.focused, 1, "focus did not follow the selection");
  s.doc.activeElement = s.rows[0].row;
  s.tix.extendPick(-1);
  assert.deepEqual(s.picked(), ["a-1", "a-2", "a-3", "a-4"], "extending off the top lost a row");
});

test("shift-click ticks the range from the last box", () => {
  const s = screen(["a-1", "a-2", "a-3", "a-4", "a-5"]);
  s.doc.dispatch("click", click(s.toggle));
  tick(s, 1);
  tick(s, 4, { shiftKey: true });
  assert.deepEqual(s.picked(), ["a-2", "a-3", "a-4", "a-5"]);
  assert.equal(s.label.textContent, "4 tasks selected");
});

test("shift- and ctrl-click on a row's empty space enter the mode", () => {
  const s = screen(["a-1", "a-2", "a-3"]);
  const ev = s.doc.dispatch("click", click(s.rows[0].space, { ctrlKey: true }));
  assert.equal(ev.prevented, 1);
  assert.equal(s.mode(), true);
  assert.deepEqual(s.picked(), ["a-1"]);
  s.doc.dispatch("click", click(s.rows[2].space, { shiftKey: true }));
  assert.deepEqual(s.picked(), ["a-1", "a-2", "a-3"]);

  const link = screen(["a-1"]);
  const followed = link.doc.dispatch("click", click(link.rows[0].link, { ctrlKey: true }));
  assert.equal(followed.prevented, 0, "ctrl-click on the title stopped opening a tab");
  assert.equal(link.mode(), false);
});

test("Escape leaves the mode before it goes back", () => {
  const s = screen(["a-1", "a-2"]);
  s.doc.activeElement = s.rows[0].row;
  s.tix.togglePick();
  const ev = s.doc.dispatch("keydown", keydown("Escape"));
  assert.equal(ev.prevented, 1);
  assert.equal(ev.immediateStopped, true, "Escape also reached Back");
  assert.equal(s.mode(), false);
  assert.deepEqual(s.picked(), []);

  const idle = screen(["a-1"]);
  const back = idle.doc.dispatch("keydown", keydown("Escape"));
  assert.equal(back.prevented, 0, "Escape was taken outside select mode");
});

function pointer(type, target, x = 10, y = 10, pointerType = "touch") {
  return { type, target, clientX: x, clientY: y, pointerType };
}

test("a long press selects the row and swallows the click its release makes", () => {
  const s = screen(["a-1", "a-2"]);
  s.doc.dispatch("pointerdown", pointer("pointerdown", s.rows[1].link));
  assert.deepEqual(s.timers.delays(), [500]);
  s.timers.fire();
  assert.equal(s.mode(), true);
  assert.deepEqual(s.picked(), ["a-2"]);
  s.doc.dispatch("pointerup", pointer("pointerup", s.rows[1].link));
  const ev = s.doc.dispatch("click", click(s.rows[1].link));
  assert.equal(ev.prevented, 1, "the release opened the task");
});

test("a press that moves, lifts early or comes from a mouse is not a long press", () => {
  const moved = screen(["a-1"]);
  moved.doc.dispatch("pointerdown", pointer("pointerdown", moved.rows[0].space));
  moved.doc.dispatch("pointermove", pointer("pointermove", moved.rows[0].space, 10, 40));
  assert.equal(moved.timers.count, 0, "scrolling kept the press alive");

  const lifted = screen(["a-1"]);
  lifted.doc.dispatch("pointerdown", pointer("pointerdown", lifted.rows[0].space));
  lifted.doc.dispatch("pointerup", pointer("pointerup", lifted.rows[0].space));
  assert.equal(lifted.timers.count, 0);

  const mouse = screen(["a-1"]);
  mouse.doc.dispatch("pointerdown", pointer("pointerdown", mouse.rows[0].space, 10, 10, "mouse"));
  assert.equal(mouse.timers.count, 0);

  const button = screen(["a-1"]);
  button.doc.dispatch("pointerdown", pointer("pointerdown", button.rows[0].check));
  assert.equal(button.timers.count, 0, "a long press on the done control selected the row");
  assert.equal(button.mode(), false);
});

test("a row redrawn by a swap keeps the mode and its tick", () => {
  const s = screen(["a-1", "a-2"]);
  s.doc.activeElement = s.rows[1].row;
  s.tix.togglePick();
  const fresh = row("a-2");
  s.page.rows = [s.rows[0], fresh];
  s.doc.dispatch("htmx:load", {});
  assert.equal(s.mode(), true);
  assert.equal(fresh.box.checked, true, "the swapped row came back unticked");
  assert.equal(s.label.textContent, "1 task selected");
});

test("d presses the focused row's done control, never a finished one", () => {
  const s = screen(["a-1"]);
  s.doc.activeElement = s.rows[0].row;
  assert.equal(s.tix.completeFocused(), true);
  assert.equal(s.rows[0].check.clicks, 1);
  s.rows[0].check.classList.add("is-done");
  assert.equal(s.tix.completeFocused(), false);
  assert.equal(s.rows[0].check.clicks, 1);
  s.doc.activeElement = element("DIV");
  assert.equal(s.tix.completeFocused(), false);
});

test("Select in a row's menu enters the mode with that row ticked", () => {
  const s = screen(["a-1", "a-2", "a-3"]);
  const link = domNode(["[data-select-row]", CONTROL], { tagName: "A", "data-select-row": "a-2" });
  const menu = domNode(["[popover]"], {}, [link]);
  let hidden = 0;
  menu.hidePopover = () => { hidden++; };
  link.parentNode = menu;
  menu.parentNode = s.rows[1].row;
  const ev = s.doc.dispatch("click", click(link));
  assert.equal(ev.prevented, 1, "the link to select mode navigated");
  assert.equal(s.mode(), true);
  assert.deepEqual(s.picked(), ["a-2"]);
  assert.equal(hidden, 1, "the menu stayed open over the ticked row");
  assert.equal(s.rows[1].row.focused, 1, "focus did not return to the row");
});

test("Select for a row with no box leaves the link to navigate", () => {
  const s = screen(["a-1"]);
  assert.equal(s.tix.pickRow("gone-9"), false);
  assert.equal(s.mode(), false);
  const link = domNode(["[data-select-row]", CONTROL], { tagName: "A", "data-select-row": "gone-9" });
  const ev = s.doc.dispatch("click", click(link));
  assert.equal(ev.prevented, 0);
});
