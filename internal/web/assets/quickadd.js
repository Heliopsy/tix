// SPDX-License-Identifier: AGPL-3.0-or-later
// Quick add from the clipboard: the button beside the quick-add box and the
// quickPaste shortcut (ctrl+shift+v) put the clipboard's first non-empty line
// in the box and focus it, without adding. The button is drawn hidden and
// revealed here only where the browser can read the clipboard; without this
// file, or without the Clipboard API, there is nothing to press.
(function () {
  "use strict";
  // say is text in the page's language; see i18n.js.
  function say(text) {
    return window.tix && window.tix.t ? window.tix.t(text) : text;
  }
  var tix = window.tix;
  if (!tix || !tix.firstLine || !document.addEventListener) {
    return;
  }
  var clipboard = window.navigator && window.navigator.clipboard;
  var readable = !!(clipboard && typeof clipboard.readText === "function");

  function reveal() {
    if (!readable) {
      return;
    }
    var buttons = document.querySelectorAll("[data-quickpaste][hidden]");
    for (var i = 0; i < buttons.length; i++) {
      buttons[i].removeAttribute("hidden");
    }
  }

  // toast says the clipboard had nothing, in the page's #toast region, which
  // is in the page before its text is, so it is announced.
  var toastTimer = null;
  function toast(text) {
    var host = document.getElementById("toast");
    if (!host) {
      return;
    }
    var old = document.querySelectorAll(".quickpaste-toast");
    for (var i = 0; i < old.length; i++) {
      old[i].remove();
    }
    var p = document.createElement("p");
    p.className = "flash error quickpaste-toast";
    p.setAttribute("role", "alert");
    p.setAttribute("aria-atomic", "true");
    host.appendChild(p);
    window.setTimeout(function () {
      p.textContent = text;
    }, 50);
    if (toastTimer) {
      window.clearTimeout(toastTimer);
    }
    toastTimer = window.setTimeout(function () {
      p.remove();
    }, 4000);
  }

  function fill(box, line) {
    if (!line) {
      toast(say("The clipboard holds no text."));
      return;
    }
    box.value = line;
    box.focus();
    box.setSelectionRange(line.length, line.length);
    box.dispatchEvent(new Event("input", { bubbles: true }));
  }

  // quickPaste answers false where the page has no quick-add box or the
  // browser cannot read the clipboard, so the shortcut leaves the key alone.
  tix.quickPaste = function () {
    var box = document.querySelector("input[data-quickadd]");
    if (!box || !readable) {
      return false;
    }
    clipboard.readText().then(function (text) {
      fill(box, tix.firstLine(text));
    }, function () {
      fill(box, "");
    });
    return true;
  };

  document.addEventListener("click", function (event) {
    var btn = event.target && event.target.closest && event.target.closest("[data-quickpaste]");
    if (btn) {
      event.preventDefault();
      tix.quickPaste();
    }
  });

  reveal();
  document.addEventListener("htmx:load", reveal);
})();
