// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/web"
)

// Hover effects are on until the reader turns them off, and the off state is
// one attribute on <html>, read from the opening tag rather than the page:
// the stylesheet link and the accent block both sit in the head too.
func TestHoverEffectsAreAPreferenceOnTheRootElement(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	if tag := htmlTag(t, b.page("/tasks")); strings.Contains(tag, "data-hover") {
		t.Fatalf("an untouched browser has hover effects off: %s", tag)
	}

	off := b.post("/hover", url.Values{"next": {"/tasks"}})
	_ = off.Body.Close()
	wantStatus(t, off, http.StatusSeeOther)
	if b.cookie(web.HoverCookie) != "0" {
		t.Fatalf("turning hover effects off stored %q", b.cookie(web.HoverCookie))
	}
	for _, path := range []string{"/tasks", "/projects", "/settings"} {
		if tag := htmlTag(t, b.page(path)); !strings.Contains(tag, `data-hover="off"`) {
			t.Errorf("%s does not carry the off attribute: %s", path, tag)
		}
	}

	on := b.post("/hover", url.Values{"next": {"/tasks"}})
	_ = on.Body.Close()
	if tag := htmlTag(t, b.page("/tasks")); strings.Contains(tag, "data-hover") {
		t.Errorf("turning hover effects back on left the attribute: %s", tag)
	}

	b.setCookie(web.HoverCookie, "something else")
	if tag := htmlTag(t, b.page("/tasks")); strings.Contains(tag, "data-hover") {
		t.Errorf("a value the control never writes turned hover effects off: %s", tag)
	}
}

// Every on/off preference on settings is a switch that says which way it is
// set, read from its own form so one switch's state is not another's.
func TestSettingsDrawsOnOffPreferencesAsSwitches(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.setCookie(web.DragMoveCookie, "0")
	main := contentOf(t, b.page("/settings"))

	for _, c := range []struct {
		route string
		on    bool
	}{
		{"/hover", true},
		{"/dragmove", false},
		{"/openafteradd", false},
		{"/advanced", true},
	} {
		form := prefForm(t, main, c.route)
		want := `role="switch" aria-checked="false"`
		if c.on {
			want = `role="switch" aria-checked="true"`
		}
		if !strings.Contains(form, want) {
			t.Errorf("%s is not a switch reading %v:\n%s", c.route, c.on, form)
		}
	}
}

// sectionIDs are the ids of the settings screen's sections, in order.
var sectionIDs = regexp.MustCompile(`<section class="setgroup" id="([^"]+)">`)

// The section navigation links every section and nothing else, read from the
// navigation itself: the sections' own ids are in the page whatever it says.
func TestSettingsNavigationLinksEverySection(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	page := contentOf(t, f.as("alice").page("/settings"))
	nav := between(t, page, `<nav class="setnav"`, "</nav>")

	ids := sectionIDs.FindAllStringSubmatch(page, -1)
	if len(ids) < 6 {
		t.Fatalf("the settings screen has %d sections, want at least six:\n%s", len(ids), page)
	}
	for _, id := range ids {
		if !strings.Contains(nav, `<a href="#`+id[1]+`">`) {
			t.Errorf("the section navigation does not link %q:\n%s", id[1], nav)
		}
	}
	if got := strings.Count(nav, "<a "); got != len(ids) {
		t.Errorf("the navigation carries %d links for %d sections", got, len(ids))
	}
}

// A settings form returns to its own section with a toast naming what was
// saved, and the return address survives the redirect intact.
func TestSavingASettingReturnsToItsSectionAndSaysSo(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	main := contentOf(t, b.page("/settings"))

	const want = "/settings?flash=Drag+to+move+saved#appearance"
	// html/template writes "+" in an attribute as "&#43;", which a browser
	// reads back as "+".
	escaped := strings.ReplaceAll(want, "+", "&#43;")
	if form := prefForm(t, main, "/dragmove"); !strings.Contains(form, `name="next" value="`+escaped+`"`) {
		t.Fatalf("the drag form does not return to its section:\n%s", form)
	}
	if form := prefForm(t, main, "/taskview"); !strings.Contains(form, `name="next" value="/settings?flash=Task&#43;view&#43;saved#listings"`) {
		t.Errorf("the shared task view switch does not return to Listings on settings:\n%s", form)
	}
	if form := prefForm(t, b.page("/tasks"), "/taskview"); strings.Contains(form, "flash=") {
		t.Errorf("the task screen's own switch was pointed at settings:\n%s", form)
	}

	resp := b.post("/dragmove", url.Values{"next": {want}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	if got := resp.Header.Get("Location"); got != want {
		t.Fatalf("location = %q, want %q", got, want)
	}
	if !strings.Contains(b.page(want), `<p class="flash">Drag to move saved</p>`) {
		t.Error("returning from the save shows no message")
	}
}

// hoverSection is the stylesheet's hover layer, from its heading to the off
// block, which is where every rule the preference governs is written.
func hoverSection(t *testing.T, sheet string) string {
	t.Helper()
	layer := between(t, sheet, "Hover effects.\n", "/* Off.")
	return cssComment.ReplaceAllString(layer[strings.Index(layer, "*/")+2:], "")
}

// Every rule in the hover layer is gated on the attribute, and every
// transition in it waits for the reader's system to allow motion.
func TestHoverRulesAreGatedOnThePreferenceAndOnMotion(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	resp := f.as("alice").get("/assets/app.css")
	wantStatus(t, resp, http.StatusOK)
	sheet := body(t, resp)
	layer := hoverSection(t, sheet)

	for _, rule := range regexp.MustCompile(`(?m)^\s*([^{}/@\n][^{}]*)\{`).FindAllStringSubmatch(layer, -1) {
		for _, sel := range strings.Split(rule[1], ",") {
			sel = strings.TrimSpace(sel)
			if sel == ":root" || strings.HasPrefix(sel, ":root[data-theme") || strings.HasPrefix(sel, `:root:not([data-theme`) {
				continue
			}
			if !strings.HasPrefix(sel, `:root:not([data-hover="off"])`) {
				t.Errorf("hover rule %q is not gated on the preference", sel)
			}
		}
	}

	motion := "@media (hover: hover) and (prefers-reduced-motion: no-preference)"
	at := strings.Index(layer, motion)
	if at < 0 {
		t.Fatal("the hover layer has no reduced-motion gate")
	}
	if strings.Contains(layer[:at], "transition") || strings.Contains(layer[:at], "transform") {
		t.Error("the hover layer moves something outside the reduced-motion gate")
	}

	if off := declarations(t, sheet, `:root[data-hover="off"] tbody tr:hover`); !strings.Contains(off, "background: transparent") {
		t.Errorf("turning hover effects off leaves the row tint: %s", off)
	}
}

// A settings row stacks its control under its description at 900px and
// below, and the token table scrolls in its own frame rather than widening
// the page when a Revoke disclosure opens.
func TestSettingsRowsStackAndTheTokenTableScrollsInItsFrame(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	resp := b.get("/assets/app.css")
	sheet := body(t, resp)

	if rule := declarations(t, sheet, "@media (max-width: 900px)"); !strings.Contains(rule, ".setrow { grid-template-columns: minmax(0, 1fr); }") {
		t.Errorf("a settings row does not stack at 900px: %s", rule)
	}
	if rule := declarations(t, sheet, "*/\ntd > details.panel > summary"); !strings.Contains(rule, "overflow-wrap: normal") {
		t.Errorf("a row disclosure's label may still break between letters: %s", rule)
	}

	page := contentOf(t, b.page("/admin/tokens"))
	wrap := strings.Index(page, `<div class="table-wrap">`)
	table := strings.Index(page, "<table>")
	if wrap < 0 || table < 0 || strings.Contains(page[wrap:table], "</div>") {
		t.Errorf("the token table is not inside a scrolling frame:\n%s", page)
	}
}
