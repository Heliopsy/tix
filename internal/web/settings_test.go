// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/web"
)

// The display preferences used to live in a sidebar disclosure repeated on
// every page; they now live on one dedicated settings page, and the sidebar
// carries a plain link to it instead. Keeping both a menu and a page holding
// the same controls is how they drift, which is why this test replaces
// TestSidebarOffersOneSettingsMenu rather than sitting beside it.
func TestSidebarLinksToTheSettingsPage(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	for _, path := range []string{"/projects", "/tasks", "/activity"} {
		page := b.page(path)
		if !strings.Contains(page, `<a href="/settings"`) || !strings.Contains(page, `>Settings</span></a>`) {
			t.Errorf("%s carries no link to the settings page:\n%s", path, page)
		}
		if strings.Contains(page, `<details class="panel settings">`) {
			t.Errorf("%s still carries the old settings disclosure alongside the link:\n%s", path, page)
		}
		if hasPlainForm(sidebarOf(t, page), "/advanced") {
			t.Errorf("%s still carries a preference form outside the settings page:\n%s", path, page)
		}
	}
}

// The colour scheme is the one preference that did not move onto the settings
// page, and later moved back off it: it is a switch at the foot of the
// sidebar, present on every signed-in screen, because it is the preference a
// reader changes while looking at the screen that made them want to.
//
// Read out of the sidebar rather than out of the page, which is the trap this
// kind of assertion keeps falling into: the sidebar renders inside the
// settings page too, so a whole-document search cannot tell the switch from
// the form it replaced.
func TestTheSidebarCarriesTheColourSchemeSwitch(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	for _, path := range []string{"/projects", "/tasks", "/activity", "/settings"} {
		side := sidebarOf(t, b.page(path))
		if !hasPlainForm(side, "/theme") {
			t.Errorf("%s has no colour scheme switch in its sidebar:\n%s", path, side)
		}
		switcher := themeSwitch(t, side)
		if !strings.Contains(switcher, `type="submit"`) {
			t.Errorf("%s's switch has no button, so nothing submits it without a script:\n%s", path, switcher)
		}
		if !strings.Contains(switcher, `name="csrf_token"`) {
			t.Errorf("%s's switch carries no CSRF token:\n%s", path, switcher)
		}
		if !strings.Contains(switcher, `hx-boost="false"`) {
			t.Errorf("%s's switch is boosted, so the swap would leave the old scheme on <html>:\n%s",
				path, switcher)
		}
	}
}

// The switch says what the scheme is and what pressing it does, in its
// accessible name rather than in a title attribute, which never reaches a
// keyboard user who does not hover. One press moves to the next scheme and
// four presses come back to where they started.
func TestTheColourSchemeSwitchCyclesAndNamesBothHalves(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	// System, then each press, back to System.
	want := []struct{ scheme, label, next string }{
		{"", "System", "Light"},
		{"light", "Light", "Dark"},
		{"dark", "Dark", "Low contrast"},
		{"dim", "Low contrast", "System"},
		{"", "System", "Light"},
	}
	for i, step := range want {
		page := b.page("/projects")
		switcher := themeSwitch(t, sidebarOf(t, page))

		if got := htmlTag(t, page); step.scheme == "" {
			if strings.Contains(got, "data-theme=") {
				t.Fatalf("press %d: the system scheme pinned a theme: %s", i, got)
			}
		} else if !strings.Contains(got, `data-theme="`+step.scheme+`"`) {
			t.Fatalf("press %d: the html element does not carry the %q scheme: %s", i, step.scheme, got)
		}

		name := `aria-label="Colour scheme: ` + step.label + `. Switch to ` + step.next + `."`
		if !strings.Contains(switcher, name) {
			t.Fatalf("press %d: the switch is not named %s:\n%s", i, name, switcher)
		}
		if !strings.Contains(switcher, `<input type="hidden" name="theme" value="`+themeValue(step.next)+`">`) {
			t.Fatalf("press %d: the switch does not post the next scheme:\n%s", i, switcher)
		}
		if !strings.Contains(switcher, ">"+step.label+"</span>") {
			t.Fatalf("press %d: the switch does not show the scheme in force:\n%s", i, switcher)
		}
		if i == len(want)-1 {
			break
		}

		apply(t, b, "/theme", url.Values{"theme": {themeValue(step.next)}, "next": {"/projects"}})
		if got := b.cookie(web.ThemeCookie); got != themeValue(step.next) {
			t.Fatalf("press %d: the cookie is %q, want %q", i, got, themeValue(step.next))
		}
	}
}

// themeValue is the cookie value behind a scheme's label, so the cycle test
// names schemes the way the reader sees them.
func themeValue(label string) string {
	for _, theme := range web.Themes {
		if web.ThemeLabel(theme) == label {
			return theme
		}
	}
	return ""
}

// themeSwitch returns the colour scheme switch out of a sidebar, from its
// action through its close. formAt cannot read it: that helper matches a
// form tag ending at the action, and this one carries hx-boost and a class
// after it.
func themeSwitch(t *testing.T, side string) string {
	t.Helper()
	form := between(t, side, `action="/theme"`, "</form>")
	if form == "" {
		t.Fatalf("the sidebar has no colour scheme switch:\n%s", side)
	}
	return form
}

// sidebarOf returns the sidebar of a rendered page. The sidebar renders on
// the settings page as well, so an assertion about where a control lives has
// to read one region or the other and never the document.
func sidebarOf(t *testing.T, page string) string {
	t.Helper()
	side := between(t, page, `<aside class="side" id="side">`, "</aside>")
	if side == "" {
		t.Fatalf("the page renders no sidebar:\n%s", page)
	}
	return side
}

// contentOf returns the main column of a rendered page, for the same reason.
func contentOf(t *testing.T, page string) string {
	t.Helper()
	main := between(t, page, `<main class="wrap">`, "</main>")
	if main == "" {
		t.Fatalf("the page renders no main column:\n%s", page)
	}
	return main
}

// The settings page gathers the advanced toggle, replacing what
// TestSidebarOffersOneSettingsMenu used to assert about the sidebar
// disclosure: the controls moved, not disappeared.
//
// The colour scheme is the exception and is asserted by
// TestTheSidebarCarriesTheColourSchemeSwitch. Settings names the scheme in
// force and points at the switch, so there is one control over one cookie
// and no second form here to drift from it.
func TestSettingsPageGathersTheAdvancedToggleAndNamesTheScheme(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	page := b.page("/settings")
	main := contentOf(t, page)

	if !strings.Contains(main, "<h1>Settings</h1>") {
		t.Errorf("the settings page carries no heading:\n%s", main)
	}
	if !hasPlainForm(main, "/advanced") {
		t.Errorf("the settings page does not gather the advanced toggle:\n%s", main)
	}
	if hasPlainForm(main, "/theme") {
		t.Errorf("the settings page carries a second colour scheme control beside the sidebar switch:\n%s", main)
	}
	if !strings.Contains(main, "<strong>System</strong>") {
		t.Errorf("the settings page does not name the scheme in force:\n%s", main)
	}
}

// Every control on the settings page is a real form, so it works with
// scripting turned off, exactly as the sidebar disclosure it replaced did.
func TestSettingsPageNeedsNoScripting(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	page := b.page("/settings")

	if strings.Contains(page, "hx-post") || strings.Contains(page, "hx-get") {
		t.Fatal("a settings control depends on scripting")
	}
	if strings.Contains(page, "onchange=") {
		t.Fatal("the theme control submits itself with a script rather than with a button")
	}

	resp := b.postRaw("/theme", url.Values{"csrf_token": {b.csrf()},
		"theme": {"dim"}, "next": {"/settings"}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)
	if got := resp.Header.Get("Location"); got != "/settings" {
		t.Errorf("location = %q, want the page the choice was made on", got)
	}
	if !strings.Contains(b.page("/settings"), `data-theme="dim"`) {
		t.Error("the low contrast scheme did not take effect")
	}
}

// All four schemes have to render the settings page, including the one an
// untouched browser gets and the low contrast one.
func TestSettingsPageRendersInEveryScheme(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	for _, theme := range web.Themes {
		name := theme
		if name == "" {
			name = "system"
		}
		t.Run(name, func(t *testing.T) {
			b := f.as("alice")
			resp := b.post("/theme", url.Values{"theme": {theme}, "next": {"/settings"}})
			_ = resp.Body.Close()

			page := b.page("/settings")
			// Checked on the <html> element rather than anywhere in the
			// document: the tenant's accent is emitted as a style block whose
			// selectors legitimately name data-theme, and a substring search
			// over the whole page cannot tell a selector from an attribute.
			html := htmlTag(t, page)
			if theme == "" {
				if strings.Contains(html, "data-theme=") {
					t.Errorf("the system scheme pinned a theme: %s", html)
				}
			} else if !strings.Contains(html, `data-theme="`+theme+`"`) {
				t.Errorf("the html element does not carry the %q scheme: %s", theme, html)
			}
			if !strings.Contains(page, "<h1>Settings</h1>") {
				t.Errorf("the settings page is missing under the %q scheme", name)
			}
			switcher := themeSwitch(t, sidebarOf(t, page))
			if !strings.Contains(switcher, ">"+web.ThemeLabel(theme)+"</span>") {
				t.Errorf("the page does not show %q as the current scheme:\n%s", name, switcher)
			}
		})
	}
}

// The advanced toggle moved onto the settings page, and still does what it
// did in both directions. What changed is where it starts: the default now
// follows the reader rather than being off for everybody, because an
// administrator could not otherwise find the screens at all. See
// TestAnAdministratorFindsTheConfigurationScreensWithoutBeingTold.
func TestSettingsPageStillTogglesTheAdvancedScreens(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	member := f.as("viewer")
	if strings.Contains(member.page("/tasks"), `href="/admin/tokens"`) {
		t.Fatal("the advanced screens are on for somebody every one of them refuses")
	}
	resp := member.post("/advanced", url.Values{"next": {"/tasks"}})
	_ = resp.Body.Close()
	if !strings.Contains(member.page("/tasks"), `href="/admin/tokens"`) {
		t.Fatal("showing the advanced screens from the settings page had no effect")
	}

	admin := f.as("alice")
	if !strings.Contains(admin.page("/tasks"), `href="/admin/tokens"`) {
		t.Fatal("an administrator has to go looking for the screens that are theirs")
	}
	resp = admin.post("/advanced", url.Values{"next": {"/tasks"}})
	_ = resp.Body.Close()
	if strings.Contains(admin.page("/tasks"), `href="/admin/tokens"`) {
		t.Fatal("hiding the advanced screens from the settings page had no effect")
	}
}

// The settings page also gathers the drag-and-drop toggle and the keyboard
// scheme picker, which used to sit in the same sidebar disclosure as theme
// and advanced.
func TestSettingsPageGathersDragMoveAndKeyScheme(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	page := b.page("/settings")

	if !hasPlainForm(page, "/dragmove") {
		t.Errorf("the settings page does not offer the drag-and-drop toggle:\n%s", page)
	}
	if !hasPlainForm(page, "/keyscheme") {
		t.Errorf("the settings page does not offer the keyboard scheme control:\n%s", page)
	}
}

// The toggle has to reach the board, and its polarity is the whole point:
// dragging defaults on, and only the one value the control writes turns it
// off, so a cookie left behind by something else leaves the board alone.
func TestTheDragToggleDecidesWhatTheBoardOffers(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	if !strings.Contains(b.page("/projects/infra"), `data-drag="1"`) {
		t.Fatalf("an untouched browser is not offered dragging")
	}

	off := b.post("/dragmove", url.Values{"drag_move": {"0"}, "next": {"/projects/infra"}})
	_ = off.Body.Close()
	wantStatus(t, off, http.StatusSeeOther)
	if !strings.Contains(b.page("/projects/infra"), `data-drag="0"`) {
		t.Errorf("turning dragging off did not reach the board")
	}

	b.setCookie(web.DragMoveCookie, "something else")
	if !strings.Contains(b.page("/projects/infra"), `data-drag="1"`) {
		t.Errorf("a value the control never writes turned dragging off")
	}
}

// The keyboard shortcuts reference is rendered from shortcuts.go's own action
// and binding tables, never a hand-written list, so it cannot list a key the
// scripted help overlay does not also know about.
func TestSettingsPageComparesEveryScheme(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	page := b.page("/settings")

	for _, scheme := range web.KeySchemes() {
		table := web.KeyBindingsFor(scheme)
		for _, action := range web.ShortcutActionNames() {
			for _, key := range table[action] {
				// html/template's text-node escaper renders "+" as "&#43;",
				// which every emacs ctrl chord carries.
				escaped := strings.ReplaceAll(key, "+", "&#43;")
				if !strings.Contains(page, "<kbd>"+escaped+"</kbd>") {
					t.Errorf("the settings page does not show %q's %q binding %q:\n%s",
						scheme, action, key, page)
				}
			}
		}
	}
}

// A DSN can carry a password, so the settings page never shows the raw
// server-configured hint: only what a caller explicitly marks safe through
// WithTargetDescribe. Without one, the page says so rather than going blank
// or falling back to something unredacted.
func TestSettingsPageShowsTheConfiguredDatabaseDescription(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	withHint := web.Handler(f.svc, web.WithTargetDescribe("local /tmp/claude-1000/example.db (from flag)"))
	page := f.settingsPage(t, withHint)
	if !strings.Contains(page, "local /tmp/claude-1000/example.db (from flag)") {
		t.Errorf("the settings page does not show the configured database description:\n%s", page)
	}

	without := web.Handler(f.svc)
	bare := f.settingsPage(t, without)
	// Read from the Database heading to the end of the page rather than over
	// the whole document: the preference sections above it legitimately set a
	// project key and a filter term in <code>, and a search over the document
	// cannot tell those from a target. What this is about is whether the
	// Database section names one.
	if section := databaseSection(t, bare); strings.Contains(section, "<code>") {
		t.Errorf("the settings page shows a database target with nothing configured:\n%s", section)
	}
}

// databaseSection is the markup from the Database heading to the end of the
// page, which is where a configured target is named and the only place one
// may appear.
func databaseSection(t *testing.T, page string) string {
	t.Helper()
	i := strings.Index(page, "<h2>Database</h2>")
	if i < 0 {
		t.Fatalf("the settings page has no Database section:\n%s", page)
	}
	return page[i:]
}

// settingsPage renders the settings page through a handler built with custom
// options, signed in as alice the same way the rest of this package's tests
// are, so a test can check what one specific Option changes without standing
// up a whole second fixture.
func (f *fixture) settingsPage(t *testing.T, handler http.Handler) string {
	t.Helper()
	server := httptest.NewServer(f.authenticate(handler))
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/settings", nil)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	req.Header.Set(actorHeader, "alice")
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("getting /settings: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /settings = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	return string(body)
}

// htmlTag returns the opening <html ...> tag, which is where a pinned colour
// scheme lives.
func htmlTag(t *testing.T, page string) string {
	t.Helper()
	i := strings.Index(page, "<html")
	if i < 0 {
		t.Fatalf("the page has no <html> element:\n%s", page)
	}
	j := strings.Index(page[i:], ">")
	if j < 0 {
		t.Fatalf("the <html> element is not closed:\n%s", page)
	}
	return page[i : i+j+1]
}

// TestATenantAccentReachesEveryScheme is the test that was missing when
// theming shipped.
//
// The accent was emitted as a plain `:root` block, and app.css writes its
// scheme rules as `:root[data-theme="dark"]` and
// `:root:not([data-theme="light"])`, which outrank it however late it appears.
// So a themed tenant rendered its own accent in the light scheme and the
// built-in one everywhere else, including the scheme a browser defaults to:
// the feature looked like it did nothing. Every test that existed asserted
// what brandFor returned, never that a scheme could not overrule it.
//
// Checked as text rather than in a browser: this package has no engine, and
// what went wrong is what the page declares, not how it paints.
func TestATenantAccentReachesEveryScheme(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	page := b.page("/settings")

	// The three places a scheme is decided: the unqualified default, the
	// pinned dark and dim schemes, and the system preference.
	for _, want := range []string{
		`:root { --accent:`,
		`:root[data-theme="dark"], :root[data-theme="dim"] { --accent:`,
		`:root:not([data-theme="light"]):not([data-theme="dim"]) { --accent:`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page declares no accent for %q; a scheme rule will outrank it", want)
		}
	}
	if !strings.Contains(page, "@media (prefers-color-scheme: dark)") {
		t.Error("the accent is not restated for the system dark preference, which is the default")
	}
}
