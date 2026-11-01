// SPDX-License-Identifier: AGPL-3.0-or-later

// The project icon picker's preview. The picker works without this: the panes
// switch in CSS and the form submits whatever is chosen. This only redraws the
// preview while the reader chooses, including a file not yet uploaded.
(function () {
  "use strict";
  var tix = window.tix;
  if (!tix) {
    return;
  }

  // iconPreview decides what the preview shows for one state of the picker:
  // the kind checked, the text typed, the symbol picked, a chosen file's
  // address and the image the project already has.
  tix.iconPreview = function (state) {
    var s = state || {};
    var text = (s.text || "").trim();
    switch (s.kind) {
      case "text":
        return text ? { kind: "text", value: text } : { kind: "none", value: "" };
      case "builtin":
        return s.name ? { kind: "builtin", value: s.name } : { kind: "none", value: "" };
      case "image":
        var src = s.fileURL || s.current || "";
        return src ? { kind: "image", value: src } : { kind: "none", value: "" };
      default:
        return { kind: "none", value: "" };
    }
  };

  if (typeof document === "undefined" || !document.addEventListener) {
    return;
  }

  var fileURL = "";

  function checked(root, name) {
    var input = root.querySelector('input[name="' + name + '"]:checked');
    return input ? input.value : "";
  }

  function draw(root) {
    var preview = root.querySelector("[data-iconpick-preview]");
    if (!preview) {
      return;
    }
    if (!preview.hasAttribute("data-current")) {
      var img = preview.querySelector("img");
      preview.setAttribute("data-current", img ? img.getAttribute("src") : "");
    }
    var textInput = root.querySelector('input[name="icon"]');
    var shown = tix.iconPreview({
      kind: checked(root, "icon_kind"),
      text: textInput ? textInput.value : "",
      name: checked(root, "icon_name"),
      fileURL: fileURL,
      current: preview.getAttribute("data-current"),
    });
    preview.textContent = "";
    if (shown.kind === "text") {
      preview.textContent = shown.value;
    } else if (shown.kind === "builtin") {
      var ns = "http://www.w3.org/2000/svg";
      var svg = document.createElementNS(ns, "svg");
      svg.setAttribute("class", "proj-icon proj-icon-svg");
      var use = document.createElementNS(ns, "use");
      use.setAttribute("href", "/assets/icons.svg#pi-" + shown.value);
      svg.appendChild(use);
      preview.appendChild(svg);
    } else if (shown.kind === "image") {
      var pic = document.createElement("img");
      pic.className = "proj-icon proj-icon-img";
      pic.alt = "";
      pic.src = shown.value;
      preview.appendChild(pic);
    }
  }

  function onChange(event) {
    var root = event.target && event.target.closest && event.target.closest("[data-iconpick]");
    if (!root) {
      return;
    }
    if (event.target.name === "icon_file") {
      if (fileURL) {
        URL.revokeObjectURL(fileURL);
      }
      var file = event.target.files && event.target.files[0];
      fileURL = file ? URL.createObjectURL(file) : "";
    }
    draw(root);
  }

  document.addEventListener("change", onChange);
  document.addEventListener("input", onChange);
})();
