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

func TestPagesReferenceNoExternalOrigin(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	page := f.as("alice").page("/projects")
	for _, forbidden := range []string{"http://", "https://", "//cdn", "unpkg"} {
		if strings.Contains(page, forbidden) {
			t.Fatalf("page references an external origin %q", forbidden)
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
