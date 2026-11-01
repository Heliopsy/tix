// SPDX-License-Identifier: AGPL-3.0-or-later

// assets/forms.js: the key a new project is offered, and what Ctrl+Enter and
// Escape do inside a create or settings form. The DOM is a stub.
import test from "node:test";
import assert from "node:assert/strict";
import { bareContext, fakeTimers, run } from "./harness.mjs";

function load() {
  const location = { assigned: [], assign(href) { this.assigned.push(href); } };
  const ctx = bareContext({ location, navigator: { platform: "Linux" } });
  run(ctx, "forms.js");
  return { forms: ctx.tixForms, location };
}

// node is an attribute bag that answers closest from a fixed table.
function node(attrs = {}, parents = {}) {
  return {
    value: "",
    placeholder: "",
    textContent: "",
    attrs: { ...attrs },
    hasAttribute(k) { return k in this.attrs; },
    getAttribute(k) { return k in this.attrs ? this.attrs[k] : null; },
    setAttribute(k, v) { this.attrs[k] = v; },
    removeAttribute(k) { delete this.attrs[k]; },
    closest(sel) { return parents[sel] || null; },
  };
}

function form({ popover = false, dialog = false, details = null, cancel = null, inside = {} } = {}) {
  const f = {
    submitted: 0,
    requestSubmit() { this.submitted++; },
    closest(sel) {
      if (sel === "form[data-form]") return this;
      if (sel === "[popover]") return popover ? {} : null;
      if (sel === "dialog") return dialog ? {} : null;
      if (sel === "details[open]") return details;
      return null;
    },
    querySelector(sel) {
      if (sel === "[data-form-cancel]") return cancel;
      return inside[sel] || null;
    },
  };
  return f;
}

function key(target, k, mods = {}) {
  return {
    target, key: k, ctrlKey: !!mods.ctrl, metaKey: !!mods.meta, defaultPrevented: false, prevented: 0,
    preventDefault() { this.prevented++; this.defaultPrevented = true; },
  };
}

test("a key is the first word of the name, in lower case", () => {
  const { forms } = load();
  assert.equal(forms.suggestKey("Data Platform", []), "data");
  assert.equal(forms.suggestKey("  Web UI", []), "web");
  assert.equal(forms.suggestKey("2026 roadmap", []), "roadmap");
  assert.equal(forms.suggestKey("Infrastructureandmore", []), "infrastructu");
  assert.equal(forms.suggestKey("Café crème", []), "cafe");
  assert.equal(forms.suggestKey("Градина", []), "gradina");
  assert.equal(forms.suggestKey("!!!", []), "");
});

test("a key already in use gets a number", () => {
  const { forms } = load();
  assert.equal(forms.suggestKey("Web shop", ["web"]), "web2");
  assert.equal(forms.suggestKey("Web shop", ["WEB", "web2"]), "web3");
});

test("typing a name fills the key until the reader types one", () => {
  const { forms } = load();
  const target = node({ "data-key-target": "", "data-taken": "infra web" });
  const shown = node();
  const f = form({ inside: { "[data-key-target]": target, "[data-key-preview]": shown } });
  const source = node({ "data-key-source": "" }, { "form[data-form]": f });
  target.closest = (sel) => (sel === "form[data-form]" ? f : null);

  source.value = "Web shop";
  forms.onInput({ target: source });
  assert.equal(target.value, "web2");
  assert.equal(shown.textContent, "web2-1");

  target.value = "shop";
  forms.onInput({ target });
  source.value = "Web shop two";
  forms.onInput({ target: source });
  assert.equal(target.value, "shop", "an edited key was overwritten");
  assert.equal(shown.textContent, "shop-1");

  target.value = "";
  forms.onInput({ target });
  forms.onInput({ target: source });
  assert.equal(target.value, "web2", "clearing the key did not hand it back to the name");
});

test("Ctrl+Enter and Cmd+Enter submit the form", () => {
  const { forms } = load();
  for (const mods of [{ ctrl: true }, { meta: true }]) {
    const f = form();
    const ev = key(node({}, { "form[data-form]": f }), "Enter", mods);
    forms.onKeydown(ev);
    assert.equal(f.submitted, 1);
    assert.equal(ev.prevented, 1);
  }
  const f = form();
  forms.onKeydown(key(node({}, { "form[data-form]": f }), "Enter"));
  assert.equal(f.submitted, 0, "a plain Enter submitted");
});

test("Escape follows Cancel in place, closes a disclosure, and leaves a popover alone", () => {
  const { forms, location } = load();

  const inPlace = form({ cancel: { href: "/projects" } });
  forms.onKeydown(key(node({}, { "form[data-form]": inPlace }), "Escape"));
  assert.deepEqual(location.assigned, ["/projects"]);

  const summary = { focused: 0, focus() { this.focused++; } };
  const details = { open: true, querySelector: () => summary };
  const folded = form({ details, cancel: { href: "/projects/web" } });
  forms.onKeydown(key(node({}, { "form[data-form]": folded }), "Escape"));
  assert.equal(details.open, false);
  assert.equal(summary.focused, 1);
  assert.equal(location.assigned.length, 1, "closing the disclosure also navigated");

  const pop = form({ popover: true, cancel: { href: "/tasks" } });
  const ev = key(node({}, { "form[data-form]": pop }), "Escape");
  forms.onKeydown(ev);
  assert.equal(ev.prevented, 0);
  assert.equal(location.assigned.length, 1);
});

test("Escape in a form drawn in the task dialog is the dialog's, which closes it", () => {
  const { forms, location } = load();
  const inDialog = form({ dialog: true, cancel: { href: "/tasks/infra-1" } });
  const ev = key(node({}, { "form[data-form]": inDialog }), "Escape");
  forms.onKeydown(ev);
  assert.equal(ev.prevented, 0, "the dialog's own Escape was taken away");
  assert.deepEqual(location.assigned, [], "Escape left the page instead of closing the dialog");
});

// quickContext loads forms.js with a hand-driven clock, a fetch that answers
// when the test says so, and one preview row.
function quickContext() {
  const timers = fakeTimers();
  const row = { innerHTML: "old" };
  const calls = [];
  const fetch = (url, opts) => new Promise((resolve) => {
    calls.push({ url, opts, answer(html, ok = true) { resolve({ ok, text: () => Promise.resolve(html) }); } });
  });
  const document = { getElementById: (id) => (id === "quickadd-preview" ? row : null) };
  const ctx = bareContext({ document, fetch, setTimeout: timers.setTimeout, clearTimeout: timers.clearTimeout,
    encodeURIComponent, navigator: { platform: "Linux" } });
  run(ctx, "forms.js");
  return { forms: ctx.tixForms, timers, row, calls };
}

function quickBox() {
  return node({ "data-quickadd": "/tasks/quickadd", "data-quickadd-target": "quickadd-preview" }, {});
}

const settle = () => new Promise((resolve) => setImmediate(resolve));

test("the quick-add preview asks once the reader pauses, for the line as typed", async () => {
  const { forms, timers, row, calls } = quickContext();
  const box = quickBox();
  box.value = "Fix";
  forms.onInput({ target: box });
  box.value = "Fix #auth & co";
  forms.onInput({ target: box });
  assert.equal(timers.count, 1, "typing again replaces the pending request");
  assert.deepEqual(timers.delays(), [200]);
  assert.equal(calls.length, 0, "nothing is asked before the pause");
  timers.fire();
  assert.equal(calls.length, 1);
  assert.equal(calls[0].url, "/tasks/quickadd?title=Fix%20%23auth%20%26%20co");
  assert.equal(calls[0].opts.credentials, "same-origin");
  calls[0].answer("<ul>chips</ul>");
  await settle();
  assert.equal(row.innerHTML, "<ul>chips</ul>");
});

test("a slow answer never overwrites a newer one", async () => {
  const { forms, timers, row, calls } = quickContext();
  const box = quickBox();
  box.value = "first";
  forms.onInput({ target: box });
  timers.fire();
  box.value = "second";
  forms.onInput({ target: box });
  timers.fire();
  calls[1].answer("second");
  await settle();
  calls[0].answer("first");
  await settle();
  assert.equal(row.innerHTML, "second");
});

test("an emptied box clears the row without asking, and a failed answer leaves it", async () => {
  const { forms, timers, row, calls } = quickContext();
  const box = quickBox();
  box.value = "x";
  forms.onInput({ target: box });
  timers.fire();
  calls[0].answer("broken", false);
  await settle();
  assert.equal(row.innerHTML, "old");
  box.value = "   ";
  forms.onInput({ target: box });
  timers.fire();
  assert.equal(calls.length, 1);
  assert.equal(row.innerHTML, "");
});
