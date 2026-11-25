// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"fmt"
	"testing"
)

// The settings navigation in Chrome: the link for the section in view carries
// aria-current as the page scrolls, the last one at the end of the page, and
// a click jumps without animating under reduced motion, sets the fragment and
// moves focus to the section.
func TestSettingsNavigationFollowsTheScrollInABrowser(t *testing.T) {
	chromeOrSkip(t)
	f := newFixture(t)
	page := browserPage(t, f)
	page.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1440, "height": 800, "deviceScaleFactor": 1, "mobile": false})
	page.load(t, f.server.URL+"/settings", `document.querySelector("nav.setnav[data-toc-live]")`)

	current := func(id string) {
		t.Helper()
		page.waitFor(t, fmt.Sprintf(`(() => { const on = [...document.querySelectorAll('nav.setnav a[aria-current="true"]')];
			return on.length === 1 && on[0].getAttribute("href") === "#%s"; })()`, id),
			"the navigation does not mark #"+id+" alone")
	}

	current("profile")
	page.eval(t, `document.getElementById("notifications").scrollIntoView({block: "start"})`, nil)
	current("notifications")
	page.eval(t, `document.getElementById("dates").scrollIntoView({block: "start"})`, nil)
	current("dates")

	var last string
	page.eval(t, `[...document.querySelectorAll("nav.setnav a")].filter(a => a.getClientRects().length).pop().getAttribute("href").slice(1)`, &last)
	page.eval(t, `scrollTo(0, document.scrollingElement.scrollHeight)`, nil)
	current(last)

	page.call("Emulation.setEmulatedMedia", map[string]any{"features": []map[string]string{{"name": "prefers-reduced-motion", "value": "reduce"}}})
	page.eval(t, `scrollTo(0, 0)`, nil)
	current("profile")
	var jump struct {
		Hash    string  `json:"hash"`
		Top     float64 `json:"top"`
		Focused bool    `json:"focused"`
	}
	page.eval(t, `(() => { document.querySelector('nav.setnav a[href="#inbox"]').click();
		const s = document.getElementById("inbox");
		return {hash: location.hash, top: s.getBoundingClientRect().top, focused: document.activeElement === s}; })()`, &jump)
	if jump.Hash != "#inbox" {
		t.Errorf("after the click the address ends in %q, want #inbox", jump.Hash)
	}
	if jump.Top < 0 || jump.Top > 40 {
		t.Errorf("under reduced motion the Inbox section is %vpx from the top straight after the click, want it jumped to", jump.Top)
	}
	if !jump.Focused {
		t.Errorf("focus did not move to the Inbox section")
	}
	current("inbox")
}
