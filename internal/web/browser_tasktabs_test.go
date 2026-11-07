// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"fmt"
	"net/url"
	"testing"
	"time"
)

// The activity tabs in Chrome: a click and the arrow keys switch them, the
// choice outlives a reload, a fragment picks the tab it points into, and a
// live update redraws the counts without moving the reader off their tab.
func TestTaskTabsSwitchRememberAndSurviveALiveUpdateInABrowser(t *testing.T) {
	chromeOrSkip(t)
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "tabbed")
	page := browserPage(t, f)
	address := f.server.URL + "/tasks/" + ref

	shown := func(want string) {
		t.Helper()
		page.waitFor(t, fmt.Sprintf(`(() => { const box = document.querySelector("[data-task-tabs]");
			const panels = [...box.querySelectorAll(":scope > [data-tab-panel]")];
			const tab = document.getElementById("tab-%[1]s");
			return box.dataset.tab === %[1]q && tab.getAttribute("aria-selected") === "true" && tab.tabIndex === 0 &&
				panels.every(p => (p.dataset.tabPanel === %[1]q) === (!p.hidden && p.offsetHeight > 0)) &&
				document.querySelectorAll('[data-task-tabs] [role="tab"][aria-selected="true"]').length === 1; })()`, want),
			"the "+want+" tab is not the one shown")
	}
	key := func(name string, code int) {
		for _, kind := range []string{"keyDown", "keyUp"} {
			page.call("Input.dispatchKeyEvent", map[string]any{"type": kind, "key": name, "code": name, "windowsVirtualKeyCode": code})
		}
	}

	page.openLive(t, address, "[data-task-tabs]")
	page.eval(t, `localStorage.clear()`, nil)
	page.openLive(t, address+"?", "[data-task-tabs]")
	shown("comments")
	var strip bool
	page.eval(t, `getComputedStyle(document.querySelector(".ttabs-list")).display !== "none"`, &strip)
	if !strip {
		t.Fatalf("the tab strip is not drawn with scripting on")
	}

	page.eval(t, `document.getElementById("tab-comments").focus()`, nil)
	key("ArrowRight", 39)
	shown("history")
	page.waitFor(t, `document.activeElement === document.getElementById("tab-history")`, "ArrowRight did not move focus to History")
	key("ArrowLeft", 37)
	shown("comments")
	key("End", 35)
	shown("artifacts")
	key("ArrowRight", 39)
	shown("comments")
	page.eval(t, `document.getElementById("tab-history").click()`, nil)
	shown("history")

	page.openLive(t, address, "[data-task-tabs]")
	shown("history")

	resp := b.post("/tasks/"+ref+"/comments", url.Values{"body": {"arrives live"}})
	_ = resp.Body.Close()
	page.eval(t, `document.dispatchEvent(new CustomEvent("tix:refresh"))`, nil)
	page.waitUntil(t, `document.getElementById("task-count-comments").textContent === "1" && window.tix.liveSettled()`,
		"the comment count never came in with the live update", 30*time.Second)
	shown("history")

	for _, tc := range []struct{ hash, want string }{{"#artifacts", "artifacts"}, {"#comments", "comments"}, {"#history", "history"}} {
		page.openLive(t, address+tc.hash, "[data-task-tabs]")
		shown(tc.want)
	}
	page.eval(t, `location.hash = "#artifacts"`, nil)
	shown("artifacts")
}
