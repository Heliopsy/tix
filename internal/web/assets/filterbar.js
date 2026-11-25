// SPDX-License-Identifier: AGPL-3.0-or-later
// Suggestions under the task screens' filter box.
//
// The box names the page's suggestion data in data-suggest: the tenant's
// projects, tags, people, statuses, priorities and deadline windows, written
// once by the server. While the caret sits in a word the box offers what that
// word completes to: term keys for a bare word, handles after @, and values
// after project:, tag:, assignee:, status:, priority:, due: and is:. Arrows
// move, Enter or Tab takes the reached option, Escape closes the list. No
// option is reached until an arrow is pressed, so Enter alone applies the
// filter as typed. Without this file the box is a plain field that still
// applies on Enter.
//
// The quick-add box uses the same list with data-suggest-mode="quick": it
// completes only #tag, @handle and +project, from its own data, and Enter
// with no option reached still adds the task.
//
// The decisions (which word, what matches, what the box becomes, what a key
// does) are in decide.js and covered by internal/web/jstest.
(function () {
  "use strict";
  var tix = window.tix;
  // say is text in the page's language; see i18n.js.
  function say(text) {
    return tix && tix.t ? tix.t(text) : text;
  }
  if (!tix || !document.addEventListener || !tix.filterTokenAt || !tix.filterSuggest || !tix.quickTokenAt) {
    return;
  }

  var open = null;

  // The parsed data is kept on its script element, so a boosted navigation,
  // which draws a new element, is read afresh: a tag added a moment ago is
  // offered on the listing the add returned to.
  function dataFor(box) {
    var el = document.getElementById(box.getAttribute("data-suggest"));
    if (!el) {
      return null;
    }
    if (el.tixSuggest === undefined) {
      try {
        el.tixSuggest = JSON.parse(el.textContent);
      } catch (e) {
        el.tixSuggest = null;
      }
    }
    return el.tixSuggest;
  }

  function quick(box) {
    return box.getAttribute("data-suggest-mode") === "quick";
  }

  function close() {
    if (!open) {
      return;
    }
    open.box.removeAttribute("aria-activedescendant");
    open.box.setAttribute("aria-expanded", "false");
    if (open.menu.parentNode) {
      open.menu.parentNode.removeChild(open.menu);
    }
    open = null;
  }

  function highlight(index) {
    var items = open.menu.querySelectorAll("[role=option]");
    open.index = index;
    for (var i = 0; i < items.length; i++) {
      items[i].setAttribute("aria-selected", i === index ? "true" : "false");
    }
    if (items[index]) {
      open.box.setAttribute("aria-activedescendant", items[index].id);
      if (items[index].scrollIntoView) {
        items[index].scrollIntoView({ block: "nearest" });
      }
    }
  }

  function take(index) {
    var box = open.box;
    var choice = open.items[index];
    var next = tix.filterInsert(box.value, open.tok, choice.insert);
    close();
    box.value = next.value;
    box.setSelectionRange(next.caret, next.caret);
    box.dispatchEvent(new Event("input", { bubbles: true }));
    box.focus();
    suggest(box);
  }

  function show(box, tok, items) {
    if (!open || open.box !== box) {
      close();
      var menu = document.createElement("ul");
      menu.className = "fbsuggest";
      menu.id = (box.id || (quick(box) ? "quickadd" : "filter")) + "-suggest-list";
      menu.setAttribute("role", "listbox");
      var adding = quick(box);
      menu.setAttribute("aria-label", say(adding ? "Quick add suggestions" : "Filter suggestions"));
      box.insertAdjacentElement("afterend", menu);
      box.setAttribute("role", "combobox");
      box.setAttribute("aria-controls", menu.id);
      box.setAttribute("aria-autocomplete", "list");
      open = { box: box, menu: menu };
    }
    open.tok = tok;
    open.items = items;
    open.index = -1;
    open.menu.textContent = "";
    items.forEach(function (item, i) {
      var li = document.createElement("li");
      li.id = open.menu.id + "-" + i;
      li.setAttribute("role", "option");
      li.setAttribute("aria-selected", "false");
      li.textContent = item.label;
      if (item.note) {
        var note = document.createElement("span");
        note.className = "fbsuggest-note";
        note.textContent = item.note;
        li.appendChild(note);
      }
      li.addEventListener("mousedown", function (event) {
        event.preventDefault();
        take(i);
      });
      open.menu.appendChild(li);
    });
    box.setAttribute("aria-expanded", "true");
    box.removeAttribute("aria-activedescendant");
  }

  function suggest(box) {
    var at = quick(box) ? tix.quickTokenAt : tix.filterTokenAt;
    var list = quick(box) ? tix.quickSuggest : tix.filterSuggest;
    var tok = at(box.value, box.selectionStart);
    var items = tok ? list(dataFor(box), tok, 8) : [];
    if (!items.length) {
      close();
      return;
    }
    show(box, tok, items);
  }

  function boxOf(target) {
    return target && target.matches && target.matches("input[data-suggest]") ? target : null;
  }

  document.addEventListener("input", function (event) {
    var box = boxOf(event.target);
    if (box && !event.isComposing) {
      suggest(box);
    }
  });

  document.addEventListener("keydown", function (event) {
    var box = boxOf(event.target);
    if (!box || event.isComposing) {
      return;
    }
    if (!open && event.key === "ArrowDown") {
      suggest(box);
    }
    var showing = open && open.box === box;
    var d = tix.filterKey(event.key, showing, showing ? open.items.length : 0, showing ? open.index : -1);
    if (d.act === "none") {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    if (d.act === "move") {
      highlight(d.index);
    } else if (d.act === "take") {
      take(d.index);
    } else if (d.act === "close") {
      close();
    }
  }, true);

  document.addEventListener("focusout", function (event) {
    if (open && event.target === open.box) {
      close();
    }
  });
})();
