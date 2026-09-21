(function () {
  "use strict";
  var feed = document.getElementById("live-feed");
  if (!feed || !window.WebSocket) {
    return;
  }
  var path = feed.getAttribute("data-events");
  if (!path) {
    return;
  }
  // The markup here has to match what activity.html renders on the server,
  // or an entry that arrives live looks unlike the ones around it.
  function verbOf(action) {
    var verb = String(action || "");
    var dot = verb.indexOf(".");
    if (dot >= 0) {
      verb = verb.slice(dot + 1);
    }
    var kinds = {
      create: ["creat", "add", "restor", "import"],
      "delete": ["delet", "remov", "revok", "prun", "archiv"],
      update: ["updat", "edit", "transition", "put", "mov"],
    };
    for (var kind in kinds) {
      for (var i = 0; i < kinds[kind].length; i++) {
        if (verb.indexOf(kinds[kind][i]) === 0) {
          return kind;
        }
      }
    }
    return "other";
  }

  function span(className, text, title) {
    var el = document.createElement("span");
    el.className = className;
    el.textContent = text;
    if (title) {
      el.title = title;
    }
    return el;
  }

  function row(event) {
    var action = event.type || "event";
    var id = event.subject_id || "";
    var item = document.createElement("li");
    item.appendChild(span("badge act-" + verbOf(action), action));
    var subject = span("subject", (event.subject_type || "") + " ");
    subject.appendChild(span("mono", id.length > 12 ? id.slice(0, 4) + "\u2026" + id.slice(-4) : id, id));
    item.appendChild(subject);
    item.appendChild(span("muted", "just now"));
    return item;
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
    if (!parsed || parsed.type !== "event" || !parsed.event) {
      return;
    }
    feed.insertBefore(row(parsed.event), feed.firstChild);
  });
})();

(function () {
  "use strict";
  var board = document.querySelector(".board");
  if (!board) {
    return;
  }
  var dragged = null;
  board.addEventListener("dragstart", function (event) {
    var card = event.target.closest(".card");
    if (card) {
      dragged = card;
    }
  });
  board.addEventListener("dragover", function (event) {
    if (dragged && event.target.closest(".column")) {
      event.preventDefault();
    }
  });
  board.addEventListener("drop", function (event) {
    var column = event.target.closest(".column");
    if (!dragged || !column) {
      return;
    }
    var form = dragged.querySelector("form");
    var select = form && form.querySelector("select[name=to]");
    if (!select) {
      return;
    }
    var wanted = column.getAttribute("data-state");
    for (var i = 0; i < select.options.length; i++) {
      if (select.options[i].value === wanted) {
        event.preventDefault();
        select.selectedIndex = i;
        form.submit();
        return;
      }
    }
  });

  // htmx does not swap a non-2xx response, so a refused form used to change
  // nothing on screen and say nothing. Every refusal here renders a full page
  // explaining itself, so swap it in.
  document.body.addEventListener("htmx:beforeSwap", function (event) {
    var status = event.detail.xhr.status;
    if (status >= 400) {
      event.detail.shouldSwap = true;
      event.detail.isError = false;
    }
  });
})();
