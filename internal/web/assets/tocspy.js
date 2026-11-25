// SPDX-License-Identifier: AGPL-3.0-or-later

// A table of contents that follows the reader: any nav[data-toc] whose links
// point at sections on the same page (the settings screen's navigation).
//
// The link for the section in view carries aria-current="true" and no other
// link does. A section is in view once its top reaches the line a jump to it
// lands on (its scroll-margin-top, plus a little), so a click and a scroll
// mark the same link even when the section is short. Two
// IntersectionObservers drive it: one over the sections with the strip above
// that line as its band, and one over the last section against
// the whole viewport, because a short last section never reaches the band and
// is only in view once the page is scrolled to its end. Each callback reads
// the sections' positions again and asks tocActive which one that is.
//
// A click on a link scrolls its section into view, smoothly unless the
// reader's system asks for reduced motion, pushes the fragment and moves
// focus to the section, as a native jump would. pushState does not change
// :target, so while this runs the navigation carries data-toc-live and the
// stylesheet's :target mark stands down. Without scripting the links are
// plain anchors and that mark is all there is.
(function (global) {
  "use strict";
  var tix = (global.tix = global.tix || {});

  // How far below its landing line a section's top may be and still count.
  var SLACK = 8;

  // tocActive is the index of the section in view: the last one once the page
  // is at its end, otherwise the last whose top has reached offset, otherwise
  // the first. tops are the sections' top edges in document order, relative
  // to the viewport; -1 when there are none.
  tix.tocActive = function (tops, offset, atEnd) {
    if (!tops.length) {
      return -1;
    }
    if (atEnd) {
      return tops.length - 1;
    }
    var pick = 0;
    for (var i = 0; i < tops.length; i++) {
      if (tops[i] <= offset) {
        pick = i;
      }
    }
    return pick;
  };

  // tocTakesClick reports whether a click on a link is one this script
  // handles: the main button with no modifier, which would otherwise jump.
  // Anything else (a new tab, a new window) is left to the browser.
  tix.tocTakesClick = function (event) {
    return !event.defaultPrevented && (event.button || 0) === 0 && !event.metaKey && !event.ctrlKey && !event.shiftKey && !event.altKey;
  };

  var doc = global.document;
  if (!doc || typeof global.IntersectionObserver !== "function") {
    return;
  }

  var live = [];

  function targetOf(link) {
    var href = link.getAttribute("href") || "";
    if (href.charAt(0) !== "#" || href.length < 2) {
      return null;
    }
    var id = href.slice(1);
    try {
      id = decodeURIComponent(id);
    } catch (e) {
      return null;
    }
    return doc.getElementById(id);
  }

  function shown(el) {
    return el.getClientRects().length > 0;
  }

  function mark(entries, index) {
    for (var i = 0; i < entries.length; i++) {
      if (i === index) {
        entries[i].link.setAttribute("aria-current", "true");
      } else {
        entries[i].link.removeAttribute("aria-current");
      }
    }
  }

  function atEnd() {
    var se = doc.scrollingElement || doc.documentElement;
    return se.scrollHeight > global.innerHeight && global.innerHeight + global.scrollY >= se.scrollHeight - 2;
  }

  function setUp(nav) {
    var entries = [];
    var links = nav.querySelectorAll("a[href^='#']");
    for (var i = 0; i < links.length; i++) {
      var section = targetOf(links[i]);
      if (section) {
        entries.push({ link: links[i], section: section });
      }
    }
    if (!entries.length) {
      return null;
    }

    function line() {
      return (parseFloat(global.getComputedStyle(entries[0].section).scrollMarginTop) || 0) + SLACK;
    }

    function update() {
      var visible = entries.filter(function (e) {
        return shown(e.section) && shown(e.link);
      });
      var tops = visible.map(function (e) {
        return e.section.getBoundingClientRect().top;
      });
      var at = tix.tocActive(tops, line(), atEnd());
      mark(entries, at < 0 ? -1 : entries.indexOf(visible[at]));
    }

    var band = null;
    function watch() {
      if (band) {
        band.disconnect();
      }
      var below = Math.max(0, Math.round(global.innerHeight - line()));
      band = new global.IntersectionObserver(update, { rootMargin: "0px 0px -" + below + "px 0px" });
      for (var j = 0; j < entries.length; j++) {
        band.observe(entries[j].section);
      }
      update();
    }
    var end = new global.IntersectionObserver(update, { threshold: [0, 1] });
    end.observe(entries[entries.length - 1].section);

    function onClick(event) {
      var link = event.target.closest ? event.target.closest("a[href^='#']") : null;
      if (!link || !tix.tocTakesClick(event)) {
        return;
      }
      var section = targetOf(link);
      if (!section) {
        return;
      }
      event.preventDefault();
      var reduce = global.document.documentElement.getAttribute("data-motion") === "reduce" ||
        typeof global.matchMedia === "function" && global.matchMedia("(prefers-reduced-motion: reduce)").matches;
      section.scrollIntoView({ behavior: reduce ? "auto" : "smooth", block: "start" });
      if (global.location.hash !== link.getAttribute("href") && global.history && global.history.pushState) {
        global.history.pushState(null, "", link.getAttribute("href"));
      }
      for (var k = 0; k < entries.length; k++) {
        if (entries[k].link === link) {
          mark(entries, k);
        }
      }
      if (!section.hasAttribute("tabindex")) {
        section.setAttribute("tabindex", "-1");
      }
      section.focus({ preventScroll: true });
    }

    nav.addEventListener("click", onClick);
    global.addEventListener("hashchange", update);
    global.addEventListener("resize", watch);
    nav.setAttribute("data-toc-live", "");
    watch();
    return function () {
      band.disconnect();
      end.disconnect();
      nav.removeEventListener("click", onClick);
      global.removeEventListener("hashchange", update);
      global.removeEventListener("resize", watch);
    };
  }

  function init() {
    var still = [];
    for (var i = 0; i < live.length; i++) {
      if (doc.contains(live[i].nav)) {
        still.push(live[i]);
      } else {
        live[i].stop();
      }
    }
    live = still;
    var navs = doc.querySelectorAll("nav[data-toc]");
    for (var j = 0; j < navs.length; j++) {
      if (navs[j].hasAttribute("data-toc-live")) {
        continue;
      }
      var stop = setUp(navs[j]);
      if (stop) {
        live.push({ nav: navs[j], stop: stop });
      }
    }
  }

  doc.addEventListener("htmx:load", init);
  if (doc.readyState === "loading") {
    doc.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})(window);
