(function () {
  "use strict";
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

  function copy(text, btn) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(
        function () { announce(btn, true); },
        function () { announce(btn, false); }
      );
      return;
    }
    announce(btn, legacyCopy(text));
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
})();
