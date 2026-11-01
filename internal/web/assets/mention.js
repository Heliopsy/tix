// SPDX-License-Identifier: AGPL-3.0-or-later
// @handle completion in comment and description boxes.
//
// A textarea that names a datalist of handles in data-mention (the list the
// assignee field already offers) gets a listbox under it while the caret
// sits in an @word: arrows move, Enter or Tab chooses, Escape closes the list
// and nothing else, and typing on narrows it. The textarea is a combobox for
// the length of it, so a screen reader hears the active option. Without this
// file the box is a plain textarea, and @handle typed in full still mentions.
//
// The decisions (where the @word is, what matches, what the text becomes)
// are in decide.js and covered by internal/web/jstest.
(function () {
  "use strict";
  var tix = window.tix;
  if (!tix || !document.addEventListener || !tix.mentionAt) {
    return;
  }

  var open = null;

  function handlesFor(area) {
    var list = document.getElementById(area.getAttribute("data-mention"));
    var out = [];
    if (!list) {
      return out;
    }
    for (var i = 0; i < list.options.length; i++) {
      if (list.options[i].value) {
        out.push(list.options[i].value);
      }
    }
    return out;
  }

  function close() {
    if (!open) {
      return;
    }
    open.area.removeAttribute("aria-activedescendant");
    open.area.setAttribute("aria-expanded", "false");
    if (open.menu.parentNode) {
      open.menu.parentNode.removeChild(open.menu);
    }
    open = null;
  }

  function highlight(index) {
    var items = open.menu.querySelectorAll("[role=option]");
    open.index = (index + items.length) % items.length;
    for (var i = 0; i < items.length; i++) {
      items[i].setAttribute("aria-selected", i === open.index ? "true" : "false");
    }
    open.area.setAttribute("aria-activedescendant", items[open.index].id);
  }

  function choose(handle) {
    var area = open.area;
    var next = tix.mentionInsert(area.value, area.selectionStart, open.at, handle);
    close();
    area.value = next.value;
    area.setSelectionRange(next.caret, next.caret);
    area.dispatchEvent(new Event("input", { bubbles: true }));
    area.focus();
  }

  function show(area, at, matches) {
    if (!open || open.area !== area) {
      close();
      var menu = document.createElement("ul");
      menu.className = "mention-menu";
      menu.id = (area.id || "mention") + "-mentions";
      menu.setAttribute("role", "listbox");
      menu.setAttribute("aria-label", tix.t ? tix.t("People to mention") : "People to mention");
      area.insertAdjacentElement("afterend", menu);
      area.setAttribute("aria-controls", menu.id);
      area.setAttribute("aria-autocomplete", "list");
      open = { area: area, menu: menu, index: 0 };
    }
    open.at = at;
    open.menu.innerHTML = "";
    for (var i = 0; i < matches.length; i++) {
      var li = document.createElement("li");
      li.id = open.menu.id + "-" + i;
      li.setAttribute("role", "option");
      li.setAttribute("data-handle", matches[i]);
      li.textContent = "@" + matches[i];
      open.menu.appendChild(li);
    }
    area.setAttribute("aria-expanded", "true");
    highlight(0);
  }

  function update(area) {
    if (area.selectionStart !== area.selectionEnd) {
      close();
      return;
    }
    var at = tix.mentionAt(area.value, area.selectionStart);
    var matches = at ? tix.mentionMatches(handlesFor(area), at.query, 8) : [];
    if (!matches.length) {
      close();
      return;
    }
    show(area, at, matches);
  }

  document.addEventListener("input", function (event) {
    var area = event.target;
    if (area && area.matches && area.matches("textarea[data-mention]")) {
      update(area);
    }
  });

  // Captured, so a chosen handle is not also a submitted comment or a
  // closed dialog.
  document.addEventListener("keydown", function (event) {
    if (!open || event.target !== open.area) {
      return;
    }
    var key = event.key;
    if (key === "ArrowDown" || key === "ArrowUp") {
      highlight(open.index + (key === "ArrowDown" ? 1 : -1));
    } else if ((key === "Enter" || key === "Tab") && !event.shiftKey && !event.ctrlKey && !event.metaKey) {
      var item = open.menu.querySelectorAll("[role=option]")[open.index];
      choose(item.getAttribute("data-handle"));
    } else if (key === "Escape") {
      close();
    } else {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
  }, true);

  document.addEventListener("mousedown", function (event) {
    if (!open) {
      return;
    }
    var item = event.target.closest && event.target.closest(".mention-menu [role=option]");
    if (item) {
      event.preventDefault();
      choose(item.getAttribute("data-handle"));
    } else if (event.target !== open.area) {
      close();
    }
  });

  document.addEventListener("focusout", function (event) {
    if (open && event.target === open.area) {
      window.setTimeout(function () {
        if (open && document.activeElement !== open.area) {
          close();
        }
      }, 0);
    }
  });
  document.addEventListener("htmx:beforeSwap", close);
})();
