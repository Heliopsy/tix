// SPDX-License-Identifier: AGPL-3.0-or-later

// assets/richtext.js over the vendored assets/prosemirror.min.js: what the
// rich view gives back for a text, and which texts it refuses to open. The
// corpus is shared with internal/markdown, which renders each pair through
// the server's own renderer.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { bareContext, run } from "./harness.mjs";

const corpus = JSON.parse(readFileSync(new URL("./testdata/richtext-corpus.json", import.meta.url), "utf8"));

function load() {
  const ctx = bareContext({});
  run(ctx, "prosemirror.min.js");
  run(ctx, "richtext.js");
  return { rich: ctx.tixRich, codec: ctx.tixRich.codec(ctx.tixPM) };
}

const { rich, codec } = load();

test("the corpus covers the constructs the editor promises to keep", () => {
  const all = corpus.keep.map((e) => e.in).join("\n");
  for (const needle of ["| :---", "- [ ]", "- [x]", "  - ", "```", "INFRA-8", "@alice", "<script>", "<b>", "https://", "![", "> "]) {
    assert.ok(all.includes(needle), `no corpus entry holds ${needle}`);
  }
});

for (const entry of corpus.keep) {
  test(`kept: ${entry.name}`, () => {
    const got = codec.check(entry.in);
    assert.equal(got.ok, true, `refused (${got.reason}), so this text could not be opened in the rich view`);
    assert.equal(got.markdown, entry.out);
    const again = codec.check(entry.out);
    assert.equal(again.ok, true);
    assert.equal(again.markdown, entry.out, "a second trip through the editor changed the text again");
  });
}

for (const entry of corpus.source) {
  test(`stays in Markdown: ${entry.name}`, () => {
    const got = codec.check(entry.in);
    assert.equal(got.ok, false, "opened in the rich view, which would lose it");
    assert.equal(got.reason, entry.reason);
  });
}

test("a checklist item keeps its box, ticked or not, through the document", () => {
  const doc = codec.parse("- [ ] open\n- [x] done\n- plain");
  const items = [];
  doc.descendants((node) => {
    if (node.type.name === "list_item") items.push(node.attrs.checked);
  });
  assert.deepEqual(items, [false, true, null]);
});

test("an image is never drawn as an image", () => {
  const spec = codec.schema.nodes.image.spec.toDOM(codec.schema.nodes.image.create({ src: "http://x/y.png", alt: "y" }));
  assert.equal(spec[0], "span");
});

test("a link to anything but http, https or mailto carries no address in the editor", () => {
  const link = codec.schema.marks.link;
  assert.equal(link.spec.toDOM(link.create({ href: "javascript:alert(1)" }))[1].href, undefined);
  assert.equal(link.spec.toDOM(link.create({ href: "https://x.y" }))[1].href, "https://x.y");
});

test("the mode is remembered per browser, and a refusing store means Markdown", () => {
  const saved = {};
  const store = { getItem: (k) => saved[k] ?? null, setItem: (k, v) => { saved[k] = v; } };
  assert.equal(rich.storedMode(store), "source");
  rich.storeMode(store, "rich");
  assert.equal(saved[rich.MODE_KEY], "rich");
  assert.equal(rich.storedMode(store), "rich");
  const refusing = { getItem() { throw new Error("denied"); }, setItem() { throw new Error("denied"); } };
  assert.equal(rich.storedMode(refusing), "source");
  assert.doesNotThrow(() => rich.storeMode(refusing, "rich"));
  assert.equal(rich.storedMode(null), "source");
});
