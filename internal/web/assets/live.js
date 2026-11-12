// SPDX-License-Identifier: AGPL-3.0-or-later

// Everything in this file is bound at the document, or rebound on htmx:load,
// and never to an element captured when the file first ran.
//
// The layout sets hx-boost on <body>, so every internal navigation is an htmx
// swap of the body's contents rather than a document load. The scripts in
// <head> run exactly once, at the first page a visitor happens to open. A
// setup that captured `document.querySelector(".board")` at that moment found
// nothing -- the first page is almost never a board -- and stood down for the
// rest of the session, so drag-and-drop worked only for somebody who typed a
// board's URL and never for somebody who clicked their way to it. The live
// feed had the same fault for the same reason. Resolving the element at event
// time, or on htmx:load, is what makes the two arrivals equivalent.

(function () {
  "use strict";
  var tix = window.tix;
  if (!tix) {
    return;
  }

  // One live connection, four screens.
  //
  // A notification refreshes what is on screen by asking the server to render
  // it again, the same way it did on load: the activity feed re-runs the
  // block templates/activity.html used (activity.go showActivityFeed), and
  // the task list, the board and one task's screen re-run theirs
  // (live.go renderFragment). That is the whole rendering. There is no
  // second, JavaScript-side copy of a row, a card, a status pill or the
  // grouping rule, and no client-side model of what a task is: the frames on
  // the socket say only that something changed, never what it now looks like.
  //
  // Everything below is one mechanism with four descriptors, because four
  // copies of a reconnect is four chances to get it wrong. What differs
  // between them is where to re-fetch from, which events to ask for, and what
  // the answer replaces; what they share is the connection: resume, backoff,
  // the fallback poll and the indicator that says when none of it is working.

  // pollInterval is how often a screen that could not open a socket at all
  // asks the server anyway. Deliberately far slower than the socket path:
  // it exists so that a reader behind an intermediary that eats WebSockets
  // still sees changes within a quarter of a minute, not so that it becomes
  // the normal way this works.
  var pollInterval = 15000;

  // The reconnect curve: 0.5s, 1, 2, 4, 8, 16, 30, 30... each jittered over
  // the lower half of its interval so a server coming back up is not met by
  // every open browser at once. Reset the moment a connection opens.
  var backoffBase = 500;
  var backoffCap = 30000;

  // How long a refresh waits before looking again at a region the reader is
  // working in. Short enough that the screen catches up as soon as they stop,
  // and free: it is one comparison, not a request.
  var busyRetry = 500;

  function setStale(text) {
    var notice = document.getElementById("live-stale");
    if (!notice) {
      return;
    }
    if (text) {
      notice.textContent = text;
      notice.removeAttribute("hidden");
      return;
    }
    notice.setAttribute("hidden", "hidden");
  }

  // liveStream is one screen's connection to the event stream. spec says how
  // to find the binding element, where its fragment lives, what it subscribes
  // to, whether the reader is busy in it, and what the fragment replaces.
  function liveStream(spec) {
    var bound = null;
    var socket = null;
    var pending = null;
    var trigger = null;
    var attempt = 0;
    var seq = 0;
    var retry = null;
    var poll = null;
    var deferred = null;
    var live = false;
    var again = false;
    var dropped = false;
    var latest = null;

    // note tells the shell's connection indicator (assets/livestatus.js)
    // what this stream just went through. It only reports; nothing here
    // waits on it or reads it back.
    function note(what) {
      if (tix.liveNote) {
        what.stream = spec.id;
        tix.liveNote(what);
      }
    }

    // A refresh asked for while one is still on its way is run again when
    // that one lands, never dropped. The coalescer's trailing run used to
    // arrive here while the leading fetch was still out on a slow server,
    // and the last changes of a burst were not drawn until some later event
    // happened to come along.
    function finished() {
      pending = null;
      if (again) {
        again = false;
        refresh();
      }
    }

    function refresh() {
      if (!bound) {
        return;
      }
      if (pending) {
        again = true;
        return;
      }
      if (spec.busy(bound)) {
        // Deferred, never dropped: see the note on busy() below. The wait is
        // its own timer rather than the coalescer's, which runs on its
        // leading edge and would call straight back into here.
        defer();
        return;
      }
      var target = bound;
      pending = fetch(spec.href(target), { credentials: "same-origin" })
        .then(function (resp) {
          note(resp.ok ? { kind: "fetched" } : { kind: "fetch-failed", status: resp.status });
          return resp.ok ? resp.text() : null;
        }, function (err) {
          // Only a request that never got an answer is the server out of
          // reach; a failure drawing the answer is caught further down.
          note({ kind: "fetch-failed", message: err && err.message ? String(err.message) : "" });
          return null;
        })
        .then(function (html) {
          // A boosted navigation can swap the page in before htmx:load moves
          // the binding, so the element this was asked for may already be
          // gone while bound still names it. The new page was rendered after
          // the change that prompted this, and drawing the old page's answer
          // over it threw away its own state: the row a just-added task was
          // marked on lost its ring.
          if (html === null || bound !== target || spec.find() !== target) {
            finished();
            return;
          }
          if (spec.busy(target)) {
            pending = null;
            again = false;
            defer();
            return;
          }
          spec.apply(target, html);
          finished();
        })
        .catch(function () {
          finished();
        });
    }

    // settled reports that nothing this stream was asked to draw is still
    // to come: no fetch out, none queued behind it, none waiting for the
    // reader or for the coalescer's trailing run.
    function settled() {
      return !pending && !again && deferred === null && !(trigger && trigger.queued && trigger.queued());
    }

    function schedule() {
      if (trigger) {
        trigger();
      }
    }

    // defer re-checks shortly, for as long as the reader is busy. It costs no
    // request and no render: the update lands the moment they stop.
    function defer() {
      if (deferred !== null) {
        return;
      }
      deferred = window.setTimeout(function () {
        deferred = null;
        refresh();
      }, busyRetry);
    }

    function stopRetry() {
      if (retry !== null) {
        window.clearTimeout(retry);
        retry = null;
      }
    }

    function stopPolling() {
      if (poll !== null) {
        window.clearInterval(poll);
        poll = null;
      }
    }

    // startPolling is the fallback for a socket that cannot be established at
    // all. It stops the moment one opens, so a connection that works never
    // pays for it.
    function startPolling() {
      if (poll !== null || !window.setInterval) {
        return;
      }
      poll = window.setInterval(function () {
        if (!document.hidden) {
          refresh();
        }
      }, pollInterval);
    }

    function announce() {
      if (document.documentElement) {
        document.documentElement.setAttribute("data-live", live ? "on" : "off");
      }
      setStale(live ? "" : tix.staleNotice(poll !== null));
    }

    // A hidden page is not reconnected: a laptop that slept wakes with every
    // open tab retrying at once, and a background tab nobody is looking at
    // has nothing to keep current. Coming back to the page connects at once
    // and re-fetches, which is the sleep case and by far the common one.
    function later() {
      stopRetry();
      if (document.hidden) {
        return;
      }
      var wait = tix.backoffDelay(attempt, backoffBase, backoffCap);
      attempt++;
      startPolling();
      announce();
      note({ kind: "retrying", attempt: attempt, wait: wait });
      retry = window.setTimeout(connect, wait);
    }

    function closed() {
      if (socket === null) {
        return;
      }
      socket = null;
      live = false;
      dropped = true;
      later();
    }

    // reconnected tells assets/deploy.js the socket came back after a drop,
    // which is what a server restart for a new release looks like from here.
    function reconnected() {
      if (typeof window.CustomEvent === "function" && document.dispatchEvent) {
        document.dispatchEvent(new window.CustomEvent("tix:live-reconnected"));
      }
    }

    function connect() {
      stopRetry();
      if (!bound || !window.WebSocket || !window.fetch || document.hidden) {
        return;
      }
      var scheme = window.location.protocol === "https:" ? "wss://" : "ws://";
      var opening;
      try {
        opening = new WebSocket(scheme + window.location.host + bound.getAttribute("data-events"));
      } catch (err) {
        later();
        return;
      }
      socket = opening;
      latest = opening;
      note({ kind: "connecting" });
      var opened = false;
      opening.addEventListener("open", function () {
        if (socket !== opening) {
          return;
        }
        opened = true;
        note({ kind: "open" });
        attempt = 0;
        live = true;
        stopPolling();
        announce();
        subscribe(opening);
        if (dropped) {
          dropped = false;
          reconnected();
        }
      });
      opening.addEventListener("message", function (message) {
        if (socket !== opening) {
          return;
        }
        received(opening, message);
      });
      // The close is reported whether or not an error got here first: the
      // error event carries nothing, and the close code is the only thing a
      // browser says about why.
      opening.addEventListener("close", function (event) {
        if (latest === opening) {
          note({
            kind: "closed", opened: opened,
            code: event && event.code, reason: event && event.reason ? String(event.reason) : ""
          });
        }
        if (socket === opening) {
          closed();
        }
      });
      opening.addEventListener("error", function () {
        if (socket === opening) {
          closed();
        }
      });
    }

    // subscribe asks for exactly what this screen can draw, and for the gap
    // since the last event it saw. The server replays the log from there, so
    // a connection that dropped loses nothing; without since_seq every change
    // made during the gap was simply never shown.
    function subscribe(conn) {
      var message = { type: "subscribe", id: spec.id };
      var filter = spec.filter(bound);
      if (filter) {
        message.filter = filter;
      }
      if (seq > 0) {
        message.since_seq = seq;
      }
      conn.send(JSON.stringify(message));
    }

    function received(conn, message) {
      var parsed;
      try {
        parsed = JSON.parse(message.data);
      } catch (err) {
        return;
      }
      if (!parsed) {
        return;
      }
      // A cursor the server no longer holds cannot be replayed, and
      // pretending otherwise would leave a gap nothing ever fills. Drop the
      // cursor, subscribe from now, and re-fetch the fragment whole: the
      // server's rendering is the answer, so a refresh is always correct
      // however much was missed.
      if (parsed.type === "error") {
        note({ kind: "refused", message: parsed.error && parsed.error.message ? String(parsed.error.message) : "" });
      }
      if (parsed.type === "error" && tix.resumeRefused(parsed.error)) {
        seq = 0;
        subscribe(conn);
        schedule();
        return;
      }
      if (parsed.type !== "event" || !parsed.event) {
        return;
      }
      if (parsed.event.seq > seq) {
        seq = parsed.event.seq;
      }
      note({ kind: "event", seq: parsed.event.seq });
      if (!spec.wants(bound, parsed.event)) {
        return;
      }
      schedule();
    }

    // rebind moves the stream to whatever binding the current page carries.
    // The layout boosts every internal navigation, so the element this
    // connected to is replaced without the script running again; it is safe
    // to call on every swap, because a page whose binding is the one already
    // held is left alone and a page with none drops the socket.
    function rebind() {
      var next = spec.find();
      if (!tix.shouldRebind(next, bound)) {
        return;
      }
      stopRetry();
      stopPolling();
      if (deferred !== null) {
        window.clearTimeout(deferred);
        deferred = null;
      }
      latest = null;
      if (socket) {
        var going = socket;
        socket = null;
        going.close();
      }
      live = false;
      bound = next;
      seq = 0;
      attempt = 0;
      note({ kind: bound ? "bound" : "unbound" });
      trigger = bound ? tix.coalescer(spec.window(bound), refresh, null, spec.cap) : null;
      setStale("");
      connect();
    }

    function woken() {
      if (document.hidden || !bound) {
        return;
      }
      if (!live) {
        attempt = 0;
        connect();
      }
      // Whatever happened while the page was away is in the fragment, so ask
      // for it rather than reasoning about what was missed.
      schedule();
    }

    // retryNow is Reconnect now: a socket that is down is opened at once,
    // whatever the backoff was waiting for, and the screen is fetched again.
    function retryNow() {
      if (!bound || live) {
        return;
      }
      attempt = 0;
      connect();
      schedule();
    }

    // now redraws at once, for the refresh shortcut, and answers whether
    // this stream has a region on the page to redraw.
    function now() {
      if (!bound) {
        return false;
      }
      refresh();
      return true;
    }

    return { rebind: rebind, woken: woken, poke: schedule, settled: settled, retryNow: retryNow, now: now };
  }

  // The activity feed. Its fragment is the row list itself, so the response
  // is the region: see activity.go showActivityFeed.
  var feed = liveStream({
    id: "feed",
    find: function () {
      var el = document.getElementById("live-feed");
      return el && el.getAttribute("data-events") && el.getAttribute("data-feed") ? el : null;
    },
    href: function (el) { return el.getAttribute("data-feed"); },
    filter: function () { return null; },
    wants: function () { return true; },
    busy: function () { return false; },
    // Rows a refresh draws are handed to htmx like the screen's regions are
    // (processed below): a Raw control on a new row was a plain link, so it
    // reloaded the page instead of fetching its dialog.
    apply: function (el, html) {
      if (!tix.morph || !document.scrollingElement) {
        el.innerHTML = html;
        processed(el);
        return;
      }
      var fresh = document.createElement("ul");
      fresh.innerHTML = html;
      var anchors = tix.liveAnchors([el]);
      tix.liveHold(anchors, tix.morph(el, fresh));
      processed(el);
    },
    window: function () { return 500; },
    cap: 1000
  });

  // The task list, the board and one task's own screen, through one element
  // the page renders (partials.html "live-screen") carrying where to
  // re-fetch from, which events to ask for, and which regions the answer
  // replaces.
  //
  // data-live carries the screen's whole state -- the filter, the deadline
  // window, the sort, the cursor, the project or the task in view -- because
  // the server wrote it into the attribute from the request that drew the
  // page. The list/board switch, the column picker and the project
  // visibility choice are cookies, which the fetch sends. So a refresh
  // answers the same question the page was already answering, rather than
  // widening it back out to the first page of everything.
  var screen = liveStream({
    id: "screen",
    find: function () {
      var el = document.getElementById("live-screen");
      return el && el.getAttribute("data-events") && el.getAttribute("data-live") ? el : null;
    },
    href: function (el) { return el.getAttribute("data-live"); },
    filter: function (el) {
      try {
        return JSON.parse(el.getAttribute("data-live-filter") || "null");
      } catch (err) {
        return null;
      }
    },
    wants: function (el, event) {
      return tix.wantsEvent(el.getAttribute("data-live-subject") || "", event);
    },
    busy: busyScreen,
    apply: applyRegions,
    window: function (el) {
      var ms = parseInt(el.getAttribute("data-live-window"), 10);
      return ms > 0 ? ms : 300;
    },
    cap: 1000
  });

  // The sidebar's unread badge (partials.html "inbox-badge"), on every
  // screen. It subscribes to inbox.updated alone, so the work stream never
  // wakes it; the event names its recipients by identifier, which the page
  // does not carry, so every one of them asks the server for the count, the
  // coalescer folding a burst into one request. The answer carries My work's
  // overdue badge too ("side-badges"): a task assigned to the reader, or one
  // of theirs changing status, is an inbox item, so the same event that moves
  // one count is the one that can move the other.
  var badge = liveStream({
    id: "inbox",
    find: function () {
      var el = document.getElementById("inbox-badge");
      return el && el.getAttribute("data-events") && el.getAttribute("data-inbox") ? el : null;
    },
    href: function (el) { return el.getAttribute("data-inbox"); },
    filter: function () { return { types: ["inbox.updated"] }; },
    wants: function () { return true; },
    busy: function () { return false; },
    apply: function (el, html) {
      var doc = new DOMParser().parseFromString(html, "text/html");
      var fresh = doc.getElementById("inbox-badge");
      if (fresh) {
        tix.applyBadge(el, fresh, document.querySelectorAll("[data-inbox-mirror]"));
      }
      tix.applyBadge(document.getElementById("mywork-badge"), doc.getElementById("mywork-badge"), [], "data-badge-count");
      if (fresh && typeof window.CustomEvent === "function") {
        document.dispatchEvent(new window.CustomEvent("tix:inbox-badge", {
          detail: { count: parseInt(fresh.getAttribute("data-inbox-count"), 10) || 0 }
        }));
      }
    },
    window: function () { return 500; },
    cap: 2000
  });

  function regionIDs(el) {
    var listed = el ? el.getAttribute("data-live-regions") : "";
    return listed ? listed.split(/\s+/) : [];
  }

  // busyScreen reports whether the reader is in the middle of something a
  // refresh would throw away: a card held in a drag, or a field inside a
  // replaced region that has been typed into or has the caret in it.
  //
  // The rule is to defer, never to drop. A refresh that lands mid-drag or
  // over a half-written comment destroys work the reader cannot get back,
  // while one that waits costs only the seconds until they stop: the trigger
  // is re-armed, so the screen converges as soon as the region is idle. The
  // alternative -- refreshing everything except the busy region -- would
  // leave a status pill disagreeing with the form under it, which is the
  // same staleness this exists to remove, in a harder place to notice.
  //
  // Only the regions actually replaced are consulted. The quick-add box, the
  // filter bar and the task screen's own edit forms sit outside them, so
  // typing in those never holds an update back.
  function busyScreen(el) {
    var dragging = !!document.querySelector(".is-dragging");
    var ids = regionIDs(el);
    for (var i = 0; i < ids.length; i++) {
      if (tix.regionBusy(document.getElementById(ids[i]), document.activeElement, dragging)) {
        return true;
      }
    }
    return dragging;
  }

  // processed hands markup this file wrote to htmx. htmx only wires up what
  // it swapped in itself, so every form a refresh drew had no hx-post and no
  // boost: the next tick or move was a plain post that reloaded the page and
  // threw the reader back to the top. An element already wired keeps its
  // wiring unless the morph changed its attributes.
  function processed(el) {
    if (window.htmx && typeof window.htmx.process === "function") {
      window.htmx.process(el);
    }
  }

  // patch draws a fresh copy of a region over the one on screen. Morphed in
  // place where the page is a real DOM (tix.morph below); the plain
  // replacement is left for a document that cannot be walked.
  function patch(here, fresh) {
    if (fresh.childNodes && here.childNodes && typeof here.insertBefore === "function") {
      return tix.morph(here, fresh);
    }
    var places = tix.scrollPlaces(here.querySelectorAll("*"));
    here.innerHTML = fresh.innerHTML;
    tix.restorePlaces(here.querySelectorAll("*"), places);
    return [];
  }

  // applyRegions draws each region's fresh copy and leaves the reader where
  // they were.
  //
  // It used to replace each region's markup whole and then put back the
  // scroll offset of every box inside it. Under a burst of changes that was
  // a new task list, new board columns and new lanes three times a second:
  // a wheel or a swipe in progress was scrolling a box that had just been
  // thrown away, so the reader's scrolling was lost and the list snapped back
  // to where it stood when the update began; every scrollbar on the screen
  // was a new one, so they all blinked; and every row played its entrance
  // animation again. Morphing keeps every row, card, column and scrolling
  // box that is still there as the same element, so none of that happens,
  // and only rows that are genuinely new are inserted.
  //
  // What scrolls is then held to the row the reader was looking at, not to
  // an offset: a task inserted above it would otherwise push it down. When
  // the reader is away from the top and new rows land above them, the pill
  // says so instead of moving the page (tix.liveAnchors, tix.livePill).
  function applyRegions(el, html) {
    var doc = new DOMParser().parseFromString(html, "text/html");
    var ids = regionIDs(el);
    var regions = [];
    for (var i = 0; i < ids.length; i++) {
      var here = document.getElementById(ids[i]);
      var fresh = doc.getElementById(ids[i]);
      if (fresh && here) {
        regions.push({ here: here, fresh: fresh });
      }
    }
    var anchors = tix.liveAnchors && document.scrollingElement
      ? tix.liveAnchors(regions.map(function (r) { return r.here; })) : null;
    var added = [];
    for (var j = 0; j < regions.length; j++) {
      var memo = tix.focusMemo(document.activeElement, regions[j].here);
      added = added.concat(patch(regions[j].here, regions[j].fresh));
      processed(regions[j].here);
      if (memo && !regions[j].here.contains(document.activeElement)) {
        tix.focusRestore(document, memo);
      }
    }
    if (anchors) {
      tix.liveHold(anchors, added);
    }
    // Tells a screen's own script that its regions were redrawn, so it can
    // mark what is new (assets/inbox.js).
    document.dispatchEvent(new CustomEvent("tix:regions", { detail: ids }));
  }

  // liveSettled reports that the screen and the activity feed have drawn
  // everything they were asked to. The browser tests wait on it rather than
  // on the clock.
  tix.liveSettled = function () {
    return screen.settled() && feed.settled();
  };

  // liveRefresh is the refresh shortcut (assets/shortcuts.js): the screen's
  // regions and the activity feed redraw in place. False when the page has
  // neither, and the shortcut reloads it instead.
  tix.liveRefresh = function () {
    var drawn = screen.now();
    return feed.now() || drawn;
  };

  function rebindAll() {
    feed.rebind();
    screen.rebind();
    badge.rebind();
  }

  // liveRetry is the connection details' Reconnect now, for every stream.
  tix.liveRetry = function () {
    feed.retryNow();
    screen.retryNow();
    badge.retryNow();
  };

  rebindAll();
  document.addEventListener("htmx:load", rebindAll);
  document.addEventListener("visibilitychange", function () {
    feed.woken();
    screen.woken();
    badge.woken();
  });
  // A save made where no event is guaranteed to reach this page in time (a
  // task saved in the dialog, a title renamed in place) asks for the
  // refresh directly. See taskopen.js.
  document.addEventListener("tix:refresh", function () {
    screen.poke();
  });
  // An inbox item marked read from this page (assets/inbox.js) asks for the
  // badge at once rather than waiting for its own event to come round.
  document.addEventListener("tix:inbox", function () {
    badge.poke();
  });
})();

// htmx does not swap a non-2xx response, so a refused form used to change
// nothing on screen and say nothing. Every refusal here renders a full page
// explaining itself, so swap it in. This runs for every boosted form on
// every page, not only the board: the drag-and-drop path below never goes
// through htmx at all, which is why it does its own fetch and its own error
// handling instead of relying on this.
(function () {
  "use strict";
  document.body.addEventListener("htmx:beforeSwap", function (event) {
    var status = event.detail.xhr.status;
    if (status >= 400) {
      event.detail.shouldSwap = true;
      event.detail.isError = false;
    }
  });
})();

(function () {
  "use strict";
  var tix = window.tix;
  if (!tix) {
    return;
  }

  // The board is resolved from the event, never captured: see the note at the
  // top of this file. Two gates, both in tix.dragEnabled. The settings toggle
  // (partials.html "settings-menu", posts to /dragmove) is server truth, read
  // here the same way the board itself reads it: as a data attribute the
  // template already rendered from the cookie, since the cookie is HttpOnly
  // and this script cannot read it directly. The Move disclosure on every
  // card needs none of this and keeps working exactly as before either way.
  //
  // Native HTML5 drag-and-drop has no working touch equivalent in mobile
  // browsers -- there is no drop event a touch gesture ever fires -- and
  // leaving "draggable" live on a coarse pointer only fights the page's own
  // scrolling and long-press behaviour for no payoff. So tix.dragEnabled
  // scopes this to a fine pointer (mouse, trackpad, pen): a touch user gets
  // the Move disclosure, which already works everywhere, rather than a
  // half-working drag.
  function boardFor(event) {
    var target = event && event.target;
    if (!target || typeof target.closest !== "function") {
      return null;
    }
    var board = target.closest(".board");
    if (!board || !tix.dragEnabled(board.getAttribute("data-drag"), window.matchMedia && window.matchMedia.bind(window))) {
      return null;
    }
    return board;
  }

  function announce(board, text) {
    var status = document.getElementById("board-status");
    if (status) {
      status.textContent = text;
    }
  }

  function columnLabel(column) {
    var heading = column.querySelector("h2");
    return heading ? tix.stripCount(heading.textContent) : column.getAttribute("data-state");
  }

  function clearDropTargets(board) {
    var marked = board.querySelectorAll(".column.is-drop-target");
    for (var i = 0; i < marked.length; i++) {
      marked[i].classList.remove("is-drop-target");
    }
  }

  // The set of states the dragged card may legally move to, read from its
  // own Move disclosure's <select> -- the same list the server already
  // computed for that card (projects.go flowRoutes) -- so a column outside
  // it never lights up as a drop target and a drop on one is refused before
  // any request is made, not just after the server says no.
  var dragged = null;
  var legal = null;

  document.addEventListener("dragstart", function (event) {
    var board = boardFor(event);
    if (!board) {
      return;
    }
    var card = event.target.closest(".card");
    var select = card && card.querySelector("form select[name=route]");
    if (!card || !select) {
      if (event.dataTransfer) {
        event.dataTransfer.effectAllowed = "none";
      }
      return;
    }
    dragged = card;
    legal = tix.legalStates(select.options);
    card.classList.add("is-dragging");
    if (event.dataTransfer) {
      event.dataTransfer.effectAllowed = "move";
      // Firefox requires data to be set for the drag to proceed at all.
      event.dataTransfer.setData("text/plain", card.getAttribute("data-task") || "");
    }
  });

  document.addEventListener("dragend", function (event) {
    if (dragged) {
      dragged.classList.remove("is-dragging");
      var board = dragged.closest(".board");
      if (board) {
        clearDropTargets(board);
      }
    }
    dragged = null;
    legal = null;
  });

  document.addEventListener("dragover", function (event) {
    var board = boardFor(event);
    var column = board && event.target.closest(".column");
    if (!dragged || !column || !tix.canDrop(legal, column.getAttribute("data-state"))) {
      return;
    }
    event.preventDefault();
    if (event.dataTransfer) {
      event.dataTransfer.dropEffect = "move";
    }
    if (!column.classList.contains("is-drop-target")) {
      clearDropTargets(board);
      column.classList.add("is-drop-target");
    }
  });

  // A refused move -- the request failed, or the server refused it, most
  // often because someone else changed the task while this page was open --
  // leaves the card exactly where it started: nothing here ever moves it in
  // the DOM before the server confirms it, so there is no position to roll
  // back. Only the explanation, through the live region, says what happened.
  document.addEventListener("drop", function (event) {
    var board = boardFor(event);
    var column = board && event.target.closest(".column");
    var card = dragged;
    if (!card || !column || !tix.canDrop(legal, column.getAttribute("data-state"))) {
      return;
    }
    event.preventDefault();
    clearDropTargets(board);

    // The Move form, not the card's first form: the star button is a form
    // too and comes first, so taking the first one posted the move to /star.
    var select = card.querySelector("form select[name=route]");
    var form = select && select.closest("form");
    if (!form || !select) {
      return;
    }
    var wanted = column.getAttribute("data-state");
    var ref = (card.getAttribute("data-task") || "").toUpperCase() || "the card";
    var destination = columnLabel(column);
    select.value = wanted;

    fetch(form.getAttribute("action"), {
      method: "POST",
      credentials: "same-origin",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: new URLSearchParams(new FormData(form)).toString()
    }).then(function (resp) {
      if (!resp.ok) {
        return resp.text().then(function (html) {
          announce(board, tix.refusedMessage(ref, destination, extractError(html)));
        });
      }
      // The server is the only source of truth for what a card may do next
      // (its legal states, its version), so success re-fetches the board
      // fragment rather than moving the card by hand here and guessing.
      return refreshBoard(board, tix.movedMessage(ref, destination));
    }).catch(function () {
      announce(board, tix.refusedMessage(ref, destination, "the request failed."));
    });
  });

  // extractError pulls the message a refused move's error page carries
  // (templates/error.html renders it as .failure-message) out of the raw HTML a
  // failed fetch returned, since that response was never swapped into the
  // page the way htmx would swap a boosted navigation.
  function extractError(html) {
    try {
      var doc = new DOMParser().parseFromString(html, "text/html");
      var el = doc.querySelector(".failure-message, .flash.error");
      return el ? el.textContent.trim() : "";
    } catch (err) {
      return "";
    }
  }

  // refreshBoard re-fetches this same board page and swaps in only the
  // board's own contents, not the whole page. The listeners above are on the
  // document, so a freshly rendered card -- with its own now-current Move
  // options -- is live whatever this replaces.
  //
  // The search string is part of the page. The task screen draws a board of
  // the same filtered, sorted, paged listing the list shows, so re-fetching
  // the path alone answered a different question than the one on screen: the
  // reader's filter was dropped and the board came back holding every task on
  // the first page. A project's own board has no query to lose and is
  // unaffected.
  var refreshing = null;
  function refreshBoard(board, announceText) {
    if (refreshing) {
      return refreshing;
    }
    refreshing = fetch(window.location.pathname + window.location.search, { credentials: "same-origin" })
      .then(function (resp) { return resp.ok ? resp.text() : null; })
      .then(function (html) {
        refreshing = null;
        if (html === null) {
          window.location.reload();
          return;
        }
        var fresh = new DOMParser().parseFromString(html, "text/html").querySelector(".board");
        if (fresh) {
          board.innerHTML = fresh.innerHTML;
          if (window.htmx && typeof window.htmx.process === "function") {
            window.htmx.process(board);
          }
        }
        announce(board, announceText);
      })
      .catch(function () {
        refreshing = null;
        window.location.reload();
      });
    return refreshing;
  }
})();

// Drawing a live update in place. Everything here runs against a real
// document; the tests that need layout drive it in a browser
// (internal/web/browser_live_test.go).
(function () {
  "use strict";
  var tix = window.tix;
  if (!tix) {
    return;
  }

  // say is text in the page's language; see i18n.js.
  function say(text) {
    return tix.t ? tix.t(text) : text;
  }

  // keyOf names an element across two renderings of the same region: a row,
  // card or lane by its id, a board column or list group by its group key.
  // Anything without one is matched by position among its unnamed siblings.
  function keyOf(node) {
    if (!node || node.nodeType !== 1) {
      return "";
    }
    var key = node.id || node.getAttribute("data-group") || node.getAttribute("data-state") || "";
    return key ? node.tagName + "#" + key : "";
  }

  function sameKind(a, b) {
    return a.nodeType === b.nodeType && (a.nodeType !== 1 || a.tagName === b.tagName);
  }

  // A field's value is the reader's once they have touched it. The region is
  // never refreshed while one is dirty (decide.js regionBusy), so this only
  // keeps a focused, untouched field from being rewritten under the caret.
  function syncField(old, fresh) {
    if (old === document.activeElement) {
      return;
    }
    if (old.tagName === "INPUT") {
      if (old.type === "checkbox" || old.type === "radio") {
        old.checked = fresh.hasAttribute("checked");
      } else if (old.type !== "file") {
        old.value = fresh.getAttribute("value") || "";
      }
    } else if (old.tagName === "TEXTAREA") {
      old.value = fresh.textContent;
    }
  }

  // keptAttribute is an attribute the reader, not the server, owns: whether a
  // disclosure is open. The server always draws it closed, and a refresh
  // shutting the panel the reader had just opened is the screen taking it
  // away from them.
  function keptAttribute(el, name) {
    return name === "open" && el.tagName === "DETAILS";
  }

  function syncAttributes(old, fresh) {
    var i;
    var attrs = fresh.attributes;
    for (i = 0; i < attrs.length; i++) {
      if (keptAttribute(old, attrs[i].name)) {
        continue;
      }
      if (old.getAttribute(attrs[i].name) !== attrs[i].value) {
        old.setAttribute(attrs[i].name, attrs[i].value);
      }
    }
    var gone = [];
    for (i = 0; i < old.attributes.length; i++) {
      var name = old.attributes[i].name;
      if (!fresh.hasAttribute(name) && !keptAttribute(old, name)) {
        gone.push(name);
      }
    }
    for (i = 0; i < gone.length; i++) {
      old.removeAttribute(gone[i]);
    }
  }

  function morphNode(old, fresh, added) {
    if (old.nodeType !== 1) {
      if (old.nodeValue !== fresh.nodeValue) {
        old.nodeValue = fresh.nodeValue;
      }
      return;
    }
    syncAttributes(old, fresh);
    if (old.tagName === "INPUT" || old.tagName === "TEXTAREA") {
      syncField(old, fresh);
    }
    if (old.tagName === "TEXTAREA" && old === document.activeElement) {
      return;
    }
    morphChildren(old, fresh, added);
  }

  // place puts a kept node at a position, atomically where the browser can
  // (moveBefore keeps focus, scroll and a running animation), otherwise by an
  // ordinary insert.
  function place(parent, node, before) {
    if (typeof parent.moveBefore === "function") {
      try {
        parent.moveBefore(node, before);
        return;
      } catch (e) {
        // fall through to the plain insert
      }
    }
    parent.insertBefore(node, before);
  }

  function morphChildren(target, source, added) {
    var keyed = {};
    var used = new Set();
    var node;
    for (node = target.firstChild; node; node = node.nextSibling) {
      var k = keyOf(node);
      if (k && !keyed[k]) {
        keyed[k] = node;
      }
    }
    var cursor = target.firstChild;
    var freshNodes = Array.prototype.slice.call(source.childNodes);
    for (var i = 0; i < freshNodes.length; i++) {
      var fresh = freshNodes[i];
      var key = keyOf(fresh);
      var match = null;
      if (key) {
        match = keyed[key] && !used.has(keyed[key]) && sameKind(keyed[key], fresh) ? keyed[key] : null;
      } else {
        for (node = cursor; node; node = node.nextSibling) {
          if (!used.has(node) && !keyOf(node) && sameKind(node, fresh)) {
            match = node;
            break;
          }
        }
      }
      if (match) {
        used.add(match);
        if (match === cursor) {
          cursor = cursor.nextSibling;
        } else {
          place(target, match, cursor);
        }
        morphNode(match, fresh, added);
        continue;
      }
      var adopted = document.importNode(fresh, true);
      target.insertBefore(adopted, cursor);
      used.add(adopted);
      if (adopted.nodeType === 1) {
        added.push(adopted);
      }
    }
    var leftover = [];
    for (node = target.firstChild; node; node = node.nextSibling) {
      if (!used.has(node)) {
        leftover.push(node);
      }
    }
    for (var j = 0; j < leftover.length; j++) {
      target.removeChild(leftover[j]);
    }
  }

  // morph makes here's contents match fresh's, keeping every element that is
  // in both, and returns the elements it had to insert.
  tix.morph = function (here, fresh) {
    var added = [];
    morphChildren(here, fresh, added);
    return added;
  };

  // scrollers are the boxes a region scrolls in: the page itself, any box
  // around the region, and every box inside it.
  function scrollers(regions) {
    var out = [document.scrollingElement || document.documentElement];
    function add(el) {
      if (el && out.indexOf(el) < 0 && (el.scrollHeight > el.clientHeight || el.scrollWidth > el.clientWidth)) {
        var style = window.getComputedStyle(el);
        if (/(auto|scroll)/.test(style.overflowY + " " + style.overflowX)) {
          out.push(el);
        }
      }
    }
    for (var i = 0; i < regions.length; i++) {
      for (var up = regions[i].parentElement; up && up !== document.body; up = up.parentElement) {
        add(up);
      }
      add(regions[i]);
      var inside = regions[i].querySelectorAll("ul, ol, section, div");
      for (var j = 0; j < inside.length; j++) {
        add(inside[j]);
      }
    }
    return out;
  }

  function viewport(box) {
    if (box === document.scrollingElement || box === document.documentElement) {
      return { top: 0, left: 0, bottom: window.innerHeight, right: window.innerWidth };
    }
    return box.getBoundingClientRect();
  }

  // firstSeen is the first named element inside box whose leading edge is in
  // view along one axis: the row the reader is reading, or the column they
  // scrolled to.
  function firstSeen(box, regions, axis) {
    var view = viewport(box);
    var lead = axis === "y" ? "top" : "left";
    var trail = axis === "y" ? "bottom" : "right";
    for (var i = 0; i < regions.length; i++) {
      var named = regions[i].querySelectorAll("[id], [data-group]");
      for (var j = 0; j < named.length; j++) {
        if (named[j] === box || !box.contains(named[j])) {
          continue;
        }
        var r = named[j].getBoundingClientRect();
        if (r.width === 0 && r.height === 0) {
          continue;
        }
        if (r[lead] >= view[lead] - 1 && r[lead] < view[trail]) {
          return { el: named[j], at: r[lead] };
        }
      }
    }
    return null;
  }

  // liveAnchors records, for every box that is scrolled away from its start,
  // which element the reader has at the top and at the left of it.
  tix.liveAnchors = function (regions) {
    var out = [];
    var boxes = scrollers(regions);
    for (var i = 0; i < boxes.length; i++) {
      var box = boxes[i];
      var entry = { box: box, top: box.scrollTop, left: box.scrollLeft, y: null, x: null };
      if (box.scrollTop > 0) {
        entry.y = firstSeen(box, regions, "y");
      }
      if (box.scrollLeft > 0) {
        entry.x = firstSeen(box, regions, "x");
      }
      out.push(entry);
    }
    return out;
  };

  // liveHold scrolls each box by however far its anchor moved, so the reader
  // is looking at the same row and the same column as before the update, and
  // counts the new rows that landed above them for the pill.
  tix.liveHold = function (anchors, added) {
    var page = document.scrollingElement || document.documentElement;
    var kept = page.style.overflowAnchor;
    page.style.overflowAnchor = "none";
    window.requestAnimationFrame(function () {
      window.requestAnimationFrame(function () {
        page.style.overflowAnchor = kept;
      });
    });
    for (var i = 0; i < anchors.length; i++) {
      var a = anchors[i];
      var box = a.box;
      if (a.y && a.y.el.isConnected) {
        box.scrollTop += a.y.el.getBoundingClientRect().top - a.y.at;
      } else if (box.scrollTop !== a.top) {
        box.scrollTop = a.top;
      }
      if (a.x && a.x.el.isConnected) {
        box.scrollLeft += a.x.el.getBoundingClientRect().left - a.x.at;
      } else if (box.scrollLeft !== a.left) {
        box.scrollLeft = a.left;
      }
      if (!a.y || !a.y.el.isConnected) {
        continue;
      }
      var above = 0;
      var edge = a.y.el.getBoundingClientRect().top;
      for (var j = 0; j < added.length; j++) {
        var row = added[j];
        if (row.isConnected && /^t-/.test(row.id || "") && box.contains(row) &&
            row.getBoundingClientRect().bottom <= edge + 1) {
          above++;
        }
      }
      if (above > 0) {
        tix.livePill(box, above);
      }
    }
  };

  // The pill. One per scrolling box, outside every live region so no refresh
  // ever redraws it, and placed over the top edge of the box it is about.
  var pills = [];

  function pillFor(box) {
    for (var i = 0; i < pills.length; i++) {
      if (pills[i].box === box) {
        return pills[i];
      }
    }
    return null;
  }

  function dismiss(entry) {
    entry.count = 0;
    entry.button.hidden = true;
  }

  function position(entry) {
    var view = viewport(entry.box);
    var top = Math.max(view.top, 0) + 8;
    entry.button.style.top = Math.round(top) + "px";
    entry.button.style.left = Math.round((view.left + view.right) / 2) + "px";
  }

  // livePill says how many new rows arrived above where the reader is, and
  // takes them there on a press. It never moves the page by itself.
  tix.livePill = function (box, count) {
    var entry = pillFor(box);
    if (!entry || !entry.button.isConnected) {
      if (entry) {
        pills.splice(pills.indexOf(entry), 1);
      }
      var button = document.createElement("button");
      button.type = "button";
      button.className = "livepill";
      button.hidden = true;
      entry = { box: box, button: button, count: 0 };
      button.addEventListener("click", function () {
        dismiss(entry);
        box.scrollTo({ top: 0, behavior: "smooth" });
      });
      var watched = box === document.scrollingElement || box === document.documentElement ? window : box;
      watched.addEventListener("scroll", function () {
        if (box.scrollTop <= 0) {
          dismiss(entry);
        }
      }, { passive: true });
      document.body.appendChild(button);
      pills.push(entry);
    }
    entry.count += count;
    entry.button.textContent = say("%d new, show").replace("%d", String(entry.count));
    position(entry);
    entry.button.hidden = false;
  };

  // A boosted navigation leaves the pills of the page it replaced behind,
  // pointing at boxes that are gone.
  document.addEventListener("htmx:beforeSwap", function (event) {
    if (event.detail && event.detail.target === document.body) {
      for (var i = 0; i < pills.length; i++) {
        pills[i].button.remove();
      }
      pills = [];
    }
  });
})();
