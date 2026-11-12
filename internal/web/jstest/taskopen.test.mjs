// SPDX-License-Identifier: AGPL-3.0-or-later

// A task opened in a dialog, and a title renamed in place: the decisions in
// decide.js and the wiring in assets/taskopen.js.
import test from "node:test";
import assert from "node:assert/strict";
import { newContext, run } from "./harness.mjs";

const ORIGIN = "http://localhost";

// -- decisions ---------------------------------------------------------------

test("only a link to a task's own screen on this origin names a task", () => {
  const { tix } = newContext();
  const cases = [
    ["/tasks/infra-4", "infra-4"],
    ["/tasks/infra-4?next=%2Ftasks%3Fq%3Dtag%3Aops", "infra-4"],
    ["/tasks/infra-4#comments", "infra-4"],
    [ORIGIN + "/tasks/infra-4", "infra-4"],
    ["/tasks", ""],
    ["/tasks?q=tag:ops", ""],
    ["/tasks/live", ""],
    ["/tasks/infra-4/transition", ""],
    ["/tasks/infra-4/live", ""],
    ["//evil.example/tasks/infra-4", ""],
    ["https://evil.example/tasks/infra-4", ""],
    ["/projects/infra", ""],
    ["", ""],
  ];
  for (const [href, want] of cases) {
    assert.equal(tix.taskRefOf(href, ORIGIN), want, href);
  }
});

test("a modifier or another button is the browser's, not the page's", () => {
  const { tix } = newContext();
  assert.equal(tix.plainClick({ button: 0 }), true);
  for (const extra of [{ ctrlKey: true }, { metaKey: true }, { shiftKey: true }, { altKey: true }, { button: 1 }, { defaultPrevented: true }]) {
    assert.equal(tix.plainClick({ button: 0, ...extra }), false, JSON.stringify(extra));
  }
});

test("an answer is drawn in the dialog only when it is the dialog's task screen", () => {
  const { tix } = newContext();
  assert.equal(tix.dialogAnswer(200, '<div class="taskdlg-frag" data-task-dialog="infra-1">'), "draw");
  assert.equal(tix.dialogAnswer(200, "<!doctype html><html><body>the listing</body></html>"), "leave");
  assert.equal(tix.dialogAnswer(409, '<div data-task-dialog="infra-1">'), "refused");
  assert.equal(tix.dialogAnswer(0, ""), "refused");
});

test("a link inside the dialog stays in it only for a task or a screen that asks to", () => {
  const { tix } = newContext();
  const cases = [
    ["/tasks/infra-4", false, "task"],
    ["/tasks/infra-4", true, "task"],
    ["/tasks/infra-4/copy", true, "load"],
    [ORIGIN + "/tasks/infra-4/copy", true, "load"],
    ["/tasks/infra-4/copy", false, "leave"],
    ["/projects/infra", false, "leave"],
    ["//evil.example/tasks/infra-4/copy", true, "leave"],
    ["https://evil.example/x", true, "leave"],
    ["#comments", true, ""],
    ["", true, ""],
  ];
  for (const [href, stays, want] of cases) {
    assert.equal(tix.dialogLinkStep(href, ORIGIN, stays), want, href + " " + stays);
  }
});

test("history moves close, reopen or reload the dialog", () => {
  const { tix } = newContext();
  const entry = { tixDialog: true, over: "/tasks?q=tag:ops" };
  assert.equal(tix.dialogPop({ htmx: true }, true, "/tasks?q=tag:ops", "/tasks?q=tag:ops"), "close");
  assert.equal(tix.dialogPop(null, true, "/tasks", "/tasks"), "close");
  assert.equal(tix.dialogPop(null, true, "/tasks", "/activity"), "leave");
  assert.equal(tix.dialogPop(entry, true, "/tasks?q=tag:ops", "/tasks/infra-1"), "load");
  assert.equal(tix.dialogPop(entry, false, "/tasks?q=tag:ops", "/tasks/infra-1"), "reopen");
  assert.equal(tix.dialogPop(entry, false, "/activity", "/tasks/infra-1"), "reload");
  assert.equal(tix.dialogPop({ htmx: true }, false, "/tasks", "/tasks"), "ignore");
});

test("finishing a title edit saves only a changed, non-empty title", () => {
  const { tix } = newContext();
  assert.equal(tix.titleStep("Fix the build", "Fix the build", null), "cancel");
  assert.equal(tix.titleStep("Fix the build", "  Fix the build ", null), "cancel");
  assert.equal(tix.titleStep("Fix the build", "   ", null), "empty");
  assert.equal(tix.titleStep("Fix the build", "Fix the CI", null), "save");
  assert.equal(tix.titleStep("Fix the build", "Fix the CI", "Fix the CI"), "keep");
  assert.equal(tix.titleStep("Fix the build", "Fix it now", "Fix the CI"), "save");
});

test("a title save succeeds on the redirect and on nothing else", () => {
  const { tix } = newContext();
  assert.equal(tix.titleSaved({ type: "opaqueredirect", status: 0 }), true);
  assert.equal(tix.titleSaved({ type: "basic", status: 200 }), true);
  assert.equal(tix.titleSaved({ type: "basic", status: 409 }), false);
  assert.equal(tix.titleSaved({ type: "basic", status: 400 }), false);
  assert.equal(tix.titleSaved(null), false);
});

test("Enter and F2 edit, with no modifier", () => {
  const { tix } = newContext();
  assert.equal(tix.editKey({ key: "Enter" }), true);
  assert.equal(tix.editKey({ key: "F2" }), true);
  assert.equal(tix.editKey({ key: "F2", ctrlKey: true }), false);
  assert.equal(tix.editKey({ key: "e" }), false);
});

// -- wiring ------------------------------------------------------------------

// node is an element with attributes, children and as much selector matching
// as taskopen.js asks of it: a tag, attribute presence and attribute values.
function node(tagName, attrs = {}, children = []) {
  const own = { ...attrs };
  const el = {
    tagName: tagName.toUpperCase(),
    parentNode: null,
    children: [],
    hidden: false,
    textContent: own.textContent || "",
    value: own.value || "",
    focused: 0,
    listeners: {},
    getAttribute: (k) => (k in own ? own[k] : null),
    setAttribute: (k, v) => { own[k] = String(v); },
    hasAttribute: (k) => k in own,
    removeAttribute: (k) => { delete own[k]; },
    focus() { this.focused++; doc.activeElement = this; },
    select() {},
    addEventListener(type, fn) { (this.listeners[type] ||= []).push(fn); },
    fire(type, event = {}) { for (const fn of this.listeners[type] || []) fn(event); },
    matches(sel) { return matches(this, sel); },
    closest(sel) {
      for (let at = this; at; at = at.parentNode) {
        if (at.matches && at.matches(sel)) return at;
      }
      return null;
    },
    contains(other) {
      for (let at = other; at; at = at.parentNode) if (at === this) return true;
      return false;
    },
    descendants() { return this.children.flatMap((c) => [c, ...c.descendants()]); },
    querySelector(sel) { return this.descendants().find((c) => c.matches(sel)) || null; },
    querySelectorAll(sel) { return this.descendants().filter((c) => c.matches(sel)); },
    appendChild(child) { child.parentNode = this; this.children.push(child); return child; },
    insertBefore(child, ref) {
      child.parentNode = this;
      const at = ref ? this.children.indexOf(ref) : -1;
      if (at < 0) this.children.push(child); else this.children.splice(at, 0, child);
      return child;
    },
    removeChild(child) {
      this.children = this.children.filter((c) => c !== child);
      child.parentNode = null;
      return child;
    },
    get nextSibling() {
      if (!this.parentNode) return null;
      const s = this.parentNode.children;
      return s[s.indexOf(this) + 1] || null;
    },
    set innerHTML(v) { this.children = []; this._html = v; },
    get innerHTML() { return this._html || ""; },
  };
  for (const c of children) el.appendChild(c);
  return el;
}

function matches(el, sel) {
  const m = /^([a-z]+)?(?:\.([a-z-]+))?((?:\[[^\]]+\])*)$/.exec(sel);
  if (!m) return false;
  if (m[1] && el.tagName !== m[1].toUpperCase()) return false;
  if (m[2] && !(el.getAttribute("class") || "").split(" ").includes(m[2])) return false;
  for (const part of m[3].match(/\[[^\]]+\]/g) || []) {
    const [, name, value] = /^\[([^=\]]+)(?:="([^"]*)")?\]$/.exec(part);
    if (name === "open") {
      if (!el.open) return false;
      continue;
    }
    if (!el.hasAttribute(name)) return false;
    if (value !== undefined && el.getAttribute(name) !== value) return false;
  }
  return true;
}

let doc;

// page loads the assets over a listing: one row with a title link, and the
// dialog when Dialog mode put it there.
function page({ dialog = true, edit = false, fetchAnswer, lane = false, pane = false, width = 1440 } = {}) {
  const listeners = {};
  const winListeners = {};
  const title = node("a", {
    href: "/tasks/infra-1?next=%2Ftasks%3Fq%3Dtag%3Aops",
    textContent: "Fix the build",
    ...(edit ? { "data-edit-title": "infra-1", "data-version": "3" } : {}),
  });
  if (lane) title.setAttribute("class", "lane-title");
  const rename = node("a", {
    href: "/tasks/infra-1?next=%2Ftasks", class: "flowgo", "data-rename": "infra-1", "data-version": "4", textContent: "Rename",
  });
  const row = lane
    ? node("article", { class: "card lane-card", "data-shortcut-row": "", "data-shortcut-href": "/tasks/infra-1", tabindex: "-1" },
      [title, node("div", { popover: "" }, [rename])])
    : node("li", { "data-shortcut-row": "", "data-shortcut-href": "/tasks/infra-1", tabindex: "-1" }, [title]);
  const csrf = node("input", { name: "csrf_token", value: "tok" });
  const body = node("div", { id: "taskdlg-body" });
  const divider = node("div", { "data-pane-resize": "", role: "separator", tabindex: "0" });
  const dlg = node("dialog", { id: "taskdlg", ...(pane ? { "data-task-pane": "" } : {}) }, pane ? [divider, body] : [body]);
  dlg.open = false;
  dlg.modal = false;
  dlg.showModal = () => { dlg.open = true; dlg.modal = true; };
  dlg.show = () => { dlg.open = true; dlg.modal = false; };
  dlg.close = () => { dlg.open = false; dlg.modal = false; };
  const other = node("li", { "data-shortcut-row": "", "data-shortcut-href": "/tasks/infra-2", tabindex: "-1" },
    [node("a", { href: "/tasks/infra-2", textContent: "Second" })]);
  const root = node("body", {}, [row, other, csrf, ...(dialog ? [dlg] : [])]);
  const styles = {};
  const html = node("html");
  html.style = { setProperty: (k, v) => { styles[k] = v; } };

  const pushed = [];
  const fetches = [];
  const ajax = [];
  const events = [];
  const nav = [];
  const location = {
    origin: ORIGIN,
    pathname: "/tasks",
    search: "?q=tag:ops",
    get href() { return ORIGIN + this.pathname + this.search; },
    assign: (u) => nav.push(["assign", u]),
    replace: (u) => nav.push(["replace", u]),
    reload: () => nav.push(["reload"]),
  };
  const setURL = (u) => {
    const [p, q] = u.split("?");
    location.pathname = p;
    location.search = q ? "?" + q : "";
  };
  const history = {
    state: null,
    backs: 0,
    pushState(state, _t, url) { pushed.push(["push", state, url]); this.state = state; setURL(url); },
    replaceState(state, _t, url) { pushed.push(["replace", state, url]); this.state = state; setURL(url); },
    back() { this.backs++; },
  };
  doc = {
    activeElement: null,
    addEventListener(type, fn) { (listeners[type] ||= []).push(fn); },
    dispatchEvent(ev) { events.push(ev.type); for (const fn of listeners[ev.type] || []) fn(ev); },
    getElementById: (id) => (id === "taskdlg" && dialog ? dlg : id === "taskdlg-body" && dialog ? body : null),
    querySelector: (sel) => root.querySelector(sel),
    querySelectorAll: (sel) => root.querySelectorAll(sel),
    documentElement: html,
    cookie: "",
    createElement: (tag) => node(tag),
    contains: (el) => root.contains(el),
  };
  const ctx = newContext({
    document: doc,
    innerWidth: width,
    location,
    history,
    htmx: { ajax: (verb, url, opts) => ajax.push({ verb, url, opts }) },
    CustomEvent: function (type) { this.type = type; },
    URLSearchParams: function () {
      const data = {};
      this.set = (k, v) => { data[k] = v; };
      this.toString = () => JSON.stringify(data);
    },
    DOMParser: function () {
      return {
        parseFromString: (html) => ({
          querySelector: (sel) => {
            const m = new RegExp('class="' + sel.slice(1) + '"[^>]*>([^<]*)<').exec(html);
            return m ? { textContent: m[1] } : null;
          },
        }),
      };
    },
    fetch: (url, opts) => {
      fetches.push({ url, opts, form: JSON.parse(opts.body) });
      return Promise.resolve(fetchAnswer || { type: "opaqueredirect", status: 0 });
    },
  });
  ctx.addEventListener = (type, fn) => { (winListeners[type] ||= []).push(fn); };
  run(ctx, "taskopen.js");

  const fire = (type, event) => {
    for (const fn of listeners[type] || []) fn(event);
    return event;
  };
  const click = (target, extra = {}) => fire("click", {
    target, button: 0, prevented: 0, stopped: 0,
    preventDefault() { this.prevented++; this.defaultPrevented = true; },
    stopPropagation() { this.stopped++; },
    ...extra,
  });
  const pop = (state) => {
    history.state = state;
    const ev = { state, stopped: 0, stopImmediatePropagation() { this.stopped++; } };
    for (const fn of winListeners.popstate || []) fn(ev);
    return ev;
  };
  // answer plays htmx's swap of an answer into the dialog body.
  const answer = (status, text, responseURL = "") => {
    const detail = { target: body, xhr: { status, responseText: text, responseURL }, shouldSwap: status < 400 };
    fire("htmx:beforeSwap", { detail });
    if (detail.shouldSwap) {
      body.innerHTML = text;
      fire("htmx:afterSwap", { detail: { target: body } });
    }
    return detail;
  };
  const winFire = (type, event = {}) => {
    for (const fn of winListeners[type] || []) fn(event);
  };
  return { ctx, tix: ctx.tix, title, rename, row, other, divider, html, styles, winFire, doc, dlg, body, pushed, fetches, ajax, events, nav, history, location, click, fire, pop, answer };
}

const FRAGMENT = '<div class="taskdlg-frag" data-task-dialog="infra-1">';
const settle = () => new Promise((resolve) => setTimeout(resolve, 0));

test("a plain click on a task opens it in the dialog, and the address follows", () => {
  const p = page();
  const ev = p.click(p.title);
  assert.equal(ev.prevented, 1, "the link was followed as well");
  assert.equal(ev.stopped, 1, "the boost saw the click too");
  assert.equal(p.ajax.length, 1, "nothing asked for the task");
  assert.equal(p.ajax[0].url, "/tasks/infra-1?next=%2Ftasks%3Fq%3Dtag%3Aops");
  assert.equal(p.ajax[0].opts.target, "#taskdlg-body");
  assert.equal(p.ajax[0].opts.headers["X-Tix-Dialog"], "1", "the server was not asked for the dialog's drawing");
  assert.equal(p.pushed.length, 0, "the address moved before the task arrived");

  p.answer(200, FRAGMENT);
  assert.equal(p.dlg.open, true, "the dialog did not open");
  assert.equal(p.pushed.length, 1);
  const [how, state, url] = p.pushed[0];
  assert.equal(how, "push");
  assert.equal(url, "/tasks/infra-1?next=%2Ftasks%3Fq%3Dtag%3Aops");
  assert.equal(state.tixDialog, true);
  assert.equal(state.over, "/tasks?q=tag:ops", "the entry does not remember the page under it");
});

test("a modified click, or a page in Page mode, is left to the browser", () => {
  const p = page();
  for (const extra of [{ ctrlKey: true }, { metaKey: true }, { button: 1 }]) {
    const ev = p.click(p.title, extra);
    assert.equal(ev.prevented, 0, JSON.stringify(extra));
  }
  assert.equal(p.ajax.length, 0, "a new-tab click opened the dialog");

  const q = page({ dialog: false });
  assert.equal(q.click(q.title).prevented, 0, "Page mode took the click");
  assert.equal(q.ajax.length, 0);
});

test("Escape goes Back, and Back closes the dialog over the live page and returns focus", () => {
  const p = page();
  p.click(p.title);
  p.answer(200, FRAGMENT);
  const cancel = { target: p.dlg, prevented: 0, preventDefault() { this.prevented++; } };
  p.fire("cancel", cancel);
  assert.equal(cancel.prevented, 1, "the browser closed the dialog under the history entry");
  assert.equal(p.history.backs, 1, "closing did not go Back");
  assert.equal(p.dlg.open, true, "the dialog closed before history moved");

  p.location.pathname = "/tasks";
  p.location.search = "?q=tag:ops";
  const ev = p.pop({ htmx: true });
  assert.equal(p.dlg.open, false, "Back left the dialog open");
  assert.equal(ev.stopped, 1, "htmx was let restore its snapshot over the live page");
  assert.ok(p.title.focused > 0, "focus did not return to the title that opened it");
});

test("the browser's own Back closes it too, and a backdrop click goes Back", () => {
  const p = page();
  p.click(p.title);
  p.answer(200, FRAGMENT);
  p.location.pathname = "/tasks";
  p.location.search = "?q=tag:ops";
  const ev = p.pop({ htmx: true });
  assert.equal(p.dlg.open, false);
  assert.equal(ev.stopped, 1);

  p.click(p.title);
  p.answer(200, FRAGMENT);
  p.click(p.dlg);
  assert.equal(p.history.backs, 1, "a click on the backdrop did not close");
});

test("Forward onto the dialog's entry opens it again over the same page", () => {
  const p = page();
  p.click(p.title);
  p.answer(200, FRAGMENT);
  const entry = p.history.state;
  p.location.pathname = "/tasks";
  p.location.search = "?q=tag:ops";
  p.pop({ htmx: true });

  p.location.pathname = "/tasks/infra-1";
  p.location.search = "";
  const ev = p.pop(entry);
  assert.equal(ev.stopped, 1);
  assert.equal(p.ajax.length, 2, "Forward did not ask for the task again");
  p.answer(200, FRAGMENT);
  assert.equal(p.dlg.open, true);
  assert.equal(p.pushed.length, 1, "reopening from history added an entry");
});

test("a save answered with another page leaves the dialog for it", () => {
  const p = page();
  p.click(p.title);
  p.answer(200, FRAGMENT);
  const d = p.answer(200, "<!doctype html><html>listing</html>", ORIGIN + "/tasks?flash=task+deleted");
  assert.equal(d.shouldSwap, false, "a whole page was drawn inside the dialog");
  assert.equal(p.dlg.open, false);
  assert.deepEqual(p.nav, [["replace", ORIGIN + "/tasks?flash=task+deleted"]]);
  // The dialog's own close event arrives after the navigation began. Going
  // Back from it would abort the page the answer named, toast and all.
  p.fire("close", { target: p.dlg });
  assert.equal(p.history.backs, 0, "the late close went Back over the page the answer named");
  assert.notEqual(p.history.state && p.history.state.tixDialog, true, "the dialog's entry outlived the dialog");
});

test("a save in the dialog redraws it and refreshes the page under it", () => {
  const p = page();
  p.click(p.title);
  p.answer(200, FRAGMENT);
  assert.equal(p.events.filter((e) => e === "tix:refresh").length, 0, "opening refreshed the page");
  p.answer(200, FRAGMENT + "saved");
  assert.equal(p.body.innerHTML, FRAGMENT + "saved");
  assert.equal(p.events.filter((e) => e === "tix:refresh").length, 1, "the page under the dialog was not refreshed");
  assert.equal(p.pushed.length, 1, "a save added a history entry");
});

test("a refused save keeps the task and says why", () => {
  const p = page();
  p.click(p.title);
  p.answer(200, FRAGMENT);
  const d = p.answer(409, '<p class="failure-message" role="alert">version conflict<');
  assert.equal(d.shouldSwap, false, "the error page replaced the task");
  assert.equal(p.dlg.open, true);
  const note = p.body.querySelector("p");
  assert.ok(note && /version conflict/.test(note.textContent), "the refusal was not shown");
});

test("a link that asks to stay is drawn in the dialog, any other screen leaves it", () => {
  const p = page();
  p.click(p.title);
  p.answer(200, FRAGMENT);
  const copy = p.body.appendChild(node("a", { href: "/tasks/infra-1/copy", "data-dialog-load": "" }));
  const ev = p.click(copy);
  assert.equal(ev.prevented, 1, "the browser followed the link out of the dialog");
  assert.equal(p.ajax.length, 2, "the form was not asked for");
  assert.equal(p.ajax[1].url, "/tasks/infra-1/copy");
  assert.equal(p.ajax[1].opts.headers["X-Tix-Dialog"], "1");
  assert.deepEqual(p.nav, [], "the page left for the form");
  p.answer(200, '<div class="taskdlg-frag" data-task-dialog="infra-1">form');
  assert.equal(p.pushed.at(-1)[0], "replace", "the form added a history entry of its own");
  assert.equal(p.pushed.at(-1)[2], "/tasks/infra-1/copy");

  p.answer(200, FRAGMENT + "copied", ORIGIN + "/tasks/infra-1?flashref=infra-9&flash=task+copied");
  assert.deepEqual(p.pushed.at(-1).slice(0, 1).concat(p.pushed.at(-1).slice(2)),
    ["replace", "/tasks/infra-1?flashref=infra-9&flash=task+copied"], "the address stayed on the form after copying");
  const entries = p.pushed.length;
  p.answer(200, FRAGMENT + "saved again", ORIGIN + "/tasks/infra-1");
  assert.equal(p.pushed.length, entries, "a save on the task screen moved the address");

  const away = p.body.appendChild(node("a", { href: "/projects/infra" }));
  p.click(away);
  assert.equal(p.ajax.length, 2, "a screen that did not ask was drawn in the dialog");
  assert.deepEqual(p.nav, [["assign", "/projects/infra"]]);
});

test("a link that asks to stay opens the dialog from the page too, in Dialog mode only", () => {
  const p = page();
  const plain = p.row.appendChild(node("a", { href: "/projects/infra" }));
  assert.equal(p.click(plain).prevented, 0, "a link that did not ask was taken over");
  const copy = p.row.appendChild(node("a", { href: "/tasks/infra-1/copy?next=%2Ftasks", "data-dialog-load": "" }));
  const ev = p.click(copy);
  assert.equal(ev.prevented, 1, "the browser followed the link instead of the dialog");
  assert.equal(p.ajax.length, 1, "the form was not asked for");
  assert.equal(p.ajax[0].url, "/tasks/infra-1/copy?next=%2Ftasks");
  assert.equal(p.ajax[0].opts.headers["X-Tix-Dialog"], "1");
  p.answer(200, '<div class="taskdlg-frag" data-task-dialog="infra-1">form');
  assert.equal(p.dlg.open, true, "the dialog did not open over the page");

  const q = page({ dialog: false });
  const other = q.row.appendChild(node("a", { href: "/tasks/infra-1/copy", "data-dialog-load": "" }));
  assert.equal(q.click(other).prevented, 0, "Page mode took the click");
  assert.equal(q.ajax.length, 0);
});

test("a task that cannot be opened is shown as its own page", () => {
  const p = page();
  p.click(p.title);
  p.answer(404, "<html>not found</html>");
  assert.equal(p.dlg.open, false);
  assert.deepEqual(p.nav, [["assign", "/tasks/infra-1?next=%2Ftasks%3Fq%3Dtag%3Aops"]]);
});

test("the open shortcut opens a selected row in the dialog", () => {
  const p = page();
  assert.equal(p.tix.openRow(p.row), true);
  assert.equal(p.ajax[0].url, "/tasks/infra-1");
  assert.equal(page({ dialog: false }).tix.openRow(p.row), false, "Page mode took the shortcut");
});

// -- editing a title -----------------------------------------------------------

function editor(p) {
  return p.row.querySelector("input");
}

test("a click on an editable title edits it and Enter saves through the edit address", async () => {
  const p = page({ edit: true, dialog: false });
  const ev = p.click(p.title);
  assert.equal(ev.prevented, 1, "the click also opened the task");
  const input = editor(p);
  assert.ok(input, "no field took the title's place");
  assert.equal(p.title.hidden, true);
  assert.equal(input.value, "Fix the build");

  input.value = "Fix the CI";
  input.fire("keydown", { key: "Enter", preventDefault() {}, stopPropagation() {} });
  assert.equal(p.fetches.length, 1, "Enter sent nothing");
  const sent = p.fetches[0];
  assert.equal(sent.url, "/tasks/infra-1");
  assert.equal(sent.opts.method, "POST");
  assert.equal(sent.opts.redirect, "manual");
  assert.deepEqual(sent.form, { csrf_token: "tok", version: "3", title: "Fix the CI" },
    "the save named more than the title, or carried no version");

  await settle();
  assert.equal(p.title.textContent, "Fix the CI");
  assert.equal(p.title.hidden, false);
  assert.equal(editor(p), null, "the field stayed after saving");
  assert.ok(p.events.includes("tix:refresh"), "the row was not refreshed");
});

test("Escape puts the title back and an unchanged title sends nothing", () => {
  const p = page({ edit: true, dialog: false });
  p.click(p.title);
  editor(p).value = "Something else";
  editor(p).fire("keydown", { key: "Escape", preventDefault() {}, stopPropagation() {} });
  assert.equal(editor(p), null);
  assert.equal(p.title.textContent, "Fix the build");

  p.click(p.title);
  editor(p).fire("blur");
  assert.equal(p.fetches.length, 0, "an unchanged title was saved");
  assert.equal(editor(p), null);
});

test("a refused rename keeps the field open and says why beside it", async () => {
  const p = page({
    edit: true, dialog: false,
    fetchAnswer: {
      type: "basic", status: 409,
      text: () => Promise.resolve('<p class="failure-message" role="alert">version 3 is stale<'),
    },
  });
  p.click(p.title);
  editor(p).value = "Fix the CI";
  editor(p).fire("keydown", { key: "Enter", preventDefault() {}, stopPropagation() {} });
  await settle();
  await settle();
  const input = editor(p);
  assert.ok(input, "the field closed on a refusal");
  const note = p.row.querySelector("span");
  assert.equal(note.hidden, false);
  assert.match(note.textContent, /version 3 is stale/);
  assert.equal(input.getAttribute("aria-invalid"), "true");
  assert.equal(p.title.textContent, "Fix the build");

  input.fire("blur");
  assert.equal(p.fetches.length, 1, "leaving the field resent the refused title");
});

test("an empty title is refused without a request", () => {
  const p = page({ edit: true, dialog: false });
  p.click(p.title);
  editor(p).value = "  ";
  editor(p).fire("keydown", { key: "Enter", preventDefault() {}, stopPropagation() {} });
  assert.equal(p.fetches.length, 0);
  assert.equal(p.row.querySelector("span").hidden, false);
});

test("F2 on a selected row edits its title, and only when titles edit", () => {
  const p = page({ edit: true, dialog: false });
  doc.activeElement = p.row;
  const ev = { key: "F2", target: p.row, prevented: 0, preventDefault() { this.prevented++; }, stopPropagation() {} };
  p.fire("keydown", ev);
  assert.equal(ev.prevented, 1);
  assert.ok(editor(p), "F2 did not edit");

  const q = page({ edit: false, dialog: false });
  doc.activeElement = q.row;
  const ev2 = { key: "F2", target: q.row, prevented: 0, preventDefault() { this.prevented++; }, stopPropagation() {} };
  q.fire("keydown", ev2);
  assert.equal(ev2.prevented, 0, "F2 acted with titles opening the task");
});

test("Rename in a lane card's menu edits the card's title even when clicks open tasks", async () => {
  for (const dialog of [false, true]) {
    const p = page({ lane: true, dialog });
    const ev = p.click(p.rename);
    assert.equal(ev.prevented, 1, "Rename followed its link to the task");
    assert.equal(p.ajax.length, 0, "Rename opened the task in the dialog");
    const input = p.row.querySelector("input");
    assert.ok(input, "no field took the title's place");
    assert.equal(p.title.hidden, true);
    input.value = "Fix the CI";
    input.fire("keydown", { key: "Enter", preventDefault() {}, stopPropagation() {} });
    assert.deepEqual(p.fetches[0].form, { csrf_token: "tok", version: "4", title: "Fix the CI" },
      "the save did not carry the version the menu holds");
    await settle();
    assert.equal(p.title.textContent, "Fix the CI");
    assert.equal(p.rename.getAttribute("data-version"), "5", "a second rename would be refused as stale");
  }
});

// -- the panel ---------------------------------------------------------------

test("Panel opens beside the list from 1100px and as the dialog below", () => {
  const { tix } = newContext();
  assert.equal(tix.paneOpensIn(true, 1440), "pane");
  assert.equal(tix.paneOpensIn(true, 1100), "pane");
  assert.equal(tix.paneOpensIn(true, 1099), "dialog");
  assert.equal(tix.paneOpensIn(true, 390), "dialog");
  assert.equal(tix.paneOpensIn(false, 1920), "dialog", "Dialog mode opened a panel");
});

test("the panel's width keeps to 360..720 and 60% of the room beside the sidebar", () => {
  const { tix } = newContext();
  assert.equal(tix.paneMax(1216), 720);
  assert.equal(tix.paneMax(800), 480);
  assert.equal(tix.paneMax(500), 360, "the panel's floor gave way to a narrow room");
  assert.equal(tix.paneClamp(100, 1216), 360);
  assert.equal(tix.paneClamp(900, 1216), 720);
  assert.equal(tix.paneClamp(600, 800), 480);
  assert.equal(tix.paneClamp(500.4, 1216), 500);
  assert.equal(tix.paneClamp("wide", 1216), null);
  assert.equal(tix.paneClamp(null, 1216), null);
  assert.equal(tix.paneDrop(1000, 1440, 1216), 440, "a drop is not measured from the right edge");
  assert.equal(tix.paneDrop(100, 1440, 1216), 720);
});

test("the divider's keys: Left widens, Right narrows, Home, End and Enter", () => {
  const { tix } = newContext();
  assert.equal(tix.paneKey("ArrowLeft", false, 460, 1216), 476);
  assert.equal(tix.paneKey("ArrowRight", false, 460, 1216), 444);
  assert.equal(tix.paneKey("ArrowLeft", true, 460, 1216), 524);
  assert.equal(tix.paneKey("ArrowRight", false, 360, 1216), 360);
  assert.equal(tix.paneKey("ArrowLeft", false, 720, 1216), 720);
  assert.equal(tix.paneKey("Home", false, 500, 1216), 360);
  assert.equal(tix.paneKey("End", false, 500, 800), 480);
  assert.equal(tix.paneKey("Enter", false, 500, 1216), "reset");
  assert.equal(tix.paneKey("a", false, 500, 1216), null);
});

test("the panel's cookie holds a bounded width and leaves the default out", () => {
  const { tix } = newContext();
  assert.equal(tix.paneParse("w=500"), 500);
  assert.equal(tix.paneParse("w=9999"), 720);
  assert.equal(tix.paneParse("w=10"), 360);
  for (const bad of ["", "w=0", "w=-4", "w=wide", "junk"]) assert.equal(tix.paneParse(bad), null, bad);
  assert.equal(tix.paneFormat(500), "w=500");
  assert.equal(tix.paneFormat(5000), "w=720");
  assert.equal(tix.paneFormat(460), "", "the default was written");
  assert.equal(tix.paneFormat(null), "");
});

test("with Panel on a wide window a task opens beside the list and its row is marked", () => {
  const p = page({ pane: true });
  p.click(p.title);
  p.answer(200, FRAGMENT);
  assert.equal(p.dlg.open, true, "the panel did not open");
  assert.equal(p.dlg.modal, false, "the panel opened as a modal over the list");
  assert.equal(p.dlg.getAttribute("data-shown"), "pane");
  assert.ok(p.html.hasAttribute("data-pane-open"), "the page was not given the panel's room");
  assert.equal(p.styles["--pane-w"], "460px");
  assert.equal(p.divider.getAttribute("aria-valuenow"), "460");
  assert.equal(p.divider.getAttribute("aria-valuemin"), "360");
  assert.equal(p.row.getAttribute("aria-current"), "true", "the row in the panel is not marked");
  assert.equal(p.other.getAttribute("aria-current"), null);
  assert.equal(p.pushed[0][0], "push");
  assert.equal(p.pushed[0][2], "/tasks/infra-1?next=%2Ftasks%3Fq%3Dtag%3Aops");
});

test("with Panel on a narrow window a task opens in the dialog", () => {
  const p = page({ pane: true, width: 900 });
  p.click(p.title);
  p.answer(200, FRAGMENT);
  assert.equal(p.dlg.modal, true, "a narrow window got the panel");
  assert.equal(p.dlg.getAttribute("data-shown"), null);
  assert.equal(p.html.hasAttribute("data-pane-open"), false);
  assert.equal(p.row.getAttribute("aria-current"), null);
});

test("another task followed from the list replaces the one in the panel", () => {
  const p = page({ pane: true });
  p.click(p.title);
  p.answer(200, FRAGMENT);
  const ev = p.click(p.other.children[0]);
  assert.equal(ev.prevented, 1, "the list's link left the page");
  assert.equal(p.ajax.length, 2);
  p.answer(200, '<div class="taskdlg-frag" data-task-dialog="infra-2">');
  assert.equal(p.pushed.length, 2);
  assert.equal(p.pushed[1][0], "replace", "each task looked at took an entry of its own");
  assert.equal(p.pushed[1][1].over, "/tasks?q=tag:ops", "the entry forgot the list under it");
  assert.equal(p.other.getAttribute("aria-current"), "true");
  assert.equal(p.row.getAttribute("aria-current"), null, "the first row kept its mark");
});

test("Escape in the panel goes Back, but not from a field or from the list", () => {
  const p = page({ pane: true });
  p.click(p.title);
  p.answer(200, FRAGMENT);
  const field = p.body.appendChild(node("input", { name: "comment" }));
  const key = (target) => p.fire("keydown", {
    key: "Escape", target, prevented: 0, defaultPrevented: false,
    preventDefault() { this.prevented++; this.defaultPrevented = true; },
  });
  key(field);
  assert.equal(p.history.backs, 0, "Escape in a field closed the panel");
  key(p.row);
  assert.equal(p.history.backs, 0, "Escape in the list was taken by the panel");
  const ev = key(p.divider);
  assert.equal(ev.prevented, 1);
  assert.equal(p.history.backs, 1, "Escape in the panel did not go Back");

  p.location.pathname = "/tasks";
  p.location.search = "?q=tag:ops";
  p.pop({ htmx: true });
  assert.equal(p.dlg.open, false);
  assert.equal(p.dlg.getAttribute("data-shown"), null);
  assert.equal(p.html.hasAttribute("data-pane-open"), false, "the page kept the closed panel's room");
  assert.equal(p.row.getAttribute("aria-current"), null, "the row kept its mark after closing");
});

test("the divider resizes from the keyboard and the pointer and keeps the width", () => {
  const p = page({ pane: true });
  p.click(p.title);
  p.answer(200, FRAGMENT);
  const key = (k, shift = false) => p.fire("keydown", {
    key: k, shiftKey: shift, target: p.divider, prevented: 0, preventDefault() { this.prevented++; },
  });
  assert.equal(key("ArrowLeft").prevented, 1);
  assert.match(p.doc.cookie, /^tix_pane=w=476; Path=\/; Max-Age=\d+; SameSite=Lax/);
  assert.equal(p.styles["--pane-w"], "476px");
  assert.equal(p.divider.getAttribute("aria-valuenow"), "476");
  key("Enter");
  assert.match(p.doc.cookie, /^tix_pane=; Path=\/; Max-Age=0;/, "Enter did not forget the width");
  assert.equal(p.styles["--pane-w"], "460px");
  assert.equal(key("a").prevented, 0, "a key the divider ignores was taken");

  p.fire("pointerdown", { target: p.divider, button: 0, pointerId: 7, preventDefault() {} });
  assert.ok(p.html.hasAttribute("data-pane-drag"));
  p.fire("pointermove", { pointerId: 7, clientX: 900 });
  assert.equal(p.styles["--pane-w"], "540px", "the panel did not follow the pointer");
  assert.match(p.doc.cookie, /^tix_pane=; /, "a drag was kept before it ended");
  p.fire("pointerup", { pointerId: 7, clientX: 900 });
  assert.match(p.doc.cookie, /^tix_pane=w=540;/);
  assert.equal(p.html.hasAttribute("data-pane-drag"), false);
});

test("a navigation beside the panel closes it and gives its entry back to the list", () => {
  const p = page({ pane: true });
  p.click(p.title);
  p.answer(200, FRAGMENT);
  p.fire("htmx:beforeHistorySave", {});
  assert.equal(p.dlg.open, false, "the snapshot would hold an open panel");
  const last = p.pushed[p.pushed.length - 1];
  assert.deepEqual([last[0], last[1], last[2]], ["replace", null, "/tasks?q=tag:ops"]);
});

test("a window resized across 1100px moves an open task between panel and dialog", () => {
  const p = page({ pane: true });
  p.click(p.title);
  p.answer(200, FRAGMENT);
  p.ctx.innerWidth = 900;
  p.winFire("resize");
  assert.equal(p.dlg.open, true);
  assert.equal(p.dlg.modal, true, "a narrowed window kept the panel");
  assert.equal(p.html.hasAttribute("data-pane-open"), false);
  p.ctx.innerWidth = 1300;
  p.winFire("resize");
  assert.equal(p.dlg.modal, false, "a widened window kept the dialog");
  assert.equal(p.dlg.getAttribute("data-shown"), "pane");
  assert.equal(p.row.getAttribute("aria-current"), "true");
});
