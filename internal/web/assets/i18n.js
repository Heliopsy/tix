// SPDX-License-Identifier: AGPL-3.0-or-later
// The words the scripts write into a page, in the page's language.
//
// The server renders the translations every script needs into
// <script id="tix-i18n" type="application/json"> in the head, keyed by the
// English text, and leaves it out for English. tix.t looks a text up there and
// answers the English when there is no table or no entry, so a script never
// shows a blank. Loaded first, without defer, because sidebar.js labels its
// toggle before the body is parsed.
(function (global) {
  "use strict";
  var tix = (global.tix = global.tix || {});
  var table = null;

  // messages reads the table the server rendered, or an empty one.
  tix.messages = function (doc) {
    try {
      var el = doc && doc.getElementById ? doc.getElementById("tix-i18n") : null;
      var parsed = el ? JSON.parse(el.textContent) : null;
      return parsed && typeof parsed === "object" ? parsed : {};
    } catch (e) {
      return {};
    }
  };

  // t is text in the page's language, or text itself when nothing says otherwise.
  tix.t = function (text) {
    if (table === null) table = tix.messages(global.document);
    return Object.prototype.hasOwnProperty.call(table, text) ? table[text] : text;
  };
})(typeof globalThis !== "undefined" ? globalThis : this);
