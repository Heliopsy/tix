// SPDX-License-Identifier: AGPL-3.0-or-later

// assets/busy.js, the part that applies a dropdown when it is chosen: that a
// form drawn by an in-place swap is wired too, and that one already wired is
// not wired twice. The DOM is a stub.
import test from "node:test";
import assert from "node:assert/strict";
import { bareContext, run, fakeDocument, element } from "./harness.mjs";

function applyForm() {
  const select = element("SELECT");
  select.listeners = 0;
  select.addEventListener = () => { select.listeners++; };
  const apply = element("BUTTON");
  const form = element("FORM", { "data-apply": "" });
  form.querySelector = (sel) => (sel === "select" ? select : sel === "button.apply" ? apply : null);
  return { form, select, apply };
}

test("a dropdown form drawn by a swap applies on change, and is wired once", () => {
  const first = applyForm();
  const forms = [first.form];
  const doc = fakeDocument({ bySelectorAll: { "form[data-dirty-save]": [], "form[data-apply]": forms } });
  doc.readyState = "complete";
  const ctx = bareContext({ setTimeout() {}, addEventListener() {} });
  ctx.document = doc;
  run(ctx, "busy.js");
  assert.equal(first.select.listeners, 1, "the form on the page was not wired");
  assert.equal(first.apply.hidden, true, "the Apply button the script replaces is still shown");

  const swapped = applyForm();
  forms.push(swapped.form);
  doc.dispatch("htmx:afterSwap", {});
  assert.equal(swapped.select.listeners, 1, "the form drawn by the swap was not wired");
  assert.equal(first.select.listeners, 1, "the form already wired was wired again");
});

// The pressed button says it is waiting in the same frame as the press, not
// a frame or a transition later, and a second press while it waits submits
// nothing: the button is only disabled a task after the first.
test("a pressed button is pending at once and a second press is swallowed", () => {
  const classes = new Set();
  const button = {
    tagName: "BUTTON",
    disabled: false,
    textContent: "Add",
    firstElementChild: null,
    attrs: {},
    classList: {
      add: (...c) => c.forEach((x) => classes.add(x)),
      remove: (...c) => c.forEach((x) => classes.delete(x)),
      contains: (c) => classes.has(c),
    },
    setAttribute(n, v) { this.attrs[n] = v; },
    removeAttribute(n) { delete this.attrs[n]; },
  };
  const formAttrs = {};
  const form = {
    tagName: "FORM",
    hasAttribute: () => false,
    setAttribute: (n, v) => { formAttrs[n] = v; },
    getAttribute: (n) => formAttrs[n] ?? null,
    querySelector: () => button,
    classList: { add() {}, remove() {} },
  };
  const doc = fakeDocument({ bySelectorAll: { "form[data-dirty-save]": [], "form[data-apply]": [] } });
  doc.readyState = "complete";
  const later = [];
  const ctx = bareContext({ setTimeout: (fn) => later.push(fn), addEventListener() {} });
  ctx.document = doc;
  run(ctx, "busy.js");

  const first = { target: form, submitter: button, prevented: false, stopped: false,
    preventDefault() { this.prevented = true; }, stopImmediatePropagation() { this.stopped = true; this.immediateStopped = true; } };
  doc.dispatch("submit", first);
  assert.ok(classes.has("is-busy"), "the button was not marked pending during the press");
  assert.equal(button.attrs["aria-busy"], "true", "the button does not say it is busy");
  assert.equal(first.prevented, false, "the first press was refused");
  assert.equal(button.disabled, false, "the button was disabled before the form had read it");

  const second = { target: form, submitter: button, prevented: false, stopped: false,
    preventDefault() { this.prevented = true; }, stopImmediatePropagation() { this.stopped = true; this.immediateStopped = true; } };
  doc.dispatch("submit", second);
  assert.ok(second.prevented && second.stopped, "a second press on a pending button submitted again");

  later.forEach((fn) => fn());
  assert.equal(button.disabled, true, "the pending button was never disabled");
});
