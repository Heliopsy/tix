// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/web"
)

// quickTitle is the quick-add title input's own tag, so an attribute found
// is the box's and not the filter box's beside it.
func quickTitle(t *testing.T, page string) string {
	t.Helper()
	return between(t, quickBox(t, page), `<input name="title"`, ">")
}

// quickCheckbox is the Add another checkbox's own tag.
func quickCheckbox(t *testing.T, page string) string {
	t.Helper()
	box := between(t, quickBox(t, page), `<input type="checkbox" name="another"`, ">")
	if box == "" && !strings.Contains(quickBox(t, page), `name="another"`) {
		t.Fatal("the quick-add form has no Add another checkbox")
	}
	return box
}

// With Add another ticked a quick add comes back to the listing it was made
// from, even for a reader who has Open a new task on, and that listing alone
// draws the box focused and empty with the checkbox still ticked.
func TestAddAnotherReturnsToTheListingWithTheBoxFocused(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.setCookie(web.OpenAfterAddCookie, "1")

	resp := b.postFrom("/tasks?q=tag%3Aops", "/tasks", url.Values{
		"quick": {"1"}, "another": {"1"}, "project_ref": {"infra"}, "title": {"first of a run"},
	})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if loc.Path != "/tasks" || loc.Query().Get("q") != "tag:ops" {
		t.Fatalf("Add another sent the reader to %q, not back to the listing", loc)
	}
	if got := b.cookie(web.AddAnotherCookie); got != "1" {
		t.Errorf("Add another is not remembered: cookie %q", got)
	}

	landed := b.page(loc.RequestURI())
	if box := quickTitle(t, landed); !strings.Contains(box, " autofocus") || strings.Contains(box, "first of a run") {
		t.Errorf("the box the add returned to is not focused and empty: %s", box)
	}
	if !strings.Contains(quickCheckbox(t, landed), " checked") {
		t.Error("the listing the add returned to draws Add another unticked")
	}

	again := b.page("/tasks")
	if box := quickTitle(t, again); strings.Contains(box, " autofocus") {
		t.Errorf("a later visit still focuses the box: %s", box)
	}
	if !strings.Contains(quickCheckbox(t, again), " checked") {
		t.Error("a later visit forgot Add another")
	}
}

// Unticking it is remembered too, and the add goes back to whatever Open a
// new task says.
func TestAddAnotherUntickedIsForgottenAndTheTaskOpens(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.setCookie(web.AddAnotherCookie, "1")
	b.setCookie(web.OpenAfterAddCookie, "1")

	resp := b.postFrom("/tasks", "/tasks", url.Values{"quick": {"1"}, "project_ref": {"infra"}, "title": {"just one"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	if loc := resp.Header.Get("Location"); !strings.HasPrefix(loc, "/tasks/") {
		t.Errorf("with Add another off and Open a new task on the add went to %q", loc)
	}
	if got := b.cookie(web.AddAnotherCookie); got != "" {
		t.Errorf("unticking Add another left the cookie at %q", got)
	}
	page := b.page("/tasks")
	if strings.Contains(quickCheckbox(t, page), " checked") {
		t.Error("the listing draws Add another ticked after it was unticked")
	}
	if box := quickTitle(t, page); strings.Contains(box, " autofocus") {
		t.Errorf("the box is focused on an ordinary visit: %s", box)
	}
}

// A refused quick add keeps what the reader posted for Add another, not
// what the browser last remembered.
func TestARefusedQuickAddKeepsAddAnother(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	resp := b.postFrom("/tasks", "/tasks", url.Values{
		"quick": {"1"}, "another": {"1"}, "project_ref": {"infra"}, "title": {"!high"},
	})
	page := body(t, resp)
	if resp.StatusCode == http.StatusSeeOther {
		t.Fatal("a line with no title was added")
	}
	if !strings.Contains(quickCheckbox(t, page), " checked") {
		t.Error("the refused add draws Add another unticked")
	}
}

// The box completes from its own data, which lists every project even on a
// project page, while the filter box there still offers none. The clipboard
// button is drawn hidden for its script to reveal.
func TestTheQuickAddBoxCompletesAndCanPaste(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	seedProject(t, f.store, f.tenantA.ID, "web")
	b := f.as("alice")
	for _, path := range []string{"/tasks", "/projects/infra"} {
		page := b.page(path)
		box := quickTitle(t, page)
		for _, want := range []string{`data-suggest="quickadd-suggest"`, `data-suggest-mode="quick"`, `data-quickadd="/tasks/quickadd"`} {
			if !strings.Contains(box, want) {
				t.Errorf("%s: the quick-add box lacks %s: %s", path, want, box)
			}
		}
		data := between(t, quickBox(t, page), `<script type="application/json" id="quickadd-suggest">`, "</script>")
		for _, want := range []string{`"k":"infra"`, `"k":"web"`, `"person":[`, `"alice"`, `"tag":`} {
			if !strings.Contains(data, want) {
				t.Errorf("%s: the quick-add suggestion data lacks %s:\n%s", path, want, data)
			}
		}
		paste := between(t, quickBox(t, page), `<button type="button" class="secondary quickpaste"`, "</button>")
		if !strings.Contains(paste, "data-quickpaste hidden") || !strings.Contains(paste, `aria-label="Quick add from the clipboard"`) {
			t.Errorf("%s: the paste button is not drawn hidden with its name: %s", path, paste)
		}
	}
	filter := between(t, b.page("/projects/infra"), `<script type="application/json" id="filter-suggest">`, "</script>")
	if strings.Contains(filter, `"k":"web"`) {
		t.Errorf("the project page's filter box now offers projects:\n%s", filter)
	}
}

// The clipboard shortcut is the app's chord in every scheme.
func TestQuickPasteIsBoundInEveryScheme(t *testing.T) {
	t.Parallel()
	for _, s := range web.KeySchemes() {
		keys := web.KeyBindingsFor(s)["quickPaste"]
		if !equalKeys(keys, []string{"ctrl+shift+v", "meta+shift+v"}) {
			t.Errorf("%s binds quickPaste to %v", s, keys)
		}
	}
}
