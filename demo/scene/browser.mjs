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
// The end card. run.sh touches LANES_GO once it has given the browser the
// screen, and this answers with LANES_SHOWN only after the lanes it is about
// to film are actually painted, so the scene cannot hold on a page that never
// switched.
const LANES_GO = process.env.SCENE_LANES_GO || "/tmp/lanes.go";
const LANES_SHOWN = process.env.SCENE_LANES_SHOWN || "/tmp/lanes.shown";
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

// A chore that outlives the build, written into one of the starter lists a
// fresh installation opens with rather than into the agents' project.
//
// It is not in PROJECT_KEY on purpose. The agents claim `-p links -s todo`,
// and a recurring task there would be claimed, finished, and put straight
// back as the next occurrence: the backlog would never drain and the scene
// would never end. In its own list nobody claims it, and it gets a lane of its
// own on the end card with the repeat marker on it.
const CHORE_PROJECT = process.env.SCENE_CHORE_PROJECT || "work";
const CHORE_TITLE = "Prune short links nobody has opened in ninety days";
const CHORE_REPEAT = "every month on day 1";

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
  // on the task list, not on the board, and it leaves the person on the list
  // with a toast linking the new task, which is followed to write the body.
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
  // The ref is the toast's flashref, and the scene needs it to tell the
  // planner which task to break down. Waiting for the toast's link rather
  // than a load state: the form posts from a page that is already loaded.
  const toastLink = page.locator(".flash a.flashref").first();
  await toastLink.waitFor({ timeout: 15000 });
  const ref = (await toastLink.textContent()).trim();
  if (!ref) throw new Error("the quick-add toast names no task");
  writeFileSync(EPIC_FILE, `${ref}\n`);
  await sleep(1600);
  await Promise.all([
    page.waitForURL((u) => u.pathname === `/tasks/${ref}`, { timeout: 15000 }),
    toastLink.click(),
  ]);
  await page.waitForLoadState("domcontentloaded");
  await sleep(600);

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
      // Off the button for the same reason as after the star below: its
      // "Nothing has changed yet" title otherwise opens over the description.
      await page.mouse.move(1700, 30);
    }
    await sleep(2200);
  }

  // The person stars the epic. It is the one task here they will want to find
  // again, and a star is theirs alone: it lists the epic first on their own
  // screens and reorders nothing for the agents. It pays off on the end card,
  // where the starred epic heads its lane above the work it was broken into.
  //
  // A miss throws rather than shrugs. The button is looked up by the name it
  // carries for this ref, so a page that lost it fails the take here, not
  // twenty minutes later as an end card with no star on it.
  const star = page.locator(`button.star[aria-label="Star ${ref}"]`).first();
  if (!(await star.count())) throw new Error(`no star button for ${ref} on its own page`);
  await sleep(500);
  await star.click();
  await page.waitForLoadState("domcontentloaded");
  await page.locator(`button.star[aria-label="Unstar ${ref}"]`).first().waitFor({ timeout: 10000 });
  // The pointer Playwright clicked with is still resting on the star when the
  // page comes back, so the button's "Unstar" title opened as a tooltip, and
  // chromium under Xvfb drew it in the middle of the description. Moving off
  // to the empty strip beside the heading closes it.
  await page.mouse.move(1700, 30);
  await sleep(1400);

  // Then a chore with a repeat rule, through the fuller add form, because the
  // Repeat field lives there. See CHORE_PROJECT for why it is not in the
  // agents' project.
  await page.goto(`${BASE}/tasks`, { waitUntil: "domcontentloaded" });
  await sleep(700);
  const more = page.locator("details.addmore");
  await more.locator("summary").click();
  await sleep(500);
  const chore = more.locator("form");
  await chore.locator("input[name=title]").pressSequentially(CHORE_TITLE, { delay: TYPE });
  await sleep(300);
  // Not caught: a missing starter list would put the chore in whatever the
  // select defaults to, which could be the agents' own project.
  await chore.locator("select[name=project_ref]").selectOption(CHORE_PROJECT);
  await sleep(300);
  await chore.locator("input[name=repeat]").pressSequentially(CHORE_REPEAT, { delay: TYPE });
  await sleep(1400);
  // Waiting for the toast, not for a load state. The form posts from a page
  // that is already loaded, so waitForLoadState returned at once and the
  // check below read the old list: a paid take died there with the redirect
  // arriving fifteen milliseconds later.
  await chore.locator("button[type=submit]").click();
  await page.locator(".flash a.flashref").first().waitFor({ timeout: 15000 });
  // The rule has to be visible where the form lands: the new, ringed row.
  const repeated = await page.locator(".is-flashed .repeatmark").count();
  if (!repeated) throw new Error(`the chore landed on ${page.url()} without its repeat rule showing`);
  await sleep(1800);

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
  let lanes = false;

  for (;;) {
    await sleep(2000);
    // Outside the heartbeat's try on purpose: that one keeps the loop alive
    // through a page that is briefly unreachable, and a failed switch to the
    // end card is not that. It ends this process, run.sh sees the browser die,
    // and the take fails saying so.
    if (drained && !lanes && existsSync(LANES_GO)) {
      await showLanes(page, ref);
      lanes = true;
      console.log("[browser] lanes on screen");
      writeFileSync(LANES_SHOWN, "lanes\n");
    }
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
        const cols = [...document.querySelectorAll("#project-board h2, #project-board h3, .lanes .lane h2, .lanes .lane h3")]
          .map((h) => h.textContent.trim().replace(/\s+/g, " ")).join(" | ");
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

// The end card: the same tasks drawn one lane per project.
//
// The board says the work is done; the lanes say what the person is left
// with. The epic leads its lane because it is starred, the finished subtasks
// sit under it ticked, and the chore has a lane of its own with the repeat
// marker on it. Switched through the view control on camera, because the
// switch is the feature, not a url.
//
// Everything the card exists to show is checked before LANES_SHOWN is
// written. The lanes are a frame that scrolls its own contents, so the page is
// scrolled to put that frame at the top of the window rather than leaving the
// quick-add and filter rows taking the space.
async function showLanes(page, epic) {
  await page.goto(`${BASE}/tasks`, { waitUntil: "domcontentloaded" });
  await sleep(1200);
  await page.locator('form.viewswitch button[value="lanes"]').click();
  await page.waitForLoadState("domcontentloaded");
  await page.locator(`.lanes #lane-${PROJECT_KEY}`).waitFor({ timeout: 15000 });
  await sleep(600);
  await page.locator(".lanes").first().evaluate((el) =>
    el.scrollIntoView({ behavior: "smooth", block: "start" }));
  await sleep(900);
  const seen = await page.evaluate(({ project, chore, epic }) => ({
    lead: document.querySelector(`#lane-${project} .lane-card`)?.dataset.laneTask ?? "",
    starred: !!document.querySelector(`#lane-${project} #t-${epic} button.star[aria-pressed="true"]`),
    ticked: document.querySelectorAll(`#lane-${project} .lane-card.is-finished`).length,
    repeat: document.querySelectorAll(`#lane-${chore} .repeatmark`).length,
  }), { project: PROJECT_KEY, chore: CHORE_PROJECT, epic });
  console.log(`[browser] lanes lead=${seen.lead} starred=${seen.starred} ticked=${seen.ticked} repeat=${seen.repeat}`);
  if (seen.lead !== epic || !seen.starred) throw new Error(`the starred epic ${epic} does not lead its lane`);
  if (seen.ticked === 0) throw new Error("no finished card in the lane");
  if (seen.repeat === 0) throw new Error(`no repeat marker in the ${CHORE_PROJECT} lane`);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
