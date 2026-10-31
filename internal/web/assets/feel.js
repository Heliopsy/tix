// SPDX-License-Identifier: AGPL-3.0-or-later

// How the interface answers a click: in place, at once, and without losing
// the reader's place.
//
// Measured on a seeded store, starring a row, or moving it, or changing its
// priority, posted the form, followed the redirect to the whole listing and
// swapped the body: the sidebar was repainted, the list scrolled back to its
// top, focus fell to <body>, and nothing on screen changed until the round
// trip was over. The task screen did the same for every tag, comment and move.
//
// Everything here is decoration over forms and links that already work
// without a script. The decisions are on window.tix so jstest can exercise
// them without a browser; the wiring under them is bound at the document,
// never to an element captured at load, because the body is swapped by every
// boosted navigation (see the note at the top of live.js).
(function (global) {
  "use strict";
  var tix = global.tix = global.tix || {};

  function say(text) {
    return tix.t ? tix.t(text) : text;
  }

  // inplaceSpec says what a form inside a [data-inplace] region swaps: the
  // selector the attribute names, or the region itself by its id. A region
  // that names nothing and has no id cannot be found again, so it opts out.
  tix.inplaceSpec = function (named, id) {
    var selector = named || (id ? "#" + id : "");
    if (!selector) {
      return null;
    }
    return { target: selector, select: selector, swap: "outerHTML show:none transition:false" };
  };

  // goesOnPress reports whether a link may be followed on the press rather
  // than the release: a screen of this origin a reader opens from a list,
  // and not the page already on screen. Anything that is not a screen
  // (assets, fragments, downloads) is left to the ordinary click.
  tix.goesOnPress = function (link, page) {
    if (!link || !page || link.origin !== page.origin) {
      return false;
    }
    if (link.pathname === page.pathname && link.search === page.search) {
      return false;
    }
    // A press on a card that can be dragged may be the start of a drag, and
    // opening the task on the press took the card away before it could move.
    // Those links open on the click, which a drag never sends.
    if (typeof link.closest === "function" && link.closest('[draggable="true"]')) {
      return false;
    }
    if (/\/(live|export|download)$/.test(link.pathname) || link.pathname.indexOf("/assets/") === 0) {
      return false;
    }
    return /^\/(tasks|projects|activity)(\/|$)/.test(link.pathname);
  };

  // statusFromRoute is the state a route of states ends in, which is what
  // the pill should say while the move is on its way.
  tix.statusFromRoute = function (route) {
    if (!route) {
      return "";
    }
    var parts = String(route).split(/[>,]/);
    return parts[parts.length - 1].trim();
  };

  // viewTransitions reports whether page swaps should cross-fade: only where
  // the browser can, and never for a reader who asked for less motion.
  tix.viewTransitions = function (doc, matchMedia) {
    if (!doc || typeof doc.startViewTransition !== "function") {
      return false;
    }
    var reduce = typeof matchMedia === "function" ? matchMedia("(prefers-reduced-motion: reduce)") : null;
    return !(reduce && reduce.matches);
  };

  if (!global.document || !global.document.addEventListener) {
    return;
  }
  var document = global.document;

  function htmxReady() {
    return global.htmx && typeof global.htmx.process === "function";
  }

  // -- toast -------------------------------------------------------------------

  // A toast says what happened and then gets out of the way. One with an Undo
  // stays longer, and none leaves while the pointer or focus is on it.
  var toastTimer = null;
  function settleToast() {
    var toast = document.querySelector("main .flash:not(.error)");
    if (toastTimer) {
      global.clearTimeout(toastTimer);
      toastTimer = null;
    }
    if (!toast) {
      return;
    }
    var wait = toast.querySelector(".flashundo") ? 10000 : 5000;
    var leave = function () {
      if (toast.matches(":hover") || toast.contains(document.activeElement)) {
        toastTimer = global.setTimeout(leave, 1500);
        return;
      }
      toast.classList.add("is-leaving");
      toastTimer = global.setTimeout(function () {
        toast.remove();
        var undo = document.getElementById("flash-undo");
        if (undo) {
          undo.remove();
        }
      }, 200);
    };
    toastTimer = global.setTimeout(leave, wait);
  }

  function showError(text) {
    var main = document.querySelector("main.wrap");
    if (!main) {
      return;
    }
    // One toast at a time: a refusal replaces whatever the last action
    // reported, which would otherwise sit under it saying the opposite.
    var stale = main.querySelectorAll(".flash.error.is-inplace, #toast .flash, #flash-undo");
    for (var i = 0; i < stale.length; i++) {
      stale[i].remove();
    }
    var p = document.createElement("p");
    p.className = "flash error is-inplace";
    p.setAttribute("role", "alert");
    p.textContent = text || say("That did not work. Nothing was changed.");
    main.insertBefore(p, main.firstChild);
    global.setTimeout(function () { p.remove(); }, 8000);
  }

  function errorText(html) {
    try {
      var doc = new global.DOMParser().parseFromString(html || "", "text/html");
      var el = doc.querySelector(".failure-message, .flash.error, .filterfault");
      return el ? el.textContent.trim() : "";
    } catch (err) {
      return "";
    }
  }

  // -- in place ----------------------------------------------------------------

  function optimistic(form, submitter, scope) {
    if (submitter && submitter.classList.contains("star")) {
      // The star answers at once: busy.js's spinner would blank the icon for
      // the length of the round trip, which reads as the opposite of a star.
      submitter.classList.remove("is-busy");
      var on = !submitter.classList.contains("is-starred");
      submitter.classList.toggle("is-starred", on);
      submitter.setAttribute("aria-pressed", on ? "true" : "false");
      return;
    }
    if (!submitter) {
      return;
    }
    var row = scope.matches("li.task") ? scope : null;
    if (submitter.name === "route" && row) {
      var pill = row.querySelector(".statuspill .pill");
      var to = tix.statusFromRoute(submitter.value);
      if (pill && to) {
        pill.textContent = to;
        pill.className = "pill status " + to;
      }
    }
    if (submitter.name === "priority" && row) {
      var chip = row.querySelector(".priobtn .prio");
      var chosen = submitter.querySelector(".prio");
      if (chip && chosen) {
        chip.className = chosen.className;
        chip.innerHTML = chosen.innerHTML;
      }
    }
  }

  var inflight = null;

  document.addEventListener("submit", function (event) {
    var form = event.target;
    if (!form || form.tagName !== "FORM" || !htmxReady()) {
      return;
    }
    if (form.getAttribute("hx-boost") === "false" || form.hasAttribute("hx-post") ||
        form.hasAttribute("hx-target") && !form.hasAttribute("data-inplace-armed")) {
      return;
    }
    var scope = form.closest("[data-inplace]");
    if (!scope || form.closest("dialog")) {
      return;
    }
    var spec = tix.inplaceSpec(scope.getAttribute("data-inplace"), scope.id);
    if (!spec) {
      return;
    }
    var target = document.querySelector(spec.target);
    if (!target) {
      return;
    }
    form.setAttribute("hx-target", spec.target);
    form.setAttribute("hx-select", spec.select);
    form.setAttribute("hx-swap", spec.swap);
    // The address stays the screen's own. A boosted form pushes wherever its
    // redirect landed, which put ?flash=... in the address bar and a history
    // entry per star.
    form.setAttribute("hx-push-url", "false");
    if (!target.querySelector("#toast")) {
      form.setAttribute("hx-select-oob", "#toast");
    }
    form.setAttribute("data-inplace-armed", "");
    inflight = {
      form: form,
      target: spec.target,
      // Remembered against main rather than the swapped part, so the row or
      // the field itself can be the id the memo names. decide.js keeps the
      // one rule for this, shared with live.js and lanes.js.
      focus: tix.focusMemo(document.activeElement, document.querySelector("main") || document.body),
      snapshot: target.cloneNode(true),
      original: target
    };
    target.classList.add("is-pending");
    optimistic(form, event.submitter, scope);
  }, true);

  function finish(ok, html) {
    var done = inflight;
    inflight = null;
    if (!done) {
      return;
    }
    var now = document.querySelector(done.target);
    if (!ok) {
      if (done.original.isConnected) {
        done.original.replaceWith(done.snapshot);
        global.htmx.process(done.snapshot);
        now = done.snapshot;
      }
      showError(errorText(html));
    }
    if (now) {
      now.classList.remove("is-pending");
    }
    tix.focusRestore(document, done.focus);
  }

  document.addEventListener("htmx:beforeSwap", function (event) {
    if (!inflight || event.detail.requestConfig && event.detail.requestConfig.elt !== inflight.form) {
      return;
    }
    // live.js swaps any refusal in so the error page can explain itself. An
    // in-place swap selects one row out of it, and an error page has no such
    // row, so the row would vanish. The old state comes back instead, with
    // the refusal in a toast.
    if (event.detail.xhr.status >= 400) {
      event.detail.shouldSwap = false;
      finish(false, event.detail.xhr.responseText);
    }
  });
  document.addEventListener("htmx:afterSettle", function (event) {
    if (inflight && event.detail.requestConfig && event.detail.requestConfig.elt === inflight.form) {
      finish(true);
    }
    settleToast();
  });
  document.addEventListener("htmx:sendError", function () {
    if (inflight) {
      finish(false, "");
      showError(say("Could not reach the server. Nothing was changed."));
    }
  });

  // -- navigation ----------------------------------------------------------------

  // Cross-fade between screens where the browser can, and only between
  // screens: a row swapped in place is not a new page, and a fade over the
  // whole window for one row reads as the page reloading.
  function configureTransitions() {
    if (!global.htmx || !global.htmx.config) {
      return;
    }
    global.htmx.config.globalViewTransitions = tix.viewTransitions(document, global.matchMedia);
  }
  document.addEventListener("htmx:beforeTransition", function (event) {
    var target = event.detail && event.detail.target;
    if (target && target !== document.body) {
      event.preventDefault();
    }
  });

  // Scroll positions inside the page. The task list scrolls inside its own
  // box, not the window, and htmx's history only remembers the window, so a
  // reader who opened a task from the fortieth row came back to the first.
  var placesKey = "tix.places";
  function scrollers() {
    var main = document.querySelector("main.wrap");
    if (!main) {
      return [];
    }
    return Array.prototype.filter.call(main.querySelectorAll("*"), function (el) {
      return el.scrollHeight > el.clientHeight + 1 || el.scrollWidth > el.clientWidth + 1;
    });
  }
  function rememberPlaces(event) {
    // htmx saves the screen being left under the path it was drawn for, and
    // on Back it does so after the address has already changed, so the path
    // comes from the event rather than from location.
    var path = event && event.detail && event.detail.path ||
      global.location.pathname + global.location.search;
    try {
      var all = JSON.parse(global.sessionStorage.getItem(placesKey) || "{}");
      all[path] = tix.scrollPlaces(scrollers());
      global.sessionStorage.setItem(placesKey, JSON.stringify(all));
    } catch (err) {
      return;
    }
  }
  function restorePlaces() {
    try {
      var all = JSON.parse(global.sessionStorage.getItem(placesKey) || "{}");
      var places = all[global.location.pathname + global.location.search];
      if (places) {
        tix.restorePlaces(scrollers(), places);
      }
    } catch (err) {
      return;
    }
  }
  document.addEventListener("htmx:beforeHistorySave", rememberPlaces);
  document.addEventListener("htmx:historyRestore", restorePlaces);
  document.addEventListener("htmx:historyCacheMissLoad", function () {
    global.setTimeout(restorePlaces, 0);
  });
  if (global.addEventListener) {
    global.addEventListener("pagehide", rememberPlaces);
  }

  // Go on press. A mouse click is a press and a release, and the release
  // comes 80 to 150ms after the press; starting the navigation on the press
  // takes that off every link a reader opens from a list. Only for the main
  // mouse button with no modifier, never for touch (a press there may be the
  // start of a scroll) and never for the keyboard, whose Enter is already a
  // single event. The navigation itself is the link's own click, so htmx's
  // boost, its history and the task dialog all see exactly what they see
  // today; the real click that follows is swallowed.
  //
  // This replaced fetching on hover. A page fetched ahead and swapped in by
  // hand skipped htmx's history bookkeeping, and Back then restored the
  // wrong screen under the right address.
  var pressed = null;

  function linkFor(event) {
    var a = event.target && event.target.closest ? event.target.closest("a[href]") : null;
    if (!a || a.target || a.hasAttribute("download") || a.closest('[hx-boost="false"], [data-no-press]') ||
        a.hasAttribute("hx-get") || a.hasAttribute("hx-target") || a.hasAttribute("data-edit-title")) {
      return null;
    }
    return tix.goesOnPress(a, global.location) ? a : null;
  }

  global.addEventListener && global.addEventListener("mousedown", function (event) {
    if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey ||
        !htmxReady()) {
      return;
    }
    var a = linkFor(event);
    if (!a) {
      return;
    }
    pressed = { link: a, at: Date.now() };
    a.click();
  }, true);

  global.addEventListener && global.addEventListener("click", function (event) {
    if (!pressed || !event.isTrusted) {
      return;
    }
    // Whatever the release lands on: by then the press may already have
    // swapped the next screen in under the pointer, and a click there must
    // not press whatever button happens to be in that spot.
    var recent = Date.now() - pressed.at < 1000;
    pressed = null;
    if (recent) {
      event.preventDefault();
      event.stopImmediatePropagation();
    }
  }, true);

  // Filter as you type. The box and the two selects beside it answer on
  // their own once the reader pauses; Apply stays, because without a script
  // it is the only thing that submits them.
  var filterTimer = null;
  var filterLast = null;
  var filterRequest = 0;
  function runFilter(form) {
    var query = new global.URLSearchParams(new global.FormData(form)).toString();
    if (query === filterLast) {
      return;
    }
    filterLast = query;
    var mine = ++filterRequest;
    var url = form.getAttribute("action") + "?" + query;
    form.setAttribute("aria-busy", "true");
    global.fetch(url, { credentials: "same-origin", headers: { "HX-Request": "true" } })
      .then(function (resp) { return resp.ok ? resp.text() : null; })
      .then(function (html) {
        if (mine !== filterRequest) {
          return;
        }
        form.removeAttribute("aria-busy");
        if (html === null) {
          return;
        }
        var doc = new global.DOMParser().parseFromString(html, "text/html");
        ["tasks-summary", "tasks-listing"].forEach(function (id) {
          var fresh = doc.getElementById(id);
          var here = document.getElementById(id);
          if (fresh && here) {
            here.innerHTML = fresh.innerHTML;
            global.htmx.process(here);
          }
        });
        var live = document.getElementById("live-screen");
        var freshLive = doc.getElementById("live-screen");
        if (live && freshLive) {
          ["data-live", "data-live-filter"].forEach(function (name) {
            live.setAttribute(name, freshLive.getAttribute(name) || "");
          });
        }
        var fault = document.querySelector(".filterfault");
        var freshFault = doc.querySelector(".filterfault");
        if (fault && !freshFault) {
          fault.remove();
        } else if (freshFault) {
          if (fault) {
            fault.textContent = freshFault.textContent;
          } else {
            var listing = document.getElementById("tasks-listing");
            if (listing) {
              listing.parentNode.insertBefore(document.importNode(freshFault, true), listing);
            }
          }
        }
        global.history.replaceState(global.history.state, "", url);
      })
      .catch(function () {
        form.removeAttribute("aria-busy");
      });
  }
  function filterForm(el) {
    var form = el && el.closest ? el.closest("form.filterbar") : null;
    return form && htmxReady() && document.getElementById("tasks-listing") ? form : null;
  }
  document.addEventListener("input", function (event) {
    var form = filterForm(event.target);
    if (!form || event.target.name !== "q" || event.isComposing) {
      return;
    }
    if (filterTimer) {
      global.clearTimeout(filterTimer);
    }
    filterTimer = global.setTimeout(function () { runFilter(form); }, 300);
  });
  document.addEventListener("change", function (event) {
    var form = filterForm(event.target);
    if (form && event.target.tagName === "SELECT") {
      runFilter(form);
    }
  });

  // The view switch answers before its round trip: the segment pressed is
  // the one marked, so the press is acknowledged while the new drawing loads.
  document.addEventListener("click", function (event) {
    var seg = event.target && event.target.closest ? event.target.closest(".viewswitch .segment") : null;
    if (!seg) {
      return;
    }
    var group = seg.parentNode.querySelectorAll(".segment");
    for (var i = 0; i < group.length; i++) {
      group[i].classList.toggle("is-on", group[i] === seg);
    }
  });

  function start() {
    configureTransitions();
    filterLast = null;
    settleToast();
    var nav = global.performance && global.performance.getEntriesByType ?
      global.performance.getEntriesByType("navigation")[0] : null;
    if (nav && nav.type === "back_forward") {
      restorePlaces();
    }
  }
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", start);
  } else {
    start();
  }
  document.addEventListener("htmx:afterSwap", function (event) {
    if (event.detail && event.detail.target === document.body) {
      filterLast = null;
    }
  });
})(window);
