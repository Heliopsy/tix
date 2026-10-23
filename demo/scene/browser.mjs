// SPDX-License-Identifier: AGPL-3.0-or-later
//
// Drives the browser the recording films.
//
// It attaches over CDP to the real headful chromium already on the Xvfb
// display, rather than launching its own. What gets filmed is therefore a
// genuine browser window that a person could be sitting in front of; what
// drives it is a script, so the same beats happen every take.
//
// Typing is done character by character with a delay. A fill() would make the
// title appear instantly, which reads as a script running rather than as
// somebody planning work, and the point of this beat is that a person plans.

import { existsSync, writeFileSync } from "node:fs";
import { chromium } from "playwright-core";

const CDP = "http://127.0.0.1:9222";
const DONE = process.env.SCENE_PLAN_DONE || "/tmp/plan.done";
const READY = process.env.SCENE_READY || "/tmp/signed-in";
const GO = process.env.SCENE_GO || "/tmp/rolling";
const PASSWORD = process.env.DEMO_PASSWORD || "tix-demo-password";
const BASE = "http://127.0.0.1:8080";

const COUNT = Number(process.env.SCENE_TASKS || 4);

// The project the demo builds, and the backlog for it.
//
// It is a real, self-contained piece of work the agent can genuinely finish
// offline in a container: a small command line tool, written from nothing, in
// four steps that each change something visible. The seeded fixture's own
// projects are left alone and stay in the sidebar, so the installation looks
// like somebody's real one rather than an empty demo, but none of the work
// happens in them. That separation is what makes the recording honest: every
// task the agent closes is a task the viewer just watched being written.
const PROJECT_KEY = process.env.SCENE_PROJECT || "toolbelt";
const PROJECT_NAME = process.env.SCENE_PROJECT_NAME || "Toolbelt";

const PLAN = [
  "Write wordcount.py that prints lines, words and characters",
  "Add a --json flag to wordcount",
  "Handle an empty file without dividing by zero",
  "Write slugify.py that turns a title into a url slug",
  "Add a --max-length flag to slugify",
  "Write a README covering both tools",
];

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function main() {
  const browser = await chromium.connectOverCDP(CDP);
  const context = browser.contexts()[0];
  const page = context.pages()[0] ?? (await context.newPage());

  // The terminal beside this is dark, and in the first take the board came up
  // light: one product, two schemes, side by side. The scheme is a cookie the
  // server reads, so it is set here rather than left to the browser's own
  // preference, which chromium reports inconsistently under Xvfb.
  await context.addCookies([
    { name: "tix_theme", value: "dark", url: BASE },
  ]);

  // Signing in is not a feature anybody needs convincing of, and it is eight
  // seconds of somebody typing a password. It happens before the camera rolls.
  await page.goto(`${BASE}/login`, { waitUntil: "domcontentloaded" });
  await page.locator("input[name=email]").fill("nadia@demo.invalid");
  await page.locator("input[name=password]").fill(PASSWORD);
  await Promise.all([
    page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 20000 }),
    page.locator("button[type=submit]").click(),
  ]);
  await page.goto(`${BASE}/projects`, { waitUntil: "domcontentloaded" });
  await sleep(400);

  // Ready, and nothing on screen yet that the recording should miss.
  writeFileSync(READY, "signed in\n");
  for (;;) {
    if (existsSync(GO)) break;
    await sleep(200);
  }
  await sleep(600);

  // The project is made on camera. It is three fields, it takes four seconds,
  // and it establishes that the work about to be done did not come with the
  // fixture: it was created just now, by the person watching.
  await page.goto(`${BASE}/projects`, { waitUntil: "domcontentloaded" });
  await sleep(900);
  const newProject = page.locator('form[action="/projects"]').first();
  await newProject.locator("input[name=key]").pressSequentially(PROJECT_KEY, { delay: 60 });
  await sleep(250);
  await newProject.locator("input[name=name]").pressSequentially(PROJECT_NAME, { delay: 55 });
  await sleep(500);
  await newProject.locator("button[type=submit]").first().click();
  await page.waitForLoadState("domcontentloaded");
  await sleep(1200);

  // Quick-add answers with the new task's own page, so the form is gone by the
  // time the next title would be typed. Each pass therefore starts from the
  // list again. That is also better to watch: the viewer sees the task that was
  // just created before the next one is written.
  for (const title of PLAN.slice(0, COUNT)) {
    await page.goto(`${BASE}/tasks`, { waitUntil: "domcontentloaded" });
    const form = page.locator("form.quickadd");
    await form.locator("select[name=project_ref]").selectOption(PROJECT_KEY).catch(() => {});
    await sleep(250);

    const field = form.locator("input[name=title]");
    await field.click();
    await field.pressSequentially(title, { delay: 38 });
    await sleep(350);
    await field.press("Enter");
    await page.waitForLoadState("domcontentloaded");
    await sleep(900);
  }

  // Count what the socket actually delivers, so a board that does not move can
  // be told apart from a board with nothing to move it.
  await context.addInitScript(() => {
    const Native = window.WebSocket;
    window.__wsMsgs = 0;
    window.__wsSent = [];
    function Wrapped(...a) {
      const ws = new Native(...a);
      ws.addEventListener("message", () => { window.__wsMsgs++; });
      const send = ws.send.bind(ws);
      ws.send = (d) => { window.__wsSent.push(String(d).slice(0, 120)); return send(d); };
      return ws;
    }
    Wrapped.prototype = Native.prototype;
    window.WebSocket = Wrapped;
  });

  // Land on the board, because the board is what the viewer watches repaint
  // while the agent works.
  await sleep(600);
  await page.goto(`${BASE}/projects/${PROJECT_KEY}`, { waitUntil: "domcontentloaded" });
  await sleep(1500);

  // The connection is held open for the rest of the scene, and this process is
  // killed by run.sh at the end.
  //
  // Neither closing nor detaching works. browser.close() over CDP closes the
  // browser it attached to. Simply exiting drops the CDP transport, and
  // chromium then stops servicing the page: in three takes the board rendered,
  // opened its socket, and went silent, so it sat on its first paint while the
  // agent emptied the backlog beside it. The window still showed a board, which
  // is why it took three takes to notice that the board was a photograph.
  //
  // So the planning beat reports that it is finished by touching a file, and
  // then does nothing at all, for as long as the scene needs the page alive.
  writeFileSync(DONE, "planned\n");

  // A heartbeat, not an idle wait. It keeps the CDP connection open, and it
  // puts the page's own view of itself in the log once a second, which is the
  // only way to tell a board that is not updating from a board with nothing to
  // update.
  for (;;) {
    await sleep(2000);
    try {
      const state = await page.evaluate(() => {
        const cols = [...document.querySelectorAll("#project-board h2, #project-board h3")]
          .map((h) => h.textContent.trim()).join(" | ");
        const el = document.querySelector("#live-screen");
        return {
          url: location.pathname,
          hidden: document.hidden,
          cols,
          msgs: window.__wsMsgs ?? "nohook",
          liveFetches: performance.getEntriesByType("resource")
            .filter((r) => r.name.includes("/live")).length,
          err: window.__liveErr ?? "",
          sent: (window.__wsSent ?? [])[0] ?? "none",
        };
      });
      console.log(`[browser] ${state.url} msgs=${state.msgs} liveFetch=${state.liveFetches} err=${state.err} ${state.cols}`);
    } catch (err) {
      console.log(`[browser] page unreachable: ${String(err).slice(0, 120)}`);
    }
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
