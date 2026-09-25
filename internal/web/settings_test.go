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
		if !strings.Contains(page, `<a href="/settings"`) || !strings.Contains(page, `>Settings</a>`) {
			t.Errorf("%s carries no link to the settings page:\n%s", path, page)
		}
		if strings.Contains(page, `<details class="panel settings">`) {
			t.Errorf("%s still carries the old settings disclosure alongside the link:\n%s", path, page)
		}
		if hasPlainForm(page, "/theme") || hasPlainForm(page, "/advanced") {
			t.Errorf("%s still carries a preference form outside the settings page:\n%s", path, page)
		}
	}
}

// The settings page gathers the theme and the advanced toggle, replacing what
// TestSidebarOffersOneSettingsMenu used to assert about the sidebar
// disclosure: the controls moved, not disappeared.
func TestSettingsPageGathersThemeAndAdvanced(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	page := b.page("/settings")

	if !strings.Contains(page, "<h1>Settings</h1>") {
		t.Errorf("the settings page carries no heading:\n%s", page)
	}
	if !hasPlainForm(page, "/theme") || !hasPlainForm(page, "/advanced") {
		t.Errorf("the settings page does not gather the theme and the advanced toggle:\n%s", page)
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
			if !strings.Contains(page, `<option value="`+theme+`" selected>`) {
				t.Errorf("the page does not show %q as the current scheme:\n%s", name, page)
			}
		})
	}
}

// The advanced toggle moved onto the settings page, and still does what it
// did, off by default.
func TestSettingsPageStillTogglesTheAdvancedScreens(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	if strings.Contains(b.page("/tasks"), `href="/admin/tokens"`) {
		t.Fatal("the advanced screens are on by default")
	}
	resp := b.post("/advanced", url.Values{"next": {"/tasks"}})
	_ = resp.Body.Close()
	if !strings.Contains(b.page("/tasks"), `href="/admin/tokens"`) {
		t.Fatal("showing the advanced screens from the settings page had no effect")
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
	if strings.Contains(bare, "<code>") {
		t.Errorf("the settings page shows a database target with nothing configured:\n%s", bare)
	}
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
	resp, err := http.DefaultClient.Do(req)
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
