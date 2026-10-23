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

import { chromium } from "playwright-core";

const CDP = "http://127.0.0.1:9222";
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
  "Write a wordcount script that counts the words in a file",
  "Give wordcount a --json flag for machine readable output",
  "Handle an empty file without dividing by zero",
  "Write a README with two worked examples",
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

  await page.goto(`${BASE}/login`, { waitUntil: "domcontentloaded" });
  await sleep(800);

  // Signing in on camera rather than starting from a session. It is two
  // seconds, and it establishes that this is a real multi-user product and
  // not a local toy.
  await page.locator("input[name=email]").pressSequentially("nadia@demo.invalid", { delay: 55 });
  await sleep(250);
  await page.locator("input[name=password]").pressSequentially("tix-demo-password", { delay: 45 });
  await sleep(400);
  await page.locator("button[type=submit]").click();
  await page.waitForLoadState("domcontentloaded");
  await sleep(1200);

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

  // Land on the board, because the board is what the viewer watches repaint
  // while the agent works.
  await sleep(600);
  await page.goto(`${BASE}/projects/${PROJECT_KEY}`, { waitUntil: "domcontentloaded" });
  await sleep(1500);

  // Deliberately not browser.close(). Over CDP that closes the browser it
  // attached to, and in the first take it did: the board rendered, the socket
  // opened, and then every request stopped. The window kept showing its last
  // paint while the agent worked, so the recording showed a board that never
  // moved, which is the opposite of the thing being demonstrated.
  //
  // Letting the process exit drops the CDP socket and leaves chromium running
  // with its page live, which is what the rest of the scene is filming.
  process.exit(0);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
