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

    function refresh() {
      if (pending || !bound) {
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
        .then(function (resp) { return resp.ok ? resp.text() : null; })
        .then(function (html) {
          pending = null;
          // A boosted navigation can swap the page in before htmx:load moves
          // the binding, so the element this was asked for may already be
          // gone while bound still names it. The new page was rendered after
          // the change that prompted this, and drawing the old page's answer
          // over it threw away its own state: the row a just-added task was
          // marked on lost its ring.
          if (html === null || bound !== target || spec.find() !== target) {
            return;
          }
          if (spec.busy(target)) {
            defer();
            return;
          }
          spec.apply(target, html);
        })
        .catch(function () {
          pending = null;
        });
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
      retry = window.setTimeout(connect, wait);
    }

    function closed() {
      if (socket === null) {
        return;
      }
      socket = null;
      live = false;
      later();
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
      opening.addEventListener("open", function () {
        if (socket !== opening) {
          return;
        }
        attempt = 0;
        live = true;
        stopPolling();
        announce();
        subscribe(opening);
      });
      opening.addEventListener("message", function (message) {
        if (socket !== opening) {
          return;
        }
        received(opening, message);
      });
      opening.addEventListener("close", function () {
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
      if (socket) {
        var going = socket;
        socket = null;
        going.close();
      }
      live = false;
      bound = next;
      seq = 0;
      attempt = 0;
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

    return { rebind: rebind, woken: woken, poke: schedule };
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
    apply: function (el, html) { el.innerHTML = html; },
    window: function () { return 300; },
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

  // processed hands markup this file wrote with innerHTML to htmx. htmx only
  // wires up what it swapped in itself, so every form a refresh drew had no
  // hx-post and no boost: the next tick or move was a plain post that
  // reloaded the page and threw the reader back to the top.
  function processed(el) {
    if (window.htmx && typeof window.htmx.process === "function") {
      window.htmx.process(el);
    }
  }

  // applyRegions replaces each region and puts back what the reader had in
  // it: focus, since the refresh that follows a reader's own change lands on
  // the region holding the control they just pressed; and the scroll position
  // of any box inside it, since the task list scrolls inside its own box and
  // went back to its first row on every update.
  function applyRegions(el, html) {
    var doc = new DOMParser().parseFromString(html, "text/html");
    var ids = regionIDs(el);
    for (var i = 0; i < ids.length; i++) {
      var fresh = doc.getElementById(ids[i]);
      var here = document.getElementById(ids[i]);
      if (fresh && here) {
        var memo = tix.focusMemo(document.activeElement, here);
        var places = tix.scrollPlaces(here.querySelectorAll("*"));
        here.innerHTML = fresh.innerHTML;
        tix.restorePlaces(here.querySelectorAll("*"), places);
        processed(here);
        tix.focusRestore(document, memo);
      }
    }
  }

  function rebindAll() {
    feed.rebind();
    screen.rebind();
  }

  rebindAll();
  document.addEventListener("htmx:load", rebindAll);
  document.addEventListener("visibilitychange", function () {
    feed.woken();
    screen.woken();
  });
  // A save made where no event is guaranteed to reach this page in time (a
  // task saved in the dialog, a title renamed in place) asks for the
  // refresh directly. See taskopen.js.
  document.addEventListener("tix:refresh", function () {
    screen.poke();
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

    var form = card.querySelector("form");
    var select = form && form.querySelector("select[name=route]");
    if (!form || !select) {
      return;
    }
    var wanted = column.getAttribute("data-state");
    var ref = card.getAttribute("data-task") || "the card";
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
