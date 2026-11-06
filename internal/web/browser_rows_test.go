// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/testenv"
	"github.com/heliopsy/tix/internal/web"
)

// Both of these were visible only in a browser. Manage on the user listing
// left a ring round its row that nothing took away, because the button's
// dialog shared its id with the row, the click fell through to the link and
// the fragment it left behind marked the row for good. And the rows of the
// tables and lists hovered seven different ways: two greys, an accent bar on
// the first cell, a border and a shadow.

// browserPage is a page in a headless Chrome sent as alice, or a skip.
func browserPage(t *testing.T, f *fixture) *cdpPage {
	t.Helper()
	chrome := findChrome()
	if chrome == "" {
		if os.Getenv("GITHUB_ACTIONS") == "true" {
			t.Fatalf("no Chrome on this CI runner: %s", chromeCapability.Why)
		}
		testenv.Skip(t, chromeCapability)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	page := launchChrome(ctx, t, chrome)
	page.call("Network.enable", nil)
	page.call("Page.enable", nil)
	page.call("Network.setExtraHTTPHeaders", map[string]any{"headers": map[string]string{actorHeader: "alice"}})
	return page
}

// load navigates and waits until the screen's scripts have run.
func (p *cdpPage) load(t *testing.T, address, ready string) {
	t.Helper()
	p.call("Page.navigate", map[string]any{"url": address})
	p.waitFor(t, `document.readyState === "complete" && window.tixForms && `+ready, "the screen never loaded at "+address)
}

func (p *cdpPage) escape() {
	for _, kind := range []string{"keyDown", "keyUp"} {
		p.call("Input.dispatchKeyEvent", map[string]any{"type": kind, "key": "Escape", "code": "Escape", "windowsVirtualKeyCode": 27})
	}
}

// rowLook is what a row shows: its background, outline and shadow, and its
// first cell's shadow, where a table row's accent bar used to sit.
const rowLook = `((el) => { const s = getComputedStyle(el), c = el.firstElementChild ? getComputedStyle(el.firstElementChild) : s;
	return [s.backgroundColor, s.outlineStyle, s.boxShadow, s.borderTopColor, c.boxShadow].join(" | "); })`

func TestManageLeavesNoMarkOnItsRowInABrowser(t *testing.T) {
	f := newFixture(t)
	b := f.as("alice")
	created := b.post("/admin/users", url.Values{"email": {"marked@example.test"},
		"password": {"correct-horse-battery"}, "role": {"member"}})
	_ = created.Body.Close()
	wantStatus(t, created, http.StatusSeeOther)
	id := userIDFor(t, b.page("/admin/users"), "marked@example.test")
	row := `document.getElementById("user-` + id + `")`
	page := browserPage(t, f)
	base := f.server.URL + "/admin/users"

	btn := row + `.querySelector("[data-dlg-open]")`
	dlg := `document.querySelector("dialog[id='" + ` + btn + `.dataset.dlgOpen + "']")`

	unmarked := func(t *testing.T, how, keep string) {
		t.Helper()
		page.mouse("mouseMoved", 1, 1)
		var address string
		page.eval(t, `location.search + location.hash`, &address)
		if address != keep {
			t.Errorf("%s: the address is %q, want %q", how, address, keep)
		}
		page.waitFor(t, `!`+row+`.classList.contains("is-target") && (`+rowLook+`(`+row+`)).startsWith("rgba(0, 0, 0, 0) | none")`,
			how+": the row is still marked: "+page.look(t, row))
	}

	t.Run("the button opens its dialog without following its link", func(t *testing.T) {
		page.load(t, base, dlg)
		x, y := page.point(t, `(() => { const r = `+btn+`.getBoundingClientRect(); return [r.left + r.width / 2, r.top + r.height / 2]; })()`)
		page.mouse("mouseMoved", x, y)
		page.mouse("mousePressed", x, y)
		page.mouse("mouseReleased", x, y)
		page.waitFor(t, dlg+`?.open`, "Manage did not open its dialog")
		var address string
		page.eval(t, `location.search + location.hash`, &address)
		if address != "" {
			t.Errorf("opening the dialog followed its link to %q", address)
		}
		x, y = page.point(t, `(() => { const r = `+dlg+`.querySelector(".actiondlg-x").getBoundingClientRect(); return [r.left + r.width / 2, r.top + r.height / 2]; })()`)
		page.mouse("mousePressed", x, y)
		page.mouse("mouseReleased", x, y)
		page.waitFor(t, `!`+dlg+`.open`, "the close button did not close the dialog")
		unmarked(t, "after closing with the mouse", "")
	})

	t.Run("a dialog the address opened is forgotten when it closes", func(t *testing.T) {
		page.load(t, base, dlg)
		var href string
		page.eval(t, btn+`.href`, &href)
		page.load(t, href, dlg+`.open`)
		page.escape()
		page.waitFor(t, `!`+dlg+`.open`, "Escape did not close the dialog")
		page.eval(t, `document.activeElement.blur()`, nil)
		unmarked(t, "after closing a linked dialog", "")
	})

	t.Run("a deep link to the row flashes and lets go", func(t *testing.T) {
		page.load(t, base+"#user-"+id, row)
		page.waitFor(t, `getComputedStyle(`+row+`).animationName === "row-flash"`, "the row a link landed on did not flash")
		unmarked(t, "after the flash", "#user-"+id)
	})
}

// Every row, in a table or a list drawn as one, takes the same tint under the
// pointer and changes nothing else: no bar, no border, no shadow, no ring.
func TestEveryRowHoversTheSameWayInABrowser(t *testing.T) {
	f := newFixture(t)
	created := f.as("alice").post("/admin/users", url.Values{"email": {"hovered@example.test"},
		"password": {"correct-horse-battery"}, "role": {"member"}})
	_ = created.Body.Close()
	wantStatus(t, created, http.StatusSeeOther)
	f.as("alice").createTask("infra", "Hovered in the list")
	page := browserPage(t, f)
	page.call("Network.setCookie", map[string]any{"name": web.TaskViewCookie, "value": web.TaskViewList, "url": f.server.URL})
	var hover bool
	page.load(t, f.server.URL+"/admin/users", `document.querySelector(".peoplelist li.person")`)
	page.eval(t, `matchMedia("(hover: hover)").matches`, &hover)
	if !hover {
		t.Fatal("this Chrome answers hover: none, so no hover effect applies and this test reads nothing")
	}
	tints := map[string]string{}
	for _, tc := range []struct{ path, row string }{
		{"/admin/users", "ul.peoplelist > li.person"},
		{"/admin/tenant", "table > tbody > tr"},
		{"/tasks", "ul.tasklist > li.task"},
		{"/projects", "ul.projlist > li.projrow"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			el := fmt.Sprintf(`document.querySelector(%q)`, tc.row)
			page.load(t, f.server.URL+tc.path, el)
			page.eval(t, `document.head.insertAdjacentHTML("beforeend", "<style>*{transition:none!important}</style>")`, nil)
			page.mouse("mouseMoved", 1, 1)
			page.waitFor(t, `!`+el+`.getAnimations({subtree: true}).some((a) => a.playState === "running")`, "the row never settled")
			var rest, hovered string
			page.eval(t, rowLook+`(`+el+`)`, &rest)
			x, y := page.point(t, `(() => { `+el+`.scrollIntoView({block: "center"});
				const r = `+el+`.getBoundingClientRect(); return [Math.min(r.right, innerWidth) - 8, r.top + r.height / 2]; })()`)
			page.mouse("mouseMoved", x, y)
			page.waitFor(t, el+`.matches(":hover")`, tc.row+" is not under the pointer: "+page.look(t, el))
			page.eval(t, rowLook+`(`+el+`)`, &hovered)
			if strip(rest) != strip(hovered) {
				t.Errorf("%s changes more than its background under the pointer:\n rest  %s\n hover %s", tc.row, rest, hovered)
			}
			tints[tc.path] = hovered[:len(hovered)-len(strip(hovered))]
			if tints[tc.path] == rest[:len(rest)-len(strip(rest))] {
				t.Errorf("%s does not tint under the pointer: %s", tc.row, hovered)
			}
		})
	}
	if tints["/admin/users"] == "" || len(tints) != 4 {
		t.Fatalf("read %d rows, so this test compares nothing", len(tints))
	}
	for path, tint := range tints {
		if tint != tints["/admin/users"] {
			t.Errorf("rows hover differently: %s tints %s, the user listing %s", path, tint, tints["/admin/users"])
		}
	}
}

// strip drops the background from a rowLook, leaving what must not change.
func strip(look string) string {
	for i := 0; i+3 <= len(look); i++ {
		if look[i:i+3] == " | " {
			return look[i:]
		}
	}
	return look
}

// look describes an element for a failure message.
func (p *cdpPage) look(t *testing.T, el string) string {
	t.Helper()
	var out string
	p.eval(t, `(`+el+`).outerHTML.slice(0, 200) + " :: " + (`+rowLook+`(`+el+`)) + " hover=" + (`+el+`).matches(":hover")`, &out)
	return out
}
