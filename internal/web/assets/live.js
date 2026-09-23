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

  // A notification refreshes the row list by asking the server to render it
  // again, the same way it did on load (activity.go's showActivityFeed runs
  // the identical template block templates/activity.html uses). That is the
  // whole rendering: there is no second, JavaScript-side copy of the
  // grouping rule or the sentence wording to keep in sync with history.go.
  var feed = null;
  var socket = null;
  var pending = null;

  function refresh() {
    if (pending || !feed) {
      return;
    }
    var target = feed;
    var feedURL = target.getAttribute("data-feed");
    pending = fetch(feedURL, { credentials: "same-origin" })
      .then(function (resp) { return resp.ok ? resp.text() : null; })
      .then(function (html) {
        pending = null;
        if (html !== null && feed === target) {
          target.innerHTML = html;
        }
      })
      .catch(function () {
        pending = null;
      });
  }

  // A multi-hop transition writes its hops as separate events a few
  // milliseconds apart. Waiting a moment for them all to land before
  // rebuilding the row list is what lets the feed group them the way a page
  // load would, rather than briefly showing the first hop on its own.
  var scheduleRefresh = tix.debouncer(300, refresh);

  // connectFeed binds whatever feed element the current page carries. It is
  // safe to call on every swap: a page with no feed drops the socket, and a
  // page whose feed is already the bound one is left alone, so clicking
  // between two pages never opens a second socket.
  function connectFeed() {
    var next = document.getElementById("live-feed");
    if (next && !(next.getAttribute("data-events") && next.getAttribute("data-feed"))) {
      next = null;
    }
    if (!tix.shouldRebind(next, feed)) {
      return;
    }
    if (socket) {
      socket.close();
      socket = null;
    }
    feed = next;
    if (!feed || !window.WebSocket || !window.fetch) {
      return;
    }
    var path = feed.getAttribute("data-events");
    var scheme = window.location.protocol === "https:" ? "wss://" : "ws://";
    socket = new WebSocket(scheme + window.location.host + path);
    socket.addEventListener("open", function () {
      socket.send(JSON.stringify({ type: "subscribe", id: "feed" }));
    });
    socket.addEventListener("message", function (message) {
      var parsed;
      try {
        parsed = JSON.parse(message.data);
      } catch (err) {
        return;
      }
      if (!parsed || parsed.type !== "event") {
        return;
      }
      scheduleRefresh();
    });
  }

  connectFeed();
  document.addEventListener("htmx:load", connectFeed);
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
  // computed for that card (projects.go targetsFrom) -- so a column outside
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
    var select = card && card.querySelector("form select[name=to]");
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
    var select = form && form.querySelector("select[name=to]");
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
  // (templates/error.html renders it as .flash.error) out of the raw HTML a
  // failed fetch returned, since that response was never swapped into the
  // page the way htmx would swap a boosted navigation.
  function extractError(html) {
    try {
      var doc = new DOMParser().parseFromString(html, "text/html");
      var el = doc.querySelector(".flash.error");
      return el ? el.textContent.trim() : "";
    } catch (err) {
      return "";
    }
  }

  // refreshBoard re-fetches this same board page and swaps in only the
  // board's own contents, not the whole page. The listeners above are on the
  // document, so a freshly rendered card -- with its own now-current Move
  // options -- is live whatever this replaces.
  var refreshing = null;
  function refreshBoard(board, announceText) {
    if (refreshing) {
      return refreshing;
    }
    refreshing = fetch(window.location.pathname, { credentials: "same-origin" })
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
