// SPDX-License-Identifier: AGPL-3.0-or-later

// Copy diagnostics: the report assets/diagnostics.js builds.
import test from "node:test";
import assert from "node:assert/strict";
import { newContext, run } from "./harness.mjs";

const ctx = newContext();
run(ctx, "diagnostics.js");
const { tix } = ctx;

const facts = {
  version: "0.58.0", latest: "0.59.0", userAgent: "Firefox/140", language: "bg", languages: "bg, en",
  zone: "Europe/Sofia", screen: "1920x1080", theme: "dark", motion: true, online: true, live: "on",
  server: "https://tix.internal.example"
};

test("the report names the versions, the browser and the connection", () => {
  const text = tix.diagnosticsText(facts, false);
  for (const want of ["tix 0.58.0", "Newest published: 0.59.0", "Browser: Firefox/140", "Language: bg (browser asks for bg, en)",
    "Time zone: Europe/Sofia", "Theme: dark, reduce motion", "Connection: online, live updates connected"]) {
    assert.ok(text.includes(want), `the report lacks ${want}:\n${text}`);
  }
});

test("the server address is left out unless asked for", () => {
  assert.ok(!tix.diagnosticsText(facts, false).includes("tix.internal.example"));
  assert.ok(tix.diagnosticsText(facts, true).includes("Server address: https://tix.internal.example"));
});

test("an unknown state says so rather than guessing", () => {
  const text = tix.diagnosticsText({ version: "0.58.0", online: false }, true);
  assert.ok(text.includes("Newest published: not known"));
  assert.ok(text.includes("Connection: offline, live updates not started"));
  assert.ok(!text.includes("Server address"));
});
