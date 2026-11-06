// SPDX-License-Identifier: AGPL-3.0-or-later
//
// Screenshots every screen of the web interface, and every popup on it, at
// phone, tablet, Full HD and 4K sizes, then writes a contact sheet.
//
// By default it builds nothing and trusts --tix: it seeds a scratch database,
// serves it on a free loopback port, signs in as the seeded administrator and
// captures. The scratch directory and the server are removed on every exit,
// including a failure or ctrl-c. With --url it captures an instance that is
// already running instead, signing in with TIX_SHOT_EMAIL and
// TIX_SHOT_PASSWORD, and changes nothing there beyond the session it opens.
//
// routes.json lists every GET route in internal/web/routes.go, and a Go test
// fails when the two disagree, so a new screen cannot be left out silently.

import { spawn, spawnSync } from "node:child_process";
import { createHmac } from "node:crypto";
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright";

const HERE = dirname(fileURLToPath(import.meta.url));
const MANIFEST = JSON.parse(readFileSync(join(HERE, "routes.json"), "utf8"));

const MOBILE_UA =
  "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1";
const TABLET_UA =
  "Mozilla/5.0 (iPad; CPU OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1";

const VIEWPORTS = {
  phone: { viewport: { width: 390, height: 844 }, deviceScaleFactor: 3, isMobile: true, hasTouch: true, userAgent: MOBILE_UA },
  tablet: { viewport: { width: 820, height: 1180 }, deviceScaleFactor: 2, isMobile: true, hasTouch: true, userAgent: TABLET_UA },
  fullhd: { viewport: { width: 1920, height: 1080 }, deviceScaleFactor: 1 },
  "4k": { viewport: { width: 3840, height: 2160 }, deviceScaleFactor: 1 },
  // A laptop, which is where the task list sits between its phone layout and
  // its widest one. Opt-in: the default four are what the docs carry.
  laptop: { viewport: { width: 1440, height: 900 }, deviceScaleFactor: 1 },
};
const DEFAULT_VIEWPORTS = ["phone", "tablet", "fullhd", "4k"];

const QUIET_CSS = `*,*::before,*::after{transition:none!important;animation:none!important;
caret-color:transparent!important;scroll-behavior:auto!important}`;

const USAGE = `usage: node shoot.mjs [flags]

  --tix PATH            tix binary to seed and serve a scratch database with (default mode)
  --url URL             capture a running instance instead; signs in with TIX_SHOT_EMAIL,
                        TIX_SHOT_PASSWORD and, when the tenant asks for a code, TIX_SHOT_TOTP
  --viewports LIST      comma separated: phone,tablet,fullhd,4k,laptop (default all but laptop)
  --theme T             light, dark or both (default light)
  --only SUBSTR         only routes whose pattern contains SUBSTR (repeatable, comma separated)
  --max-popups N        popups captured per page state, after dedupe (default 40)
  --out DIR             parent of the timestamped output directory (default out/screenshots)
  --headed              show the browser
  --hostile             also seed a project of tasks with long titles, long tags and long
                        Markdown descriptions (headings, tables, code, unbroken tokens), sorted
                        to the top of /tasks, so the row checks have something to break on
  --strict              exit non-zero when the layout check finds a control on two lines, a
                        word broken in the middle, or a task row or card out of shape

  TIX_SHOT_DEBUG=1 prints how long each step took.
`;

function parseArgs(argv) {
  const a = { viewports: DEFAULT_VIEWPORTS, theme: "light", only: [], maxPopups: 40, out: "out/screenshots" };
  for (let i = 0; i < argv.length; i++) {
    const k = argv[i];
    const v = () => {
      if (i + 1 >= argv.length) throw new Error(`${k} needs a value`);
      return argv[++i];
    };
    if (k === "--tix") a.tix = v();
    else if (k === "--url") a.url = v().replace(/\/+$/, "");
    else if (k === "--viewports") a.viewports = v().split(",").map((s) => s.trim()).filter(Boolean);
    else if (k === "--theme") a.theme = v();
    else if (k === "--only") a.only.push(...v().split(",").filter(Boolean));
    else if (k === "--max-popups") a.maxPopups = Number(v());
    else if (k === "--out") a.out = v();
    else if (k === "--headed") a.headed = true;
    else if (k === "--strict") a.strict = true;
    else if (k === "--hostile") a.hostile = true;
    else if (k === "-h" || k === "--help") {
      out(USAGE);
      process.exit(0);
    } else throw new Error(`unknown flag ${k}\n\n${USAGE}`);
  }
  for (const vp of a.viewports) if (!VIEWPORTS[vp]) throw new Error(`unknown viewport ${vp}`);
  if (!["light", "dark", "both"].includes(a.theme)) throw new Error(`--theme must be light, dark or both`);
  if (!(a.maxPopups >= 0)) throw new Error("--max-popups must be a number");
  if (!a.url && !a.tix) throw new Error("pass --tix PATH, or --url URL for a running instance");
  if (a.hostile && a.url) throw new Error("--hostile seeds a scratch database, so it cannot be used with --url");
  return a;
}

// ---------------------------------------------------------------- lifecycle

const cleanups = [];
let cleaning = null;

// Runs every registered cleanup once, newest first. Every caller, the normal
// exit and a signal arriving during it alike, waits on the same run, so the
// process never exits halfway through stopping the server.
function cleanup() {
  cleaning ??= (async () => {
    while (cleanups.length) {
      try {
        await cleanups.pop()();
      } catch (err) {
        say(`cleanup: ${err.message}`);
      }
    }
  })();
  return cleaning;
}

// Output that outlives its reader: once a pipe is closed, as in
// `just screenshots | head`, a write fails, and the run carries on quietly
// rather than dying with the server still up.
function write(stream, line) {
  try {
    stream.write(`${line}\n`);
  } catch {
    // nobody is reading any more
  }
}
const say = (line) => write(process.stderr, line);
const out = (line) => write(process.stdout, line);
for (const stream of [process.stdout, process.stderr]) stream.on("error", () => {});

for (const event of ["uncaughtException", "unhandledRejection"]) {
  process.on(event, async (err) => {
    say(`screenshots: ${err?.message || err}`);
    await cleanup();
    process.exit(1);
  });
}

for (const sig of ["SIGINT", "SIGTERM", "SIGHUP"]) {
  process.on(sig, async () => {
    say(`\n${sig}: stopping`);
    await cleanup();
    process.exit(130);
  });
}

function freePort() {
  return new Promise((ok, fail) => {
    const s = createServer();
    s.unref();
    s.on("error", fail);
    s.listen(0, "127.0.0.1", () => {
      const { port } = s.address();
      s.close(() => ok(port));
    });
  });
}

// The environment every tix child runs in: no TIX_* variable from the
// caller's shell, and XDG directories inside the scratch directory, so no
// configuration file, context or default store of the caller's is reachable.
function isolatedEnv(dir) {
  const env = {};
  for (const [k, v] of Object.entries(process.env)) if (!k.startsWith("TIX_")) env[k] = v;
  for (const x of ["CONFIG", "DATA", "STATE", "CACHE"]) env[`XDG_${x}_HOME`] = join(dir, x.toLowerCase());
  return env;
}

async function startLocal(tix, hostile) {
  const dir = mkdtempSync(join(tmpdir(), "tix-shots-"));
  cleanups.push(() => rmSync(dir, { recursive: true, force: true }));
  const db = join(dir, "tix.db");
  const env = isolatedEnv(dir);
  const password = "tix-demo-password";
  const seed = spawnSync(tix, ["demo", "seed", "--db", db, "--no-discovery", "--password", password, "-q"], {
    env,
    cwd: dir,
    encoding: "utf8",
  });
  if (seed.status !== 0) throw new Error(`tix demo seed failed: ${seed.stderr || seed.stdout}`);
  if (hostile) seedHostile(tix, db, env, dir);
  const port = await freePort();
  const base = `http://127.0.0.1:${port}`;
  const server = spawn(
    tix,
    ["serve", "--db", db, "--no-discovery", "--listen", `127.0.0.1:${port}`, "--no-webhook-dispatcher", "--log-level", "warn"],
    { env, cwd: dir, stdio: ["ignore", "ignore", "pipe"] },
  );
  let serverLog = "";
  server.stderr.on("data", (d) => (serverLog = (serverLog + d).slice(-4000)));
  cleanups.push(
    () =>
      new Promise((ok) => {
        if (server.exitCode !== null) return ok();
        const kill = setTimeout(() => server.kill("SIGKILL"), 5000);
        server.once("exit", () => {
          clearTimeout(kill);
          ok();
        });
        server.kill("SIGTERM");
      }),
  );
  for (let i = 0; i < 100; i++) {
    if (server.exitCode !== null) throw new Error(`tix serve exited: ${serverLog}`);
    try {
      if ((await fetch(`${base}/readyz`)).ok) {
        out(`serving a scratch database at ${base}`);
        return { base, email: "nadia@demo.invalid", password, local: true };
      }
    } catch {
      // not listening yet
    }
    await new Promise((r) => setTimeout(r, 100));
  }
  throw new Error(`tix serve did not become ready: ${serverLog}`);
}

// Content shaped like a real imported checklist and worse: Markdown bodies
// with headings, tables, fenced code, nested lists and tokens with no break
// opportunity, long titles and long tags. Every task is highest priority and
// overdue so urgency puts them on the first page of /tasks, and there are
// more than nine so the references change width part way down the list.
const HOSTILE_TOKEN = "checklists/01-product-development-readiness/1.3-openspec-and-working-agreement.md";
function hostileBody(i) {
  return [
    `**Checklist item \`1.3.${i}\`** \u2014 section 01 \u00b7 Product & Development Readiness`,
    "Subsection: OpenSpec",
    "Tier: **V1** \u00b7 Owner: **SWE**",
    "",
    "# A heading inside a description",
    "",
    "Definition of done includes updating OpenSpec on any feature or behaviour-changing fix, written into the team's working agreement, so the obligation outlives any one person.",
    "",
    "| column | another |",
    "|---|---|",
    `| ${"x".repeat(120)} | y |`,
    "",
    "```",
    "y".repeat(200),
    "```",
    "",
    "- **Audit notes:**",
    `  - nested \`${HOSTILE_TOKEN}\``,
    "",
    `https://example.com/${"z".repeat(150)}`,
    "",
    `_Source: \`${HOSTILE_TOKEN}\`_`,
  ].join("\n");
}

function seedHostile(tix, db, env, dir) {
  const run = (argv) => {
    const r = spawnSync(tix, [...argv, "--db", db, "-q"], { env, cwd: dir, encoding: "utf8" });
    if (r.status !== 0) throw new Error(`tix ${argv.slice(0, 3).join(" ")} failed: ${r.stderr || r.stdout}`);
  };
  run(["project", "create", "HOSTILE", "Hostile content"]);
  const due = new Date(Date.now() - 86400000).toISOString().slice(0, 10);
  const titles = [
    "1.1.1 Core customer problem works end-to-end",
    "A very long title that keeps going and going well past any reasonable width for a single row in the list view of tasks, and then some more",
    HOSTILE_TOKEN,
    "w".repeat(90),
    "tagged",
    "1.3.15 OpenSpec updated on every behaviour change",
  ];
  for (let i = 0; i < 12; i++) {
    const argv = ["task", "add", titles[i % titles.length], "-p", "HOSTILE", "--priority", "highest", "--due", due, "--body", hostileBody(i)];
    if (i % 3 === 1) argv.push("--tag", "an-extremely-long-tag-name-without-any-spaces-at-all-anywhere", "--tag", "another-long-tag-name-here");
    if (i % 4 === 2) argv.push("--assignee", "nadia");
    run(argv);
  }
}

// ---------------------------------------------------------------- sign in

function totp(secret, now = Date.now()) {
  const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
  let bits = "";
  for (const c of secret.toUpperCase().replace(/[^A-Z2-7]/g, "")) bits += alphabet.indexOf(c).toString(2).padStart(5, "0");
  const key = Buffer.from(bits.match(/.{8}/g).map((b) => parseInt(b, 2)));
  const counter = Buffer.alloc(8);
  counter.writeBigUInt64BE(BigInt(Math.floor(now / 30000)));
  const h = createHmac("sha1", key).update(counter).digest();
  const o = h[h.length - 1] & 15;
  return String((h.readUInt32BE(o) & 0x7fffffff) % 1e6).padStart(6, "0");
}

async function signIn(page, target) {
  await page.goto(`${target.base}/login`, { waitUntil: "domcontentloaded" });
  await page.fill("#email", target.email);
  await page.fill("#password", target.password);
  const away = (u) => !u.pathname.startsWith("/login");
  const landed = () =>
    Promise.race([
      page.waitForURL(away, { timeout: 15000 }),
      page.locator("input#code").waitFor({ timeout: 15000 }),
      page.locator("p.flash.error").waitFor({ timeout: 15000 }),
    ]).catch(() => {});
  await page.click("form[action='/login'] button[type=submit]");
  await landed();
  if (await page.locator("input#code").count()) {
    if (!target.totp) throw new Error("the instance asks for a second factor; set TIX_SHOT_TOTP to the account's TOTP secret");
    if (await page.locator("text=/recovery codes|scan|enrol/i").count() && !(await page.locator("input[name=ticket]").count())) {
      throw new Error("the account has no second factor enrolled yet; enrol it once by hand first");
    }
    await page.fill("input#code", totp(target.totp));
    await page.locator("input#code").press("Enter");
    await page.waitForURL(away, { timeout: 15000 }).catch(() => {});
  }
  if (!away(new URL(page.url()))) {
    const why = (await page.locator("p.flash.error").first().textContent().catch(() => "")) || "still on /login";
    throw new Error(`sign in failed: ${why.trim()}`);
  }
}

// Things the seed does not write, so the screens that show one have one.
// Only ever run against the scratch database.
async function addFixtures(page, base, taskRef) {
  const post = async (path, data) => {
    const r = await page.request.post(`${base}/api/v1${path}`, { data });
    if (!r.ok()) say(`fixture ${path}: ${r.status()} ${(await r.text()).slice(0, 200)}`);
  };
  await post("/templates", { name: "Release checklist", task_ref: taskRef, with_subtasks: true, due_in: "3d" });
  await post("/filters", { name: "Urgent", query: "priority:1", shared: true });
  await post("/filters", { name: "Mine open", query: "assignee:me" });
  await post("/channels", { name: "Phone", url: "https://ntfy.invalid/tix-demo", format: "ntfy" });
  await post("/calendar", {});
}

// ---------------------------------------------------------------- page work

// Waits for the page to stop changing. networkidle is only worth waiting for
// after a navigation, and never for long: a live screen keeps a connection
// open and would hold every shot to the timeout.
async function settle(page, { navigated = false } = {}) {
  await page.waitForLoadState("domcontentloaded").catch(() => {});
  if (navigated) await page.waitForLoadState("networkidle", { timeout: 1500 }).catch(() => {});
  await page
    .waitForFunction(() => !document.querySelector(".htmx-request, .htmx-settling, .htmx-swapping, .htmx-added"), null, {
      timeout: 4000,
    })
    .catch(() => {});
  await page.evaluate(async () => {
    await document.fonts.ready;
    await Promise.all(
      [...document.images].filter((i) => !i.complete).map((i) => new Promise((r) => ((i.onload = i.onerror = r), setTimeout(r, 3000)))),
    );
    await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
  });
}

const DEBUG = !!process.env.TIX_SHOT_DEBUG;
let lastMark = Date.now();
function mark(what) {
  if (!DEBUG) return;
  const now = Date.now();
  say(`  [${now - lastMark}ms] ${what}`);
  lastMark = now;
}

// A touch viewport is not captured with fullPage. Playwright draws a full-page
// shot under a device metrics override of its own that drops touch emulation,
// so while it captures, (hover: none) and (pointer: coarse) stops matching and
// the shot shows what a phone hides. Growing the viewport itself to the page's
// height keeps the context's touch and mobile emulation, and the shot is then
// one plain viewport capture.
async function shoot(page, file, layout) {
  mkdirSync(dirname(file), { recursive: true });
  await page.evaluate(() => window.scrollTo(0, 0));
  mark(`before ${file.split("/").pop()}`);
  const opts = { path: file, animations: "disabled", caret: "hide" };
  const touch = await page.evaluate(() => matchMedia("(hover: none) and (pointer: coarse)").matches);
  if (!touch) {
    await page.screenshot({ ...opts, fullPage: true });
    mark("shot");
    if (layout) for (const f of await page.evaluate(layoutFindings, LAYOUT)) layout(f);
    return;
  }
  const size = page.viewportSize();
  try {
    let height = size.height;
    for (let i = 0; i < 4; i++) {
      const want = Math.max(size.height, await page.evaluate(() => document.documentElement.scrollHeight));
      if (want === height) break;
      height = want;
      await page.setViewportSize({ width: size.width, height });
      await page.evaluate(() => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r))));
    }
    await page.screenshot(opts);
  } finally {
    await page.setViewportSize(size);
  }
  mark("shot");
  if (layout) for (const f of await page.evaluate(layoutFindings, LAYOUT)) layout(f);
}

// ---------------------------------------------------------------- layout check

// What counts as a control, whose label is one unit and must sit on one line,
// and what counts as a label or heading, whose words must never be split.
// A "Save" drawn as "Sav" over "e" is what this exists to catch: the browser
// draws it without complaint, and no screenshot reviewer reads every button.
const LAYOUT = {
  controls: [
    "button", "summary", "[role=button]", "[role=menuitem]", "[role=tab]",
    ".flowgo", ".ghostlink", ".segment", ".badge", ".pill", ".chip", ".qchip", ".role", ".state",
    ".evchip", ".statuspill", ".navbadge", ".pager-step", ".backlink a", "nav.tabs a", "kbd",
  ].join(","),
  words: "h1,h2,h3,h4,h5,h6,label,legend,th,dt",
  rows: "ul.tasklist",
  cards: "article.card",
  blocks: "p,ul,ol,h1,h2,h3,h4,h5,h6,pre,table,blockquote,hr",
  aside: "[popover],details,dialog",
  columnsFrom: 640,
};

// Runs in the page. Returns every visible control whose label takes more than
// one line or runs past the window's edge, and every control, label or
// heading with a word split across two lines. Lines are told apart by the
// rectangles a Range reports for each text node, which is the only place the
// browser says where it broke a line.
function layoutFindings({ controls, words, rows, cards, blocks, aside, columnsFrom }) {
  const found = [];
  const shown = (el) => {
    if (el.closest("[hidden], .sr-only, [aria-hidden=true] > .sr-only")) return false;
    const r = el.getBoundingClientRect();
    if (r.width <= 2 || r.height <= 2) return false;
    const cs = getComputedStyle(el);
    return cs.visibility !== "hidden" && cs.display !== "none";
  };
  const texts = (el) => {
    const out = [];
    const walk = document.createTreeWalker(el, NodeFilter.SHOW_TEXT);
    for (let n = walk.nextNode(); n; n = walk.nextNode()) {
      if (!n.nodeValue.trim()) continue;
      const p = n.parentElement;
      if (!p || p.closest(".sr-only") || p.getBoundingClientRect().width <= 2) continue;
      out.push(n);
    }
    return out;
  };
  const rects = (range) => [...range.getClientRects()].filter((r) => r.width > 0.5 && r.height > 0.5);
  const lines = (nodes) => {
    const tops = [];
    for (const n of nodes) {
      const r = document.createRange();
      r.selectNodeContents(n);
      for (const b of rects(r)) {
        const mid = b.top + b.height / 2;
        if (!tops.some((t) => Math.abs(t - mid) < b.height / 2)) tops.push(mid);
      }
    }
    return tops.length;
  };
  const broken = (nodes) => {
    for (const n of nodes) {
      for (const m of n.nodeValue.matchAll(/\p{L}{2,}/gu)) {
        const r = document.createRange();
        r.setStart(n, m.index);
        r.setEnd(n, m.index + m[0].length);
        const tops = new Set(rects(r).map((b) => Math.round(b.top)));
        if (tops.size > 1) return m[0];
      }
    }
    return "";
  };
  const scrolls = (el) => {
    for (let a = el.parentElement; a && a !== document.body; a = a.parentElement) {
      if (/(auto|scroll|hidden|clip)/.test(getComputedStyle(a).overflowX)) return true;
    }
    return false;
  };
  const pastEdge = (el) => el.getBoundingClientRect().right > document.documentElement.clientWidth + 1 && !scrolls(el);
  const name = (el) => {
    let s = el.tagName.toLowerCase();
    if (el.id) s += `#${el.id}`;
    for (const c of el.classList) s += `.${c}`;
    return s;
  };
  const label = (el) => (el.innerText || el.textContent || "").replace(/\s+/g, " ").trim().slice(0, 60);
  const seen = new Set();
  const report = (el, kind, extra) => {
    const key = `${kind}|${name(el)}|${label(el)}`;
    if (seen.has(key)) return;
    seen.add(key);
    found.push({ kind, el: name(el), text: label(el), word: extra || "" });
  };
  for (const el of document.querySelectorAll(controls)) {
    if (!shown(el)) continue;
    if (el.parentElement && el.parentElement.closest(controls) && !el.matches("kbd, .badge, .pill, .navbadge")) continue;
    const nodes = texts(el);
    if (!nodes.length) continue;
    const w = broken(nodes);
    if (w) report(el, "word split", w);
    else if (lines(nodes) > 1) report(el, "control on two lines");
    else if (pastEdge(el)) report(el, "control past the window edge");
  }
  for (const el of document.querySelectorAll(words)) {
    if (!shown(el)) continue;
    const w = broken(texts(el));
    if (w) report(el, "word split", w);
  }

  // A task row or card is a summary of a task. Its description, rendered,
  // belongs on the task, and a row wider than its list or far taller than
  // its neighbours is the list losing its shape. Popovers and folds inside a
  // row are its menus, drawn elsewhere, so their paragraphs do not count.
  const wider = (el, box) => el.scrollWidth > el.clientWidth + 1 || el.getBoundingClientRect().right > box.getBoundingClientRect().right + 1;
  const leaked = (el) => [...el.querySelectorAll(blocks)].find((b) => !b.closest(aside) && shown(b));
  for (const list of document.querySelectorAll(rows)) {
    if (!shown(list)) continue;
    const items = [...list.querySelectorAll(":scope > li.task")].filter(shown);
    for (const li of items) {
      if (wider(li, list)) report(li, "row wider than its list");
      const b = leaked(li);
      if (b) report(li, "markdown block inside a row", b.tagName.toLowerCase());
    }
    if (items.length < 3) continue;
    const heights = items.map((li) => li.getBoundingClientRect().height).sort((x, y) => x - y);
    const median = heights[Math.floor(heights.length / 2)];
    for (const li of items) {
      if (li.getBoundingClientRect().height > 2 * median + 1) report(li, "row over twice the height of the others", `${Math.round(median)}px`);
    }
    if (document.documentElement.clientWidth < columnsFrom) continue;
    const at = new Map();
    for (const li of items) {
      for (const cell of li.querySelectorAll(".task-meta > .tm")) {
        if (!shown(cell)) continue;
        const kind = [...cell.classList].find((c) => c.startsWith("tm-"));
        const x = Math.round(cell.getBoundingClientRect().left);
        if (!at.has(kind)) at.set(kind, { x, li });
        else if (Math.abs(at.get(kind).x - x) > 1) {
          report(li, "column out of line with the rows above", `${kind} ${at.get(kind).x}px vs ${x}px`);
          at.set(kind, { x, li });
        }
      }
    }
  }
  for (const card of document.querySelectorAll(cards)) {
    if (!shown(card)) continue;
    const box = card.parentElement.closest(".column, .lane, .board, .lanes") || card.parentElement;
    if (wider(card, box)) report(card, "card wider than its column");
    const b = leaked(card);
    if (b) report(card, "markdown block inside a card", b.tagName.toLowerCase());
  }
  return found;
}

function slug(s, max = 60) {
  return (
    s
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-+|-+$/g, "")
      .slice(0, max)
      .replace(/-+$/, "") || "root"
  );
}

function pageSlug(pattern) {
  return slug(pattern.replace(/\{([a-z]+)\}/g, (_, n) => n.toUpperCase()), 80);
}

// Marks every popup the page can show with data-shot and describes it. A
// popup is a [popover] with an id, a <details> or a <dialog>. Its kind and
// text say whether it was already captured on another page, which keeps the
// sidebar and a tip with the same words to one shot per viewport. Its family
// is its kind without the row it belongs to plus the shape of its markup, so
// the same menu on fifty rows is captured once per page.
function discover() {
  const norm = (s) => (s || "").replace(/\s+/g, " ").trim();
  const out = [];
  let n = 0;
  const scope = document.querySelector("dialog[open]:modal") || document;
  const REF = /\b[A-Za-z0-9]+-[0-9]+\b/g;
  const skeleton = (el) =>
    [...el.querySelectorAll("*")].map((d) => d.tagName + "." + [...d.classList].sort().join(".")).join(" ");
  const label = (el) => norm(el.getAttribute("aria-label") || el.getAttribute("title") || el.textContent).slice(0, 60);
  for (const el of scope.querySelectorAll("[popover][id]")) {
    if (el.matches(":popover-open")) continue;
    const kind = "popover-" + el.id.replace(/-[a-z0-9]+-[0-9]+$/, "").replace(/[0-9a-zA-Z]{26}/g, "ID");
    const perRow = kind !== `popover-${el.id}`;
    const family = perRow ? kind : `${kind.includes("-", 8) ? kind.replace(/-[^-]+$/, "") : kind}#${skeleton(el)}`;
    el.dataset.shot = String(++n);
    out.push({ shot: n, type: "popover", kind, family, row: perRow, name: el.id, text: norm(el.textContent).slice(0, 2000) });
  }
  for (const el of scope.querySelectorAll("details")) {
    if (el.open) continue;
    const summary = el.querySelector(":scope > summary");
    if (!summary) continue;
    const kind = "details-" + ([...el.classList].join(".") || "plain");
    const name = label(summary) || kind;
    el.dataset.shot = String(++n);
    const general = name.replace(REF, "REF");
    out.push({ shot: n, type: "details", kind, family: `${kind}:${general}#${skeleton(el)}`, row: general !== name, name, text: norm(el.textContent).slice(0, 2000) });
  }
  for (const el of scope.querySelectorAll("dialog[id]")) {
    if (el.open) continue;
    el.dataset.shot = String(++n);
    out.push({ shot: n, type: "dialog", kind: "dialog-" + el.id, family: el.id, name: el.id, text: norm(el.textContent).slice(0, 2000) });
  }
  return out;
}

// Puts the page back the way it loaded: every popover hidden, every
// disclosure this script opened closed, every dialog shut.
function resetPopups() {
  document.querySelectorAll(":popover-open").forEach((p) => {
    try {
      p.hidePopover();
    } catch {
      // already gone
    }
  });
  document.querySelectorAll("details[data-shot][open]").forEach((d) => d.removeAttribute("open"));
  document.querySelectorAll("dialog[open]:not(#taskdlg)").forEach((d) => d.close());
  const help = document.getElementById("shortcuts-help");
  if (help && !help.hidden) help.querySelector("[data-shortcuts-close]")?.click();
  if (document.activeElement && document.activeElement !== document.body) document.activeElement.blur();
}

// The popups enclosing one, outermost first, so they can be opened in turn.
function ancestors(shot) {
  const el = document.querySelector(`[data-shot="${shot}"]`);
  const chain = [];
  for (let p = el?.parentElement; p; p = p.parentElement) {
    if (p.matches("details") && !p.open) chain.unshift({ type: "details", shot: p.dataset.shot });
    if (p.matches("[popover][id]") && !p.matches(":popover-open")) chain.unshift({ type: "popover", shot: p.dataset.shot, id: p.id });
  }
  return chain;
}

async function visibleTrigger(page, c) {
  const loc =
    c.type === "popover"
      ? page.locator(`[popovertarget="${c.name}"]:not([popovertargetaction="hide"])`)
      : page.locator(`[data-shot="${c.shot}"] > summary`);
  const count = await loc.count();
  for (let i = 0; i < count; i++) if (await loc.nth(i).isVisible()) return loc.nth(i);
  return null;
}

async function isOpen(page, shot) {
  return page.evaluate((s) => {
    const el = document.querySelector(`[data-shot="${s}"]`);
    if (!el) return false;
    if (el.matches("details")) return el.open;
    if (el.matches("dialog")) return el.open;
    return el.matches(":popover-open");
  }, shot);
}

async function openPopup(page, c) {
  if (c.type === "dialog") return false;
  let trig = await visibleTrigger(page, c);
  if (!trig) {
    for (const a of await page.evaluate(ancestors, c.shot)) {
      if (a.type === "details") await page.evaluate((s) => document.querySelector(`[data-shot="${s}"]`)?.setAttribute("open", ""), a.shot);
      else {
        const t = await visibleTrigger(page, { type: "popover", name: a.id });
        if (t) await t.click();
      }
    }
    trig = await visibleTrigger(page, c);
  }
  if (!trig) return false;
  await trig.scrollIntoViewIfNeeded().catch(() => {});
  await trig.click({ timeout: 1500 }).catch(() => {});
  return isOpen(page, c.shot);
}

async function applyState(page, state) {
  if (state.click) {
    const loc = page.locator(state.click);
    const n = await loc.count();
    let clicked = false;
    for (let i = 0; i < n && !clicked; i++) {
      if (await loc.nth(i).isVisible()) {
        await loc.nth(i).click();
        clicked = true;
      }
    }
    if (!clicked) return false;
  }
  if (state.press) {
    await page.evaluate(() => document.activeElement?.blur());
    await page.keyboard.press(state.press);
  }
  if (state.wait) {
    try {
      await page.locator(state.wait).first().waitFor({ state: "attached", timeout: 5000 });
    } catch {
      return false;
    }
  }
  await settle(page);
  return true;
}

// ---------------------------------------------------------------- plan

async function resolveParams(page, base, wanted) {
  const values = {};
  for (const name of wanted) {
    const p = MANIFEST.params[name];
    const re = new RegExp(p.match);
    await page.goto(`${base}${p.from}`, { waitUntil: "domcontentloaded" });
    const hrefs = await page.$$eval("a[href]", (as) => as.map((a) => new URL(a.href, location.href).pathname));
    const found = [...new Set(hrefs.map((h) => h.match(re)?.[1]).filter(Boolean))];
    values[name] = found.includes(p.prefer) ? p.prefer : found[0];
  }
  return values;
}

function plan(args, values) {
  const pages = [];
  const skipped = [];
  for (const r of MANIFEST.routes) {
    if (args.only.length && !args.only.some((o) => r.pattern.includes(o))) continue;
    if (r.skip) {
      skipped.push({ pattern: r.pattern, why: r.skip });
      continue;
    }
    let path = r.pattern;
    let missing = null;
    for (const [ph, resolver] of Object.entries(r.params || {})) {
      const v = values[resolver];
      if (!v) missing = `no ${resolver} found on ${MANIFEST.params[resolver].from}`;
      else path = path.replace(`{${ph}}`, encodeURIComponent(v));
    }
    if (missing) {
      skipped.push({ pattern: r.pattern, why: missing });
      continue;
    }
    pages.push({ pattern: r.pattern, path, slug: pageSlug(r.pattern), public: !!r.public, states: r.states || [] });
  }
  return { pages, skipped };
}

// ---------------------------------------------------------------- capture

async function newContext(browser, vp, theme, target, storageState) {
  const ctx = await browser.newContext({
    ...VIEWPORTS[vp],
    colorScheme: theme,
    reducedMotion: "reduce",
    storageState,
  });
  const host = new URL(target.base).hostname;
  await ctx.addCookies([
    { name: "tix_theme", value: theme, domain: host, path: "/" },
    { name: "tix_advanced", value: "1", domain: host, path: "/" },
  ]);
  await ctx.addInitScript((css) => {
    const add = () => {
      const s = document.createElement("style");
      s.dataset.shotQuiet = "";
      s.textContent = css;
      (document.head || document.documentElement).appendChild(s);
    };
    if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", add);
    else add();
  }, QUIET_CSS);
  return ctx;
}

async function captureViewport(browser, args, target, storageState, vp, theme, runDir, plans, stats) {
  const dirName = theme === "light" ? vp : `${vp}-dark`;
  const shots = [];
  const seen = new Set();
  const globalsDone = new Set();
  const host = new URL(target.base).hostname;
  const layout = (pg, popup) => (f) => {
    const where = `${dirName} ${pg.path}${popup ? ` [${popup}]` : ""}`;
    const line = `${where}: ${f.kind}${f.word ? ` "${f.word}"` : ""} in ${f.el} "${f.text}"`;
    const sig = `${dirName}|${pg.pattern}|${f.kind}|${f.el}|${f.text}`;
    if (stats.layoutSeen.has(sig)) return;
    stats.layoutSeen.add(sig);
    stats.layout.push(line);
  };
  const record = (pg, state, popup, file) => {
    shots.push({ page: pg.pattern, slug: pg.slug, state, popup, viewport: dirName, file: relative(runDir, file) });
    stats.images++;
    if (popup) stats.popups++;
  };

  for (const pg of plans.pages) {
    const ctx = await newContext(browser, vp, theme, target, pg.public ? undefined : storageState);
    const page = await ctx.newPage();
    try {
      const variants = [{ name: "", cookies: {} }, ...pg.states];
      for (const st of variants) {
        const cookies = Object.entries(st.cookies || {}).map(([name, value]) => ({ name, value, domain: host, path: "/" }));
        if (cookies.length) await ctx.addCookies(cookies);
        const res = await page.goto(`${target.base}${pg.path}${st.query || ""}`, { waitUntil: "domcontentloaded" });
        if (!pg.public && new URL(page.url()).pathname === "/login") throw new Error("session lost: landed on /login");
        if (res && res.status() >= 400) stats.problems.push(`${dirName} ${pg.path}: HTTP ${res.status()}`);
        await settle(page, { navigated: true });
        if (st.name && !(await applyState(page, st))) {
          stats.problems.push(`${dirName} ${pg.pattern}: state ${st.name} could not be reached`);
          for (const name of Object.keys(st.cookies || {})) await ctx.clearCookies({ name });
          continue;
        }
        const base = join(runDir, dirName, pg.slug + (st.name ? `--${st.name}` : ""));
        await shoot(page, `${base}.png`, layout(pg, st.name));
        record(pg, st.name, "", `${base}.png`);
        if (!pg.public) await capturePopups(page, args, pg, st, base, seen, record, stats, dirName, layout);
        if (!pg.public && !st.name) {
          for (const g of MANIFEST.global) {
            if (globalsDone.has(g.name)) continue;
            await page.evaluate(resetPopups);
            if (await applyState(page, g)) {
              const f = `${base}--${g.name}.png`;
              await shoot(page, f, layout(pg, g.name));
              record(pg, st.name, g.name, f);
              globalsDone.add(g.name);
            }
            await page.keyboard.press("Escape").catch(() => {});
            await page.evaluate(resetPopups);
          }
        }
        for (const name of Object.keys(st.cookies || {})) await ctx.clearCookies({ name });
      }
      stats.pages.add(pg.pattern);
    } catch (err) {
      stats.problems.push(`${dirName} ${pg.pattern}: ${err.message.split("\n")[0]}`);
    } finally {
      await ctx.close();
    }
    out(`  ${dirName} ${pg.path}`);
  }
  for (const g of MANIFEST.global) if (!globalsDone.has(g.name)) stats.problems.push(`${dirName}: ${g.name} never opened`);
  return shots;
}

async function capturePopups(page, args, pg, st, base, seen, record, stats, dirName, layout) {
  const found = await page.evaluate(discover);
  const where = new URL(page.url()).pathname;
  const perRow = new Set();
  const names = new Set();
  let taken = 0;
  for (const c of found) {
    const sig = c.row ? c.family.replace(/#.*/, "") : `${c.kind}\u0000${c.text}`;
    if (seen.has(sig)) continue;
    if (perRow.has(c.family)) continue;
    if (taken >= args.maxPopups) {
      stats.capped++;
      continue;
    }
    await page.evaluate(resetPopups);
    let opened = false;
    try {
      opened = await openPopup(page, c);
    } catch {
      opened = false;
    }
    if (!opened) {
      perRow.add(c.family);
      continue;
    }
    seen.add(sig);
    perRow.add(c.family);
    await settle(page);
    const stem = slug(c.type === "details" ? `${c.kind.replace(/^details-/, "")}-${c.name}` : c.kind, 70);
    let name = stem;
    for (let i = 2; names.has(name); i++) name = `${stem}-${i}`;
    names.add(name);
    const f = `${base}--${name}.png`;
    await shoot(page, f, layout(pg, name));
    record(pg, st.name, c.type === "details" ? `${c.kind}: ${c.name}` : c.kind, f);
    taken++;
    if (new URL(page.url()).pathname !== where) {
      stats.problems.push(`${dirName} ${pg.pattern}: opening ${c.kind} navigated away`);
      return;
    }
  }
  await page.evaluate(resetPopups);
}

// ---------------------------------------------------------------- report

function esc(s) {
  return String(s).replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c]);
}

function writeIndex(runDir, shots, viewports, summary) {
  const byPage = new Map();
  for (const s of shots) {
    if (!byPage.has(s.page)) byPage.set(s.page, new Map());
    const key = `${s.state}\u0000${s.popup}`;
    const rows = byPage.get(s.page);
    if (!rows.has(key)) rows.set(key, { state: s.state, popup: s.popup, cells: {} });
    rows.get(key).cells[s.viewport] = s.file;
  }
  const head = viewports.map((v) => `<th>${esc(v)}</th>`).join("");
  const sections = [...byPage]
    .map(([page, rows]) => {
      const trs = [...rows.values()]
        .map((r) => {
          const what = [r.state && `state: ${r.state}`, r.popup || (r.state ? "" : "page")].filter(Boolean).join(" / ");
          const tds = viewports
            .map((v) =>
              r.cells[v]
                ? `<td><a href="${esc(r.cells[v])}"><img loading="lazy" src="${esc(r.cells[v])}" alt="${esc(`${page} ${what} ${v}`)}"></a></td>`
                : `<td class="none">deduped or n/a</td>`,
            )
            .join("");
          return `<tr><th scope="row">${esc(what)}</th>${tds}</tr>`;
        })
        .join("\n");
      return `<section id="${esc(pageSlug(page))}"><h2>${esc(page)}</h2>
<table><thead><tr><th></th>${head}</tr></thead><tbody>
${trs}
</tbody></table></section>`;
    })
    .join("\n");
  const toc = [...byPage.keys()].map((p) => `<a href="#${esc(pageSlug(p))}">${esc(p)}</a>`).join(" ");
  const html = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>tix screenshots</title>
<style>
:root{color-scheme:light dark;--bg:#fff;--fg:#1b1b1f;--muted:#666;--line:#ddd}
@media (prefers-color-scheme:dark){:root{--bg:#141418;--fg:#e8e8ec;--muted:#999;--line:#333}}
body{margin:0 16px 48px;background:var(--bg);color:var(--fg);font:14px/1.4 system-ui,sans-serif}
h1{font-size:20px}h2{font-size:16px;margin:32px 0 8px;font-family:ui-monospace,monospace}
nav{display:flex;flex-wrap:wrap;gap:4px 12px;font-family:ui-monospace,monospace;font-size:12px}
table{border-collapse:collapse}th,td{border-top:1px solid var(--line);padding:6px;vertical-align:top;text-align:left}
th[scope=row]{font-weight:500;max-width:220px;word-break:break-word}
td img{display:block;width:180px;max-height:320px;object-fit:cover;object-position:top;border:1px solid var(--line)}
td.none{color:var(--muted);font-size:12px;width:180px}
p.meta{color:var(--muted)}
</style></head><body>
<h1>tix screenshots</h1>
<p class="meta">${esc(summary)}</p>
<nav>${toc}</nav>
${sections}
</body></html>
`;
  const file = join(runDir, "index.html");
  writeFileSync(file, html);
  return file;
}

// ---------------------------------------------------------------- main

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const started = Date.now();
  let target;
  if (args.url) {
    const email = process.env.TIX_SHOT_EMAIL;
    const password = process.env.TIX_SHOT_PASSWORD;
    if (!email || !password) throw new Error("--url needs TIX_SHOT_EMAIL and TIX_SHOT_PASSWORD in the environment");
    target = { base: args.url, email, password, totp: process.env.TIX_SHOT_TOTP, local: false };
  } else {
    target = await startLocal(resolve(args.tix), args.hostile);
  }

  // Playwright's own signal handlers exit the process after closing the
  // browser, which would leave the server running; ours close everything.
  const browser = await chromium.launch({
    headless: !args.headed,
    channel: "chromium",
    handleSIGINT: false,
    handleSIGTERM: false,
    handleSIGHUP: false,
  });
  cleanups.push(() => browser.close());

  const login = await browser.newContext({ viewport: { width: 1920, height: 1080 } });
  const lp = await login.newPage();
  await signIn(lp, target);
  const wanted = new Set();
  for (const r of MANIFEST.routes) for (const v of Object.values(r.params || {})) wanted.add(v);
  if (target.local) {
    const pre = await resolveParams(lp, target.base, ["task"]);
    await addFixtures(lp, target.base, pre.task);
  }
  const values = await resolveParams(lp, target.base, [...wanted]);
  const storageState = await login.storageState();
  await login.close();

  const plans = plan(args, values);
  const stamp = new Date().toISOString().replace(/[:.]/g, "-").replace(/-\d+Z$/, "Z");
  const runDir = resolve(args.out, stamp);
  mkdirSync(runDir, { recursive: true });
  out(`capturing ${plans.pages.length} pages into ${runDir}`);

  const themes = args.theme === "both" ? ["light", "dark"] : [args.theme];
  const stats = { pages: new Set(), images: 0, popups: 0, capped: 0, problems: [], layout: [], layoutSeen: new Set() };
  const shots = [];
  const columns = [];
  for (const theme of themes) {
    for (const vp of args.viewports) {
      columns.push(theme === "light" ? vp : `${vp}-dark`);
      shots.push(...(await captureViewport(browser, args, target, storageState, vp, theme, runDir, plans, stats)));
    }
  }

  const secs = Math.round((Date.now() - started) / 1000);
  const summary =
    `${stats.pages.size} pages, ${stats.popups} popup shots, ${stats.images} images across ${columns.join(", ")}` +
    ` in ${secs}s` +
    (stats.capped ? `; ${stats.capped} popups over --max-popups not captured` : "");
  const index = writeIndex(runDir, shots, columns, summary);
  writeFileSync(
    join(runDir, "report.json"),
    JSON.stringify({ summary, skipped: plans.skipped, problems: stats.problems, layout: stats.layout, shots }, null, 2) + "\n",
  );
  out(summary);
  if (plans.skipped.length) {
    out(`not captured:`);
    for (const s of plans.skipped) out(`  ${s.pattern}: ${s.why}`);
  }
  if (stats.problems.length) {
    out(`problems:`);
    for (const p of stats.problems) out(`  ${p}`);
  }
  if (stats.layout.length) {
    out(`layout: ${stats.layout.length} findings (controls on two lines, words split, rows or cards out of shape)`);
    for (const l of stats.layout) out(`  ${l}`);
    if (args.strict) process.exitCode = 1;
  } else out("layout: no control on two lines, no word split, every row and card in shape");
  out(`\ncontact sheet: ${index}`);
}

try {
  await main();
} catch (err) {
  say(`screenshots: ${err.message}`);
  process.exitCode = 1;
} finally {
  await cleanup();
}
