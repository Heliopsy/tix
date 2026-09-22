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

test("debouncer collapses a burst into one run after the quiet period", () => {
  const timers = fakeTimers();
  let runs = 0;
  const trigger = tix.debouncer(300, () => runs++, timers);

  trigger();
  trigger();
  trigger();
  assert.equal(runs, 0, "it ran before the quiet period elapsed");
  assert.equal(timers.count, 1, "an earlier timer was left armed");
  assert.deepEqual(timers.delays(), [300]);

  timers.fire();
  assert.equal(runs, 1, "a burst of three did not group into one refresh");
});

test("debouncer arms again after it has run", () => {
  const timers = fakeTimers();
  let runs = 0;
  const trigger = tix.debouncer(300, () => runs++, timers);

  trigger();
  timers.fire();
  trigger();
  timers.fire();
  assert.equal(runs, 2);
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
