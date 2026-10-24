// SPDX-License-Identifier: AGPL-3.0-or-later

// The decisions assets/decide.js makes on behalf of every other script.
import test from "node:test";
import assert from "node:assert/strict";
import { newContext, fakeTimers } from "./harness.mjs";

const ctx = newContext();
const tix = ctx.tix;

// The scripts run in their own vm realm, so an object they build has that
// realm's prototype and a strict deep comparison against a host literal fails
// on identity alone. Comparing the JSON both sides agree on sidesteps that.
const plain = (value) => JSON.parse(JSON.stringify(value));

test("decide.js exports itself and nothing else", () => {
  const fresh = newContext();
  const leaked = Object.keys(fresh).filter((k) => k !== "tix" && k !== "window");
  assert.deepEqual(leaked, [], "decide.js defined an unexpected global");
  assert.equal(typeof fresh.tix, "object");
});

test("comboOf names a key the way the binding table does", () => {
  const cases = [
    [{ key: "j" }, "j"],
    [{ key: "ArrowDown" }, "ArrowDown"],
    [{ key: "?" }, "?"],
    [{ key: "s", ctrlKey: true }, "ctrl+s"],
    [{ key: "S", ctrlKey: true }, "ctrl+s"],
    // A modifier on a named key is not a chord any shipped scheme uses.
    [{ key: "ArrowDown", ctrlKey: true }, "ArrowDown"],
    [{ key: "j", shiftKey: true }, "j"],
    [{}, ""],
  ];
  for (const [event, want] of cases) {
    assert.equal(tix.comboOf(event), want, JSON.stringify(event));
  }
});

test("actionsByCombo inverts one scheme's table", () => {
  const got = tix.actionsByCombo({ up: ["k", "ArrowUp"], down: ["j"], help: ["?"] });
  assert.deepEqual(plain(got), { k: "up", ArrowUp: "up", j: "down", "?": "help" });
});

test("actionsByCombo survives a table that binds nothing", () => {
  assert.deepEqual(plain(tix.actionsByCombo(null)), {});
  assert.deepEqual(plain(tix.actionsByCombo({})), {});
  assert.deepEqual(plain(tix.actionsByCombo({ up: [] })), {});
});

test("isTypingTarget holds a shortcut back from every text entry", () => {
  for (const tagName of ["INPUT", "TEXTAREA", "SELECT"]) {
    assert.equal(tix.isTypingTarget({ tagName }), true, tagName);
  }
  assert.equal(tix.isTypingTarget({ tagName: "DIV", isContentEditable: true }), true);
});

test("isTypingTarget lets a shortcut through everywhere else", () => {
  for (const tagName of ["DIV", "BODY", "A", "BUTTON", "LI"]) {
    assert.equal(tix.isTypingTarget({ tagName, isContentEditable: false }), false, tagName);
  }
  assert.equal(tix.isTypingTarget(null), false);
  assert.equal(tix.isTypingTarget(undefined), false);
});

test("coalescer draws the first event at once", () => {
  const timers = fakeTimers();
  let runs = 0;
  const trigger = tix.coalescer(300, () => runs++, timers);

  trigger();
  assert.equal(runs, 1, "a single event waited out the window for nothing to coalesce");
  assert.equal(timers.count, 1, "the window was not opened after the leading run");
});

test("coalescer collapses the rest of a burst into one more run", () => {
  const timers = fakeTimers();
  let runs = 0;
  const trigger = tix.coalescer(300, () => runs++, timers);

  trigger();
  trigger();
  trigger();
  assert.equal(runs, 1, "a burst rendered more than its leading edge");
  timers.fire();
  assert.equal(runs, 2, "the hops after the first were never drawn");
  timers.fire();
  assert.equal(runs, 2, "the window ran again with nothing queued");
});

test("coalescer does not let a steady stream starve the trailing run", () => {
  const timers = fakeTimers();
  let runs = 0;
  const trigger = tix.coalescer(300, () => runs++, timers, 1000);

  trigger();
  // Every arrival restarts the quiet period, which is exactly how a
  // trailing-only debounce is pushed off forever under sustained change.
  for (let i = 0; i < 20; i++) {
    trigger();
  }
  assert.ok(timers.delays().includes(1000), "no maximum wait was armed");
  timers.fire();
  assert.equal(runs, 2, "the bounded wait did not force a render");
});

test("coalescer arms again after the window closes", () => {
  const timers = fakeTimers();
  let runs = 0;
  const trigger = tix.coalescer(300, () => runs++, timers);

  trigger();
  timers.fire();
  trigger();
  assert.equal(runs, 2, "an event after a quiet window waited again");
});

test("backoffDelay doubles, caps and stays inside its jitter band", () => {
  const curve = [0, 1, 2, 3, 4, 5, 6, 7].map((n) => tix.backoffDelay(n, 500, 30000, () => 1));
  assert.deepEqual(curve, [500, 1000, 2000, 4000, 8000, 16000, 30000, 30000]);

  for (const attempt of [0, 1, 2, 3, 9]) {
    const low = tix.backoffDelay(attempt, 500, 30000, () => 0);
    const high = tix.backoffDelay(attempt, 500, 30000, () => 1);
    assert.equal(low, high / 2, "the jitter band is not the lower half of the interval");
    assert.ok(high <= 30000, "the curve ran past its cap");
  }
});

test("resumeRefused tells a retired cursor from any other refusal", () => {
  assert.equal(
    tix.resumeRefused({ code: "invalid", message: "cursor 12 is no longer available; the oldest retained event is 90" }),
    true
  );
  assert.equal(tix.resumeRefused({ code: "invalid", message: "unknown event type \"task.*\"" }), false);
  assert.equal(tix.resumeRefused(null), false);
  assert.equal(tix.resumeRefused({}), false);
});

test("staleNotice says the view may be out of date either way", () => {
  for (const polling of [true, false]) {
    assert.match(tix.staleNotice(polling), /out of date/);
  }
  assert.match(tix.staleNotice(true), /15 seconds/);
  assert.match(tix.staleNotice(false), /reconnecting/);
});

test("wantsEvent keeps a one-record screen off its neighbours' changes", () => {
  const mine = { subject_id: "task-1" };
  const comment = { subject_id: "comment-9", payload: { task_id: "task-1" } };
  const other = { subject_id: "task-2", payload: { task_id: "task-2" } };

  assert.equal(tix.wantsEvent("task-1", mine), true);
  assert.equal(tix.wantsEvent("task-1", comment), true, "a comment on this task was ignored");
  assert.equal(tix.wantsEvent("task-1", other), false, "another task's change rebuilt the screen");
  assert.equal(tix.wantsEvent("", other), true, "a listing turned an event away");
  assert.equal(tix.wantsEvent("task-1", null), false);
});

test("fieldDirty reads a control against what the server rendered into it", () => {
  assert.equal(tix.fieldDirty({ tagName: "INPUT", value: "same", defaultValue: "same" }), false);
  assert.equal(tix.fieldDirty({ tagName: "INPUT", value: "typed", defaultValue: "" }), true);
  assert.equal(tix.fieldDirty({ tagName: "INPUT", type: "checkbox", checked: true, defaultChecked: false }), true);
  assert.equal(tix.fieldDirty({ tagName: "INPUT", type: "checkbox", checked: false, defaultChecked: false }), false);
  assert.equal(tix.fieldDirty({
    tagName: "SELECT",
    options: [{ selected: true, defaultSelected: true }, { selected: false, defaultSelected: false }]
  }), false);
  assert.equal(tix.fieldDirty({
    tagName: "SELECT",
    options: [{ selected: false, defaultSelected: true }, { selected: true, defaultSelected: false }]
  }), true);
  assert.equal(tix.fieldDirty(null), false);
});

// A select whose markup names no selected option is the board's Move control,
// and reading the browser's own pick as the reader's froze every board for as
// long as it was open: the region never stopped being busy, so the refresh
// deferred forever and the board sat on its first paint.
test("fieldDirty does not read the browser's own pick as the reader's", () => {
  const untouched = {
    tagName: "SELECT",
    selectedIndex: 0,
    options: [
      { selected: true, defaultSelected: false },
      { selected: false, defaultSelected: false },
    ],
  };
  assert.equal(tix.fieldDirty(untouched), false,
    "an untouched select with no declared default was reported dirty");

  const chosen = {
    tagName: "SELECT",
    selectedIndex: 1,
    options: [
      { selected: false, defaultSelected: false },
      { selected: true, defaultSelected: false },
    ],
  };
  assert.equal(tix.fieldDirty(chosen), true,
    "a select the reader moved off the first option was reported clean");

  // A declared default is still the thing a choice is read against, so a
  // select sitting on an option that is not its default is dirty even though
  // it is the first one.
  const declared = {
    tagName: "SELECT",
    selectedIndex: 0,
    options: [
      { selected: true, defaultSelected: false },
      { selected: false, defaultSelected: true },
    ],
  };
  assert.equal(tix.fieldDirty(declared), true,
    "a select with a declared default was not read against it");
});

// A <select multiple> declares no default and the browser picks nothing for
// it, so selectedIndex starts at -1 rather than 0. Under the single-select
// rule the first option was the browser's pick, so a reader who selected only
// the first scope on the token form was reported as having changed nothing.
test("fieldDirty reads the first option of a multiple select as a choice", () => {
  const empty = {
    tagName: "SELECT",
    multiple: true,
    selectedIndex: -1,
    options: [
      { selected: false, defaultSelected: false },
      { selected: false, defaultSelected: false },
    ],
  };
  assert.equal(tix.fieldDirty(empty), false,
    "a multiple select nobody has touched was reported dirty");

  const first = {
    tagName: "SELECT",
    multiple: true,
    selectedIndex: 0,
    options: [
      { selected: true, defaultSelected: false },
      { selected: false, defaultSelected: false },
    ],
  };
  assert.equal(tix.fieldDirty(first), true,
    "picking only the first option of a multiple select was reported clean");

  // A single select with nothing selected at all is still nothing the reader
  // chose, so the -1 that reaches the same branch is not a choice there.
  const none = {
    tagName: "SELECT",
    selectedIndex: -1,
    options: [
      { selected: false, defaultSelected: false },
      { selected: false, defaultSelected: false },
    ],
  };
  assert.equal(tix.fieldDirty(none), false,
    "a select with nothing selected was reported dirty");
});

test("dragEnabled needs both the setting and a fine pointer", () => {
  const fine = () => ({ matches: true });
  const coarse = () => ({ matches: false });
  assert.equal(tix.dragEnabled("1", fine), true);
  assert.equal(tix.dragEnabled("1", coarse), false, "drag was attached on a coarse pointer");
  assert.equal(tix.dragEnabled("0", fine), false);
  assert.equal(tix.dragEnabled(null, fine), false);
  assert.equal(tix.dragEnabled("1", undefined), false, "a browser without matchMedia got drag");
  assert.equal(tix.dragEnabled("1", () => null), false);
});

test("dragEnabled asks for the fine pointer query", () => {
  const asked = [];
  tix.dragEnabled("1", (q) => {
    asked.push(q);
    return { matches: true };
  });
  assert.deepEqual(asked, ["(pointer: fine)"]);
});

test("legalStates reads the card's own move options", () => {
  const legal = tix.legalStates([{ value: "doing" }, { value: "done" }]);
  assert.equal(tix.canDrop(legal, "doing"), true);
  assert.equal(tix.canDrop(legal, "done"), true);
  assert.equal(tix.canDrop(legal, "blocked"), false, "a column outside the card's options accepted a drop");
});

test("canDrop refuses a state that only looks like a member", () => {
  const legal = tix.legalStates([{ value: "doing" }]);
  for (const state of ["constructor", "toString", "__proto__", "hasOwnProperty", "valueOf"]) {
    assert.equal(tix.canDrop(legal, state), false, state);
  }
});

test("canDrop refuses an empty or absent state", () => {
  const legal = tix.legalStates([{ value: "doing" }]);
  assert.equal(tix.canDrop(legal, ""), false);
  assert.equal(tix.canDrop(legal, null), false);
  assert.equal(tix.canDrop(null, "doing"), false, "a card with no options accepted a drop");
  assert.equal(tix.canDrop(tix.legalStates(null), "doing"), false);
  assert.equal(tix.canDrop(tix.legalStates([{ value: "" }]), ""), false);
});

test("stripCount turns a column heading into its state name", () => {
  assert.equal(tix.stripCount("In progress (3)"), "In progress");
  assert.equal(tix.stripCount("Done (12) "), "Done");
  assert.equal(tix.stripCount("Backlog"), "Backlog");
  assert.equal(tix.stripCount("Review (beta) (0)"), "Review (beta)");
  assert.equal(tix.stripCount(null), "");
});

test("a confirmed move and a refused one read differently", () => {
  assert.equal(tix.movedMessage("TIX-7", "Done"), "TIX-7 moved to Done.");
  assert.equal(
    tix.refusedMessage("TIX-7", "Done", "someone else changed it."),
    "TIX-7 was not moved to Done: someone else changed it."
  );
});

test("a refusal with no explanation still says the card did not move", () => {
  for (const reason of ["", "   ", null, undefined]) {
    assert.equal(
      tix.refusedMessage("TIX-7", "Done", reason),
      "TIX-7 was not moved to Done: the move was refused."
    );
  }
});

test("copyText uses the async clipboard when there is one", async () => {
  let written = null;
  const outcome = await new Promise((resolve) => {
    tix.copyText("hello", { clipboard: { writeText: (t) => { written = t; return Promise.resolve(); } } }, resolve);
  });
  assert.equal(written, "hello");
  assert.equal(outcome, true);
});

test("copyText reports a refused clipboard rather than throwing", async () => {
  const outcome = await new Promise((resolve) => {
    tix.copyText("hello", { clipboard: { writeText: () => Promise.reject(new Error("denied")) } }, resolve);
  });
  assert.equal(outcome, false);
});

test("copyText survives a clipboard that throws synchronously", () => {
  let outcome = "not called";
  assert.doesNotThrow(() => {
    tix.copyText("hello", { clipboard: { writeText: () => { throw new Error("boom"); } } }, (ok) => { outcome = ok; });
  });
  assert.equal(outcome, false);
});

test("copyText falls back when there is no async clipboard", () => {
  const seen = [];
  const outcomes = [];
  for (const clipboard of [undefined, null, {}, { writeText: "not a function" }]) {
    tix.copyText("hello", { clipboard, legacy: (t) => { seen.push(t); return true; } }, (ok) => outcomes.push(ok));
  }
  assert.deepEqual(seen, ["hello", "hello", "hello", "hello"]);
  assert.deepEqual(outcomes, [true, true, true, true]);
});

test("copyText survives a fallback that fails or throws", () => {
  const outcomes = [];
  assert.doesNotThrow(() => {
    tix.copyText("x", { legacy: () => false }, (ok) => outcomes.push(ok));
    tix.copyText("x", { legacy: () => { throw new Error("no"); } }, (ok) => outcomes.push(ok));
    tix.copyText("x", {}, (ok) => outcomes.push(ok));
    tix.copyText("x", null, (ok) => outcomes.push(ok));
  });
  assert.deepEqual(outcomes, [false, false, false, false]);
});

test("copyText with no callback still does not throw", () => {
  assert.doesNotThrow(() => tix.copyText("x", { legacy: () => true }));
  assert.doesNotThrow(() => tix.copyText("x", { clipboard: { writeText: () => { throw new Error("boom"); } } }));
});

test("fileChoiceLabel reads out what a styled file field holds", () => {
  assert.equal(tix.fileChoiceLabel([{ name: "bundle.ndjson" }]), "bundle.ndjson");
  assert.equal(tix.fileChoiceLabel([{ name: "a" }, { name: "b" }]), "2 files chosen");
  assert.equal(tix.fileChoiceLabel([]), "No file chosen");
  assert.equal(tix.fileChoiceLabel(null), "No file chosen");
  assert.equal(tix.fileChoiceLabel(undefined), "No file chosen");
});
