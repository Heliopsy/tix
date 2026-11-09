// SPDX-License-Identifier: AGPL-3.0-or-later

// Keeps an open (i) note on screen, and closes it when the reader is done.
//
// The note of a tip or a field-info (templates/partials.html) is CSS: it
// floats under its circle, anchored at the circle's left edge, or its right
// edge where a rule says the circle sits at the end of a row. A circle near
// the window's right edge opened a note that ran past it, and the page
// scrolled sideways; rules per screen only moved the problem to the next
// screen. When a floating note opens this measures it and slides it back
// inside the window, narrowing it first if the window is narrower than the
// note. A note laid out in the flow (a heading's or a label's) is left alone.
// Without a script the note is drawn where CSS puts it.
//
// A <details> only closes from its own summary, so a reader who opened a
// note and moved on found it still open three tips later. With a script a
// note is light-dismissed the way a popover is: a press anywhere outside it
// and its circle closes it, Escape closes it and puts focus back on the
// circle, and opening one note closes any other. A press inside the note, on
// its doc name or a link, leaves it open. The summary's aria-expanded follows
// the note.
(function (global) {
  "use strict";
  var tix = global.tix = global.tix || {};

  // tipFit says how far to move a box laid out from left to right, in a
  // viewport width wide with gutter pixels kept clear at each side, and how
  // wide it may be: shift is the pixels to move it (negative is leftwards),
  // width the most it may take, or 0 when it fits as it is.
  tix.tipFit = function (left, right, width, gutter) {
    var room = Math.max(0, width - 2 * gutter);
    var wide = right - left;
    var narrow = wide > room ? room : 0;
    var span = narrow || wide;
    var shift = 0;
    if (left + span > width - gutter) shift = width - gutter - span - left;
    if (left + shift < gutter) shift = gutter - left;
    return { shift: Math.round(shift), width: narrow };
  };

  function place(tip) {
    var note = tip.querySelector(":scope > p");
    if (!note) return;
    note.style.removeProperty("--tip-shift");
    note.style.removeProperty("--tip-width");
    var pos = global.getComputedStyle(note).position;
    if (pos !== "absolute" && pos !== "fixed") return;
    var rect = note.getBoundingClientRect();
    var fit = tix.tipFit(rect.left, rect.right,
      global.document.documentElement.clientWidth, 8);
    if (fit.width) note.style.setProperty("--tip-width", fit.width + "px");
    if (fit.width) rect = note.getBoundingClientRect();
    fit = tix.tipFit(rect.left, rect.right,
      global.document.documentElement.clientWidth, 8);
    if (fit.shift) note.style.setProperty("--tip-shift", fit.shift + "px");
  }

  var TIP = "details.field-info";

  function summary(tip) {
    return tip.querySelector(":scope > summary");
  }

  function mark(tip) {
    var s = summary(tip);
    if (s) s.setAttribute("aria-expanded", tip.open ? "true" : "false");
  }

  function shutExcept(keep) {
    var open = doc.querySelectorAll(TIP + "[open]");
    var shut = [];
    for (var i = 0; i < open.length; i++) {
      if (open[i] === keep) continue;
      open[i].open = false;
      mark(open[i]);
      shut.push(open[i]);
    }
    return shut;
  }

  function tipOf(node) {
    return node && typeof node.closest === "function" ? node.closest(TIP) : null;
  }

  var doc = global.document;
  if (!doc || !doc.addEventListener) return;
  doc.addEventListener("toggle", function (event) {
    var tip = event.target;
    if (!tip || !tip.matches || !tip.matches(TIP)) return;
    mark(tip);
    if (!tip.open) return;
    shutExcept(tip);
    place(tip);
  }, true);
  doc.addEventListener("pointerdown", function (event) {
    shutExcept(tipOf(event.target));
  }, true);
  doc.addEventListener("keydown", function (event) {
    if (event.key !== "Escape" || event.defaultPrevented) return;
    var focused = tipOf(doc.activeElement);
    var shut = shutExcept(null);
    if (!shut.length) return;
    event.preventDefault();
    event.stopPropagation();
    var back = summary(focused && shut.indexOf(focused) !== -1 ? focused : shut[shut.length - 1]);
    if (back && back.focus) back.focus();
  }, true);
})(window);
