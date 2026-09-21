package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/auth"
)

func TestLoginIssuesASessionCookie(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	admin := f.as("alice")

	created := admin.post("/admin/users", url.Values{
		"email": {"operator@example.test"}, "password": {"correct-horse-battery"},
		"display_name": {"Operator"}, "role": {"member"}})
	_ = created.Body.Close()
	wantStatus(t, created, http.StatusSeeOther)

	anonymous := f.as("")
	resp := anonymous.post("/login", url.Values{
		"email": {"operator@example.test"}, "password": {"correct-horse-battery"},
		"next": {"/tasks"}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)
	if got := resp.Header.Get("Location"); got != "/tasks" {
		t.Fatalf("location = %q, want /tasks", got)
	}
	if !strings.Contains(resp.Header.Get("Set-Cookie"), auth.SessionCookieName) {
		t.Fatalf("no session cookie was issued")
	}
}

func TestLoginRefusesWrongCredentials(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	resp := f.as("").post("/login", url.Values{
		"email": {"nobody@example.test"}, "password": {"not-the-password"}})
	page := body(t, resp)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	if !strings.Contains(page, "invalid credentials") {
		t.Fatalf("the refusal does not explain itself:\n%s", page)
	}
}

func TestLoginRedirectStaysOnThisOrigin(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	admin := f.as("alice")
	created := admin.post("/admin/users", url.Values{
		"email": {"elsewhere@example.test"}, "password": {"correct-horse-battery"},
		"role": {"member"}})
	_ = created.Body.Close()

	resp := f.as("").post("/login", url.Values{
		"email": {"elsewhere@example.test"}, "password": {"correct-horse-battery"},
		"next": {"//evil.example/steal"}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)
	if got := resp.Header.Get("Location"); got != "/projects" {
		t.Fatalf("location = %q, want the off-origin target to be discarded", got)
	}
}
