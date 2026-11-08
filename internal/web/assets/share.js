// SPDX-License-Identifier: AGPL-3.0-or-later
// Sharing a task (templates/share.html): the task screen's Share menu and the
// share items in a lane card's more actions menu.
//
// A control carries data-share="native|link|markdown|text" on or inside an element
// carrying data-share-task with the task's reference, title and path. The
// address is the path on this page's own origin, so it is the host the reader
// is on whatever proxy sits in front. internal/tasklink builds the same forms
// for the command line, and the tests on both sides share their cases.
//
// Share… opens the system share sheet where the browser has one, and is left
// hidden where it has not. Every copy reports itself in a toast that is
// announced, the menu closes, and focus goes back to the button that opened it.
(function (global) {
  "use strict";
  var tix = (global.tix = global.tix || {});

  function say(text) {
    return tix.t ? tix.t(text) : text;
  }

  // shareURL is path made absolute on origin. Only a path of this origin is
  // widened; a scheme-relative or absolute value is returned as written.
  tix.shareURL = function (path, origin) {
    var p = typeof path === "string" ? path : "";
    if (origin && p.charAt(0) === "/" && p.charAt(1) !== "/") {
      return String(origin).replace(/\/+$/, "") + p;
    }
    return p;
  };

  function oneLine(s) {
    return String(s || "").split(/\s+/).filter(Boolean).join(" ");
  }

  // shareText is the reference and the title on one line.
  tix.shareText = function (ref, title) {
    var t = oneLine(title);
    return t ? ref + " " + t : String(ref || "");
  };

  // shareMarkdown is a Markdown link to url labelled with the reference and
  // the title, escaping what would end the label early.
  tix.shareMarkdown = function (ref, title, url) {
    var label = tix.shareText(ref, title).replace(/[\\\[\]]/g, function (c) {
      return "\\" + c;
    });
    return "[" + label + "](" + url + ")";
  };

  // shareValue is what one kind of copy puts on the clipboard, or "" for a
  // kind that copies nothing.
  tix.shareValue = function (kind, task, origin) {
    var t = task || {};
    var url = tix.shareURL(t.path, origin);
    switch (kind) {
      case "link":
        return url;
      case "markdown":
        return tix.shareMarkdown(t.ref, t.title, url);
      case "text":
        return tix.shareText(t.ref, t.title);
      default:
        return "";
    }
  };

  // shareMessage is the toast a copy of kind leaves, in the page's language.
  tix.shareMessage = function (kind, ok) {
    if (!ok) return say("Copy failed");
    if (kind === "markdown") return say("Markdown link copied");
    if (kind === "text") return say("Key and title copied");
    return say("Link copied");
  };

  // shareCanNative reports whether nav offers a share sheet.
  tix.shareCanNative = function (nav) {
    return !!(nav && typeof nav.share === "function");
  };

  // shareNative opens the share sheet, and copies the link instead when the
  // sheet is refused. A reader who dismisses the sheet (AbortError) chose not
  // to share, so nothing else happens.
  tix.shareNative = function (nav, task, origin, fallback) {
    var t = task || {};
    var data = { title: tix.shareText(t.ref, t.title), url: tix.shareURL(t.path, origin) };
    var promise;
    try {
      promise = nav.share(data);
    } catch (err) {
      fallback();
      return;
    }
    if (promise && typeof promise.then === "function") {
      promise.then(null, function (err) {
        if (!err || err.name !== "AbortError") fallback();
      });
    }
  };

  var doc = global.document;
  if (!doc || !doc.addEventListener) return;

  function reveal() {
    if (!tix.shareCanNative(global.navigator)) return;
    var natives = doc.querySelectorAll('[data-share="native"]');
    for (var i = 0; i < natives.length; i++) natives[i].hidden = false;
  }

  function taskOf(holder) {
    return {
      ref: holder.getAttribute("data-share-ref") || "",
      title: holder.getAttribute("data-share-title") || "",
      path: holder.getAttribute("data-share-path") || "",
    };
  }

  // leave closes the menu btn sits in and hands focus back to its opener.
  function leave(btn) {
    var menu = btn.closest("[popover]");
    if (!menu) return;
    var opener = menu.id ? doc.querySelector('[popovertarget="' + menu.id + '"]:not([popovertargetaction="hide"])') : null;
    try {
      if (typeof menu.hidePopover === "function") menu.hidePopover();
    } catch (e) {
      return;
    }
    if (opener && typeof opener.focus === "function") opener.focus({ preventScroll: true });
  }

  // toast says what happened. It goes in the open dialog when there is one,
  // since the page under a modal dialog is inert, and in #toast otherwise.
  // The region is in the page before its text is, so it is announced.
  var toastTimer = null;
  function toast(btn, text, ok) {
    var host = btn.closest("dialog[open]") || doc.getElementById("toast");
    if (!host) return;
    var old = doc.querySelectorAll(".share-toast");
    for (var i = 0; i < old.length; i++) old[i].remove();
    var p = doc.createElement("p");
    p.className = "flash share-toast" + (ok ? "" : " error");
    p.setAttribute("role", ok ? "status" : "alert");
    p.setAttribute("aria-atomic", "true");
    host.appendChild(p);
    global.setTimeout(function () {
      p.textContent = text;
    }, 50);
    if (toastTimer) global.clearTimeout(toastTimer);
    toastTimer = global.setTimeout(function () {
      p.remove();
    }, 4000);
  }

  function copy(btn, kind, task) {
    var text = tix.shareValue(kind, task, global.location.origin);
    var deps = { clipboard: global.navigator && global.navigator.clipboard, legacy: tix.legacyCopy };
    var copier = tix.copyText || function (_t, _d, done) { done(false); };
    copier(text, deps, function (ok) {
      toast(btn, tix.shareMessage(kind, ok), ok);
    });
  }

  doc.addEventListener("click", function (event) {
    var btn = event.target && event.target.closest ? event.target.closest("[data-share]") : null;
    var holder = btn && btn.closest("[data-share-task]");
    if (!holder) return;
    var kind = btn.getAttribute("data-share");
    var task = taskOf(holder);
    leave(btn);
    if (kind === "native" && tix.shareCanNative(global.navigator)) {
      tix.shareNative(global.navigator, task, global.location.origin, function () {
        copy(btn, "link", task);
      });
      return;
    }
    copy(btn, kind === "native" ? "link" : kind, task);
  });

  reveal();
  doc.addEventListener("DOMContentLoaded", reveal);
  doc.addEventListener("htmx:load", reveal);
})(window);
