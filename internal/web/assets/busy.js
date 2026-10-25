// SPDX-License-Identifier: AGPL-3.0-or-later
// Acknowledge the click.
//
// Every mutation in this interface is a form post that repaints the whole
// page, and until that paint arrives nothing on screen says anything happened.
// Press Save, or Apply, or any of the Apply buttons on /settings, and the
// button sits there looking idle for however long the round trip takes. On a
// fast local server that reads as nothing happening; on a slow link it reads
// as a broken button, and people press it again.
//
// So the submitting button says so itself: it goes busy, and it stops taking a
// second press.
//
// Why the timeout. Disabling a submit button inside its own submit handler
// drops it from the form data, because a disabled control is not successful
// and its name=value never reaches the server. Several screens branch on
// exactly that value, so the disable is deferred to the next task: the
// submission has already been assembled by then, and the button is still in
// it.
//
// Why pageshow. A page restored from the back/forward cache comes back with
// the DOM exactly as it was left, which means a button frozen mid-submit. The
// restore is the one moment to undo that, and it is not a load.
//
// No window.tix, no framework: this file stands alone, and every screen still
// works with JavaScript off, because the button is only decoration over a form
// post the server handles either way.
(function () {
  "use strict";

  var BUSY = "is-busy";

  function markBusy(form, submitter) {
    var button =
      submitter ||
      form.querySelector('button[type="submit"], input[type="submit"]');
    if (!button || button.disabled) return;

    form.setAttribute("aria-busy", "true");
    button.classList.add(BUSY);

    // Next task, not this one: see the note above about disabled controls
    // being dropped from the submission.
    window.setTimeout(function () {
      button.disabled = true;
    }, 0);
  }

  function clear(root) {
    var scope = root || document;
    var busy = scope.querySelectorAll("." + BUSY);
    for (var i = 0; i < busy.length; i++) {
      busy[i].classList.remove(BUSY);
      busy[i].disabled = false;
    }
    var forms = scope.querySelectorAll('form[aria-busy="true"]');
    for (var j = 0; j < forms.length; j++) {
      forms[j].removeAttribute("aria-busy");
    }
  }

  document.addEventListener(
    "submit",
    function (event) {
      var form = event.target;
      if (!form || form.tagName !== "FORM") return;
      // A form the page intends to handle itself is not ours to freeze.
      if (form.hasAttribute("data-no-busy")) return;
      markBusy(form, event.submitter);
    },
    true,
  );

  // htmx swaps the page in place rather than navigating, so nothing resets on
  // its own: the button that was frozen is still the button on screen.
  document.addEventListener("htmx:afterSwap", function (event) {
    clear(event.target);
  });
  document.addEventListener("htmx:responseError", function () {
    clear();
  });
  document.addEventListener("htmx:sendError", function () {
    clear();
  });

  window.addEventListener("pageshow", function (event) {
    if (event.persisted) clear();
  });
})();

// A Save that says whether there is anything to save.
//
// The task form opened with four filled fields and an enabled Save, so the
// button looked the same whether you had typed something or not: the only way
// to find out whether a change had taken was to press it and read the toast.
// A form that is identical to what the server already holds has nothing to
// submit, and saying so is the cheapest kind of feedback there is.
//
// Opt-in, by data-dirty-save on the form, because most forms here are a single
// action rather than an edit of existing values: "Add a tag" has nothing to
// compare against.
//
// Progressive: with no JavaScript the button is simply always enabled, which
// is what it was before, and the server answers an unchanged submission with
// an unchanged task.
(function () {
  "use strict";

  function fields(form) {
    return Array.prototype.filter.call(form.elements, function (el) {
      return el.name && el.type !== "hidden" && el.type !== "submit";
    });
  }

  function snapshot(form) {
    return fields(form)
      .map(function (el) {
        return el.name + "=" + (el.type === "checkbox" ? el.checked : el.value);
      })
      .join("\u0000");
  }

  function wire(form) {
    var save = form.querySelector('button[type="submit"]');
    if (!save) return;
    var clean = snapshot(form);

    function settle() {
      var dirty = snapshot(form) !== clean;
      // Not disabled: a disabled control is unreachable by keyboard and says
      // nothing to a screen reader about why. aria-disabled with a visual
      // state keeps it focusable and explains itself.
      save.classList.toggle("is-idle", !dirty);
      save.setAttribute("aria-disabled", dirty ? "false" : "true");
      save.title = dirty ? "" : "Nothing has changed yet";
    }

    form.addEventListener("input", settle);
    form.addEventListener("change", settle);
    form.addEventListener("submit", function (event) {
      if (snapshot(form) === clean) {
        event.preventDefault();
        save.focus();
      }
    });
    settle();
  }

  function start() {
    var forms = document.querySelectorAll("form[data-dirty-save]");
    for (var i = 0; i < forms.length; i++) wire(forms[i]);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", start);
  } else {
    start();
  }
  document.addEventListener("htmx:afterSwap", start);
})();
