// SPDX-License-Identifier: AGPL-3.0-or-later
// Opening an unread inbox item marks it read on the way (templates/inbox.html).
//
// The link is an ordinary link to the task, so the dialog setting decides
// whether it opens over the inbox or as its own page (assets/taskopen.js),
// exactly as it does from any listing. This only posts the row's own Mark
// read form beside the navigation, with keepalive so a page that is leaving
// does not cancel it, and draws the row read at once. Without this file the
// link still opens the task and the row's button still marks it read.
(function () {
  "use strict";
  var tix = window.tix;
  if (!tix || !document.addEventListener || !window.fetch) {
    return;
  }

  function markRead(link) {
    var row = link.closest(".inbox-item");
    var form = row && row.querySelector("form.inbox-mark");
    if (!form) {
      return;
    }
    link.removeAttribute("data-inbox-unread");
    row.classList.remove("is-unread");
    var body = new URLSearchParams(new FormData(form));
    fetch("/inbox/read", {
      method: "POST",
      credentials: "same-origin",
      keepalive: true,
      redirect: "manual",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: body.toString()
    }).then(function () {
      form.setAttribute("action", "/inbox/unread");
      var button = form.querySelector("button");
      if (button) {
        button.textContent = tix.t ? tix.t("Mark unread") : "Mark unread";
      }
      document.dispatchEvent(new CustomEvent("tix:inbox"));
    }).catch(function () {
      row.classList.add("is-unread");
      link.setAttribute("data-inbox-unread", "");
    });
  }

  // Captured, so it runs before the dialog's own handler takes the click.
  document.addEventListener("click", function (event) {
    var target = event.target;
    if (!target || typeof target.closest !== "function" || !tix.plainClick(event)) {
      return;
    }
    var link = target.closest("a[data-inbox-open][data-inbox-unread]");
    if (link) {
      markRead(link);
    }
  }, true);
})();
