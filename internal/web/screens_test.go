// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestScreensRenderForEntitledActor(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "first task")

	cases := []struct {
		name string
		path string
		want []string
	}{
		{"projects", "/projects", []string{"Projects", "infra"}},
		{"board", "/projects/infra", []string{"board", "To do", "first task"}},
		{"fields", "/projects/infra/fields", []string{"Field definitions", "Define a field"}},
		{"workflows", "/workflows", []string{"Workflows", "default"}},
		{"workflow", "/workflows/default", []string{"Workflow default", "Transitions"}},
		{"tasks", "/tasks", []string{"Tasks", "first task", "New task"}},
		{"task", "/tasks/" + ref, []string{"first task", "Comments", "Dependencies",
			"Artifacts", "History", "Subtasks", "Custom fields"}},
		{"activity", "/activity", []string{"Activity"}},
		{"tenant", "/admin/tenant", []string{"Tenant", "Members", "Retention"}},
		{"domains", "/admin/domains", []string{"Domains", "Map a hostname"}},
		{"users", "/admin/users", []string{"Users", "Create a user"}},
		{"tokens", "/admin/tokens", []string{"API tokens", "Issue a token"}},
		{"ssh keys", "/admin/ssh-keys", []string{"SSH keys", "Enrol a key"}},
		{"webhooks", "/admin/webhooks", []string{"Webhooks", "Delivery log"}},
		{"transfer", "/transfer", []string{"Import and export", "Export a snapshot"}},
		{"sync", "/sync", []string{"External sync", "Configure a source"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page := b.page(tc.path)
			for _, want := range tc.want {
				if !strings.Contains(page, want) {
					t.Errorf("page %s does not contain %q", tc.path, want)
				}
			}
		})
	}
}

func TestRootRedirectsToProjects(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	resp := f.as("alice").get("/")
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)
	if got := resp.Header.Get("Location"); got != "/projects" {
		t.Fatalf("location = %q, want /projects", got)
	}
}

func TestUnknownPathIsNotFound(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	resp := f.as("alice").get("/nowhere")
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusNotFound)
}

// A refused mutation offers its way back to the screen it was submitted from,
// and a refused page offers the dashboard: there is nowhere else a GET could
// return to, and sending it back to its own failing path would loop.
func TestTheErrorPageLeadsBackToWhereTheRequestCameFrom(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	refused := b.post("/workflows", url.Values{"key": {"broken"}, "name": {"Broken"},
		"initial": {"nowhere"}, "states": {"open|Open|open"}, "transitions": {""}})
	page := body(t, refused)
	wantStatus(t, refused, http.StatusBadRequest)
	if !strings.Contains(page, `<a href="/workflows">Back to where you were</a>`) {
		t.Errorf("a refused submission does not lead back to the screen it came from:\n%s", page)
	}
	// The error page is the whole answer. A second write after it renders
	// appends its own words to the bottom of the document.
	if strings.Contains(page, "internal error") {
		t.Errorf("the error page carries a second answer after it:\n%s", page)
	}

	missing := body(t, f.as("alice").get("/nowhere"))
	if !strings.Contains(missing, `<a href="/">Back to where you were</a>`) {
		t.Errorf("a page that does not exist does not lead back to the dashboard:\n%s", missing)
	}
}

// Every mutation answers with a message for the screen it returns to, and
// the message travels in the redirect target.
func TestAMutationCarriesItsMessageToTheNextScreen(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	saved := b.post("/workflows", url.Values{
		"key": {"review"}, "name": {"Review"}, "initial": {"open"},
		"states":      {"open|Open|open\nclosed|Closed|terminal"},
		"transitions": {"open>closed"},
	})
	_ = saved.Body.Close()
	wantStatus(t, saved, http.StatusSeeOther)
	if got := saved.Header.Get("Location"); got != "/workflows/review?flash=workflow+saved" {
		t.Errorf("location = %q, want the listing carrying the message", got)
	}
}

func TestUnauthenticatedRequestRedirectsToLogin(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	anonymous := f.as("")

	resp := anonymous.get("/tasks")
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)
	location := resp.Header.Get("Location")
	if !strings.HasPrefix(location, "/login") {
		t.Fatalf("location = %q, want the login screen", location)
	}
	if !strings.Contains(location, url.QueryEscape("/tasks")) {
		t.Fatalf("location = %q, want it to remember the requested page", location)
	}
}

func TestLoginScreenIsPublic(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	page := f.as("").page("/login")
	for _, want := range []string{"Sign in", "csrf_token", `name="password"`} {
		if !strings.Contains(page, want) {
			t.Errorf("login screen does not contain %q", want)
		}
	}
}

func TestSignedInBrowserSkipsLoginScreen(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	resp := f.as("alice").get("/login")
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)
}

func TestAssetsAreServedFromTheBinary(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for _, asset := range []string{"/assets/app.css", "/assets/htmx.min.js", "/assets/live.js",
		"/assets/decide.js", "/assets/shortcuts.js", "/assets/copy.js"} {
		resp := b.get(asset)
		wantStatus(t, resp, http.StatusOK)
		if len(body(t, resp)) == 0 {
			t.Fatalf("asset %s is empty", asset)
		}
	}
}

// TestPagesReferenceNoExternalOrigin checks every screen, not one of them.
// It used to fetch /projects alone, which meant the guard watched a single
// page while prose was being added to a dozen others -- and prose is exactly
// where a helpful example url gets written down. A screen naming a real
// external host would both leak a reference off this install and hand
// somebody a link this product did not vouch for.
func TestPagesReferenceNoExternalOrigin(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "first task")
	// One source of each system, because the sync screen describes a source
	// with the settings its own adapter reads, and an example url written
	// into one of those descriptions would only appear once a source of that
	// system existed.
	for _, system := range []string{"generic", "jira", "openproject"} {
		addSource(t, b, system, system+" source")
	}
	for _, path := range []string{
		"/projects", "/projects/infra", "/projects/infra/fields", "/workflows",
		"/workflows/default", "/tasks", "/tasks/" + ref, "/activity",
		"/admin/tenant", "/admin/domains", "/admin/users", "/admin/tokens",
		"/admin/ssh-keys", "/admin/webhooks", "/admin/connections",
		"/transfer", "/bundles", "/sync", "/sync/runs", "/settings",
	} {
		page := b.page(path)
		for _, forbidden := range []string{"http://", "https://", "//cdn", "unpkg"} {
			if strings.Contains(page, forbidden) {
				t.Errorf("%s references an external origin %q", path, forbidden)
			}
		}
	}
}

func TestReadOnlyActorSeesNoEditForms(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ref := f.as("alice").createTask("infra", "read only view")

	viewer := f.as("viewer")
	page := viewer.page("/tasks/" + ref)
	if !strings.Contains(page, "You may read this task but not change it.") {
		t.Fatalf("read-only task page offers editing")
	}
	if strings.Contains(page, `action="/tasks/`+ref+`/transition"`) {
		t.Fatalf("read-only task page still offers a transition form")
	}

	resp := viewer.post("/tasks/"+ref, url.Values{"title": {"changed"}})
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

func TestProgressiveEnhancementIsOptional(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	page := f.as("alice").page("/tasks")
	if !strings.Contains(page, `hx-boost="true"`) {
		t.Fatalf("pages are not enhanced when scripts are available")
	}
	if !hasPlainForm(page, "/tasks") {
		t.Fatalf("the enhanced form is not a plain form underneath")
	}
	if strings.Contains(page, "hx-post") || strings.Contains(page, "hx-get") {
		t.Fatalf("a control depends on scripting rather than on its own action")
	}
}
