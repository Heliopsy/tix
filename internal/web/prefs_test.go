// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/docsmd"
	"github.com/heliopsy/tix/internal/web"
)

// The settings screen owns every per-browser preference, which is what the
// reader asked for: four of the nine were on it, three were set only from the
// screen that used them, and two had a control nowhere a reader would look.
//
// Driven off web.Preferences rather than a list written here, so a preference
// added to the package with no control on this screen fails rather than
// arriving undiscoverable.
func TestSettingsOwnsEveryPreference(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	page := f.as("alice").page("/settings")

	for _, p := range web.Preferences {
		if !hasPlainForm(page, p.Route) {
			t.Errorf("the settings screen has no control for %s (%s), which posts to %s:\n%s",
				p.Label, p.Cookie, p.Route, page)
		}
	}
}

// Every preference the package has is a row in the documentation table, and
// every row is a preference the package has. The table transcribes a list the
// code already holds, which is the kind that rots.
func TestEveryPreferenceIsDocumented(t *testing.T) {
	t.Parallel()
	md, err := docsmd.Read("docs/web-ui.md")
	if err != nil {
		t.Fatalf("reading docs/web-ui.md: %v", err)
	}
	rows, err := docsmd.Table(md, "Preference", "Cookie", "Values")
	if err != nil {
		t.Fatalf("reading the preferences table: %v", err)
	}
	documented := map[string]bool{}
	for _, row := range rows {
		for _, code := range docsmd.Codes(row[1]) {
			documented[code] = true
		}
	}
	held := map[string]bool{}
	for _, p := range web.Preferences {
		held[p.Cookie] = true
		if !documented[p.Cookie] {
			t.Errorf("%s (%s) has no row in the preferences table", p.Label, p.Cookie)
		}
	}
	for cookie := range documented {
		if !held[cookie] {
			t.Errorf("the preferences table has a row for %s, which this build has no preference for", cookie)
		}
	}
}

// Settings offers a picker for every listing whose columns can be chosen, and
// for no listing that has none. columnSets decides which listings exist and
// columnPages decides how settings names them; nothing but this holds them
// together.
func TestSettingsOffersEveryListingsColumns(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	page := f.as("alice").page("/settings")

	for _, listing := range web.ColumnListings() {
		if !strings.Contains(page, `<input type="hidden" name="page" value="`+listing+`">`) {
			t.Errorf("the settings screen offers no column picker for the %q listing:\n%s", listing, page)
		}
	}
	// Each picker is a form of its own, so the count is what says none has
	// been offered twice and no listing has been left out.
	if got, want := strings.Count(page, `action="/columns"`), len(web.ColumnListings()); got != want {
		t.Errorf("the settings screen carries %d column pickers, want one per listing (%d)", got, want)
	}
}

// Changing a preference on settings takes effect on the screen that uses it.
// This is the half that was already true for theme and the scheme and was
// never true for these three, because settings had no control for them.
func TestAPreferenceChangedOnSettingsReachesTheScreenThatUsesIt(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	t.Run("task view", func(t *testing.T) {
		b := f.as("alice")
		b.page("/settings")
		apply(t, b, "/taskview", url.Values{"view": {"board"}, "next": {"/settings"}})
		if got := viewSwitch(t, b.page("/tasks")); !selected(got, "board") {
			t.Errorf("the task screen's switch does not show the board:\n%s", got)
		}
	})

	t.Run("hidden projects", func(t *testing.T) {
		seedProject(t, f.store, f.tenantA.ID, "ops")
		b := f.as("alice")
		b.page("/settings")
		// Every project but infra, which is how this form says "hide infra".
		apply(t, b, "/visibility", url.Values{"project": {"ops"}, "next": {"/settings"}})
		tasks := b.page("/tasks")
		if !strings.Contains(tasks, "project is hidden") {
			t.Errorf("the task screen does not account for the hidden project:\n%s", tasks)
		}
		if !checkedIn(t, projectChoices(t, tasks), "ops") {
			t.Error("the task screen's own control lost the project that was kept")
		}
		if checkedIn(t, projectChoices(t, tasks), "infra") {
			t.Error("the task screen's own control still shows the project settings hid")
		}
	})

	t.Run("columns", func(t *testing.T) {
		b := f.as("alice")
		b.page("/settings")
		// Scopes and Created kept, Expires dropped, on the tokens listing.
		apply(t, b, "/columns", url.Values{"page": {"tokens"},
			"column": {"scopes", "created"}, "next": {"/settings"}})
		tokens := b.page("/admin/tokens")
		if strings.Contains(tableHead(t, tokens), "Expires") {
			t.Errorf("the tokens listing still has the column settings hid:\n%s", tableHead(t, tokens))
		}
		if !strings.Contains(tableHead(t, tokens), "Scopes") {
			t.Errorf("the tokens listing lost a column that was kept:\n%s", tableHead(t, tokens))
		}
	})
}

// The one that matters. A preference changed on the screen that uses it is
// what the settings screen shows, asserted on the rendered control rather
// than on the cookie: a cookie both controls write and only one reads is
// exactly the disagreement this is here to prevent, and it would pass a
// cookie check.
func TestAPreferenceChangedInContextIsReflectedOnSettings(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	t.Run("task view", func(t *testing.T) {
		b := f.as("alice")
		if got := viewSwitch(t, b.page("/settings")); !selected(got, "list") {
			t.Fatalf("an untouched browser does not read as the list on settings:\n%s", got)
		}
		b.page("/tasks")
		apply(t, b, "/taskview", url.Values{"view": {"board"}, "next": {"/tasks"}})
		got := viewSwitch(t, b.page("/settings"))
		if !selected(got, "board") {
			t.Errorf("settings does not show the board the task screen switched to:\n%s", got)
		}
		if selected(got, "list") {
			t.Errorf("settings shows both positions as current:\n%s", got)
		}
	})

	t.Run("hidden projects", func(t *testing.T) {
		seedProject(t, f.store, f.tenantA.ID, "ops")
		b := f.as("alice")
		b.page("/tasks")
		apply(t, b, "/visibility", url.Values{"project": {"infra"}, "next": {"/tasks"}})
		choices := projectChoices(t, b.page("/settings"))
		if checkedIn(t, choices, "ops") {
			t.Errorf("settings still shows a project the task screen hid:\n%s", choices)
		}
		if !checkedIn(t, choices, "infra") {
			t.Errorf("settings hides a project the task screen kept:\n%s", choices)
		}
	})

	t.Run("columns", func(t *testing.T) {
		b := f.as("alice")
		b.page("/admin/tokens")
		apply(t, b, "/columns", url.Values{"page": {"tokens"},
			"column": {"scopes"}, "next": {"/admin/tokens"}})
		block := columnPicker(t, b.page("/settings"), "tokens")
		if !checkedValue(block, "scopes") {
			t.Errorf("settings does not show the column the listing kept:\n%s", block)
		}
		for _, dropped := range []string{"created", "expires"} {
			if checkedValue(block, dropped) {
				t.Errorf("settings shows %q ticked, which the listing dropped:\n%s", dropped, block)
			}
		}
	})
}

// A hidden key naming a project that no longer exists breaks neither screen,
// is named rather than left as a listing one project shorter for no stated
// reason, and is forgotten by the next submission. It has no checkbox,
// because the project it names is gone.
func TestAHiddenKeyForADeletedProjectIsNamedAndForgotten(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	seedProject(t, f.store, f.tenantA.ID, "ops")
	b := f.as("alice")

	b.setCookie(web.HiddenProjectsCookie, "infra.gone-away")

	for _, path := range []string{"/settings", "/tasks"} {
		page := b.page(path)
		if !strings.Contains(page, "gone-away") {
			t.Errorf("%s does not name the hidden key no project answers to:\n%s", path, page)
		}
		if strings.Contains(page, `name="project" value="gone-away"`) {
			t.Errorf("%s offers a checkbox for a project that does not exist:\n%s", path, page)
		}
	}
	// The screen is still a screen: the listing it governs still renders and
	// still excludes the project that does exist.
	if !checkedIn(t, projectChoices(t, b.page("/tasks")), "ops") {
		t.Error("a stale key cost the task screen its live projects")
	}

	b.page("/settings")
	apply(t, b, "/visibility", url.Values{"project": {"ops", "infra"}, "next": {"/settings"}})
	page := b.page("/settings")
	if strings.Contains(page, "gone-away") {
		t.Errorf("applying the form did not forget the stale key:\n%s", page)
	}
}

// Every preference form on settings is a plain form a browser submits on its
// own: method, action, a submit button and no scripted handler. Working with
// scripting off is a project principle, and a preference screen that needs it
// to save is the worst place to break it.
func TestEveryPreferenceFormWorksWithoutScripting(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	seedProject(t, f.store, f.tenantA.ID, "ops")
	b := f.as("alice")
	page := b.page("/settings")

	for _, bad := range []string{"hx-post", "hx-get", "onchange=", "onclick=", "onsubmit="} {
		if strings.Contains(page, bad) {
			t.Errorf("a settings control depends on scripting (%s):\n%s", bad, page)
		}
	}
	for _, p := range web.Preferences {
		form := prefForm(t, page, p.Route)
		if !strings.Contains(form, `method="post"`) {
			t.Errorf("%s's control is not a posting form:\n%s", p.Label, form)
		}
		if !strings.Contains(form, `type="submit"`) {
			t.Errorf("%s's control has no submit button, so nothing submits it without a script:\n%s",
				p.Label, form)
		}
	}

	// And they actually store the value when submitted as a bare form, with
	// no header a script would have added.
	for _, c := range []struct {
		route string
		form  url.Values
	}{
		{"/taskview", url.Values{"view": {"board"}}},
		{"/visibility", url.Values{"project": {"ops"}}},
		{"/columns", url.Values{"page": {"tokens"}, "column": {"scopes"}}},
		{"/theme", url.Values{"theme": {"dim"}}},
		{"/advanced", url.Values{}},
		{"/dragmove", url.Values{}},
		{"/keyscheme", url.Values{"keyscheme": {"emacs"}}},
		{"/timezone", url.Values{"timezone": {"UTC"}}},
		{"/timeformat", url.Values{"time_format": {"iso"}}},
	} {
		form := url.Values{"csrf_token": {b.csrf()}, "next": {"/settings"}}
		for k, v := range c.form {
			form[k] = v
		}
		resp := b.postRaw(c.route, form)
		location := resp.Header.Get("Location")
		status := resp.StatusCode
		_ = resp.Body.Close()
		if status != http.StatusSeeOther {
			t.Errorf("POST %s with no script = %d, want 303", c.route, status)
		}
		if location != "/settings" {
			t.Errorf("POST %s returned to %q, want the screen the form was on", c.route, location)
		}
	}
}

// apply submits one preference form and insists it was accepted, so a test
// that goes on to assert a rendered control is not reading the state of a
// submission that was refused.
func apply(t *testing.T, b *browser, route string, form url.Values) {
	t.Helper()
	resp := b.post(route, form)
	status := resp.StatusCode
	_ = resp.Body.Close()
	if status != http.StatusSeeOther {
		t.Fatalf("POST %s = %d, want 303", route, status)
	}
}

// selected reports whether one position of the switch is the current one.
func selected(form, value string) bool {
	i := strings.Index(form, `value="`+value+`"`)
	if i < 0 {
		return false
	}
	rest := form[i:]
	end := strings.Index(rest, ">")
	if end < 0 {
		return false
	}
	return strings.Contains(rest[:end], `aria-current="true"`)
}

// projectChoices returns the visibility form's own markup.
func projectChoices(t *testing.T, page string) string {
	t.Helper()
	return prefForm(t, page, "/visibility")
}

// prefForm returns one preference form and nothing else, found by its action
// whatever other attributes the tag carries, so an assertion about one
// control on a screen of controls reads that control. formAt wants the action
// to be the tag's last attribute, which half of these are not.
func prefForm(t *testing.T, page, action string) string {
	t.Helper()
	i := strings.Index(page, `action="`+action+`"`)
	if i < 0 {
		t.Fatalf("the page has no form posting to %s:\n%s", action, page)
	}
	start := strings.LastIndex(page[:i], "<form")
	end := strings.Index(page[i:], "</form>")
	if start < 0 || end < 0 {
		t.Fatalf("the form posting to %s is not closed", action)
	}
	return page[start : i+end]
}

// checkedIn reports whether the visibility form shows one project as shown.
func checkedIn(t *testing.T, form, key string) bool {
	t.Helper()
	return checkedValue(form, key)
}

// checkedValue reports whether a checkbox carrying one value is ticked. It
// reads the one input element rather than the block around it, since every
// other checkbox in the block carries the same attribute.
func checkedValue(block, value string) bool {
	for _, attr := range []string{`value="` + value + `"`} {
		i := strings.Index(block, attr)
		for i >= 0 {
			start := strings.LastIndex(block[:i], "<input")
			end := strings.Index(block[i:], ">")
			if start >= 0 && end >= 0 {
				if strings.Contains(block[start:i+end], "checked") {
					return true
				}
			}
			next := strings.Index(block[i+len(attr):], attr)
			if next < 0 {
				break
			}
			i += len(attr) + next
		}
	}
	return false
}

// columnPicker returns the settings screen's picker for one listing, found by
// the hidden field naming that listing, so an assertion about the Tokens
// picker cannot be satisfied by the Tasks one above it.
func columnPicker(t *testing.T, page, listing string) string {
	t.Helper()
	marker := `<input type="hidden" name="page" value="` + listing + `">`
	i := strings.Index(page, marker)
	if i < 0 {
		t.Fatalf("the page has no column picker for the %q listing:\n%s", listing, page)
	}
	start := strings.LastIndex(page[:i], "<form")
	end := strings.Index(page[i:], "</form>")
	if start < 0 || end < 0 {
		t.Fatalf("the %q column picker is not a closed form", listing)
	}
	return page[start : i+end]
}

// tableHead returns the first table's header row, which is where a listing
// says which columns it is drawing.
func tableHead(t *testing.T, page string) string {
	t.Helper()
	i := strings.Index(page, "<thead>")
	if i < 0 {
		t.Fatalf("the page has no table header:\n%s", page)
	}
	end := strings.Index(page[i:], "</thead>")
	if end < 0 {
		t.Fatalf("the table header is never closed")
	}
	return page[i : i+end]
}
