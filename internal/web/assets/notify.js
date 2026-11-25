// SPDX-License-Identifier: AGPL-3.0-or-later
// Browser notifications for new inbox items, while a tix tab is open.
//
// Opt-in, kept in this browser's storage: the two switches under Settings,
// Notifications turn it on (asking the browser for permission) and hide what
// a notification says. The trigger is the sidebar badge's own live refresh
// (assets/live.js, "tix:inbox-badge"): when the unread count rises, the
// newest unread items are read from the API and each one not seen before is
// shown, unless the reader is looking at this tab. The item id is the
// notification's tag, so two open tabs show it once.
(function (global) {
  "use strict";
  var tix = (global.tix = global.tix || {});

  var onKey = "tix.notify";
  var hideKey = "tix.notify.hide";
  var seenKey = "tix.notify.seen";
  var seenCap = 200;

  function say(text) {
    return tix.t ? tix.t(text) : text;
  }

  // notifyContent is what one item's notification says: a title and a body,
  // or only that something arrived when hide is set.
  tix.notifyContent = function (item, hide) {
    if (hide || !item) {
      return { title: "tix", body: say("Something new reached your inbox.") };
    }
    var by = item.actor_handle || say("somebody");
    var what;
    switch (item.kind) {
      case "mention": what = say("%s mentioned you"); break;
      case "assigned": what = say("%s assigned you"); break;
      case "status": what = say("%s moved it from %s to %s"); break;
      default: what = say("%s commented");
    }
    var args = [by, item.from_status || "", item.to_status || ""];
    var n = 0;
    what = what.replace(/%s/g, function () { return args[n++]; });
    var task = item.task_ref ? String(item.task_ref).toUpperCase() + " " + (item.task_title || "") : "";
    return { title: what, body: task.trim() };
  };

  // notifyFresh is the items not in seen, oldest first, so they are shown in
  // the order they arrived.
  tix.notifyFresh = function (items, seen) {
    var out = [];
    for (var i = 0; i < (items || []).length; i++) {
      if (items[i] && items[i].id && seen.indexOf(items[i].id) < 0) {
        out.push(items[i]);
      }
    }
    return out.reverse();
  };

  if (!global.document || !global.document.addEventListener) {
    return;
  }
  var document = global.document;

  function store() {
    try {
      return global.localStorage || null;
    } catch (e) {
      return null;
    }
  }

  function read(key) {
    var s = store();
    try {
      return s ? s.getItem(key) : null;
    } catch (e) {
      return null;
    }
  }

  function write(key, value) {
    var s = store();
    try {
      if (s && value === null) {
        s.removeItem(key);
      } else if (s) {
        s.setItem(key, value);
      }
    } catch (e) {
      // Storage refused: the switch reads off on the next page, which is true.
    }
  }

  function seenIDs() {
    try {
      var list = JSON.parse(read(seenKey) || "[]");
      return Array.isArray(list) ? list : [];
    } catch (e) {
      return [];
    }
  }

  function remember(ids) {
    var list = seenIDs().concat(ids);
    write(seenKey, JSON.stringify(list.slice(-seenCap)));
  }

  function supported() {
    return typeof global.Notification === "function";
  }

  function enabled() {
    return read(onKey) === "1" && supported() && global.Notification.permission === "granted";
  }

  function unread() {
    return global.fetch("/api/v1/inbox?unread=true&limit=20", { credentials: "same-origin" })
      .then(function (resp) { return resp.ok ? resp.json() : null; })
      .then(function (page) { return page && Array.isArray(page.items) ? page.items : []; });
  }

  function show(item) {
    var hide = read(hideKey) === "1";
    var c = tix.notifyContent(item, hide);
    try {
      var n = new global.Notification(c.title, { body: c.body, tag: "tix-inbox-" + item.id, icon: "/assets/tix.svg" });
      n.onclick = function () {
        global.focus();
        global.location.href = !hide && item.task_ref && !item.task_gone ? "/tasks/" + encodeURIComponent(item.task_ref) : "/inbox";
        n.close();
      };
    } catch (e) {
      // Some browsers only allow notifications from a service worker.
    }
  }

  // baseline marks the items already unread as seen, so turning this on, or
  // opening a tab, never announces old news.
  function baseline() {
    if (!enabled() || !global.fetch) {
      return;
    }
    unread().then(function (items) {
      remember(items.map(function (it) { return it.id; }));
    }).catch(function () {});
  }

  var last = null;

  function countNow() {
    var badge = document.getElementById("inbox-badge");
    return badge ? parseInt(badge.getAttribute("data-inbox-count"), 10) || 0 : 0;
  }

  document.addEventListener("tix:inbox-badge", function (event) {
    var count = event.detail && typeof event.detail.count === "number" ? event.detail.count : countNow();
    var rose = last !== null && count > last;
    last = count;
    if (!rose || !enabled() || !global.fetch) {
      return;
    }
    unread().then(function (items) {
      var fresh = tix.notifyFresh(items, seenIDs());
      remember(fresh.map(function (it) { return it.id; }));
      var looking = !document.hidden && typeof document.hasFocus === "function" && document.hasFocus();
      if (looking) {
        return;
      }
      for (var i = 0; i < fresh.length; i++) {
        show(fresh[i]);
      }
    }).catch(function () {});
  });

  // -- the settings switches --------------------------------------------------

  function setSwitch(button, on) {
    if (!button) {
      return;
    }
    button.setAttribute("aria-checked", on ? "true" : "false");
    var state = button.querySelector(".switch-state");
    if (state) {
      state.textContent = on ? say("On") : say("Off");
    }
  }

  function state(text) {
    var el = document.querySelector("[data-notify-state]");
    if (el && text) {
      el.textContent = text;
    }
  }

  function drawSwitches() {
    var toggle = document.querySelector("[data-notify-toggle]");
    if (!toggle) {
      return;
    }
    setSwitch(toggle, enabled());
    setSwitch(document.querySelector("[data-notify-hide]"), read(hideKey) === "1");
    if (!supported()) {
      toggle.setAttribute("disabled", "disabled");
      state(say("This browser cannot show notifications."));
    } else if (global.Notification.permission === "denied") {
      state(say("Notifications are blocked for this site. Allow them in the browser's site settings, then turn this on."));
    }
  }

  document.addEventListener("click", function (event) {
    var target = event.target && event.target.closest ? event.target.closest("[data-notify-toggle], [data-notify-hide]") : null;
    if (!target) {
      return;
    }
    if (target.hasAttribute("data-notify-hide")) {
      write(hideKey, read(hideKey) === "1" ? null : "1");
      drawSwitches();
      return;
    }
    if (enabled()) {
      write(onKey, null);
      drawSwitches();
      return;
    }
    if (!supported()) {
      drawSwitches();
      return;
    }
    var asked = global.Notification.permission === "granted"
      ? Promise.resolve("granted")
      : Promise.resolve(global.Notification.requestPermission());
    asked.then(function (answer) {
      if (answer === "granted") {
        write(onKey, "1");
        baseline();
      }
      drawSwitches();
    });
  });

  document.addEventListener("htmx:load", drawSwitches);
  document.addEventListener("DOMContentLoaded", function () {
    last = countNow();
    drawSwitches();
    baseline();
  });
})(typeof globalThis !== "undefined" ? globalThis : this);
