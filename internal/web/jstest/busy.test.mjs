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
