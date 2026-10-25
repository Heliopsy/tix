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
const EPIC_FILE = process.env.SCENE_EPIC_FILE || "/tmp/epic.ref";
// The scene's end condition, written by the one piece of code that can see it.
// The board is the only place that knows whether the backlog has drained, so
// it is the board that says so: run.sh waits on this file instead of counting
// releases, and the two can no longer be satisfied by different states.
const DRAINED = process.env.SCENE_DRAINED || "/tmp/drained";
const PASSWORD = process.env.DEMO_PASSWORD || "tix-demo-password";

// How fast the person in the recording types, in milliseconds per character.
//
// The body of the epic was going in at 14ms, which is about 70 characters a
// second: faster than anybody reads, and the take is then sped up again on
// encode, so the one thing the viewer most needs to read went past unread.
// These are deliberately slower than real typing. The recording is not a
// speed test, and the words are the point of the beat.
const TYPE = Number(process.env.SCENE_TYPE_DELAY || 55);
const TYPE_BODY = Number(process.env.SCENE_TYPE_BODY_DELAY || 30);
const BASE = "http://127.0.0.1:8080";


// The project the demo builds, and the backlog for it.
//
// It is a real, self-contained piece of work the agent can genuinely finish
// offline in a container: a small command line tool, written from nothing, in
// four steps that each change something visible. The seeded fixture's own
// projects are left alone and stay in the sidebar, so the installation looks
// like somebody's real one rather than an empty demo, but none of the work
// happens in them. That separation is what makes the recording honest: every
// task the agent closes is a task the viewer just watched being written.
const PROJECT_KEY = process.env.SCENE_PROJECT || "links";
const PROJECT_COLOR = process.env.SCENE_PROJECT_COLOR || "violet";
const PROJECT_ICON = process.env.SCENE_PROJECT_ICON || "\u{1F517}";
const PROJECT_NAME = process.env.SCENE_PROJECT_NAME || "Link shortener";

// One task, deliberately too big for one sitting.
//
// The earlier version of this scene had a person writing six small tasks by
// hand, which showed agents working a queue but not why the queue exists. The
// argument for tix over a long agent loop is that a plan written into tasks is
// rows rather than context: a second agent can start on step three while the
// first is still on step one, a human can reorder it, and a crash loses
// nothing, because the work was never inside the process.
//
// So the person writes the intent, and an agent breaks it down on camera.
const EPIC_TITLE = "Build a link shortener service";
const EPIC_BODY = [
  "A small HTTP service that mints a short slug for a url and redirects it back.",
  "",
  "It needs the two handlers, errors that a client can act on, a health check,",
  "and enough documentation that somebody else could call it.",
  "",
  "Links expire. What a request for an expired link should get back is not",
  "decided yet, and it changes the client contract either way.",
  "",
  "Make no mistake.",
].join("\n");

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
  // The form is folded away until it is asked for, so it is asked for: the
  // button exists in the DOM while the disclosure is shut, which is not the
  // same as being clickable, and Playwright waited the full thirty seconds
  // for a control that was never going to become actionable.
  //
  // Opened on camera rather than through the DOM, because a reader is about
  // to watch a project being made and the first step of making one is this.
  const unfold = page.locator("details.addmore > summary").first();
  if (await unfold.count()) {
    await unfold.click();
    await sleep(500);
  }

  const newProject = page.locator('form[action="/projects"]').first();
  await newProject.locator("input[name=key]").pressSequentially(PROJECT_KEY, { delay: TYPE });
  await sleep(250);
  await newProject.locator("input[name=name]").pressSequentially(PROJECT_NAME, { delay: TYPE });
  await sleep(400);
  // A colour and an icon, because the fixture's projects have them and a
  // project made on camera that does not looks like a lesser kind of project.
  // They also carry through to the terminal board and the project list, which
  // are otherwise five columns of the same grey.
  await newProject.locator("select[name=color]").selectOption(PROJECT_COLOR).catch(() => {});
  await sleep(350);
  await newProject.locator("input[name=icon]").pressSequentially(PROJECT_ICON, { delay: TYPE }).catch(() => {});
  await sleep(500);
  await newProject.locator("button[type=submit]").first().click();
  await page.waitForLoadState("domcontentloaded");
  await sleep(1200);

  // One task, typed by a person, with the intent in its body. Quick-add lives
  // on the task list, not on the board, and it answers with the new task's own
  // page, which is where the body is written.
  await page.goto(`${BASE}/tasks`, { waitUntil: "domcontentloaded" });
  await sleep(700);
  const form = page.locator("form.quickadd");
  await form.locator("select[name=project_ref]").selectOption(PROJECT_KEY).catch(() => {});
  await sleep(300);
  const field = form.locator("input[name=title]");
  await field.click();
  await field.pressSequentially(EPIC_TITLE, { delay: TYPE });
  // A beat on the finished line before it is submitted, and another after, so
  // the title can be read rather than glimpsed.
  await sleep(1400);
  await field.press("Enter");
  await page.waitForLoadState("domcontentloaded");
  await sleep(1600);

  // The ref is in the url the quick-add landed on, and the scene needs it to
  // tell the planner which task to break down. The redirect carries a ?flash=
  // query string, so the path is parsed rather than the whole url split.
  const ref = new URL(page.url()).pathname.split("/").filter(Boolean).pop();
  writeFileSync(EPIC_FILE, `${ref}\n`);

  const body = page.locator("textarea[name=body]").first();
  if (await body.count()) {
    await body.click();
    await body.pressSequentially(EPIC_BODY, { delay: TYPE_BODY });
    // The longest dwell in the scene. This paragraph is the assignment the
    // agent is about to break down, and everything after it only makes sense
    // to somebody who read it.
    await sleep(2600);
    const save = page.locator('form:has(textarea[name=body]) button[type=submit]').first();
    if (await save.count()) {
      await save.click();
      await page.waitForLoadState("domcontentloaded");
    }
    await sleep(2200);
  }

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
  // The board wraps rather than scrolling sideways: auto-fit puts as many
  // columns in a row as fit and drops the rest onto a second row. At the width
  // the browser has here that is three across, so Done and Cancelled sit below
  // the fold and the one thing the scene exists to show, work arriving in
  // Done, happened off screen for every take so far.
  //
  // So the page follows the work. While there is anything left to take the
  // view stays on the backlog, which is where the movement is; once To do and
  // Doing are empty the board scrolls down to Done and stays there, which is
  // the end card.
  //
  // The epic does not count as work. The planner moves it to Doing and nothing
  // ever finishes it, because it is the container the subtasks hang off rather
  // than a task anybody takes: a take sat for fifteen minutes on
  // "To do (0) | Doing (1) | Blocked (1) | Done (5)", where the one in Doing
  // was the epic and there was nothing left for either agent to do. So it is
  // excluded here by its own ref, and the board ends with the epic still open
  // above a column of finished subtasks and one parked on a question, which is
  // what actually happened.
  // Drained and scrolled are two different things, and the marker belongs to
  // the first. The board is full width now, so all five columns sit in one row
  // and Done needs no scrolling to be on camera; if a longer backlog ever
  // pushes it under the fold again, the scroll still happens, but the scene no
  // longer waits on it having been necessary.
  let drained = false;

  for (;;) {
    await sleep(2000);
    try {
      if (!drained) {
        drained = await page.evaluate((epic) => {
          const count = (state) =>
            document.querySelectorAll(
              `#project-board .column[data-state="${state}"] .card:not([data-task="${epic}"])`,
            ).length;
          const done = document.querySelector('#project-board .column[data-state="done"]');
          if (!done || count("done") === 0) return false;
          if (count("todo") > 0 || count("doing") > 0) return false;
          // start, not center. The Done column is taller than the viewport
          // once it holds six cards, so centring it put the "Done" heading
          // above the fold and a take ended on an unlabelled column of cards:
          // the board was showing the payoff without saying what it was.
          // start pins the heading to the top with the cards under it.
          //
          // Only when it is actually out of view. On the full width layout it
          // is not, and scrolling a board that is already showing everything
          // is movement the viewer has to account for and learns nothing from.
          const box = done.getBoundingClientRect();
          if (box.top < 0 || box.top > window.innerHeight * 0.6) {
            done.scrollIntoView({ behavior: "smooth", block: "start" });
          }
          return true;
        }, ref);
        if (drained) {
          console.log("[browser] backlog empty, holding on Done");
          writeFileSync(DRAINED, "drained\n");
        }
      }
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
