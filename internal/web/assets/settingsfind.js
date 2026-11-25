// SPDX-License-Identifier: AGPL-3.0-or-later
// Settings search (templates/settings.html): typing narrows the screen to the
// rows whose name or help mentions every word typed, and to the sections left
// with one. A section whose heading matches keeps all its rows. Escape clears.
// Without this file the box is not drawn and every row shows.
(function (global) {
  "use strict";
  var tix = (global.tix = global.tix || {});

  // fold lower-cases text and drops accents, so "cafe" finds "Café".
  function fold(text) {
    var s = String(text == null ? "" : text).toLowerCase();
    return typeof s.normalize === "function" ? s.normalize("NFD").replace(/[̀-ͯ]/g, "") : s;
  }

  // settingMatches reports whether text holds every word of query, in any
  // order. An empty query matches everything.
  tix.settingMatches = function (text, query) {
    var words = fold(query).split(/\s+/).filter(Boolean);
    var hay = fold(text);
    for (var i = 0; i < words.length; i++) {
      if (hay.indexOf(words[i]) < 0) {
        return false;
      }
    }
    return true;
  };

  if (!global.document || !global.document.addEventListener) {
    return;
  }
  var document = global.document;

  function textOf(el) {
    if (!el) {
      return "";
    }
    var parts = [];
    var names = el.querySelectorAll(".setname, .sethelp, .field-info p, label, .setlead");
    for (var i = 0; i < names.length; i++) {
      parts.push(names[i].textContent);
    }
    return parts.join(" ") || el.textContent;
  }

  function show(el, on) {
    if (on) {
      el.removeAttribute("hidden");
    } else {
      el.setAttribute("hidden", "hidden");
    }
  }

  function apply(input) {
    var query = input.value;
    var groups = document.querySelectorAll(".setbody > .setgroup");
    var any = false;
    for (var i = 0; i < groups.length; i++) {
      var group = groups[i];
      var head = group.querySelector("h2");
      var whole = query !== "" && head && tix.settingMatches(head.textContent, query);
      var rows = group.querySelectorAll(".setlist > .setrow, .setlist > ul > .setrow");
      var kept = 0;
      for (var j = 0; j < rows.length; j++) {
        var on = whole || tix.settingMatches(textOf(rows[j]), query);
        show(rows[j], on);
        if (on && !rows[j].classList.contains("setlead")) {
          kept++;
        }
      }
      var visible = query === "" || whole || kept > 0;
      show(group, visible);
      any = any || visible;
      var link = group.id ? document.querySelector('.setnav a[href="#' + group.id + '"]') : null;
      if (link) {
        show(link, visible);
      }
    }
    var none = document.querySelector("[data-setfind-none]");
    if (none) {
      var said = none.querySelector("[data-setfind-say]");
      if (said) {
        said.textContent = said.getAttribute("data-setfind-say").replace("%s", query.trim());
      }
      show(none, !any);
    }
  }

  document.addEventListener("input", function (event) {
    var input = event.target;
    if (input && input.matches && input.matches("[data-setfind]")) {
      apply(input);
    }
  });
  document.addEventListener("keydown", function (event) {
    var input = event.target;
    if (event.key === "Escape" && input && input.matches && input.matches("[data-setfind]") && input.value) {
      event.preventDefault();
      input.value = "";
      apply(input);
    }
  });
})(typeof globalThis !== "undefined" ? globalThis : this);
