// SPDX-License-Identifier: AGPL-3.0-or-later

// The Preview toggle beside a Markdown textarea.
//
// The textarea stays the editor. Preview posts what is typed to the server,
// which renders it with the same sanitising renderer the saved text goes
// through, so what the preview shows is exactly what the page will show. No
// Markdown is parsed in the browser. Without a script the toggle is never
// revealed and the textarea is all there is, as before.
(function () {
  "use strict";

  function say(text) {
    return window.tix && window.tix.t ? window.tix.t(text) : text;
  }

  // nextMode is what the toggle switches to and the label it then carries.
  function nextMode(previewing) {
    return previewing ? { previewing: false, label: say("Preview") } : { previewing: true, label: say("Write") };
  }

  function csrf(from) {
    var form = from && from.closest ? from.closest("form") : null;
    var field = (form && form.querySelector('input[name="csrf_token"]')) ||
      document.querySelector('input[name="csrf_token"]');
    return field ? field.value : "";
  }

  function toggle(button) {
    var area = document.getElementById(button.getAttribute("data-md-preview"));
    var pane = document.getElementById(button.getAttribute("data-md-pane"));
    if (!area || !pane) return;
    var mode = nextMode(button.getAttribute("aria-pressed") === "true");
    button.setAttribute("aria-pressed", mode.previewing ? "true" : "false");
    button.textContent = mode.label;
    if (!mode.previewing) {
      pane.hidden = true;
      area.hidden = false;
      area.focus();
      return;
    }
    var body = new URLSearchParams();
    body.set("csrf_token", csrf(button));
    body.set("body", area.value);
    pane.setAttribute("aria-busy", "true");
    pane.textContent = say("Loading preview…");
    pane.hidden = false;
    area.hidden = true;
    fetch("/markdown/preview", {
      method: "POST",
      credentials: "same-origin",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: body.toString()
    }).then(function (resp) {
      return resp.ok ? resp.text() : Promise.reject(resp);
    }).then(function (html) {
      if (html.trim()) {
        pane.innerHTML = html;
      } else {
        pane.textContent = say("Nothing to preview.");
      }
    }).catch(function () {
      pane.textContent = say("The preview could not be drawn. Your text is unchanged.");
    }).then(function () {
      pane.removeAttribute("aria-busy");
    });
  }

  function reveal(root) {
    var buttons = (root || document).querySelectorAll("[data-md-preview]");
    for (var i = 0; i < buttons.length; i++) buttons[i].hidden = false;
  }

  window.tixMarkdown = { nextMode: nextMode, reveal: reveal };

  if (typeof document === "undefined" || !document.addEventListener) return;
  document.addEventListener("click", function (event) {
    var button = event.target && event.target.closest ? event.target.closest("[data-md-preview]") : null;
    if (button) {
      event.preventDefault();
      toggle(button);
    }
  });
  reveal(document);
  document.addEventListener("htmx:load", function (event) {
    reveal(event.target && event.target.querySelectorAll ? event.target : document);
  });
})();
