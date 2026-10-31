// SPDX-License-Identifier: AGPL-3.0-or-later

// The decisions assets/feel.js makes about acting in place, following links on press and
// cross-fading. The wiring under them needs a browser and is checked there;
// these are the rules it follows.
import test from "node:test";
import assert from "node:assert/strict";
import { newContext, run } from "./harness.mjs";

const ctx = newContext();
run(ctx, "feel.js");
const tix = ctx.tix;
const plain = (value) => JSON.parse(JSON.stringify(value));

test("feel.js loads without a document and adds only decisions", () => {
  assert.equal(typeof tix.inplaceSpec, "function");
  assert.equal(typeof tix.goesOnPress, "function");
});

test("inplaceSpec swaps the region a form sits in, and nothing else", () => {
  assert.deepEqual(plain(tix.inplaceSpec("", "t-infra-3")), {
    target: "#t-infra-3", select: "#t-infra-3", swap: "outerHTML show:none transition:false",
  });
  assert.equal(tix.inplaceSpec("main.wrap", "").target, "main.wrap");
  assert.equal(tix.inplaceSpec("main.wrap", "ignored").select, "main.wrap");
  assert.equal(tix.inplaceSpec("", ""), null, "a region that cannot be found again was armed");
});

test("inplaceSpec never scrolls the page and never fades it", () => {
  const swap = tix.inplaceSpec("", "t-1").swap;
  assert.match(swap, /show:none/);
  assert.match(swap, /transition:false/);
});

const page = { origin: "http://h", pathname: "/tasks", search: "?q=tag:ops" };
const link = (href) => {
  const u = new URL(href, "http://h");
  return { origin: u.origin, pathname: u.pathname, search: u.search };
};

test("goesOnPress takes the screens a reader opens from a list", () => {
  for (const href of ["/tasks/infra-3?next=%2Ftasks", "/tasks?q=tag:ops&cursor=x", "/projects/infra", "/activity"]) {
    assert.equal(tix.goesOnPress(link(href), page), true, href);
  }
});

test("goesOnPress leaves a link on a draggable card to its click", () => {
  const onCard = (draggable) => ({
    ...link("/tasks/infra-3"),
    closest: (sel) => (draggable && sel === '[draggable="true"]' ? {} : null),
  });
  assert.equal(tix.goesOnPress(onCard(true), page), false, "a press on a draggable card opened the task");
  assert.equal(tix.goesOnPress(onCard(false), page), true);
});

test("goesOnPress leaves alone what is not a screen, not ours, or already open", () => {
  for (const href of [
    "http://elsewhere/tasks/1", "/tasks?q=tag:ops", "/tasks/live", "/tasks/infra-3/live",
    "/assets/app.css", "/settings", "/logout", "/export",
  ]) {
    assert.equal(tix.goesOnPress(link(href), page), false, href);
  }
  assert.equal(tix.goesOnPress(null, page), false);
});

test("statusFromRoute reads where a route of states ends", () => {
  assert.equal(tix.statusFromRoute("doing>review>done"), "done");
  assert.equal(tix.statusFromRoute("done"), "done");
  assert.equal(tix.statusFromRoute(""), "");
});

test("viewTransitions is off for reduced motion and where unsupported", () => {
  const can = { startViewTransition() {} };
  const media = (reduce) => (q) => ({ matches: q === "(prefers-reduced-motion: reduce)" && reduce });
  assert.equal(tix.viewTransitions(can, media(false)), true);
  assert.equal(tix.viewTransitions(can, media(true)), false, "a reader who asked for less motion got a fade");
  assert.equal(tix.viewTransitions({}, media(false)), false);
  assert.equal(tix.viewTransitions(null, media(false)), false);
});
