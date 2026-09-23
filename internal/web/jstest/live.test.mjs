// SPDX-License-Identifier: AGPL-3.0-or-later

import test from "node:test";
import assert from "node:assert/strict";
import { newContext, run, fakeDocument, domNode } from "./harness.mjs";

// The layout boosts every internal navigation (hx-boost on <body>), so the
// scripts in <head> run once, against whichever page a visitor first opened.
// A board reached by clicking is therefore a board that did not exist when
// live.js ran. These tests hold the line that made drag-and-drop work only
// for somebody who typed the URL: the handlers must be on the document, and
// the board must be resolved from the event.

// board builds the part of a project board the drag handlers actually read:
// a board with a data-drag gate, columns naming their state, and one card
// whose Move disclosure names the states that card may move to.
function board({ drag = "1", from = "todo", targets = ["doing"] } = {}) {
  const select = domNode(["form select[name=to]", "select[name=to]"], {
    options: targets.map((t) => ({ value: t })),
  });
  const form = domNode(["form"], { action: "/projects/infra/move", fields: { ref: "infra-1" } }, [select]);
  const card = domNode([".card"], { "data-task": "infra-1" }, [form]);

  const columns = [];
  const fromColumn = domNode([".column"], { "data-state": from }, [card]);
  columns.push(fromColumn);
  for (const state of targets) {
    const heading = domNode(["h2"], { textContent: state + " (0)" });
    columns.push(domNode([".column"], { "data-state": state }, [heading]));
  }
  const root = domNode([".board"], { "data-drag": drag }, columns);
  return { root, card, columns, select, form };
}

// context loads decide.js and live.js against a document that has no board on
// it, which is the situation the file is always in when the browser runs it.
function context() {
  const fetches = [];
  const doc = fakeDocument();
  const ctx = newContext({
    document: doc,
    matchMedia: () => ({ matches: true }),
    location: { protocol: "http:", host: "localhost", pathname: "/projects/infra" },
    WebSocket: function () { throw new Error("no feed on this page"); },
    DOMParser: function () { return { parseFromString: () => ({ querySelector: () => null }) }; },
    FormData: function (form) { return form.fields; },
    URLSearchParams: function (fields) {
      this.toString = () =>
        Object.keys(fields).map((k) => k + "=" + fields[k]).join("&");
    },
    fetch: (url, opts) => {
      fetches.push({ url, body: opts && opts.body });
      return Promise.resolve({ ok: true, text: () => Promise.resolve("") });
    },
  });
  run(ctx, "live.js");
  return { ctx, doc, fetches };
}

// drag plays the three events a browser fires, in order, at the document.
function drag(doc, { card, column, dataTransfer = { setData() {} } }) {
  const ev = (target) => ({
    target,
    dataTransfer,
    prevented: 0,
    preventDefault() { this.prevented++; },
  });
  const start = doc.dispatch("dragstart", ev(card));
  const over = doc.dispatch("dragover", ev(column));
  const drop = doc.dispatch("drop", ev(column));
  return { start, over, drop };
}

test("the drag handlers are on the document, not on a board captured at load", () => {
  const { doc } = context();
  for (const type of ["dragstart", "dragover", "drop", "dragend"]) {
    assert.equal(
      doc.listenerCount(type),
      1,
      `no ${type} handler: a board reached by a boosted click would never drag`
    );
  }
});

test("a board that appears after the script ran still moves a card", async () => {
  const { doc, fetches } = context();
  const b = board({ targets: ["doing"] });
  const { over, drop } = drag(doc, { card: b.card, column: b.columns[1] });

  assert.equal(over.prevented, 1, "a legal column did not accept the drag");
  assert.equal(drop.prevented, 1, "the drop was not claimed");
  assert.equal(fetches.length, 1, "no move was posted");
  assert.equal(fetches[0].url, "/projects/infra/move");
  assert.match(fetches[0].body, /ref=infra-1/);
  assert.equal(b.select.value, "doing", "the move form was not pointed at the drop column");
});

test("a column the card may not move to is refused before any request", () => {
  const { doc, fetches } = context();
  const b = board({ targets: ["doing"] });
  const illegal = domNode([".column"], { "data-state": "done" });
  illegal.parentNode = b.root;
  const { over, drop } = drag(doc, { card: b.card, column: illegal });

  assert.equal(over.prevented, 0, "an illegal column accepted the drag");
  assert.equal(drop.prevented, 0, "an illegal column claimed the drop");
  assert.equal(fetches.length, 0, "an illegal move reached the server");
});

test("the board's own data-drag gate still turns dragging off", () => {
  const { doc, fetches } = context();
  const b = board({ drag: "0", targets: ["doing"] });
  const { over } = drag(doc, { card: b.card, column: b.columns[1] });

  assert.equal(over.prevented, 0, "a board with dragging switched off still accepted a drag");
  assert.equal(fetches.length, 0, "a board with dragging switched off still posted a move");
});

test("a drag that starts outside any board is ignored", () => {
  const { doc, fetches } = context();
  const loose = domNode([".card"], { "data-task": "infra-9" });
  doc.dispatch("dragstart", { target: loose, preventDefault() {} });
  assert.equal(fetches.length, 0);
});

test("shouldRebind tells a replaced element from the one already held", () => {
  const { ctx } = context();
  const rebind = ctx.tix.shouldRebind;
  const a = {};
  const b = {};
  assert.equal(rebind(a, a), false, "the same element rebound, opening a second socket");
  assert.equal(rebind(b, a), true, "a swapped-in element was not picked up");
  assert.equal(rebind(a, null), true, "a first bind was skipped");
  assert.equal(rebind(null, a), true, "a page with no feed did not release the old one");
  assert.equal(rebind(null, null), false, "a page with no feed rebound to nothing");
});
