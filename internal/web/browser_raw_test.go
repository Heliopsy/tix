// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import "testing"

// Raw on an activity row opened a <details> inside the row that the row's
// flex line sized to the word itself. In a browser it opens a wide modal
// dialog, Escape closes it and focus goes back to the control, and the long
// unbroken title inside wraps rather than widening anything.
func TestRawOpensAWideDialogInABrowser(t *testing.T) {
	b, ref := rawFixture(t)
	f := b.fix
	page := browserPage(t, f)
	page.call("Emulation.setDeviceMetricsOverride", map[string]any{
		"width": 1280, "height": 900, "deviceScaleFactor": 1, "mobile": false})
	for _, path := range []string{"/activity", "/tasks/" + ref} {
		t.Run(path, func(t *testing.T) {
			btn := `document.querySelector("a.rawopen")`
			dlg := `document.getElementById(` + btn + `.dataset.dlgOpen)`
			page.load(t, f.server.URL+path, btn)
			// The fixture serves no event stream, so live.js gives up on its
			// socket and says so in a notice. That notice once sat above the
			// feed and moved it down under a press already aimed at Raw; it is
			// a bar at the foot of the window now, which a press at the centre
			// of a control can still land on. Wait until it is up, as open()
			// does for the board, so press() measures what is really there.
			page.waitFor(t, `(() => { const s = document.getElementById("live-stale"); return !s || !s.hidden || !document.querySelector("[data-events]"); })()`,
				"the screen never said it was not live")
			if path != "/activity" {
				page.eval(t, `document.getElementById("history").open = true`, nil)
			}
			// What a press reached, recorded in the capture phase before any
			// handler can stop it, and the requests Raw itself made. Any other
			// htmx request, a live refresh say, is not Raw's.
			page.eval(t, `window.__rawHits = []; window.__rawFetched = 0;
				document.addEventListener("click", (e) => { window.__rawHits.push(e.target.closest("a.rawopen") === `+btn+` ? "raw" : (e.target.outerHTML || "").slice(0, 120)); }, true);
				document.addEventListener("htmx:beforeRequest", (e) => { if (e.detail.elt === `+btn+`) window.__rawFetched++; })`, nil)
			press(t, page, btn)
			page.waitFor(t, `window.__rawFetched > 0 || `+dlg+`?.open`, "Raw took the press but neither fetched its dialog nor opened one")
			page.waitFor(t, dlg+`?.matches(":modal")`, "Raw did not open its dialog as a modal one")
			var address string
			page.eval(t, `location.pathname + location.search`, &address)
			if address != path {
				t.Errorf("opening the dialog followed its link to %q", address)
			}
			var box struct{ Width, Height, Client, Scroll, View float64 }
			page.eval(t, `(() => { const d = `+dlg+`, r = d.getBoundingClientRect();
				return {Width: r.width, Height: r.height, Client: d.clientWidth, Scroll: d.scrollWidth, View: innerWidth}; })()`, &box)
			if box.Width < 800 {
				t.Errorf("the dialog is %.0fpx wide in a %.0fpx window, want at least 800", box.Width, box.View)
			}
			if box.Height > 900*0.81 {
				t.Errorf("the dialog is %.0fpx tall, more than 80%% of the window", box.Height)
			}
			if box.Scroll > box.Client+1 {
				t.Errorf("the dialog scrolls sideways: %.0fpx of content in %.0fpx", box.Scroll, box.Client)
			}
			page.escape()
			page.waitFor(t, `!`+dlg+`.open`, "Escape did not close the dialog")
			page.waitFor(t, `document.activeElement === `+btn, "focus did not go back to Raw")
			press(t, page, btn)
			page.waitFor(t, dlg+`?.matches(":modal")`, "Raw did not open its dialog a second time")
			var fetched int
			page.eval(t, `document.querySelectorAll(".rawwrap").length`, &fetched)
			if fetched != 1 {
				t.Errorf("the second press fetched the dialog again: %d copies", fetched)
			}
			page.eval(t, dlg+`.querySelector(".actiondlg-x").click()`, nil)
			page.waitFor(t, `!`+dlg+`.open`, "the close button did not close the dialog")
		})
	}
}

// press clicks the control btn names the way a reader would: a real mouse
// press at its centre, once it has stopped moving. The live feed has drawn
// everything it was asked to, htmx has wired the control, it is scrolled to
// the middle of the window and nothing covers its centre. elementFromPoint
// and Input.dispatchMouseEvent both work in CSS pixels of the viewport, so
// the point measured is the point pressed. The press must reach the control:
// one that lands on something else means the page moved between measuring
// and pressing, so it is aimed again, and the test fails only if it never
// lands.
func press(t *testing.T, page *cdpPage, btn string) {
	t.Helper()
	ready := `(() => { const b = ` + btn + `; if (!b || !window.tix.liveSettled() || !b["htmx-internal-data"]) return false;
		b.scrollIntoView({block: "center", behavior: "instant"});
		const r = b.getBoundingClientRect(), x = r.left + r.width / 2, y = r.top + r.height / 2;
		if (r.width === 0 || x < 0 || y < 0 || x > innerWidth || y > innerHeight) return false;
		const hit = document.elementFromPoint(x, y);
		return !!hit && hit.closest("a.rawopen") === b; })()`
	for attempt := 1; ; attempt++ {
		page.eval(t, `window.__rawHits.length = 0`, nil)
		page.waitFor(t, ready, "Raw never settled under the pointer, wired by htmx")
		x, y := page.point(t, `(() => { const r = `+btn+`.getBoundingClientRect(); return [r.left + r.width / 2, r.top + r.height / 2]; })()`)
		page.mouse("mouseMoved", x, y)
		page.mouse("mousePressed", x, y)
		page.mouse("mouseReleased", x, y)
		page.waitFor(t, `window.__rawHits.length > 0`, "pressing Raw delivered no click at all")
		var hits []string
		page.eval(t, `window.__rawHits`, &hits)
		if len(hits) > 0 && hits[0] == "raw" {
			return
		}
		if attempt == 3 {
			t.Fatalf("three presses aimed at Raw landed elsewhere: %q", hits)
		}
		t.Logf("a press aimed at Raw at (%.0f, %.0f) landed on %q; aiming again", x, y, hits)
	}
}
