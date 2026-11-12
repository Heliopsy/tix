// SPDX-License-Identifier: AGPL-3.0-or-later

import test from "node:test";
import assert from "node:assert/strict";
import { newContext, run } from "./harness.mjs";

// The Live events screen's decisions, loaded with no document: which events
// the actor filter keeps, what a row names and links to, how the list is
// bounded and what a pause holds back.
function liveEvents() {
  const ctx = newContext();
  run(ctx, "events.js");
  return ctx.tix;
}

// seqs copies the sequence numbers out of the script's realm.
function seqs(list) {
  return Array.from(list, (e) => e.seq);
}

function ev(seq, extra = {}) {
  return { seq, type: "task.created", subject_type: "task", subject_id: "01T" + seq, payload: {}, ...extra };
}

test("the actor filter matches a handle in any case, with or without @, or an actor id", () => {
  const tix = liveEvents();
  const event = ev(1, { actor_id: "act-1", payload: { actor_handle: "Alice" } });
  assert.equal(tix.liveEventKeeps([], event), true);
  assert.equal(tix.liveEventKeeps(["@alice"], event), true);
  assert.equal(tix.liveEventKeeps(["ALICE"], event), true);
  assert.equal(tix.liveEventKeeps(["act-1"], event), true);
  assert.equal(tix.liveEventKeeps(["bob", "@carol"], event), false);
  assert.equal(tix.liveEventKeeps(["@"], event), false);
});

test("a row names its task by ref, its project by key, else the subject id, and links only to a screen", () => {
  const tix = liveEvents();
  const task = ev(1, { payload: { ref: "INFRA-5" } });
  const project = ev(2, { subject_type: "project", subject_id: "p1", payload: { key: "infra" } });
  const projectByID = ev(3, { subject_type: "project", subject_id: "p2", payload: {} });
  const token = ev(4, { subject_type: "api_token", subject_id: "tok-1" });
  assert.equal(tix.liveEventSubject(ev(8, { payload: { ref: "infra-5" } })), "INFRA-5");
  assert.equal(tix.liveEventSubject(task), "INFRA-5");
  assert.equal(tix.liveEventHref(task, {}), "/tasks/INFRA-5");
  assert.equal(tix.liveEventSubject(project), "INFRA");
  assert.equal(tix.liveEventHref(project, {}), "/projects/infra");
  assert.equal(tix.liveEventHref(projectByID, { p2: "ops" }), "/projects/ops");
  assert.equal(tix.liveEventSubject(token), "tok-1");
  assert.equal(tix.liveEventHref(token, {}), "");
  assert.equal(tix.liveEventActor(ev(5, { actor_id: "act-9", payload: { actor_handle: "raj" } })), "@raj");
  assert.equal(tix.liveEventActor(ev(6, { actor_id: "act-9" })), "act-9");
  assert.equal(tix.liveEventActor(ev(7)), "");
});

test("the list is newest first and keeps only the latest cap events", () => {
  const tix = liveEvents();
  const model = tix.liveEventsModel(3, []);
  for (let seq = 1; seq <= 5; seq++) {
    assert.equal(model.add(ev(seq)), "shown");
  }
  assert.deepEqual(seqs(model.events), [5, 4, 3]);
  assert.equal(model.add(ev(4)), "skipped", "an event seen already, as a replay overlapping the tail sends it");
  model.clear();
  assert.equal(model.events.length, 0);
});

test("a pause holds new events back and counts them, and resume puts them on top", () => {
  const tix = liveEvents();
  const model = tix.liveEventsModel(4, ["@alice"]);
  model.add(ev(1, { payload: { actor_handle: "alice" } }));
  model.pause();
  assert.equal(model.add(ev(2, { payload: { actor_handle: "alice" } })), "held");
  assert.equal(model.add(ev(3, { payload: { actor_handle: "bob" } })), "skipped");
  assert.equal(model.add(ev(4, { payload: { actor_handle: "alice" } })), "held");
  assert.deepEqual(seqs(model.events), [1]);
  assert.equal(model.held.length, 2);
  assert.equal(tix.liveEventsPausedText(model.held.length), "Paused, 2 new events.");
  assert.equal(tix.liveEventsPausedText(1), "Paused, 1 new event.");
  assert.equal(tix.liveEventsPausedText(0), "Paused.");
  assert.equal(model.find(4).seq, 4);
  model.resume();
  assert.deepEqual(seqs(model.events), [4, 2, 1]);
  assert.equal(model.held.length, 0);
});

test("the subscription sends the server half of the filter and replays only after a real sequence number", () => {
  const tix = liveEvents();
  assert.deepEqual(JSON.parse(JSON.stringify(tix.liveEventsSubscribe(null, 0))), { type: "subscribe", id: "events" });
  assert.deepEqual(JSON.parse(JSON.stringify(tix.liveEventsSubscribe({ types: ["task.*"], project_ids: ["p1"] }, 42))), {
    type: "subscribe", id: "events", filter: { types: ["task.*"], project_ids: ["p1"] }, since_seq: 42,
  });
  assert.equal(tix.liveEventsSubscribe({ types: [] }, 0).filter, undefined);
});
