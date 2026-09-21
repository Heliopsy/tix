package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/web"
)

// The display preferences live in one control that is on screen at all times,
// rather than a bare select stacked on a button.
func TestSidebarOffersOneSettingsMenu(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	for _, path := range []string{"/projects", "/tasks", "/activity"} {
		page := b.page(path)
		if !strings.Contains(page, `<details class="panel settings">`) {
			t.Errorf("%s has no settings menu in the sidebar:\n%s", path, page)
		}
		if !strings.Contains(page, "<summary>Settings</summary>") {
			t.Errorf("%s does not label the settings menu", path)
		}
		if !hasPlainForm(page, "/theme") || !hasPlainForm(page, "/advanced") {
			t.Errorf("%s does not gather the theme and the advanced toggle", path)
		}
	}
}

// Every control inside it is a real form, so the menu works with scripting
// turned off.
func TestSettingsMenuNeedsNoScripting(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	page := b.page("/tasks")

	if strings.Contains(page, "hx-post") || strings.Contains(page, "hx-get") {
		t.Fatal("a settings control depends on scripting")
	}
	if strings.Contains(page, "onchange=") {
		t.Fatal("the theme control submits itself with a script rather than with a button")
	}

	resp := b.postRaw("/theme", url.Values{"csrf_token": {b.csrf()},
		"theme": {"dim"}, "next": {"/tasks"}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)
	if got := resp.Header.Get("Location"); got != "/tasks" {
		t.Errorf("location = %q, want the page the choice was made on", got)
	}
	if !strings.Contains(b.page("/tasks"), `data-theme="dim"`) {
		t.Error("the low contrast scheme did not take effect")
	}
}

// All four schemes have to render the menu, including the one an untouched
// browser gets and the low contrast one.
func TestSettingsMenuRendersInEveryScheme(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	for _, theme := range web.Themes {
		name := theme
		if name == "" {
			name = "system"
		}
		t.Run(name, func(t *testing.T) {
			b := f.as("alice")
			resp := b.post("/theme", url.Values{"theme": {theme}, "next": {"/tasks"}})
			_ = resp.Body.Close()

			page := b.page("/tasks")
			if theme == "" {
				if strings.Contains(page, "data-theme=") {
					t.Errorf("the system scheme pinned a theme:\n%s", page)
				}
			} else if !strings.Contains(page, `data-theme="`+theme+`"`) {
				t.Errorf("the page does not carry the %q scheme", theme)
			}
			if !strings.Contains(page, `<details class="panel settings">`) {
				t.Errorf("the settings menu is missing under the %q scheme", name)
			}
			if !strings.Contains(page, `<option value="`+theme+`" selected>`) {
				t.Errorf("the menu does not show %q as the current scheme:\n%s", name, page)
			}
		})
	}
}

// The advanced toggle moved into the menu, and still does what it did.
func TestSettingsMenuStillTogglesTheAdvancedScreens(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	if strings.Contains(b.page("/tasks"), `href="/admin/tokens"`) {
		t.Fatal("the advanced screens are on by default")
	}
	resp := b.post("/advanced", url.Values{"next": {"/tasks"}})
	_ = resp.Body.Close()
	if !strings.Contains(b.page("/tasks"), `href="/admin/tokens"`) {
		t.Fatal("showing the advanced screens from the settings menu had no effect")
	}
}
