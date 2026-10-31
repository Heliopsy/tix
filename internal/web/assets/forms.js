// SPDX-License-Identifier: AGPL-3.0-or-later
// The create and settings forms: Add with details, New project and a
// project's settings, each marked form[data-form].
//
// Ctrl+Enter, or Cmd+Enter, submits through requestSubmit, so the browser's
// own required checks and assets/busy.js still run. Escape inside a popover
// is the popover's own and is left alone; in a disclosure it closes the
// disclosure, and in a form drawn in place after a refusal it follows the
// form's Cancel link.
//
// The new project form suggests its key from the name as the name is typed,
// until the reader types a key of their own, and says what the project's
// tasks will be numbered.
//
// Everything here is decoration over an ordinary form post: with scripting
// off every form still submits, and the keyboard line stays hidden because
// nothing would honour it.
(function () {
  "use strict";

  // Bulgarian is the one non-Latin language the interface speaks, so a name
  // typed in it still yields a key. The table is the official streamlined
  // transliteration.
  var CYRILLIC = {
    "а": "a", "б": "b", "в": "v", "г": "g", "д": "d", "е": "e", "ж": "zh",
    "з": "z", "и": "i", "й": "y", "к": "k", "л": "l", "м": "m", "н": "n",
    "о": "o", "п": "p", "р": "r", "с": "s", "т": "t", "у": "u", "ф": "f",
    "х": "h", "ц": "ts", "ч": "ch", "ш": "sh", "щ": "sht", "ъ": "a", "ь": "y",
    "ю": "yu", "я": "ya",
  };

  var MAX = 12;

  // suggestKey derives a project key from a name: the first word that has a
  // letter in it, lower case, letters and digits only, at most MAX long, with
  // a number added when the key is already taken. Empty when nothing usable
  // is left.
  function suggestKey(name, taken) {
    var plain = "";
    var lower = String(name || "").toLowerCase();
    for (var i = 0; i < lower.length; i++) {
      var c = lower.charAt(i);
      plain += Object.prototype.hasOwnProperty.call(CYRILLIC, c) ? CYRILLIC[c] : c;
    }
    plain = plain.normalize ? plain.normalize("NFD").replace(/[̀-ͯ]/g, "") : plain;
    var words = plain.split(/[^a-z0-9]+/);
    var key = "";
    for (var j = 0; j < words.length; j++) {
      var word = words[j].replace(/^[0-9]+/, "");
      if (word) {
        key = word.slice(0, MAX);
        break;
      }
    }
    if (!key) return "";
    var used = {};
    for (var k = 0; k < (taken || []).length; k++) used[String(taken[k]).toLowerCase()] = true;
    if (!used[key]) return key;
    for (var n = 2; n < 100; n++) {
      var candidate = key.slice(0, MAX - String(n).length) + n;
      if (!used[candidate]) return candidate;
    }
    return key;
  }

  function closestForm(node) {
    return node && node.closest ? node.closest("form[data-form]") : null;
  }

  function preview(form) {
    var target = form.querySelector("[data-key-target]");
    var shown = form.querySelector("[data-key-preview]");
    if (!target || !shown) return;
    shown.textContent = (target.value.trim().toLowerCase() || target.placeholder || "key") + "-1";
  }

  function onInput(event) {
    var input = event.target;
    var form = closestForm(input);
    if (!form) return;
    if (input.hasAttribute("data-key-target")) {
      if (input.value.trim()) input.setAttribute("data-key-edited", "");
      else input.removeAttribute("data-key-edited");
      preview(form);
      return;
    }
    if (!input.hasAttribute("data-key-source")) return;
    var target = form.querySelector("[data-key-target]");
    if (!target || target.hasAttribute("data-key-edited")) return;
    var taken = (target.getAttribute("data-taken") || "").split(" ").filter(Boolean);
    target.value = suggestKey(input.value, taken);
    preview(form);
  }

  function onKeydown(event) {
    var form = closestForm(event.target);
    if (!form) return;
    if (event.key === "Enter" && (event.ctrlKey || event.metaKey)) {
      event.preventDefault();
      if (form.requestSubmit) form.requestSubmit();
      else form.submit();
      return;
    }
    if (event.key !== "Escape" || event.defaultPrevented) return;
    if (form.closest("[popover]")) return;
    var fold = form.closest("details[open]");
    if (fold) {
      event.preventDefault();
      fold.open = false;
      var summary = fold.querySelector("summary");
      if (summary) summary.focus();
      return;
    }
    var cancel = form.querySelector("[data-form-cancel]");
    if (cancel && cancel.href) {
      event.preventDefault();
      window.location.assign(cancel.href);
    }
  }

  function reveal(root) {
    var scope = root || document;
    var mac = /Mac|iPhone|iPad/.test((window.navigator && window.navigator.platform) || "");
    var lines = scope.querySelectorAll("[data-form-keys]");
    for (var i = 0; i < lines.length; i++) {
      lines[i].hidden = false;
      var mod = lines[i].querySelector("[data-form-mod]");
      if (mod && mac) mod.textContent = "⌘";
    }
    var forms = scope.querySelectorAll("form[data-form]");
    for (var j = 0; j < forms.length; j++) {
      var target = forms[j].querySelector("[data-key-target]");
      if (target && target.value.trim()) target.setAttribute("data-key-edited", "");
      preview(forms[j]);
    }
  }

  window.tixForms = { suggestKey: suggestKey, onKeydown: onKeydown, onInput: onInput, reveal: reveal };

  if (typeof document === "undefined" || !document.addEventListener) return;
  document.addEventListener("keydown", onKeydown);
  document.addEventListener("input", onInput);
  reveal(document);
  document.addEventListener("htmx:load", function (event) {
    reveal(event.target && event.target.querySelectorAll ? event.target : document);
  });
})();
