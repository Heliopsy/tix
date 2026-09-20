package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/thereisnotime/tix/internal/web"
)

func TestCSRFRejection(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	good := b.csrf()

	cases := []struct {
		name string
		form url.Values
	}{
		{"missing", url.Values{"project_ref": {"infra"}, "title": {"no token"}}},
		{"empty", url.Values{"project_ref": {"infra"}, "title": {"blank"}, "csrf_token": {""}}},
		{"wrong", url.Values{"project_ref": {"infra"}, "title": {"wrong"},
			"csrf_token": {"not-" + good}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := b.postRaw("/tasks", tc.form)
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", resp.StatusCode)
			}
		})
	}

	page := b.page("/tasks")
	for _, title := range []string{"no token", "blank", "wrong"} {
		if strings.Contains(page, title) {
			t.Fatalf("a rejected submission created %q", title)
		}
	}
}

func TestCSRFRejectedWithoutCookie(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	resp := b.postRaw("/tasks", url.Values{
		"project_ref": {"infra"}, "title": {"cookieless"}, "csrf_token": {"anything"}})
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

func TestEveryFormCarriesACSRFField(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "form audit")

	paths := []string{"/projects", "/projects/infra", "/projects/infra/fields", "/workflows",
		"/workflows/default", "/tasks", "/tasks/" + ref, "/admin/tenant", "/admin/domains",
		"/admin/users", "/admin/tokens", "/admin/webhooks", "/transfer", "/sync", "/activity"}
	for _, path := range paths {
		page := b.page(path)
		for _, form := range splitForms(page) {
			if !strings.Contains(form, `method="post"`) {
				continue
			}
			if !strings.Contains(form, web.CSRFFieldName) {
				t.Errorf("a form on %s carries no csrf field:\n%s", path, form)
			}
		}
	}
}

// splitForms returns each form element of a page.
func splitForms(page string) []string {
	var out []string
	rest := page
	for {
		start := strings.Index(rest, "<form")
		if start < 0 {
			return out
		}
		rest = rest[start:]
		end := strings.Index(rest, "</form>")
		if end < 0 {
			return out
		}
		out = append(out, rest[:end])
		rest = rest[end:]
	}
}

func TestForeignTenantIdentifierIsNotFound(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	foreign := f.as("bob").createTask("other", "tenant b secret title")

	alice := f.as("alice")
	resp := alice.get("/tasks/" + foreign)
	page := body(t, resp)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	if strings.Contains(page, "tenant b secret title") {
		t.Fatalf("another tenant's task title was disclosed")
	}
}

func TestForeignTenantIdentifierInAFormIsRefused(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	bob := f.as("bob")
	foreign := bob.createTask("other", "tenant b task")

	alice := f.as("alice")
	resp := alice.post("/tasks/"+foreign+"/comments", url.Values{"body": {"leaked"}})
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}

	page := bob.page("/tasks/" + foreign)
	if strings.Contains(page, "leaked") {
		t.Fatalf("a cross-tenant comment was written")
	}
}

func TestTenantParameterIsRefusedRatherThanHonoured(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	alice := f.as("alice")

	resp := alice.get("/tasks?tenant_id=" + url.QueryEscape(f.tenantB.ID))
	page := body(t, resp)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
	if !strings.Contains(page, "taken from the signed-in session") {
		t.Fatalf("refusal does not explain itself: %s", page)
	}

	form := url.Values{"project_ref": {"infra"}, "title": {"aimed elsewhere"},
		"tenant_id": {f.tenantB.ID}}
	postResp := alice.post("/tasks", form)
	defer func() { _ = postResp.Body.Close() }()
	if postResp.StatusCode != http.StatusForbidden {
		t.Fatalf("form status = %d, want 403", postResp.StatusCode)
	}
}

func TestListingsAreScopedToTheSessionTenant(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.as("bob").createTask("other", "belongs to tenant b")
	f.as("alice").createTask("infra", "belongs to tenant a")

	page := f.as("alice").page("/tasks")
	if strings.Contains(page, "belongs to tenant b") {
		t.Fatalf("the task list showed another tenant's task")
	}
	if !strings.Contains(page, "belongs to tenant a") {
		t.Fatalf("the task list omitted this tenant's task")
	}
}

func TestUserContentContainingHTMLIsEscaped(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	const payload = `<script>alert("xss")</script>`
	ref := b.createTask("infra", payload)

	resp := b.post("/tasks/"+ref+"/comments", url.Values{"body": {payload}})
	wantStatus(t, resp, http.StatusSeeOther)
	_ = resp.Body.Close()

	for _, path := range []string{"/tasks", "/tasks/" + ref} {
		page := b.page(path)
		if strings.Contains(page, payload) {
			t.Fatalf("page %s rendered user content as markup", path)
		}
		if !strings.Contains(page, "&lt;script&gt;") {
			t.Fatalf("page %s does not show the escaped content", path)
		}
	}
}

func TestBrandingFollowsTheTenantAndDoesNotLeak(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	alice := f.as("alice").page("/projects")
	bob := f.as("bob").page("/projects")

	if !strings.Contains(alice, "Acme Works") || strings.Contains(alice, "Other Ltd") {
		t.Fatalf("tenant A page does not carry only its own branding")
	}
	if !strings.Contains(bob, "Other Ltd") || strings.Contains(bob, "Acme Works") {
		t.Fatalf("tenant B page does not carry only its own branding")
	}
	for _, accent := range []string{accentOf(t, alice), accentOf(t, bob)} {
		if len(accent) != 7 || accent[0] != '#' {
			t.Fatalf("accent %q is not a hex colour this package wrote", accent)
		}
	}
}

// accentOf reads the accent colour a page declares.
func accentOf(t *testing.T, page string) string {
	t.Helper()
	const marker = "--accent: "
	idx := strings.Index(page, marker)
	if idx < 0 {
		t.Fatalf("page declares no accent colour")
	}
	rest := page[idx+len(marker):]
	end := strings.Index(rest, ";")
	if end < 0 {
		t.Fatalf("page declares a malformed accent colour")
	}
	return rest[:end]
}

func TestDefaultBrandingAppliesWithoutATenant(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	page := f.as("").page("/login")
	if !strings.Contains(page, "<title>Sign in &middot; tix</title>") {
		t.Fatalf("the sign-in screen does not carry the default branding")
	}
}

func TestErrorPageCarriesNoStackTraceOrSQL(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	resp := b.get("/tasks/infra-9999")
	page := body(t, resp)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	for _, forbidden := range []string{"SELECT", "select ", "INSERT", "goroutine",
		".go:", "internal/service", "sqlite"} {
		if strings.Contains(page, forbidden) {
			t.Fatalf("error page discloses %q:\n%s", forbidden, page)
		}
	}
}

func TestInternalFailureIsReportedGenerically(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.svc.exported = ""
	b := f.as("alice")

	resp := b.post("/transfer/export", url.Values{})
	page := body(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if strings.Contains(page, "goroutine") {
		t.Fatalf("export disclosed a stack trace")
	}
}

func TestWebhookSecretIsNeverDisplayed(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	resp := b.post("/admin/webhooks", url.Values{
		"url": {"https://example.test/hook"}, "secret": {"super-secret-value"},
		"event_types": {"task.*"}, "active": {"1"}})
	wantStatus(t, resp, http.StatusSeeOther)
	_ = resp.Body.Close()

	page := b.page("/admin/webhooks")
	if strings.Contains(page, "super-secret-value") {
		t.Fatalf("the webhook signing secret was displayed")
	}
	if !strings.Contains(page, "example.test/hook") {
		t.Fatalf("the endpoint was not listed")
	}
}
