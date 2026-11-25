// SPDX-License-Identifier: AGPL-3.0-or-later

// assets/tipplace.js: where an open (i) note goes so it stays inside the
// window. That the page stops scrolling sideways with a note open is the
// screenshot gate's (just ui-check), which reads real layout.
import test from "node:test";
import assert from "node:assert/strict";
import { newContext, run } from "./harness.mjs";

function load() {
  const ctx = newContext();
  run(ctx, "tipplace.js");
  return ctx.tix;
}

test("tipFit moves a note only as far as it has to", () => {
  const tix = load();
  for (const [name, left, right, width, want] of [
    ["fits", 20, 300, 390, { shift: 0, width: 0 }],
    ["runs past the right edge", 200, 520, 390, { shift: -138, width: 0 }],
    ["starts left of the gutter", -30, 200, 390, { shift: 38, width: 0 }],
    ["touches the gutter exactly", 62, 382, 390, { shift: 0, width: 0 }],
    ["wider than the window", 100, 600, 390, { shift: -92, width: 374 }],
  ]) {
    assert.deepEqual({ ...tix.tipFit(left, right, width, 8) }, want, name);
  }
});

test("tipFit never moves a note off the left edge to fit the right", () => {
  const tix = load();
  const fit = tix.tipFit(300, 800, 390, 8);
  assert.equal(300 + fit.shift, 8);
  assert.equal(fit.width, 374);
});

// page builds a document holding tips, each a details.field-info with a
// summary and a note that may hold a link. Only what tipplace.js reads is
// here: closest, the open flag, the summary and the open tips.
function page(count) {
  const listeners = new Map();
  const tips = [];
  function node(kind, parent, extra = {}) {
    return {
      kind,
      parentNode: parent,
      attrs: {},
      focused: 0,
      ...extra,
      setAttribute(name, value) {
        this.attrs[name] = value;
      },
      focus() {
        this.focused++;
        doc.activeElement = this;
      },
      closest(sel) {
        assert.equal(sel, "details.field-info");
        let at = this;
        while (at && at.kind !== "tip") at = at.parentNode;
        return at || null;
      },
    };
  }
  const body = node("body", null);
  const doc = {
    activeElement: body,
    addEventListener(type, fn, capture) {
      assert.equal(capture, true, `${type} is listened for in the capture phase`);
      listeners.set(type, fn);
    },
    querySelectorAll(sel) {
      assert.equal(sel, "details.field-info[open]");
      return tips.filter((t) => t.open);
    },
    documentElement: { clientWidth: 1024 },
  };
  for (let i = 0; i < count; i++) {
    const tip = node("tip", body, { open: false });
    tip.matches = (sel) => sel === "details.field-info";
    tip.summary = node("summary", tip);
    tip.note = node("p", tip);
    tip.link = node("a", tip.note);
    tip.querySelector = (sel) => (sel === ":scope > summary" ? tip.summary : null);
    tips.push(tip);
  }
  const ctx = newContext({ document: doc });
  run(ctx, "tipplace.js");
  const fire = (type, event) => {
    const e = {
      prevented: 0,
      stopped: 0,
      defaultPrevented: false,
      preventDefault() {
        this.prevented++;
        this.defaultPrevented = true;
      },
      stopPropagation() {
        this.stopped++;
      },
      ...event,
    };
    listeners.get(type)(e);
    return e;
  };
  return {
    doc,
    body,
    tips,
    open(tip) {
      tip.open = true;
      fire("toggle", { target: tip });
    },
    press(target) {
      return fire("pointerdown", { target });
    },
    key(key) {
      return fire("keydown", { key, target: doc.activeElement });
    },
  };
}

test("a press outside an open tip closes it", () => {
  const p = page(1);
  const [tip] = p.tips;
  p.open(tip);
  assert.equal(tip.summary.attrs["aria-expanded"], "true");
  p.press(p.body);
  assert.equal(tip.open, false);
  assert.equal(tip.summary.attrs["aria-expanded"], "false");
});

test("a press inside a tip's note, on its link, leaves it open", () => {
  const p = page(1);
  const [tip] = p.tips;
  p.open(tip);
  p.press(tip.link);
  assert.equal(tip.open, true);
  p.press(tip.note);
  assert.equal(tip.open, true);
});

test("a press on the tip's own circle leaves the toggling to the summary", () => {
  const p = page(1);
  const [tip] = p.tips;
  p.open(tip);
  p.press(tip.summary);
  assert.equal(tip.open, true);
});

test("opening a second tip closes the first", () => {
  const p = page(2);
  const [first, second] = p.tips;
  p.open(first);
  p.press(second.summary);
  assert.equal(first.open, false, "the press on the second circle closes the first");
  first.open = true;
  p.open(second);
  assert.equal(first.open, false, "opening the second closes the first");
  assert.equal(first.summary.attrs["aria-expanded"], "false");
  assert.equal(second.open, true);
});

test("Escape closes the open tip and puts focus back on its circle", () => {
  const p = page(2);
  const [first, second] = p.tips;
  p.open(second);
  second.summary.focus();
  second.summary.focused = 0;
  const e = p.key("Escape");
  assert.equal(second.open, false);
  assert.equal(second.summary.attrs["aria-expanded"], "false");
  assert.equal(second.summary.focused, 1);
  assert.equal(first.summary.focused, 0);
  assert.equal(e.prevented, 1, "Escape is claimed, so the back shortcut does not leave the page");
  assert.equal(e.stopped, 1);
});

test("Escape with focus elsewhere still returns focus to the tip's circle", () => {
  const p = page(1);
  const [tip] = p.tips;
  p.open(tip);
  p.key("Escape");
  assert.equal(tip.open, false);
  assert.equal(tip.summary.focused, 1);
});

test("Escape with no tip open is left alone", () => {
  const p = page(1);
  const e = p.key("Escape");
  assert.equal(e.prevented, 0);
  assert.equal(e.stopped, 0);
  assert.equal(p.key("Enter").prevented, 0);
});
