// SPDX-License-Identifier: AGPL-3.0-or-later

// Keeps an open (i) note on screen.
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

  var doc = global.document;
  if (!doc || !doc.addEventListener) return;
  doc.addEventListener("toggle", function (event) {
    var tip = event.target;
    if (!tip || !tip.matches || !tip.matches("details.field-info") || !tip.open) return;
    place(tip);
  }, true);
})(window);
