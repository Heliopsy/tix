// SPDX-License-Identifier: AGPL-3.0-or-later

// The Live events screen (templates/events.html): the tenant's events as
// they arrive over the WebSocket, newest first, the way `tix watch` prints
// them and the app's Live events screen lists them.
//
// Loaded by that screen alone, at the foot of its content. A boosted
// navigation runs the file again every time the screen is swapped in, so
// the second run only rebinds the stream the first one set up.

(function () {
  "use strict";
  var tix = window.tix;
  if (!tix) {
    return;
  }
  if (tix.liveEvents) {
    tix.liveEvents.rebind();
    return;
  }

  function say(text) {
    return tix.t ? tix.t(text) : text;
  }

  function payloadText(event, key) {
    var p = event && event.payload;
    var v = p ? p[key] : null;
    return typeof v === "string" ? v : "";
  }

  // liveEventActor is who caused an event as a row names them: the handle
  // tix puts in the payload, else the actor's id, else "" for the system.
  tix.liveEventActor = function (event) {
    var handle = payloadText(event, "actor_handle");
    if (handle) {
      return "@" + handle;
    }
    return (event && event.actor_id) || "";
  };

  // liveEventKeeps reports whether an event passes the actor filter: a
  // handle (with or without @, any case) or an actor id, as `tix watch
  // --actor` matches. No actors keeps every event.
  tix.liveEventKeeps = function (actors, event) {
    if (!actors || actors.length === 0) {
      return true;
    }
    var handle = payloadText(event, "actor_handle").toLowerCase();
    for (var i = 0; i < actors.length; i++) {
      var want = String(actors[i]);
      if (want.charAt(0) === "@") {
        want = want.slice(1);
      }
      if (want && (want === event.actor_id || want.toLowerCase() === handle)) {
        return true;
      }
    }
    return false;
  };

  // liveEventSubject is what an event is about as a person names it: a
  // task's ref or a project's key, upper case as the interface draws them,
  // else the subject's id.
  tix.liveEventSubject = function (event) {
    var named = payloadText(event, "ref") || payloadText(event, "key");
    return named ? named.toUpperCase() : (event && event.subject_id) || "";
  };

  // liveEventHref is where an event's Open goes: its task, or its project,
  // or "" for anything without a screen.
  tix.liveEventHref = function (event, keys) {
    var ref = payloadText(event, "ref");
    if (ref) {
      return "/tasks/" + encodeURIComponent(ref);
    }
    var key = payloadText(event, "key");
    if (event && event.subject_type === "project") {
      key = key || (keys && keys[event.subject_id]) || "";
      if (key) {
        return "/projects/" + encodeURIComponent(key);
      }
    }
    return "";
  };

  // liveEventsPausedText is the line shown while the list is held still.
  tix.liveEventsPausedText = function (count) {
    if (count === 1) {
      return say("Paused, 1 new event.");
    }
    if (count > 1) {
      return say("Paused, %d new events.").replace("%d", String(count));
    }
    return say("Paused.");
  };

  // liveEventsModel is the list the screen draws: the events shown, newest
  // first, and the ones held while paused, each at most cap long. add
  // answers "shown", "held" or "skipped" (filtered out, or seen already).
  tix.liveEventsModel = function (cap, actors) {
    var limit = cap > 0 ? cap : 500;
    var model = { events: [], held: [], paused: false, lastSeq: 0 };
    function capped(list) {
      if (list.length > limit) {
        list.length = limit;
      }
      return list;
    }
    model.add = function (event) {
      if (!event || typeof event.seq !== "number") {
        return "skipped";
      }
      if (event.seq <= model.lastSeq) {
        return "skipped";
      }
      model.lastSeq = event.seq;
      if (!tix.liveEventKeeps(actors, event)) {
        return "skipped";
      }
      if (model.paused) {
        model.held.unshift(event);
        capped(model.held);
        return "held";
      }
      model.events.unshift(event);
      capped(model.events);
      return "shown";
    };
    model.pause = function () {
      model.paused = true;
    };
    model.resume = function () {
      model.paused = false;
      model.events = capped(model.held.concat(model.events));
      model.held = [];
    };
    model.clear = function () {
      model.events = [];
      model.held = [];
    };
    model.find = function (seq) {
      var all = model.held.concat(model.events);
      for (var i = 0; i < all.length; i++) {
        if (all[i].seq === seq) {
          return all[i];
        }
      }
      return null;
    };
    return model;
  };

  // liveEventsSubscribe is the subscribe message: the server half of the
  // filter, and the sequence number to replay after when there is one.
  tix.liveEventsSubscribe = function (filter, since) {
    var message = { type: "subscribe", id: "events" };
    if (filter && ((filter.types && filter.types.length) || (filter.project_ids && filter.project_ids.length))) {
      message.filter = filter;
    }
    if (since > 0) {
      message.since_seq = since;
    }
    return message;
  };

  var doc = window.document;
  if (!doc || !doc.addEventListener) {
    return;
  }

  function parsed(text, fallback) {
    try {
      return JSON.parse(text || "") || fallback;
    } catch (err) {
      return fallback;
    }
  }

  var bound = null;
  var socket = null;
  var retry = null;
  var attempt = 0;
  var refused = false;
  var model = null;
  var since = 0;
  var keys = {};
  var filter = null;

  function part(name) {
    return bound ? bound.querySelector("[" + name + "]") : null;
  }

  function setState(state, text) {
    var el = part("data-le-state");
    if (el) {
      el.setAttribute("data-le-state", state);
      el.textContent = text;
    }
  }

  function notice(text) {
    var el = part("data-le-notice");
    if (!el) {
      return;
    }
    el.textContent = text || "";
    if (text) {
      el.removeAttribute("hidden");
    } else {
      el.setAttribute("hidden", "hidden");
    }
  }

  function showPaused() {
    var el = part("data-le-paused");
    var btn = part("data-le-pause");
    if (btn) {
      btn.textContent = say(model.paused ? "Resume" : "Pause");
      btn.setAttribute("aria-pressed", model.paused ? "true" : "false");
    }
    if (!el) {
      return;
    }
    if (model.paused) {
      el.textContent = tix.liveEventsPausedText(model.held.length);
      el.removeAttribute("hidden");
    } else {
      el.setAttribute("hidden", "hidden");
    }
  }

  function clock(iso) {
    var at = new Date(iso);
    if (isNaN(at.getTime())) {
      return "";
    }
    try {
      return at.toLocaleTimeString(doc.documentElement.lang || undefined, { hour: "2-digit", minute: "2-digit", second: "2-digit" });
    } catch (err) {
      return at.toTimeString().slice(0, 8);
    }
  }

  function cell(row, text, className) {
    var td = doc.createElement("td");
    if (className) {
      td.className = className;
    }
    if (text !== undefined) {
      td.textContent = text;
    }
    row.appendChild(td);
    return td;
  }

  function rowFor(event) {
    var row = doc.createElement("tr");
    row.setAttribute("data-seq", String(event.seq));
    cell(row, "#" + event.seq, "num");
    var type = cell(row, undefined, "liveevents-type");
    var code = doc.createElement("code");
    code.textContent = event.type || "";
    type.appendChild(code);
    var when = cell(row);
    var time = doc.createElement("time");
    time.setAttribute("datetime", event.occurred_at || "");
    time.setAttribute("title", event.occurred_at || "");
    time.textContent = clock(event.occurred_at);
    when.appendChild(time);
    cell(row, tix.liveEventActor(event) || say("system"));
    var subject = cell(row, undefined, "liveevents-subject");
    var name = tix.liveEventSubject(event);
    var project = event.project_id && keys[event.project_id];
    subject.textContent = (event.subject_type ? event.subject_type + " " : "") + name;
    if (project && name.indexOf(project.toUpperCase()) !== 0) {
      subject.textContent += " (" + project.toUpperCase() + ")";
    }
    var actions = cell(row, undefined, "liveevents-actions");
    var href = tix.liveEventHref(event, keys);
    if (href) {
      var open = doc.createElement("a");
      open.className = "btn link small";
      open.href = href;
      open.textContent = say("Open");
      actions.appendChild(open);
    }
    var details = doc.createElement("button");
    details.type = "button";
    details.className = "btn link small";
    details.setAttribute("data-le-details", String(event.seq));
    details.setAttribute("aria-haspopup", "dialog");
    details.textContent = say("Details");
    actions.appendChild(details);
    return row;
  }

  function drawEmpty() {
    var empty = part("data-le-empty");
    if (empty) {
      if (model.events.length) {
        empty.setAttribute("hidden", "hidden");
      } else {
        empty.removeAttribute("hidden");
      }
    }
  }

  function drawAll() {
    var rows = part("data-le-rows");
    if (!rows) {
      return;
    }
    rows.textContent = "";
    for (var i = 0; i < model.events.length; i++) {
      rows.appendChild(rowFor(model.events[i]));
    }
    drawEmpty();
  }

  function drawOne(event) {
    var rows = part("data-le-rows");
    if (!rows) {
      return;
    }
    rows.insertBefore(rowFor(event), rows.firstChild);
    while (rows.children.length > model.events.length) {
      rows.removeChild(rows.lastChild);
    }
    drawEmpty();
  }

  function stopRetry() {
    if (retry !== null) {
      window.clearTimeout(retry);
      retry = null;
    }
  }

  function drop() {
    stopRetry();
    if (socket) {
      var going = socket;
      socket = null;
      going.close();
    }
  }

  function later() {
    stopRetry();
    setState("offline", say("Offline. Reconnecting."));
    if (refused || doc.hidden) {
      return;
    }
    var wait = tix.backoffDelay(attempt, 500, 30000);
    attempt++;
    retry = window.setTimeout(connect, wait);
  }

  function subscribe(conn) {
    var from = model.lastSeq > 0 ? model.lastSeq : since;
    conn.send(JSON.stringify(tix.liveEventsSubscribe(filter, from)));
  }

  function received(conn, message) {
    var frame = parsed(message.data, null);
    if (!frame) {
      return;
    }
    if (frame.type === "subscribed") {
      attempt = 0;
      setState("live", say("Live"));
      return;
    }
    if (frame.type === "error") {
      if (tix.resumeRefused(frame.error)) {
        var gone = model.lastSeq > 0 ? model.lastSeq : since;
        notice(say("Events after #%d are no longer kept on the server. Showing new events from now.").replace("%d", String(gone)));
        since = 0;
        model.lastSeq = 0;
        subscribe(conn);
        return;
      }
      refused = true;
      notice(say("The server refused this subscription: %s").replace("%s", (frame.error && frame.error.message) || ""));
      setState("refused", say("Not following events."));
      drop();
      return;
    }
    if (frame.type !== "event" || !frame.event) {
      return;
    }
    var outcome = model.add(frame.event);
    if (outcome === "shown") {
      drawOne(frame.event);
    } else if (outcome === "held") {
      showPaused();
    }
  }

  function connect() {
    stopRetry();
    if (!bound || refused || !window.WebSocket || doc.hidden) {
      return;
    }
    var scheme = window.location.protocol === "https:" ? "wss://" : "ws://";
    var opening;
    try {
      opening = new window.WebSocket(scheme + window.location.host + bound.getAttribute("data-events"));
    } catch (err) {
      later();
      return;
    }
    socket = opening;
    setState("connecting", say("Connecting to the event stream"));
    opening.addEventListener("open", function () {
      if (socket === opening) {
        subscribe(opening);
      }
    });
    opening.addEventListener("message", function (message) {
      if (socket === opening) {
        received(opening, message);
      }
    });
    function closed() {
      if (socket === opening) {
        socket = null;
        later();
      }
    }
    opening.addEventListener("close", closed);
    opening.addEventListener("error", closed);
  }

  function rebind() {
    var next = doc.getElementById("live-events");
    if (next === bound) {
      return;
    }
    drop();
    bound = next && next.getAttribute("data-events") ? next : null;
    refused = false;
    attempt = 0;
    if (!bound) {
      return;
    }
    filter = parsed(bound.getAttribute("data-filter"), null);
    keys = parsed(bound.getAttribute("data-keys"), {});
    since = parseInt(bound.getAttribute("data-since"), 10) || 0;
    model = tix.liveEventsModel(parseInt(bound.getAttribute("data-cap"), 10), parsed(bound.getAttribute("data-actors"), []));
    showPaused();
    drawAll();
    connect();
  }

  function details(seq) {
    var event = model && model.find(seq);
    var dlg = doc.getElementById("live-event");
    if (!event || !dlg) {
      return;
    }
    var json = JSON.stringify(event, null, 2);
    dlg.querySelector("[data-le-title]").textContent = say("Event #%d").replace("%d", String(event.seq));
    dlg.querySelector("[data-le-json]").textContent = json;
    dlg.querySelector("[data-le-copy]").setAttribute("data-copy-text", json);
    var open = dlg.querySelector("[data-le-open]");
    var href = tix.liveEventHref(event, keys);
    if (href) {
      open.setAttribute("href", href);
      open.removeAttribute("hidden");
    } else {
      open.setAttribute("hidden", "hidden");
    }
    if (typeof dlg.showModal === "function" && !dlg.open) {
      dlg.showModal();
    } else {
      dlg.setAttribute("open", "");
    }
  }

  doc.addEventListener("click", function (event) {
    if (!bound || !event.target || !event.target.closest) {
      return;
    }
    if (event.target.closest("[data-le-pause]")) {
      if (model.paused) {
        model.resume();
        drawAll();
      } else {
        model.pause();
      }
      showPaused();
      return;
    }
    if (event.target.closest("[data-le-clear]")) {
      model.clear();
      notice("");
      showPaused();
      drawAll();
      return;
    }
    var more = event.target.closest("[data-le-details]");
    if (more) {
      details(parseInt(more.getAttribute("data-le-details"), 10));
    }
  });

  doc.addEventListener("htmx:load", rebind);
  doc.addEventListener("visibilitychange", function () {
    if (!doc.hidden && bound && !socket) {
      attempt = 0;
      connect();
    }
  });

  tix.liveEvents = { rebind: rebind };
  rebind();
})();
