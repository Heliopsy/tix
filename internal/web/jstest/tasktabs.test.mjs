// SPDX-License-Identifier: AGPL-3.0-or-later

// The activity tabs on a task screen: assets/tasktabs.js. The switching
// itself, focus and the live update are driven in Chrome by
// TestTaskTabsSwitchRememberAndSurviveALiveUpdateInABrowser.
import test from "node:test";
import assert from "node:assert/strict";
import { bareContext, run } from "./harness.mjs";

function loaded() {
  const ctx = bareContext();
  run(ctx, "tasktabs.js");
  return ctx.tix;
}

const names = ["comments", "history", "artifacts"];

test("a fragment beats a forced tab, which beats the remembered one and the default", () => {
  const tix = loaded();
  assert.equal(tix.taskTabPick(names, "artifacts", "history", "comments", "comments"), "artifacts");
  assert.equal(tix.taskTabPick(names, "", "history", "artifacts", "comments"), "history");
  assert.equal(tix.taskTabPick(names, "", "", "artifacts", "comments"), "artifacts");
  assert.equal(tix.taskTabPick(names, "", "", "", "comments"), "comments");
});

test("a remembered tab that no longer exists falls back to the default", () => {
  const tix = loaded();
  assert.equal(tix.taskTabPick(names, "", "", "attachments", "comments"), "comments");
  assert.equal(tix.taskTabPick(names, "", "", "", ""), "comments");
});

test("the arrow keys wrap, Home and End go to the ends, other keys do nothing", () => {
  const tix = loaded();
  assert.equal(tix.taskTabStep("ArrowRight", 2, 3), 0);
  assert.equal(tix.taskTabStep("ArrowLeft", 0, 3), 2);
  assert.equal(tix.taskTabStep("ArrowRight", 0, 3), 1);
  assert.equal(tix.taskTabStep("Home", 2, 3), 0);
  assert.equal(tix.taskTabStep("End", 0, 3), 2);
  assert.equal(tix.taskTabStep("Enter", 1, 3), -1);
  assert.equal(tix.taskTabStep("ArrowDown", 1, 3), -1);
});

test("tabSearch sets one parameter and keeps the others", () => {
  const tix = loaded2().ctx.tix;
  assert.equal(tix.tabSearch("?client=zed&tab=cli", "tab", "mcp"), "?client=zed&tab=mcp");
  assert.equal(tix.tabSearch("", "tab", "ssh"), "?tab=ssh");
  assert.equal(tix.tabSearch("?tab=mcp", "client", "codex"), "?tab=mcp&client=codex");
});

// A tree just big enough for tasktabs.js: attributes, parents, and a
// selector matcher for the compound and descendant selectors it uses.
function compoundMatches(node, compound) {
  const tag = compound.match(/^[a-z]+/);
  if (tag && node.tagName !== tag[0].toUpperCase()) {
    return false;
  }
  for (const [, name, value] of compound.matchAll(/\[([a-z-]+)(?:="([^"]*)")?\]/g)) {
    if (!node.hasAttribute(name) || (value !== undefined && node.getAttribute(name) !== value)) {
      return false;
    }
  }
  return true;
}

function nodeMatches(node, selector) {
  const parts = selector.trim().split(/\s+/);
  if (!compoundMatches(node, parts.pop())) {
    return false;
  }
  let at = node.parentNode;
  while (parts.length && at) {
    if (compoundMatches(at, parts[parts.length - 1])) {
      parts.pop();
    }
    at = at.parentNode;
  }
  return parts.length === 0;
}

function el(tag, attrs = {}, children = []) {
  const own = { ...attrs };
  const node = {
    tagName: tag.toUpperCase(),
    parentNode: null,
    children,
    hidden: "hidden" in own,
    tabIndex: own.tabindex === "-1" ? -1 : 0,
    focused: 0,
    focus() {
      this.focused++;
    },
    hasAttribute: (n) => Object.prototype.hasOwnProperty.call(own, n),
    getAttribute: (n) => (Object.prototype.hasOwnProperty.call(own, n) ? own[n] : null),
    setAttribute: (n, v) => {
      own[n] = String(v);
    },
    matches(sel) {
      return nodeMatches(this, sel);
    },
    closest(sel) {
      for (let at = this; at && at.matches; at = at.parentNode) {
        if (at.matches(sel)) {
          return at;
        }
      }
      return null;
    },
    querySelectorAll(sel) {
      const out = [];
      const walk = (n) => {
        for (const c of n.children) {
          if (c.matches(sel)) {
            out.push(c);
          }
          walk(c);
        }
      };
      walk(this);
      return out;
    },
  };
  for (const c of children) {
    c.parentNode = node;
  }
  return node;
}

function tab(id, on, param) {
  return el("a", { role: "tab", "data-tab-for": id, href: `/connect?${param}=${id}`, "aria-selected": on ? "true" : "false", ...(on ? {} : { tabindex: "-1" }) });
}

function panel(id, on, children = []) {
  return el("section", { role: "tabpanel", "data-tab-panel": id, ...(on ? {} : { hidden: "" }) }, children);
}

function connectPage() {
  const cliTab = tab("cli", true, "tab");
  const mcpTab = tab("mcp", false, "tab");
  const claude = tab("claude-code", true, "client");
  const codex = tab("codex", false, "client");
  const inner = el("div", { "data-task-tabs": "", "data-tab": "claude-code", "data-tab-forced": "", "data-tab-param": "client" }, [
    el("div", { role: "tablist" }, [claude, codex]),
    panel("claude-code", true),
    panel("codex", false),
  ]);
  const form = el("form", { "data-tab-keep": "", action: "/connect/token?tab=cli" });
  const outer = el("div", { "data-task-tabs": "", "data-tab": "cli", "data-tab-forced": "", "data-tab-param": "tab" }, [
    el("section", { id: "token" }, [form]),
    el("section", { id: "connect" }, [el("div", { role: "tablist" }, [cliTab, mcpTab])]),
    panel("cli", true),
    panel("mcp", false, [inner]),
  ]);
  const root = el("body", {}, [outer]);
  return { root, outer, inner, cliTab, mcpTab, claude, codex, form };
}

// loaded2 runs tasktabs.js against a document, a location and a history
// that record what the script did to them.
function loaded2(root = el("body"), search = "") {
  const listeners = {};
  const stored = [];
  const replaced = [];
  const location = { pathname: "/connect", search, hash: "" };
  const doc = {
    readyState: "complete",
    addEventListener: (type, fn) => {
      (listeners[type] = listeners[type] || []).push(fn);
    },
    getElementById: () => null,
    querySelectorAll: (sel) => root.querySelectorAll(sel),
  };
  const ctx = bareContext({
    document: doc,
    location,
    URLSearchParams,
    addEventListener() {},
    history: {
      state: null,
      replaceState(_state, _title, url) {
        replaced.push(url);
        location.search = url.slice(url.indexOf("?"));
      },
    },
    localStorage: {
      getItem: () => "",
      setItem: (k, v) => stored.push([k, v]),
    },
  });
  run(ctx, "tasktabs.js");
  const fire = (type, event) => {
    let prevented = false;
    for (const fn of listeners[type] || []) {
      fn({ button: 0, preventDefault: () => (prevented = true), ...event });
    }
    return prevented;
  };
  return { ctx, fire, stored, replaced, location };
}

test("a connect tab link switches in place and writes the address, not storage", () => {
  const p = connectPage();
  const { fire, stored, replaced, location } = loaded2(p.root, "?tab=cli");
  assert.equal(p.outer.children[2].hidden, false);
  assert.equal(p.outer.children[3].hidden, true);
  assert.equal(fire("click", { target: p.mcpTab }), true, "the navigation is prevented");
  assert.equal(p.outer.children[2].hidden, true);
  assert.equal(p.outer.children[3].hidden, false);
  assert.equal(p.mcpTab.getAttribute("aria-selected"), "true");
  assert.equal(p.mcpTab.tabIndex, 0);
  assert.equal(p.cliTab.tabIndex, -1);
  assert.deepEqual(replaced, ["/connect?tab=mcp"]);
  assert.equal(location.search, "?tab=mcp");
  assert.deepEqual(stored, []);
  assert.equal(p.codex.getAttribute("href"), "/connect?tab=mcp&client=codex");
  assert.equal(p.form.getAttribute("action"), "/connect/token?tab=mcp");
  assert.equal(p.claude.getAttribute("aria-selected"), "true", "the nested box is left alone");
});

test("a modified click on a tab link is left to the browser", () => {
  const p = connectPage();
  const { fire, replaced } = loaded2(p.root, "?tab=cli");
  assert.equal(fire("click", { target: p.mcpTab, ctrlKey: true }), false);
  assert.equal(fire("click", { target: p.mcpTab, button: 1 }), false);
  assert.equal(p.outer.children[3].hidden, true);
  assert.deepEqual(replaced, []);
});

test("arrow keys move the strip they are pressed in, and only that one", () => {
  const p = connectPage();
  const { fire, location } = loaded2(p.root, "?tab=mcp");
  fire("click", { target: p.mcpTab });
  assert.equal(fire("keydown", { target: p.claude, key: "ArrowRight" }), true);
  assert.equal(p.codex.getAttribute("aria-selected"), "true");
  assert.equal(p.codex.focused, 1);
  assert.equal(p.inner.children[2].hidden, false);
  assert.equal(p.mcpTab.getAttribute("aria-selected"), "true", "the outer strip did not move");
  assert.equal(location.search, "?tab=mcp&client=codex");
  fire("keydown", { target: p.mcpTab, key: "Home" });
  assert.equal(p.cliTab.getAttribute("aria-selected"), "true");
  assert.equal(p.cliTab.focused, 1);
  assert.equal(location.search, "?tab=cli&client=codex");
});

test("a task screen box without data-tab-param still remembers in storage", () => {
  const comments = el("button", { role: "tab", "data-tab-for": "comments", "aria-selected": "true" });
  const history = el("button", { role: "tab", "data-tab-for": "history", "aria-selected": "false" });
  const box = el("div", { "data-task-tabs": "", "data-tab": "comments" }, [
    el("div", { role: "tablist" }, [comments, history]),
    panel("comments", true),
    panel("history", false),
  ]);
  const { fire, stored, replaced } = loaded2(el("body", {}, [box]));
  assert.equal(fire("click", { target: history }), false, "a button click has no navigation to prevent");
  assert.equal(box.children[2].hidden, false);
  assert.deepEqual(stored, [["tix.taskTab", "history"]]);
  assert.deepEqual(replaced, []);
});
