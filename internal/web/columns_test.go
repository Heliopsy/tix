// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/web"
)

// An installation nobody has touched renders what it rendered before the
// picker existed: every declared column, in its declared order.
func TestDefaultColumnsRenderEveryListingUnchanged(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "default columns")

	tasks := b.page("/tasks")
	for _, want := range []string{`<span class="badge todo">todo</span>`, `<span class="ref mono">`} {
		if !strings.Contains(tasks, want) {
			t.Errorf("the task list lost %q by default:\n%s", want, tasks)
		}
	}

	headings := map[string][]string{
		"/admin/users":    {"<th>Email</th>", "<th>Name</th>", "<th>State</th>"},
		"/admin/tokens":   {"<th>Name</th>", "<th>Scopes</th>", "<th>Expires</th>"},
		"/admin/webhooks": {"<th>URL</th>", "<th>Events</th>", "<th>Active</th>"},
		"/admin/domains":  {"<th>Hostname</th>", "<th>Verified</th>", "<th>Certificate</th>"},
		"/projects":       {"<th>Key</th>", "<th>Name</th>", "<th>Colour</th>", "<th>State</th>", "<th>Screens</th>"},
	}
	for path, wants := range headings {
		page := b.page(path)
		for _, want := range wants {
			if !strings.Contains(page, want) {
				t.Errorf("%s lost its %s heading by default", path, want)
			}
		}
	}
}

// Choosing a set changes what renders, and the browser is still shown that
// choice on the next request rather than the default.
func TestChosenColumnsApplyAndSurviveASecondRequest(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "narrowed list")

	resp := b.post("/columns", url.Values{
		"page": {"tasks"}, "column": {"ref", "project"}, "next": {"/tasks"}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)
	if got := resp.Header.Get("Location"); got != "/tasks" {
		t.Fatalf("location = %q, want the listing it was chosen from", got)
	}

	page := b.page("/tasks")
	if strings.Contains(page, `<span class="badge todo">`) {
		t.Errorf("the status column was not hidden:\n%s", page)
	}
	if !strings.Contains(page, `<span class="ref mono">`) {
		t.Errorf("the reference column was hidden although it was chosen")
	}
	if !strings.Contains(page, `<span class="tag project">infra</span>`) {
		t.Errorf("the project column was not shown although it was chosen:\n%s", page)
	}
	if !strings.Contains(page, `value="ref" checked`) {
		t.Errorf("the picker does not show the choice back")
	}

	again := b.page("/tasks")
	if strings.Contains(again, `<span class="badge todo">`) {
		t.Fatalf("the choice did not survive a second request")
	}

	// Hiding a column withholds nothing: the value is still on the record.
	detail := b.page("/tasks/" + b.createTask("infra", "still reachable"))
	if !strings.Contains(detail, `<span class="badge todo">todo</span>`) {
		t.Fatalf("the hidden value is not reachable on the task's own screen")
	}
}

// A table keeps its identifying column whatever is chosen, so no choice can
// leave an empty listing behind.
func TestAdminListingKeepsItsIdentifyingColumn(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	created := b.post("/admin/users", url.Values{"email": {"columns@example.test"},
		"password": {"correct-horse-battery"}, "role": {"member"}})
	_ = created.Body.Close()

	resp := b.post("/columns", url.Values{"page": {"users"}, "next": {"/admin/users"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)

	page := b.page("/admin/users")
	if strings.Contains(page, "<th>State</th>") {
		t.Errorf("an unchosen column still renders:\n%s", page)
	}
	if !strings.Contains(page, "<th>Email</th>") || !strings.Contains(page, "columns@example.test") {
		t.Fatalf("the listing lost the column naming its rows:\n%s", page)
	}
}

// Reset puts a listing back to the declared default.
func TestResetRestoresTheDefaultColumns(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "reset me")

	chosen := b.post("/columns", url.Values{"page": {"tasks"}, "column": {"ref"}, "next": {"/tasks"}})
	_ = chosen.Body.Close()
	if strings.Contains(b.page("/tasks"), `<span class="badge todo">`) {
		t.Fatalf("the choice did not apply")
	}

	reset := b.post("/columns", url.Values{"page": {"tasks"}, "reset": {"1"}, "next": {"/tasks"}})
	defer func() { _ = reset.Body.Close() }()
	wantStatus(t, reset, http.StatusSeeOther)
	if !strings.Contains(b.page("/tasks"), `<span class="badge todo">todo</span>`) {
		t.Fatalf("reset did not restore the default columns")
	}
}

// A value no build of this program wrote is ignored rather than obeyed, so a
// stale or hand-edited cookie cannot empty a listing or fail a page.
func TestMalformedPreferenceFallsBackToTheDefault(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	for _, raw := range []string{"garbage", "tasks", "planets:mars", "tasks:mars",
		"tasks:status" + strings.Repeat("x", 600), "~~~:::"} {
		b := f.as("alice")
		b.createTask("infra", "still listed")
		b.setCookie(web.ColumnsCookie, raw)

		page := b.page("/tasks")
		if !strings.Contains(page, `<span class="badge todo">todo</span>`) {
			t.Errorf("cookie %q did not fall back to the default columns:\n%s", raw, page)
		}
		if !strings.Contains(page, "still listed") {
			t.Errorf("cookie %q emptied the listing", raw)
		}

		users := b.page("/admin/users")
		if !strings.Contains(users, "<th>Email</th>") {
			t.Errorf("cookie %q emptied the user table", raw)
		}
	}
}

// A listing this build does not declare is refused rather than stored.
func TestUnknownListingIsRefused(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	resp := f.as("alice").post("/columns", url.Values{"page": {"planets"}, "column": {"mars"}})
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for a listing that does not exist", resp.StatusCode)
	}
}

// The control is a plain form: it carries its own action and method, and needs
// no script to submit.
func TestColumnPickerWorksWithoutScripting(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	for _, path := range []string{"/tasks", "/projects", "/admin/users", "/admin/tokens",
		"/admin/webhooks", "/admin/domains"} {
		page := b.page(path)
		if !hasPlainForm(page, "/columns") {
			t.Errorf("%s offers no plain column picker", path)
		}
		if strings.Contains(page, "hx-post") || strings.Contains(page, "hx-get") {
			t.Errorf("%s has a control depending on scripting", path)
		}
	}

	raw := b.postRaw("/columns", url.Values{"csrf_token": {b.csrf()},
		"page": {"projects"}, "column": {"name"}, "next": {"/projects"}})
	defer func() { _ = raw.Body.Close() }()
	wantStatus(t, raw, http.StatusSeeOther)
	page := b.page("/projects")
	if strings.Contains(page, "<th>Colour</th>") {
		t.Fatalf("a plain form submission did not apply:\n%s", page)
	}
}

// The choice belongs to the browser that made it, not to the tenant or to
// anyone else signed in, so it cannot carry data or a view between them.
func TestColumnChoiceIsPerBrowserAndCarriesNoTenantData(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	alice := f.as("alice")
	alice.createTask("infra", "alice's task")

	resp := alice.post("/columns", url.Values{"page": {"tasks"}, "column": {"ref"}, "next": {"/tasks"}})
	_ = resp.Body.Close()
	if strings.Contains(alice.page("/tasks"), `<span class="badge todo">`) {
		t.Fatalf("alice's choice did not apply")
	}

	fresh := f.as("alice")
	page := fresh.page("/tasks")
	if !strings.Contains(page, `<span class="badge todo">todo</span>`) {
		t.Errorf("a second browser inherited the choice, so it is not per-browser:\n%s", page)
	}

	elsewhere := f.as("bob").page("/tasks")
	if strings.Contains(elsewhere, "alice&#39;s task") || strings.Contains(elsewhere, "alice's task") {
		t.Fatalf("the listing disclosed another tenant's task")
	}
}

// The picker returns to the listing the reader was on, filter and all.
func TestPickerReturnsToTheFilteredListing(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	page := b.page("/tasks?q=" + url.QueryEscape("status:todo"))
	if !strings.Contains(page, `name="next" value="/tasks?q=status%3atodo"`) &&
		!strings.Contains(page, `name="next" value="/tasks?q=status%3Atodo"`) {
		t.Fatalf("the picker does not return to the filtered listing:\n%s",
			between(t, page, `action="/columns"`, "</form>"))
	}
}

// setCookie plants a value in the browser's jar, as a stale install or another
// program on the same host could leave behind.
func (b *browser) setCookie(name, value string) {
	b.t.Helper()
	parsed, err := url.Parse(b.fix.server.URL)
	if err != nil {
		b.t.Fatalf("parsing server url: %v", err)
	}
	b.client.Jar.SetCookies(parsed, []*http.Cookie{{Name: name, Value: value, Path: "/"}})
}
