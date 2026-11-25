// SPDX-License-Identifier: AGPL-3.0-or-later

import test from "node:test";
import assert from "node:assert/strict";
import { newContext, run } from "./harness.mjs";

// livestatus.js folds what each live stream reports into the one state the
// shell's indicator shows. Loaded with no document, only the model is there.
function model() {
  const ctx = newContext();
  run(ctx, "livestatus.js");
  return { tix: ctx.tix, m: ctx.tix.liveModel() };
}

test("a stream goes connecting, live, reconnecting, polling and live again", () => {
  const { m } = model();
  const at = () => m.details();
  m.note({ stream: "screen", kind: "bound" }, 1);
  assert.equal(at().mode, "connecting");
  m.note({ stream: "screen", kind: "open" }, 2);
  assert.equal(at().mode, "live");
  m.note({ stream: "screen", kind: "closed", opened: true, code: 1006 }, 3);
  m.note({ stream: "screen", kind: "retrying", attempt: 1, wait: 500 }, 3);
  assert.equal(at().mode, "reconnecting", "a dropped socket did not read as reconnecting");
  assert.equal(at().failures, 1);
  assert.equal(at().nextTry, 503, "the next try is not when the retry is due");
  m.note({ stream: "screen", kind: "fetched" }, 4);
  assert.equal(at().mode, "polling", "a poll getting through with the socket down did not read as polling");
  m.note({ stream: "screen", kind: "open" }, 5);
  assert.equal(at().mode, "live");
  assert.equal(at().since, 5);
  assert.deepEqual(JSON.parse(JSON.stringify(at().history.map((h) => h.mode || "error:" + h.error))),
    ["live", "error:closed", "reconnecting", "polling", "live"]);
});

test("the worst stream speaks for the page", () => {
  const { m } = model();
  for (const s of ["screen", "inbox"]) {
    m.note({ stream: s, kind: "bound" }, 1);
    m.note({ stream: s, kind: "open" }, 1);
  }
  m.note({ stream: "inbox", kind: "closed", opened: true, code: 1001 }, 2);
  m.note({ stream: "inbox", kind: "retrying", attempt: 1, wait: 0 }, 2);
  assert.equal(m.details().mode, "reconnecting", "one live stream hid another one's drop");
  m.note({ stream: "inbox", kind: "unbound" }, 3);
  assert.equal(m.details().mode, "live", "a stream that went away still counted");
  m.note({ stream: "screen", kind: "unbound" }, 4);
  assert.equal(m.details().mode, "none");
});

test("a navigation's fresh connection does not read as news", () => {
  const { m } = model();
  m.note({ stream: "screen", kind: "bound" }, 1);
  m.note({ stream: "screen", kind: "open" }, 2);
  m.note({ stream: "screen", kind: "bound" }, 10);
  m.note({ stream: "screen", kind: "connecting" }, 10);
  assert.equal(m.details().mode, "live", "every page change flashed connecting");
  assert.equal(m.details().since, 2, "every page change moved Since");
  assert.equal(m.details().history.length, 1);
});

test("offline is the browser saying so, or the server not answering at all", () => {
  const { m } = model();
  m.note({ stream: "screen", kind: "bound" }, 1);
  m.note({ stream: "screen", kind: "open" }, 1);
  m.online(false, 2);
  assert.equal(m.details().mode, "live", "an open socket was called offline");
  m.note({ stream: "screen", kind: "closed", opened: true, code: 1006 }, 3);
  assert.equal(m.details().mode, "offline");
  m.online(true, 4);
  m.note({ stream: "screen", kind: "retrying", attempt: 1, wait: 0 }, 4);
  m.note({ stream: "screen", kind: "fetch-failed", message: "Failed to fetch" }, 5);
  assert.equal(m.details().mode, "offline", "a fetch that got no answer was not offline");
  assert.equal(m.details().lastError.kind, "unreachable");
  m.note({ stream: "screen", kind: "fetch-failed", status: 502 }, 6);
  assert.equal(m.details().mode, "reconnecting", "an HTTP answer was taken for no answer");
  assert.equal(m.details().lastError.status, 502);
});

test("the hints follow the app's rules", () => {
  const { tix } = model();
  const hints = (d) => [...tix.liveHints(d)];
  assert.deepEqual(hints({ mode: "polling", lastError: { kind: "unreachable" } }), ["upgrade"],
    "polling with HTTP working did not point at the proxy's upgrades");
  assert.deepEqual(hints({ mode: "reconnecting", lastError: { kind: "closed", code: 1006 } }), ["idle"]);
  assert.deepEqual(hints({ mode: "reconnecting", lastError: { kind: "closed", code: 1011 } }), []);
  assert.deepEqual(hints({ mode: "offline", lastError: { kind: "unreachable" } }), ["unreachable"]);
  assert.deepEqual(hints({ mode: "reconnecting", lastError: { kind: "http", status: 502 } }), ["http"]);
  assert.equal(tix.liveErrorKind({ code: 1008, opened: true }), "refused");
  assert.equal(tix.liveErrorKind({ code: 1006, opened: false }), "unreachable");
  assert.equal(tix.liveErrorKind({ code: 1006, opened: true }), "closed");
});

test("a cursor the server no longer keeps is not reported as a refusal", () => {
  const { m } = model();
  m.note({ stream: "screen", kind: "bound" }, 1);
  m.note({ stream: "screen", kind: "refused", message: "cursor 4 is no longer available" }, 2);
  assert.equal(m.details().lastError, null);
  m.note({ stream: "screen", kind: "refused", message: "forbidden" }, 3);
  assert.equal(m.details().lastError.kind, "refused");
});

test("the report says what the dialog says and nothing that signs in", () => {
  const { m, tix } = model();
  m.note({ stream: "screen", kind: "bound" }, Date.UTC(2026, 0, 1));
  m.note({ stream: "screen", kind: "closed", opened: false, code: 1006 }, Date.UTC(2026, 0, 1, 0, 0, 1));
  m.note({ stream: "screen", kind: "retrying", attempt: 2, wait: 1000 }, Date.UTC(2026, 0, 1, 0, 0, 1));
  const text = tix.liveReport(m.details(), {
    host: "tix.example", build: "0.58.0", agent: "Test/1", online: true, now: Date.UTC(2026, 0, 1, 0, 0, 2),
  });
  for (const want of [
    "server: tix.example", "build: 0.58.0", "state: reconnecting", "failures in a row: 2",
    "last error: unreachable during websocket at 2026-01-01T00:00:01.000Z, close code 1006",
  ]) {
    assert.ok(text.includes(want), "the report lacks " + JSON.stringify(want) + ":\n" + text);
  }
  assert.ok(!/cookie|csrf|session|token/i.test(text), "the report carries a credential's name:\n" + text);
});

test("Recent changes keeps the latest twenty", () => {
  const { m } = model();
  m.note({ stream: "screen", kind: "bound" }, 0);
  for (let i = 1; i <= 30; i++) {
    m.note({ stream: "screen", kind: "closed", opened: true, code: 1006 }, i);
  }
  assert.equal(m.details().history.length, 20);
  assert.equal(m.details().history[19].at, 30);
});
