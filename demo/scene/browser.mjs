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

const PROJECT = process.env.SCENE_PROJECT || "web";
const COUNT = Number(process.env.SCENE_TASKS || 4);

// The backlog a person would write before handing it to an agent: small,
// concrete, and obviously finishable, so the recording shows work being
// cleared rather than an agent reading an epic.
const PLAN = [
  "Add a --json flag to the status command",
  "Write the quickstart section of the README",
  "Cache the project lookup in the board handler",
  "Add a test for the empty backlog case",
  "Tidy the error message when a ref is unknown",
  "Document the exit codes in one table",
];

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function main() {
  const browser = await chromium.connectOverCDP(CDP);
  const context = browser.contexts()[0];
  const page = context.pages()[0] ?? (await context.newPage());

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

  await page.goto(`${BASE}/tasks`, { waitUntil: "domcontentloaded" });
  await sleep(1000);

  const form = page.locator("form.quickadd");
  await form.locator("select[name=project_ref]").selectOption(PROJECT).catch(() => {});
  await sleep(400);

  for (const title of PLAN.slice(0, COUNT)) {
    const field = form.locator("input[name=title]");
    await field.click();
    await field.pressSequentially(title, { delay: 38 });
    await sleep(350);
    await field.press("Enter");
    // The list is boosted, so the new row arrives without a navigation. Waiting
    // for the text rather than for a load event is what makes that visible and
    // is also the only reliable signal here.
    await page.getByText(title, { exact: false }).first().waitFor({ timeout: 15000 });
    await sleep(500);
  }

  // Land on the board, because the board is what the viewer watches repaint
  // while the agent works.
  await sleep(600);
  await page.goto(`${BASE}/projects/${PROJECT}`, { waitUntil: "domcontentloaded" });
  await sleep(1500);

  await browser.close();
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
