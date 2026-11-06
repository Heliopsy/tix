// SPDX-License-Identifier: AGPL-3.0-or-later

// Places the tip a focused timestamp shows (time.ts, written by the when
// template helper) so it stays on screen.
//
// The tip itself is CSS: time.ts:focus::after draws data-full under the
// timestamp. Drawn in place it was cut off by a table that scrolls sideways
// and ran off the right edge of a phone for a time at the end of a row. On
// focus this pins it to the viewport instead, under the timestamp, moved left
// as far as it has to be to fit. Without a script the tip is still drawn, in
// place.
(function (global) {
  "use strict";
  var tix = global.tix = global.tix || {};

  // timeTipPlace says where a tip at most `widest` pixels wide goes for a
  // timestamp whose box is rect, in a viewport `width` wide with `gutter`
  // pixels kept clear at each side.
  tix.timeTipPlace = function (rect, width, widest, gutter) {
    var room = Math.max(0, width - 2 * gutter);
    var tip = Math.min(widest, room);
    var left = Math.min(Math.max(rect.left, gutter), width - gutter - tip);
    return { left: Math.max(gutter, left), top: rect.bottom + 4, width: tip };
  };

  var doc = global.document;
  if (!doc || !doc.addEventListener) return;
  doc.addEventListener("focusin", function (event) {
    var el = event.target;
    if (!el || !el.matches || !el.matches("time.ts")) return;
    var place = tix.timeTipPlace(el.getBoundingClientRect(), global.innerWidth,
      22 * 16, 16);
    el.style.setProperty("--ts-left", place.left + "px");
    el.style.setProperty("--ts-top", place.top + "px");
    el.style.setProperty("--ts-width", place.width + "px");
    el.classList.add("ts-placed");
  });
})(window);
