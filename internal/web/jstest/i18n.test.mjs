// SPDX-License-Identifier: AGPL-3.0-or-later

// The scripts' own words: assets/i18n.js.
import test from "node:test";
import assert from "node:assert/strict";
import { bareContext, run } from "./harness.mjs";

function withTable(text) {
  const document = {
    getElementById: (id) => (id === "tix-i18n" && text !== null ? { textContent: text } : null),
  };
  const ctx = bareContext({ document });
  run(ctx, "i18n.js");
  return ctx;
}

test("a page with no table speaks English", () => {
  const ctx = withTable(null);
  assert.equal(ctx.tix.t("Copied"), "Copied");
});

test("a table translates what it holds and nothing else", () => {
  const ctx = withTable(JSON.stringify({ Copied: "Копирано" }));
  assert.equal(ctx.tix.t("Copied"), "Копирано");
  assert.equal(ctx.tix.t("Copy failed"), "Copy failed");
  assert.equal(ctx.tix.t("toString"), "toString");
});

test("an unreadable table is no table", () => {
  const ctx = withTable("{not json");
  assert.equal(ctx.tix.t("Copied"), "Copied");
  assert.deepEqual(Object.keys(ctx.tix.messages(undefined)), []);
});

test("the file label reads the table", () => {
  const ctx = withTable(JSON.stringify({ "%d files chosen": "Избрани файлове: %d", "No file chosen": "Няма избран файл" }));
  run(ctx, "decide.js");
  assert.equal(ctx.tix.fileChoiceLabel([]), "Няма избран файл");
  assert.equal(ctx.tix.fileChoiceLabel([{ name: "a" }, { name: "b" }]), "Избрани файлове: 2");
});
