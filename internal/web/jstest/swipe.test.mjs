// SPDX-License-Identifier: AGPL-3.0-or-later

// Swiping a task list row: the decisions in assets/decide.js and the stand
// down in assets/swipe.js.
import test from "node:test";
import assert from "node:assert/strict";
import { newContext, bareContext, run } from "./harness.mjs";

const tix = newContext().tix;
const both = { complete: true, snooze: true };
const plain = (value) => JSON.parse(JSON.stringify(value));

test("a finger or a pen swipes; a mouse does not", () => {
  assert.equal(tix.swipesWith("touch"), true);
  assert.equal(tix.swipesWith("pen"), true);
  assert.equal(tix.swipesWith("mouse"), false);
  assert.equal(tix.swipesWith(""), false);
  assert.equal(tix.swipesWith(undefined), false);
});

test("a move under the slop is still a tap", () => {
  assert.equal(tix.swipeAxis(0, 0, 10), "");
  assert.equal(tix.swipeAxis(9, -9, 10), "");
});

test("a sideways move is a swipe and anything steeper is a scroll", () => {
  assert.equal(tix.swipeAxis(12, 3, 10), "x");
  assert.equal(tix.swipeAxis(-30, 10, 10), "x");
  assert.equal(tix.swipeAxis(3, 12, 10), "y");
  assert.equal(tix.swipeAxis(2, -40, 10), "y");
  // A perfect diagonal is left to the browser, so the list keeps scrolling.
  assert.equal(tix.swipeAxis(20, 20, 10), "y");
});

test("towards the line's end completes, towards its start snoozes", () => {
  assert.equal(tix.swipeSide(40, false, both), "complete");
  assert.equal(tix.swipeSide(-40, false, both), "snooze");
  assert.equal(tix.swipeSide(40, true, both), "snooze");
  assert.equal(tix.swipeSide(-40, true, both), "complete");
  assert.equal(tix.swipeSide(0, false, both), "");
});

test("a side the row does not offer reveals nothing", () => {
  assert.equal(tix.swipeSide(40, false, { complete: false, snooze: true }), "");
  assert.equal(tix.swipeSide(-40, false, { complete: true, snooze: false }), "");
  assert.equal(tix.swipeSide(40, false, null), "");
});

test("the action runs only past 30% of the row's width", () => {
  assert.equal(tix.swipeThreshold, 0.3);
  assert.equal(tix.swipeAction(119, 400, false, both), "");
  assert.equal(tix.swipeAction(120, 400, false, both), "complete");
  assert.equal(tix.swipeAction(-119, 400, false, both), "");
  assert.equal(tix.swipeAction(-200, 400, false, both), "snooze");
  assert.equal(tix.swipeAction(-200, 400, true, both), "complete");
});

test("released past the threshold towards a missing action does nothing", () => {
  assert.equal(tix.swipeAction(300, 400, false, { complete: false, snooze: true }), "");
  assert.equal(tix.swipeAction(-300, 400, false, { complete: true, snooze: false }), "");
});

test("a row of no width never acts", () => {
  assert.equal(tix.swipeAction(300, 0, false, both), "");
  assert.equal(tix.swipeAction(300, undefined, false, both), "");
});

test("the row follows the finger, never past its width, and not towards a missing action", () => {
  assert.equal(tix.swipeOffset(80, 400, false, both), 80);
  assert.equal(tix.swipeOffset(-80, 400, false, both), -80);
  assert.equal(tix.swipeOffset(900, 400, false, both), 400);
  assert.equal(tix.swipeOffset(-900, 400, false, both), -400);
  assert.equal(tix.swipeOffset(80, 400, false, { complete: false, snooze: true }), 0);
  assert.equal(tix.swipeOffset(-80, 400, false, { complete: true, snooze: false }), 0);
});

test("a complete posts the token and the page to come back to", () => {
  assert.deepEqual(plain(tix.swipeFields("complete", "tok", "/tasks?q=x", "tomorrow")),
    [["csrf_token", "tok"], ["next", "/tasks?q=x"]]);
});

test("a snooze adds the wake time the row names", () => {
  assert.deepEqual(plain(tix.swipeFields("snooze", "tok", "/tasks", "tomorrow")),
    [["csrf_token", "tok"], ["next", "/tasks"], ["until", "tomorrow"]]);
  assert.deepEqual(plain(tix.swipeFields("snooze")),
    [["csrf_token", ""], ["next", ""], ["until", ""]]);
});

test("swipe.js stands down without decide.js or a document", () => {
  const bare = bareContext({ document: { addEventListener() { throw new Error("must not listen"); } } });
  run(bare, "swipe.js");
  const ctx = newContext();
  run(ctx, "swipe.js");
});

test("swipe.js listens for pointers at the document, not on rows", () => {
  const events = [];
  const ctx = newContext({ document: { addEventListener(name) { events.push(name); } } });
  run(ctx, "swipe.js");
  for (const name of ["pointerdown", "pointermove", "pointerup", "pointercancel", "click"]) {
    assert.ok(events.includes(name), `no ${name} listener`);
  }
});
