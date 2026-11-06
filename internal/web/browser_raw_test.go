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
			if path != "/activity" {
				page.eval(t, `document.getElementById("history").open = true`, nil)
			}
			// The press waits on explicit signals, not the clock: the live
			// feed has drawn everything it was asked to (a refresh landing
			// between measuring and pressing moves the row, and the press
			// hits whatever slid under it), htmx has wired the control, and
			// the point about to be pressed is the control itself. Without
			// them a slow runner pressed empty space and waited for a dialog
			// that was never asked for.
			ready := `(() => { const b = ` + btn + `; if (!b || !window.tix.liveSettled() || !b["htmx-internal-data"]) return false;
				b.scrollIntoView({block: "center"});
				const r = b.getBoundingClientRect(), hit = document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2);
				return !!hit && hit.closest("a.rawopen") === b; })()`
			click := func() {
				page.waitFor(t, ready, "Raw never settled under the pointer, wired by htmx")
				x, y := page.point(t, `(() => { const r = `+btn+`.getBoundingClientRect(); return [r.left + r.width / 2, r.top + r.height / 2]; })()`)
				page.mouse("mouseMoved", x, y)
				page.mouse("mousePressed", x, y)
				page.mouse("mouseReleased", x, y)
			}
			page.eval(t, `window.__rawFetched = 0; document.addEventListener("htmx:afterRequest", () => { window.__rawFetched++; })`, nil)
			click()
			page.waitFor(t, `window.__rawFetched > 0 || `+dlg+`?.open`, "pressing Raw neither fetched its dialog nor opened one")
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
			click()
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
