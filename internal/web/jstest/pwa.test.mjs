// SPDX-License-Identifier: AGPL-3.0-or-later

// The installable app: assets/pwa.js registers assets/sw.js, which caches
// nothing and answers nothing.
import test from "node:test";
import assert from "node:assert/strict";
import { bareContext, run } from "./harness.mjs";

test("the page registers the worker at the root scope", () => {
  const calls = [];
  const serviceWorker = {
    register: (url, opts) => {
      calls.push([url, opts.scope]);
      return Promise.resolve();
    },
  };
  run(bareContext({ navigator: { serviceWorker } }), "pwa.js");
  assert.deepEqual(calls, [["/sw.js", "/"]]);
});

test("a browser without service workers is left alone", () => {
  assert.doesNotThrow(() => run(bareContext({ navigator: {} }), "pwa.js"));
});

function worker() {
  const handlers = {};
  const touched = [];
  const self = {
    addEventListener: (type, fn) => {
      handlers[type] = fn;
    },
    skipWaiting: () => {},
    clients: { claim: () => Promise.resolve() },
  };
  const trap = (name) =>
    new Proxy({}, { get: () => { touched.push(name); return () => {}; } });
  const ctx = bareContext({
    caches: trap("caches"),
    indexedDB: trap("indexedDB"),
    fetch: () => touched.push("fetch"),
  });
  ctx.self = self;
  run(ctx, "sw.js");
  return { handlers, touched };
}

test("the worker answers no request and stores nothing", () => {
  const { handlers, touched } = worker();
  assert.equal(typeof handlers.fetch, "function");
  for (const url of ["/tasks", "/api/v1/tasks", "/assets/app.css", "/login", "/settings"]) {
    let answered = false;
    handlers.fetch({ request: { url, method: "GET" }, respondWith: () => { answered = true; } });
    assert.equal(answered, false, url + " was answered by the worker");
  }
  handlers.install({});
  handlers.activate({ waitUntil: () => {} });
  assert.deepEqual(touched, []);
});
