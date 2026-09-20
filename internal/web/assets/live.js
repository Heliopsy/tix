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
    var item = document.createElement("li");
    var strong = document.createElement("strong");
    strong.textContent = parsed.event.type || "event";
    var span = document.createElement("span");
    span.className = "meta";
    span.textContent = " " + (parsed.event.subject_type || "") + " " + (parsed.event.subject_id || "");
    item.appendChild(strong);
    item.appendChild(span);
    feed.insertBefore(item, feed.firstChild);
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
})();
