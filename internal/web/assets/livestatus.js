// SPDX-License-Identifier: AGPL-3.0-or-later

// The shell's live connection indicator and its connection details.
//
// assets/live.js keeps up to three streams open at once (the activity feed,
// the screen, the sidebar's inbox badge), each with its own socket, backoff
// and fallback poll. It used to reconnect without a word beyond the stale
// notice on the screen itself, so nobody could tell a proxy eating WebSocket
// upgrades from a server that was down. Each stream reports here what it
// went through (tix.liveNote); this file folds those reports into one state,
// draws it beside the tenant's name, and explains it in a dialog with the
// last error, Reconnect now and a report to copy.
//
// The model is pure and takes the time as a value, so internal/web/jstest
// drives it without a browser; the wiring at the bottom is all that touches
// the document.
(function (global) {
  "use strict";
  var tix = (global.tix = global.tix || {});

  // How far each state is from working, so the worst stream speaks for all.
  var rank = { live: 0, connecting: 1, polling: 2, reconnecting: 3, offline: 4 };

  // How many lines Recent changes keeps.
  var historyCap = 20;

  // liveStreamMode is one bound stream's state: live on an open socket;
  // offline when the browser says so or a fetch could not reach the server;
  // connecting before its first try has failed; polling once the fallback
  // poll has got through with the socket still down; reconnecting otherwise.
  tix.liveStreamMode = function (st, online) {
    if (st.live) return "live";
    if (online === false || st.netDown) return "offline";
    if (!st.everOpen && !(st.attempt > 0)) return "connecting";
    if (st.fetchOk) return "polling";
    return "reconnecting";
  };

  // liveOverall is the worst of the bound streams' states, "none" with none.
  tix.liveOverall = function (modes) {
    var out = "none";
    for (var i = 0; i < modes.length; i++) {
      if (out === "none" || rank[modes[i]] > rank[out]) out = modes[i];
    }
    return out;
  };

  // liveErrorKind words a socket's close the way the app does. A browser
  // gives no reason for a socket that never opened: 1006 then means the host
  // was not reached or the upgrade was turned away, which the hints tell apart
  // by whether plain HTTP gets through.
  tix.liveErrorKind = function (note) {
    if (note.code === 1008) return "refused";
    if (!note.opened && (note.code === 1006 || !note.code)) return "unreachable";
    return "closed";
  };

  // liveHints are what a state and its last error may mean, most specific
  // first, by the same rules as the app's connection details.
  tix.liveHints = function (d) {
    var out = [];
    var e = d.lastError;
    if (e) {
      if (e.kind === "closed" && (!e.code || e.code === 1001 || e.code === 1006)) out.push("idle");
      if (e.kind === "unreachable" && d.mode !== "polling") out.push("unreachable");
      if (e.kind === "refused") out.push("refused");
      if (e.kind === "http") out.push("http");
    }
    if (d.mode === "polling") out.push("upgrade");
    return out;
  };

  // liveModel folds stream reports into the details the indicator draws.
  // now is the time as milliseconds, passed in with every call.
  tix.liveModel = function () {
    var streams = {};
    var online = true;
    var d = {
      mode: "none", since: 0, lastContact: 0, lastEvent: 0, lastSeq: 0,
      failures: 0, nextTry: 0, lastError: null, history: []
    };

    function remember(entry) {
      d.history.push(entry);
      if (d.history.length > historyCap) d.history.shift();
    }

    function recompute(now) {
      var modes = [];
      d.failures = 0;
      d.nextTry = 0;
      for (var id in streams) {
        if (!Object.prototype.hasOwnProperty.call(streams, id)) continue;
        var st = streams[id];
        modes.push(tix.liveStreamMode(st, online));
        if (!st.live) {
          d.failures = Math.max(d.failures, st.attempt || 0);
          if (st.retryAt && (!d.nextTry || st.retryAt < d.nextTry)) d.nextTry = st.retryAt;
        }
      }
      var next = tix.liveOverall(modes);
      // Every boosted navigation opens the page's streams afresh. Connecting
      // after live is that, not news, so it neither moves Since nor writes a
      // line in Recent changes.
      if (next === "connecting" && d.mode === "live") next = "live";
      if (next === d.mode) return;
      d.mode = next;
      d.since = now;
      if (next !== "none" && next !== "connecting") remember({ at: now, mode: next });
    }

    function fail(now, error) {
      error.at = now;
      d.lastError = error;
      remember({ at: now, error: error.kind, source: error.source });
    }

    function note(n, now) {
      var st = streams[n.stream];
      if (n.kind === "bound") {
        streams[n.stream] = { live: false, everOpen: false, attempt: 0, fetchOk: false, netDown: false, retryAt: 0 };
      } else if (n.kind === "unbound") {
        delete streams[n.stream];
      } else if (st) {
        apply(st, n, now);
      }
      recompute(now);
    }

    function apply(st, n, now) {
      switch (n.kind) {
        case "open":
          st.live = true;
          st.everOpen = true;
          st.attempt = 0;
          st.netDown = false;
          st.retryAt = 0;
          d.lastContact = now;
          break;
        case "event":
          d.lastEvent = now;
          d.lastContact = now;
          if (n.seq > d.lastSeq) d.lastSeq = n.seq;
          break;
        case "closed":
          st.live = false;
          st.fetchOk = false;
          fail(now, { kind: tix.liveErrorKind(n), source: "websocket", code: n.code || 0, reason: n.reason || "" });
          break;
        case "retrying":
          st.attempt = n.attempt;
          st.retryAt = now + (n.wait || 0);
          break;
        case "fetched":
          d.lastContact = now;
          st.netDown = false;
          if (!st.live) st.fetchOk = true;
          break;
        case "fetch-failed":
          if (n.status) {
            st.netDown = false;
            fail(now, { kind: "http", source: "fetch", status: n.status });
          } else {
            st.netDown = true;
            fail(now, { kind: "unreachable", source: "fetch", message: n.message || "" });
          }
          st.fetchOk = false;
          break;
        case "refused":
          if (!(tix.resumeRefused && tix.resumeRefused({ message: n.message }))) {
            fail(now, { kind: "refused", source: "websocket", message: n.message || "" });
          }
          break;
      }
    }

    return {
      note: note,
      online: function (value, now) {
        online = value;
        recompute(now);
      },
      details: function () { return d; }
    };
  };

  function pad(n) { return (n < 10 ? "0" : "") + n; }

  // liveClock writes an instant as a time of day, with the date when it is
  // not the same day as now. Zero is "never".
  tix.liveClock = function (at, now, never) {
    if (!at) return never;
    var t = new Date(at);
    var n = new Date(now);
    var time = pad(t.getHours()) + ":" + pad(t.getMinutes()) + ":" + pad(t.getSeconds());
    if (t.toDateString() === n.toDateString()) return time;
    return t.getFullYear() + "-" + pad(t.getMonth() + 1) + "-" + pad(t.getDate()) + " " + time;
  };

  // liveReport is the text Copy details puts on the clipboard: everything
  // the dialog shows, in English for whoever is asked to read it, and no
  // cookie, token or session in it.
  tix.liveReport = function (d, ctx) {
    var iso = function (at) { return at ? new Date(at).toISOString() : "never"; };
    var lines = [
      "tix live updates report",
      "server: " + ctx.host,
      "build: " + (ctx.build || "unknown"),
      "browser: " + ctx.agent,
      "online: " + (ctx.online === false ? "no" : "yes"),
      "now: " + iso(ctx.now),
      "state: " + d.mode,
      "since: " + iso(d.since),
      "last reached the server: " + iso(d.lastContact),
      "last event: " + iso(d.lastEvent) + (d.lastSeq ? " (seq " + d.lastSeq + ")" : ""),
      "failures in a row: " + d.failures,
      "next try: " + (d.nextTry ? iso(d.nextTry) : "none")
    ];
    var e = d.lastError;
    if (e) {
      lines.push("last error: " + e.kind + " during " + e.source + " at " + iso(e.at) +
        (e.code ? ", close code " + e.code : "") + (e.reason ? ", reason " + e.reason : "") +
        (e.status ? ", HTTP " + e.status : "") + (e.message ? ", " + e.message : ""));
    } else {
      lines.push("last error: none");
    }
    var hints = tix.liveHints(d);
    if (hints.length) lines.push("hints: " + hints.join(", "));
    lines.push("recent:");
    for (var i = d.history.length - 1; i >= 0; i--) {
      var h = d.history[i];
      lines.push("  " + iso(h.at) + " " + (h.error ? "error " + h.error + " (" + h.source + ")" : h.mode));
    }
    return lines.join("\n");
  };

  var doc = global.document;
  if (!doc || !doc.addEventListener) {
    return;
  }

  var model = tix.liveModel();
  var words = null;

  function now() { return Date.now(); }

  // word is one of the indicator's texts in the page's language, from the
  // table the layout renders (livestatus.go).
  function word(key) {
    if (words === null) {
      try {
        var el = doc.getElementById("tix-live-words");
        words = el ? JSON.parse(el.textContent) : {};
      } catch (err) {
        words = {};
      }
    }
    return Object.prototype.hasOwnProperty.call(words, key) ? words[key] : key;
  }

  function set(root, name, text) {
    var el = root.querySelector('[data-live-field="' + name + '"]');
    if (el) el.textContent = text;
  }

  function show(el, on) {
    if (!el) return;
    if (on) el.removeAttribute("hidden");
    else el.setAttribute("hidden", "hidden");
  }

  function drawIndicator(d) {
    var button = doc.querySelector("[data-live-status]");
    if (!button) return;
    if (d.mode === "none") {
      button.setAttribute("hidden", "hidden");
      return;
    }
    button.removeAttribute("hidden");
    button.setAttribute("data-live-mode", d.mode);
    var label = word("mode:" + d.mode);
    button.setAttribute("aria-label", word("Live updates: %s").replace("%s", label));
    button.setAttribute("title", word("info:" + d.mode));
    var text = button.querySelector("[data-live-label]");
    // Live and connecting are the dot alone; anything worse adds its word,
    // so a degraded state is never only a colour.
    if (text) text.textContent = d.mode === "live" || d.mode === "connecting" ? "" : label;
  }

  function errorWords(e) {
    return word("error:" + e.kind);
  }

  function drawDetails(d) {
    var dialog = doc.getElementById("live-details");
    if (!dialog || !dialog.open) return;
    var t = now();
    var never = word("Not yet");
    set(dialog, "summary", word("info:" + d.mode));
    set(dialog, "host", global.location ? global.location.host : "");
    set(dialog, "mode", word("mode:" + d.mode));
    set(dialog, "transport", word("transport:" + (d.mode === "live" ? "websocket" : d.mode === "polling" ? "poll" : "none")));
    set(dialog, "since", tix.liveClock(d.since, t, never));
    set(dialog, "contact", tix.liveClock(d.lastContact, t, never));
    set(dialog, "event", tix.liveClock(d.lastEvent, t, never));
    set(dialog, "failures", String(d.failures));
    set(dialog, "next", d.nextTry && d.mode !== "live" ? tix.liveClock(d.nextTry, t, never) : "–");
    var e = d.lastError;
    show(dialog.querySelector("[data-live-noerror]"), !e);
    show(dialog.querySelector("[data-live-error]"), !!e);
    if (e) {
      set(dialog, "kind", errorWords(e));
      set(dialog, "at", tix.liveClock(e.at, t, never));
      set(dialog, "source", word("source:" + e.source));
      set(dialog, "code", e.code ? String(e.code) + (e.reason ? " · " + e.reason : "") : "–");
      set(dialog, "status", e.status ? String(e.status) : "–");
      set(dialog, "message", e.message || "–");
    }
    var hints = dialog.querySelector("[data-live-hints]");
    if (hints) {
      hints.textContent = "";
      var list = tix.liveHints(d);
      for (var i = 0; i < list.length; i++) {
        var p = doc.createElement("p");
        p.className = "livehint";
        p.textContent = word("hint:" + list[i]).replace("%s", e && e.status ? String(e.status) : "?");
        hints.appendChild(p);
      }
      show(dialog.querySelector("[data-live-hints-head]"), list.length > 0);
    }
    var history = dialog.querySelector("[data-live-history]");
    if (history) {
      history.textContent = "";
      for (var j = d.history.length - 1; j >= 0; j--) {
        var h = d.history[j];
        var li = doc.createElement("li");
        var when = doc.createElement("span");
        when.className = "livewhen";
        when.textContent = tix.liveClock(h.at, t, never);
        var what = doc.createElement("span");
        if (h.error) {
          what.className = "liveerr";
          what.textContent = word("error:" + h.error) + " (" + word("source:" + h.source) + ")";
        } else {
          what.textContent = word("mode:" + h.mode);
        }
        li.appendChild(when);
        li.appendChild(what);
        history.appendChild(li);
      }
      show(dialog.querySelector("[data-live-history-empty]"), d.history.length === 0);
    }
  }

  function draw() {
    var d = model.details();
    drawIndicator(d);
    drawDetails(d);
  }

  tix.liveNote = function (n) {
    model.note(n, now());
    draw();
  };

  // liveDetails exposes the folded state, for the browser tests.
  tix.liveDetails = function () { return model.details(); };

  function copyReport(dialog) {
    var text = tix.liveReport(model.details(), {
      host: global.location ? global.location.host : "",
      build: doc.body ? doc.body.getAttribute("data-build") : "",
      agent: global.navigator ? global.navigator.userAgent : "",
      online: global.navigator ? global.navigator.onLine : true,
      now: now()
    });
    var status = dialog.querySelector("[data-live-copied]");
    var done = function (ok) {
      if (status) status.textContent = word(ok ? "copied" : "copy-failed");
    };
    if (global.navigator && global.navigator.clipboard && global.navigator.clipboard.writeText) {
      global.navigator.clipboard.writeText(text).then(function () { done(true); }, function () { done(false); });
    } else {
      done(false);
    }
  }

  doc.addEventListener("click", function (event) {
    var target = event.target && event.target.closest ? event.target : null;
    if (!target) return;
    var dialog = doc.getElementById("live-details");
    if (target.closest("[data-live-status]") && dialog && dialog.showModal) {
      var status = dialog.querySelector("[data-live-copied]");
      if (status) status.textContent = "";
      dialog.showModal();
      draw();
      return;
    }
    if (target.closest("[data-live-reconnect]")) {
      if (tix.liveRetry) tix.liveRetry();
      draw();
      return;
    }
    if (target.closest("[data-live-copy]") && dialog) {
      copyReport(dialog);
      return;
    }
    if (target.closest("[data-live-close]") && dialog) {
      dialog.close();
    }
  });

  // A boosted navigation draws the shell again, indicator included, so it is
  // drawn from the model rather than left at the markup's default.
  doc.addEventListener("htmx:load", draw);
  if (global.addEventListener) {
    global.addEventListener("online", function () { model.online(true, now()); draw(); });
    global.addEventListener("offline", function () { model.online(false, now()); draw(); });
  }
  if (global.navigator && global.navigator.onLine === false) {
    model.online(false, now());
  }
  // The details tick while open, so Since and Next try stay true.
  if (global.setInterval) {
    global.setInterval(function () {
      var dialog = doc.getElementById("live-details");
      if (dialog && dialog.open) drawDetails(model.details());
    }, 1000);
  }
})(typeof globalThis !== "undefined" ? globalThis : this);
