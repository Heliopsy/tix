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
  const select = domNode(["form select[name=route]", "select[name=route]"], {
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
function context({ pathname = "/projects/infra", search = "" } = {}) {
  const fetches = [];
  const doc = fakeDocument();
  const ctx = newContext({
    document: doc,
    matchMedia: () => ({ matches: true }),
    location: { protocol: "http:", host: "localhost", pathname, search },
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

// The task screen draws a board of a filtered, sorted, paged listing, so the
// query string is part of which tasks are on it. Re-fetching the path alone
// answered a different question: the reader's filter was dropped and the board
// came back holding the first page of everything.
test("a move refreshes the board it is on, query string included", async () => {
  const { doc, fetches } = context({ pathname: "/tasks", search: "?q=tag%3Aops&sort=urgency" });
  const b = board({ targets: ["doing"] });
  drag(doc, { card: b.card, column: b.columns[1] });
  await new Promise((resolve) => setTimeout(resolve, 0));

  const refresh = fetches[fetches.length - 1];
  assert.equal(fetches.length, 2, "the move did not refresh the board afterwards");
  assert.equal(
    refresh.url,
    "/tasks?q=tag%3Aops&sort=urgency",
    "the refresh dropped the query, so a filtered board came back unfiltered"
  );
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

// ---------------------------------------------------------------------------
// The live connection: one mechanism behind the activity feed, the task list,
// the board and one task's screen. What is asserted here is everything that
// used to be absent -- a reconnect, a resume, a fallback and an indicator --
// plus the rule that a refresh never lands on top of work in progress.

// fakeClock drives setTimeout and setInterval by hand, so a backoff and a
// poll are asserted rather than waited out.
function fakeClock() {
  let next = 1;
  const timers = new Map();
  return {
    setTimeout(fn, ms) {
      const id = next++;
      timers.set(id, { fn, ms, repeat: false });
      return id;
    },
    clearTimeout(id) {
      timers.delete(id);
    },
    setInterval(fn, ms) {
      const id = next++;
      timers.set(id, { fn, ms, repeat: true });
      return id;
    },
    clearInterval(id) {
      timers.delete(id);
    },
    get count() {
      return timers.size;
    },
    delays() {
      return [...timers.values()].map((t) => t.ms);
    },
    fire() {
      const due = [...timers.entries()];
      for (const [id, t] of due) {
        if (!t.repeat) {
          timers.delete(id);
        }
        t.fn();
      }
      return due.length;
    },
  };
}

// region is a swappable part of a screen: live.js reads its innerHTML out of
// the fetched fragment and writes it here.
function region(id, { fields = [] } = {}) {
  const node = domNode([], { id });
  node.innerHTML = "before";
  node.contains = () => false;
  node.querySelectorAll = () => fields;
  return node;
}

// liveScreen builds the page a task screen is: a binding element, the regions
// it names, and the stale notice.
function liveScreen({ subject = "", regions = ["task-pills"], window: win = 150 } = {}) {
  const binding = domNode([], {
    "data-events": "/api/v1/events",
    "data-live": "/tasks/infra-1/live",
    "data-live-filter": '{"project_ids":["p1"],"types":["task.*"]}',
    "data-live-regions": regions.join(" "),
    "data-live-window": String(win),
    "data-live-subject": subject,
  });
  const notice = domNode([], {});
  notice.hiddenAttr = true;
  notice.setAttribute = function (name) { if (name === "hidden") { this.hiddenAttr = true; } };
  notice.removeAttribute = function (name) { if (name === "hidden") { this.hiddenAttr = false; } };
  return { binding, notice };
}

// liveContext loads the assets against a page carrying a screen binding, a
// controllable clock, and a WebSocket that does nothing until a test says so.
function liveContext({ subject = "", regions = ["task-pills"], fresh = {} } = {}) {
  const clock = fakeClock();
  const sockets = [];
  const fetches = [];
  const page = liveScreen({ subject, regions });
  const nodes = { "live-screen": page.binding, "live-stale": page.notice };
  for (const id of regions) {
    nodes[id] = region(id);
  }
  const bySelector = {};
  const doc = fakeDocument({ byId: nodes, bySelector });
  doc.hidden = false;
  doc.activeElement = null;

  let fetchBody = "fragment";
  const ctx = newContext({
    document: doc,
    matchMedia: () => ({ matches: true }),
    location: { protocol: "http:", host: "localhost", pathname: "/tasks/infra-1", search: "" },
    setTimeout: clock.setTimeout.bind(clock),
    clearTimeout: clock.clearTimeout.bind(clock),
    setInterval: clock.setInterval.bind(clock),
    clearInterval: clock.clearInterval.bind(clock),
    WebSocket: function (url) {
      const listeners = new Map();
      const socket = {
        url,
        sent: [],
        closed: false,
        addEventListener(type, fn) {
          if (!listeners.has(type)) {
            listeners.set(type, []);
          }
          listeners.get(type).push(fn);
        },
        send(raw) {
          this.sent.push(JSON.parse(raw));
        },
        close() {
          this.closed = true;
        },
        emit(type, event) {
          for (const fn of listeners.get(type) || []) {
            fn(event);
          }
        },
        event(payload) {
          this.emit("message", { data: JSON.stringify({ type: "event", event: payload }) });
        },
      };
      sockets.push(socket);
      return socket;
    },
    DOMParser: function () {
      return {
        parseFromString: () => ({
          getElementById: (id) => (Object.prototype.hasOwnProperty.call(fresh, id)
            ? { innerHTML: fresh[id] } : null),
        }),
      };
    },
    FormData: function (form) { return form.fields; },
    URLSearchParams: function (fields) { this.toString = () => JSON.stringify(fields); },
    fetch: (url) => {
      fetches.push(url);
      return Promise.resolve({ ok: true, text: () => Promise.resolve(fetchBody) });
    },
  });
  run(ctx, "live.js");
  return { ctx, doc, clock, sockets, fetches, page, nodes, bySelector };
}

const settle = () => new Promise((resolve) => setTimeout(resolve, 0));

test("a screen subscribes with its own filter and nothing wider", () => {
  const { sockets } = liveContext();
  assert.equal(sockets.length, 1, "the screen opened no connection");
  sockets[0].emit("open", {});
  assert.deepEqual(sockets[0].sent, [{
    type: "subscribe",
    id: "screen",
    filter: { project_ids: ["p1"], types: ["task.*"] },
  }], "the subscription did not carry the screen's filter");
});

test("the first event is drawn at once and the fragment swapped in by region", async () => {
  const { sockets, fetches, nodes } = liveContext({ fresh: { "task-pills": "doing" } });
  sockets[0].emit("open", {});
  sockets[0].event({ seq: 7, subject_id: "task-1" });

  assert.deepEqual(fetches, ["/tasks/infra-1/live"], "the first event waited for a timer");
  await settle();
  assert.equal(nodes["task-pills"].innerHTML, "doing", "the region was not replaced");
});

test("an answer for a page already navigated away from is not drawn over the new one", async () => {
  const { sockets, fetches, nodes } = liveContext({ fresh: { "task-pills": "stale" } });
  sockets[0].emit("open", {});
  sockets[0].event({ seq: 7, subject_id: "task-1" });
  assert.equal(fetches.length, 1, "the event asked for nothing");
  nodes["live-screen"] = liveScreen({}).binding;
  nodes["task-pills"].innerHTML = "fresh page";
  await settle();
  assert.equal(nodes["task-pills"].innerHTML, "fresh page", "the old page's answer replaced the new page's region");
});

test("an event about another record leaves a one-task screen alone", () => {
  const { sockets, fetches } = liveContext({ subject: "task-1" });
  sockets[0].emit("open", {});
  sockets[0].event({ seq: 2, subject_id: "task-2", payload: { task_id: "task-2" } });
  assert.deepEqual(fetches, [], "a neighbour's change rebuilt the screen");
  sockets[0].event({ seq: 3, subject_id: "comment-5", payload: { task_id: "task-1" } });
  assert.equal(fetches.length, 1, "a comment on this task did not reach it");
});

test("a refresh waits for a drag rather than landing in the middle of one", async () => {
  const c = liveContext();
  c.bySelector[".is-dragging"] = domNode([], {});
  c.sockets[0].emit("open", {});
  c.sockets[0].event({ seq: 1, subject_id: "task-1" });
  assert.deepEqual(c.fetches, [], "a refresh landed mid-drag");

  delete c.bySelector[".is-dragging"];
  c.clock.fire();
  await settle();
  assert.equal(c.fetches.length, 1, "the deferred refresh never arrived after the drag");
});

test("a refresh waits for a field the reader has typed into", async () => {
  const c = liveContext();
  const typed = { tagName: "TEXTAREA", value: "half a comment", defaultValue: "" };
  c.nodes["task-pills"].querySelectorAll = () => [typed];
  c.sockets[0].emit("open", {});
  c.sockets[0].event({ seq: 1, subject_id: "task-1" });
  assert.deepEqual(c.fetches, [], "a refresh discarded what the reader was typing");

  typed.value = "";
  c.clock.fire();
  await settle();
  assert.equal(c.fetches.length, 1, "the refresh never landed once the field was clean");
});

// The board is the one screen with a <select> inside a live region, and it
// never updated. Its Move control named no selected option, so the browser
// picked the first; the DOM then reported a selection nobody made, the region
// was busy on every pass, and the refresh deferred forever. This is the whole
// mechanism, end to end: a board that has only been rendered must refresh.
test("a freshly rendered board is not held busy by its own Move controls", async () => {
  const c = liveContext({ regions: ["project-board"], fresh: { "project-board": "columns" } });
  const move = {
    tagName: "SELECT",
    selectedIndex: 0,
    options: [
      { selected: true, defaultSelected: true },
      { selected: false, defaultSelected: false },
    ],
  };
  c.nodes["project-board"].querySelectorAll = () => [move];
  c.sockets[0].emit("open", {});
  c.sockets[0].event({ seq: 1, subject_id: "task-1" });

  assert.deepEqual(c.fetches, ["/tasks/infra-1/live"],
    "the board deferred its first refresh, so it never updates at all");
  await settle();
  assert.equal(c.nodes["project-board"].innerHTML, "columns", "the board was never replaced");
});

// The same board once a reader has actually opened a Move menu and chosen a
// column. That is work a refresh would throw away, so it still waits.
test("a board whose reader has chosen a move still holds the refresh back", async () => {
  const c = liveContext({ regions: ["project-board"], fresh: { "project-board": "columns" } });
  const move = {
    tagName: "SELECT",
    selectedIndex: 1,
    options: [
      { selected: false, defaultSelected: true },
      { selected: true, defaultSelected: false },
    ],
  };
  c.nodes["project-board"].querySelectorAll = () => [move];
  c.sockets[0].emit("open", {});
  c.sockets[0].event({ seq: 1, subject_id: "task-1" });
  assert.deepEqual(c.fetches, [], "a refresh discarded the move the reader had picked");

  move.selectedIndex = 0;
  move.options[0].selected = true;
  move.options[1].selected = false;
  c.clock.fire();
  await settle();
  assert.equal(c.fetches.length, 1, "the refresh never landed once the menu was back at its default");
});

test("a dropped connection is reopened on a backing-off timer and resumes from the last event", () => {
  const c = liveContext();
  c.sockets[0].emit("open", {});
  c.sockets[0].event({ seq: 41, subject_id: "task-1" });
  c.sockets[0].emit("close", {});

  assert.equal(c.page.notice.hiddenAttr, false, "a dead connection said nothing on screen");
  assert.match(c.page.notice.textContent, /out of date/);
  assert.ok(c.clock.delays().some((ms) => ms >= 250 && ms <= 500), `no reconnect armed: ${c.clock.delays()}`);

  c.clock.fire();
  assert.equal(c.sockets.length, 2, "the connection was never reopened");
  c.sockets[1].emit("open", {});
  assert.deepEqual(c.sockets[1].sent[0], {
    type: "subscribe",
    id: "screen",
    filter: { project_ids: ["p1"], types: ["task.*"] },
    since_seq: 41,
  }, "the reconnect lost every change made while it was down");
  assert.equal(c.page.notice.hiddenAttr, true, "the notice stayed up after the stream came back");
});

test("a resume the server will not honour falls back to a whole refresh", async () => {
  const c = liveContext();
  c.sockets[0].emit("open", {});
  c.sockets[0].event({ seq: 9, subject_id: "task-1" });
  await settle();
  c.fetches.length = 0;
  c.sockets[0].sent.length = 0;
  c.sockets[0].emit("message", {
    data: JSON.stringify({
      type: "error",
      id: "screen",
      error: { code: "invalid", message: "cursor 9 is no longer available; the oldest retained event is 400" },
    }),
  });

  assert.equal(c.sockets[0].sent.length, 1, "the screen did not resubscribe");
  assert.equal(c.sockets[0].sent[0].since_seq, undefined, "it asked again from a cursor the server refused");
  c.clock.fire();
  await settle();
  assert.equal(c.fetches.length, 1, "the gap was left on screen instead of being re-fetched");
});

test("a screen that cannot open a socket polls instead, and says it is not live", async () => {
  const c = liveContext();
  c.sockets[0].emit("error", {});
  assert.equal(c.page.notice.hiddenAttr, false, "a screen with no connection looked up to date");
  assert.match(c.page.notice.textContent, /15 seconds/);
  assert.ok(c.clock.delays().includes(15000), `no fallback poll was started: ${c.clock.delays()}`);

  c.clock.fire();
  await settle();
  assert.ok(c.fetches.length >= 1, "the fallback poll never asked the server anything");

  // The socket the backoff reopened succeeds, and the poll stops.
  c.sockets[1].emit("open", {});
  assert.equal(c.page.notice.hiddenAttr, true);
  assert.ok(!c.clock.delays().includes(15000), "the poll kept running after the socket came back");
});

test("a hidden page does not reconnect, and catches up when it comes back", async () => {
  const c = liveContext();
  c.sockets[0].emit("open", {});
  c.doc.hidden = true;
  c.sockets[0].emit("close", {});
  assert.equal(c.sockets.length, 1, "a background tab kept retrying");

  c.doc.hidden = false;
  c.doc.dispatch("visibilitychange", {});
  assert.equal(c.sockets.length, 2, "coming back to the page did not reconnect");
  await settle();
  assert.ok(c.fetches.length >= 1, "coming back to the page did not re-read what it missed");
});

// A save made in the task dialog or by renaming a title in place asks the
// page to refresh directly, so the row changes without waiting on a socket
// that may never open.
test("a refresh asked for by the page itself fetches the fragment without any event", async () => {
  const { doc, fetches, nodes } = liveContext({ fresh: { "task-pills": "renamed" } });
  doc.dispatch("tix:refresh", {});
  assert.deepEqual(fetches, ["/tasks/infra-1/live"], "asking for a refresh fetched nothing");
  await settle();
  assert.equal(nodes["task-pills"].innerHTML, "renamed", "the refreshed region was not drawn");
});
