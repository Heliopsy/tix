// SPDX-License-Identifier: AGPL-3.0-or-later

(function () {
  "use strict";
  var tix = window.tix;
  if (!tix) {
    return;
  }
  // One generic copy-to-clipboard affordance for the whole site: any button
  // carrying data-copy-target="<id>" copies the text content of the element
  // with that id. Nothing here is specific to any one screen, so a future
  // screen that needs a copy button reuses this rather than writing another.
  document.addEventListener("click", function (event) {
    var btn = event.target && event.target.closest && event.target.closest("[data-copy-target]");
    if (!btn) {
      return;
    }
    var id = btn.getAttribute("data-copy-target");
    var source = id && document.getElementById(id);
    if (!source) {
      return;
    }
    copy(source.textContent || "", btn);
  });

  // tix.copyText owns the choice of mechanism and every failure path, so a
  // clipboard the browser refuses becomes a label rather than an error.
  function copy(text, btn) {
    tix.copyText(text, { clipboard: navigator.clipboard, legacy: legacyCopy }, function (ok) {
      announce(btn, ok);
    });
  }

  // legacyCopy supports a browser without the async Clipboard API, using a
  // hidden textarea only for the moment of the copy itself.
  function legacyCopy(text) {
    var area = document.createElement("textarea");
    area.value = text;
    area.setAttribute("readonly", "");
    area.style.position = "fixed";
    area.style.opacity = "0";
    document.body.appendChild(area);
    area.select();
    var ok = false;
    try {
      ok = document.execCommand("copy");
    } catch (err) {
      ok = false;
    }
    document.body.removeChild(area);
    return ok;
  }

  function announce(btn, ok) {
    var original = btn.getAttribute("data-copy-label") || btn.textContent;
    btn.setAttribute("data-copy-label", original);
    btn.textContent = ok ? "Copied" : "Copy failed";
    window.setTimeout(function () {
      btn.textContent = original;
    }, 1500);
  }

  // A styled file field (.file-field, app.css) hides the browser's own
  // "Choose file / No file chosen" text, so the one thing that native widget
  // told you for free -- which file, if any, is selected -- has to be read
  // out here, or the replacement would be worse than what it replaced.
  // Delegated at the document like the copy button above, so a file field
  // added to a future screen starts working with no extra wiring.
  document.addEventListener("change", function (event) {
    var input = event.target;
    if (!input || input.type !== "file" || !input.closest) {
      return;
    }
    var field = input.closest(".file-field");
    var name = field && field.querySelector("[data-file-name]");
    if (!name) {
      return;
    }
    name.textContent = tix.fileChoiceLabel(input.files);
  });
})();
