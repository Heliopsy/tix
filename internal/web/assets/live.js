(function () {
  "use strict";
  var feed = document.getElementById("live-feed");
  if (!feed || !window.WebSocket || !window.fetch) {
    return;
  }
  var path = feed.getAttribute("data-events");
  var feedURL = feed.getAttribute("data-feed");
  if (!path || !feedURL) {
    return;
  }

  // A notification refreshes the row list by asking the server to render it
  // again, the same way it did on load (activity.go's showActivityFeed runs
  // the identical template block templates/activity.html uses). That is the
  // whole rendering: there is no second, JavaScript-side copy of the
  // grouping rule or the sentence wording to keep in sync with history.go.
  var pending = null;
  function refresh() {
    if (pending) {
      return;
    }
    pending = fetch(feedURL, { credentials: "same-origin" })
      .then(function (resp) { return resp.ok ? resp.text() : null; })
      .then(function (html) {
        pending = null;
        if (html !== null) {
          feed.innerHTML = html;
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
  var timer = null;
  function scheduleRefresh() {
    if (timer) {
      clearTimeout(timer);
    }
    timer = setTimeout(function () {
      timer = null;
      refresh();
    }, 300);
  }

  var scheme = window.location.protocol === "https:" ? "wss://" : "ws://";
  var socket = new WebSocket(scheme + window.location.host + path);
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
  var board = document.querySelector(".board");
  // The settings toggle (partials.html "settings-menu", posts to /dragmove)
  // is server truth, read here the same way the board itself reads it: as a
  // data attribute the template already rendered from the cookie, since the
  // cookie is HttpOnly and this script cannot read it directly. The Move
  // disclosure on every card needs none of this and keeps working exactly
  // as before either way.
  if (!board || board.getAttribute("data-drag") !== "1") {
    return;
  }
  // Native HTML5 drag-and-drop has no working touch equivalent in mobile
  // browsers -- there is no drop event a touch gesture ever fires -- and
  // leaving "draggable" live on a coarse pointer only fights the page's own
  // scrolling and long-press behaviour for no payoff. So this is scoped to a
  // fine pointer (mouse, trackpad, pen): a touch user gets the Move
  // disclosure, which already works everywhere, rather than a half-working
  // drag.
  if (!window.matchMedia || !window.matchMedia("(pointer: fine)").matches) {
    return;
  }

  var status = document.getElementById("board-status");
  function announce(text) {
    if (status) {
      status.textContent = text;
    }
  }

  function columnLabel(column) {
    var heading = column.querySelector("h2");
    return heading ? heading.textContent.replace(/\s*\(\d+\)\s*$/, "").trim()
      : column.getAttribute("data-state");
  }

  function clearDropTargets() {
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

  board.addEventListener("dragstart", function (event) {
    var card = event.target.closest(".card");
    var select = card && card.querySelector("form select[name=to]");
    if (!card || !select) {
      if (event.dataTransfer) {
        event.dataTransfer.effectAllowed = "none";
      }
      return;
    }
    dragged = card;
    legal = {};
    for (var i = 0; i < select.options.length; i++) {
      legal[select.options[i].value] = true;
    }
    card.classList.add("is-dragging");
    if (event.dataTransfer) {
      event.dataTransfer.effectAllowed = "move";
      // Firefox requires data to be set for the drag to proceed at all.
      event.dataTransfer.setData("text/plain", card.getAttribute("data-task") || "");
    }
  });

  board.addEventListener("dragend", function () {
    if (dragged) {
      dragged.classList.remove("is-dragging");
    }
    dragged = null;
    legal = null;
    clearDropTargets();
  });

  board.addEventListener("dragover", function (event) {
    var column = event.target.closest(".column");
    if (!dragged || !column || !legal[column.getAttribute("data-state")]) {
      return;
    }
    event.preventDefault();
    if (event.dataTransfer) {
      event.dataTransfer.dropEffect = "move";
    }
    if (!column.classList.contains("is-drop-target")) {
      clearDropTargets();
      column.classList.add("is-drop-target");
    }
  });

  // A refused move -- the request failed, or the server refused it, most
  // often because someone else changed the task while this page was open --
  // leaves the card exactly where it started: nothing here ever moves it in
  // the DOM before the server confirms it, so there is no position to roll
  // back. Only the explanation, through the live region, says what happened.
  board.addEventListener("drop", function (event) {
    var column = event.target.closest(".column");
    var card = dragged;
    if (!card || !column || !legal[column.getAttribute("data-state")]) {
      return;
    }
    event.preventDefault();
    clearDropTargets();

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
          announce(ref + " was not moved to " + destination + ": " + (extractError(html) || "the move was refused."));
        });
      }
      // The server is the only source of truth for what a card may do next
      // (its legal states, its version), so success re-fetches the board
      // fragment rather than moving the card by hand here and guessing.
      return refreshBoard(ref + " moved to " + destination + ".");
    }).catch(function () {
      announce(ref + " was not moved to " + destination + ": the request failed.");
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
  // board's own contents, not the whole page: the listeners above are bound
  // to the .board element itself, which this leaves in place, so a freshly
  // rendered card -- with its own now-current Move options -- is live
  // without re-running any setup.
  var refreshing = null;
  function refreshBoard(announceText) {
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
        announce(announceText);
      })
      .catch(function () {
        refreshing = null;
        window.location.reload();
      });
    return refreshing;
  }
})();
