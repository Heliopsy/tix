// SPDX-License-Identifier: AGPL-3.0-or-later

// assets/datepick.js: the calendar button beside a typed due date. That it
// shows itself, opens the native picker or falls back to focusing it, and
// writes the picked day into the field in the field's own pattern. The DOM
// is a stub; that the server reads the written day back is a Go test
// (TestAPickedDayReadsBackAsTheSameDay).
import test from "node:test";
import assert from "node:assert/strict";
import { newContext, run, fakeDocument } from "./harness.mjs";

function listener() {
  const on = {};
  return {
    on,
    addEventListener(type, fn) { on[type] = fn; },
  };
}

function setup(pattern, showPicker) {
  const field = { value: "", focused: 0, events: [], getAttribute: (n) => (n === "data-date-pattern" ? pattern : null),
    dispatchEvent(e) { this.events.push(e.type); }, focus() { this.focused++; } };
  const button = listener();
  const native = { ...listener(), value: "", focused: 0, focus() { this.focused++; }, showPicker };
  const pick = { hidden: true, dataset: {}, getAttribute: () => "new-due",
    querySelector: (sel) => (sel === ".datepick-open" ? button : sel === ".datepick-native" ? native : null) };
  const doc = fakeDocument({ byId: { "new-due": field }, bySelectorAll: { "[data-datepick]": [pick] } });
  doc.readyState = "complete";
  const ctx = newContext({ Event: class { constructor(type) { this.type = type; } } });
  ctx.document = doc;
  run(ctx, "datepick.js");
  return { ctx, field, button, native, pick };
}

test("writeDate fills each pattern the server draws", () => {
  const { ctx } = setup("dd.mm.yyyy", () => {});
  for (const [pattern, want] of [["dd.mm.yyyy", "06.11.2026"], ["dd/mm/yyyy", "06/11/2026"],
    ["mm/dd/yyyy", "11/06/2026"], ["yyyy-mm-dd", "2026-11-06"]]) {
    assert.equal(ctx.tix.writeDate(pattern, "2026-11-06"), want);
  }
  assert.equal(ctx.tix.writeDate("dd.mm.yyyy", ""), "", "an emptied picker wrote something");
});

test("the button shows, opens the picker, and the picked day lands in the field", () => {
  let opened = 0;
  const { field, button, native, pick } = setup("dd.mm.yyyy", () => { opened++; });
  assert.equal(pick.hidden, false, "the button stayed hidden with scripting on");
  button.on.click();
  assert.equal(opened, 1, "the native picker was not opened");
  native.value = "2026-11-06";
  native.on.change();
  assert.equal(field.value, "06.11.2026");
  assert.deepEqual(field.events, ["input"]);
  assert.equal(field.focused, 1);
});

test("a browser without showPicker focuses the date control instead", () => {
  const { button, native } = setup("yyyy-mm-dd", () => { throw new Error("not allowed"); });
  button.on.click();
  assert.equal(native.focused, 1);
});
