// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// makeGlobalAdmin gives alice a user row that is a global admin, so the
// fixture's "alice" reads as one.
func (f *fixture) makeGlobalAdmin() {
	f.t.Helper()
	ctx := context.Background()
	if err := f.store.Update(ctx, core.TenantScope{TenantID: f.tenantA.ID}, func(tx store.Tx) error {
		if err := tx.CreateUser(ctx, &core.User{ID: f.actorA.ID, Email: "alice@acme.test"}, ""); err != nil {
			return err
		}
		return tx.SetUserGlobalAdmin(ctx, f.actorA.ID, true)
	}); err != nil {
		f.t.Fatalf("making alice a global admin: %v", err)
	}
}

func TestTheGlobalAdminEntryIsOfferedOnlyToAGlobalAdmin(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	if strings.Contains(nav(t, f.as("alice").page("/tasks")), `href="/admin/global"`) {
		t.Error("a tenant admin is offered the global admin area")
	}
	resp := f.as("alice").get("/admin/global")
	wantStatus(t, resp, http.StatusForbidden)
	_ = resp.Body.Close()

	f.makeGlobalAdmin()
	if !strings.Contains(nav(t, f.as("alice").page("/tasks")), `href="/admin/global"`) {
		t.Error("a global admin is not offered the global admin area")
	}
}

var copyValue = regexp.MustCompile(`<code id="initial-password-[a-z]">([^<]+)</code>`)

func TestCreatingATenantShowsItsGeneratedPasswordsOnce(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.makeGlobalAdmin()
	b := f.as("alice")
	b.page("/admin/global")

	resp := b.post("/admin/global/tenants", url.Values{
		"key": {"test"}, "name": {"Test"}, "admins": {"villy@z09.test\nicek@z09.test"},
	})
	wantStatus(t, resp, http.StatusSeeOther)
	_ = resp.Body.Close()

	shown := b.page("/admin/global")
	secret := between(t, shown, `<section class="secret"`, "</section>")
	values := copyValue.FindAllStringSubmatch(secret, -1)
	if len(values) != 2 {
		t.Fatalf("the screen shows %d passwords, want 2:\n%s", len(values), secret)
	}
	for _, email := range []string{"villy@z09.test", "icek@z09.test"} {
		if !strings.Contains(secret, email) {
			t.Errorf("the passwords are not labelled with %s", email)
		}
	}
	if !strings.Contains(secret, `data-copy-target="initial-password-a"`) {
		t.Error("the first password has no copy button")
	}

	again := b.page("/admin/global")
	for _, v := range values {
		if strings.Contains(again, v[1]) {
			t.Errorf("a reload shows the password for the second time")
		}
	}
	users := b.page("/admin/global/tenants/test")
	if !strings.Contains(users, "villy@z09.test") || !strings.Contains(users, `role-admin`) {
		t.Errorf("the new tenant's users screen does not show villy as admin")
	}
}

func TestATenantAdminCannotUseTheGlobalAdminArea(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.page("/tasks")
	for path, form := range map[string]url.Values{
		"/admin/global/tenants":        {"key": {"test"}, "name": {"Test"}},
		"/admin/global/admins":         {"email": {"alice@acme.test"}},
		"/admin/global/users":          {"target": {"other"}, "email": {"x@other.test"}},
		"/admin/global/tenants/delete": {"target": {"other"}},
	} {
		resp := b.post(path, form)
		if resp.StatusCode == http.StatusSeeOther {
			t.Errorf("POST %s by a tenant admin succeeded", path)
		}
		if strings.Contains(readAll(t, resp), "taken from the signed-in session") {
			t.Errorf("POST %s was refused for naming a tenant, before the service decided", path)
		}
	}
	resp := b.get("/admin/global/tenants/other")
	wantStatus(t, resp, http.StatusForbidden)
	_ = resp.Body.Close()
}

func TestTheGlobalAdminScreenRenamesDeletesGrantsAndRevokes(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.makeGlobalAdmin()
	b := f.as("alice")
	b.page("/admin/global")

	for _, step := range []struct {
		path string
		form url.Values
	}{
		{"/admin/global/tenants/rename", url.Values{"target": {"other"}, "name": {"Renamed Ltd"}}},
		{"/admin/global/admins", url.Values{"email": {"alice@acme.test"}}},
	} {
		resp := b.post(step.path, step.form)
		wantStatus(t, resp, http.StatusSeeOther)
		_ = resp.Body.Close()
	}
	if !strings.Contains(b.page("/admin/global"), "Renamed Ltd") {
		t.Error("the renamed tenant is not listed under its new name")
	}

	refused := b.post("/admin/global/tenants/rename", url.Values{"target": {"other"}, "name": {""}})
	if refused.StatusCode == http.StatusSeeOther {
		t.Error("renaming a tenant to nothing was accepted")
	}
	_ = refused.Body.Close()
	unknown := b.post("/admin/global/admins", url.Values{"email": {"nobody@acme.test"}})
	if unknown.StatusCode == http.StatusSeeOther {
		t.Error("granting an unknown address was accepted")
	}
	_ = unknown.Body.Close()
	last := b.post("/admin/global/admins/revoke", url.Values{"email": {"alice@acme.test"}})
	if last.StatusCode == http.StatusSeeOther {
		t.Error("revoking the last global admin was accepted")
	}
	_ = last.Body.Close()

	deleted := b.post("/admin/global/tenants/delete", url.Values{"target": {"other"}})
	wantStatus(t, deleted, http.StatusSeeOther)
	_ = deleted.Body.Close()
	if strings.Contains(b.page("/admin/global"), "Renamed Ltd") {
		t.Error("the deleted tenant is still listed")
	}
}

func TestTheGlobalAdminManagesAnotherTenantsUsersInTheBrowser(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.makeGlobalAdmin()
	b := f.as("alice")
	b.page("/admin/global/tenants/other")

	created := b.post("/admin/global/users", url.Values{"target": {"other"}, "email": {"carol@other.test"},
		"password": {"correct-horse-battery"}, "role": {"viewer"}})
	wantStatus(t, created, http.StatusSeeOther)
	_ = created.Body.Close()

	page := b.page("/admin/global/tenants/other")
	id := userIDFor(t, page, "carol@other.test")

	refused := b.post("/admin/global/users", url.Values{"target": {"other"}, "email": {""}})
	if refused.StatusCode == http.StatusSeeOther {
		t.Error("a user with no address was created")
	}
	if body := readAll(t, refused); !strings.Contains(body, "Users of") {
		t.Error("the refusal did not draw the tenant's users screen again")
	}

	updated := b.post("/admin/global/users/update", url.Values{"target": {"other"}, "id": {id},
		"display_name": {"Carol"}, "role": {"admin"}})
	wantStatus(t, updated, http.StatusSeeOther)
	_ = updated.Body.Close()
	if got := selectedRole(t, b.page("/admin/global/tenants/other"), id); got != "admin" {
		t.Errorf("carol's role after the update is %q, want admin", got)
	}

	deleted := b.post("/admin/global/users/delete", url.Values{"target": {"other"}, "id": {id}})
	wantStatus(t, deleted, http.StatusSeeOther)
	_ = deleted.Body.Close()
	if strings.Contains(b.page("/admin/global/tenants/other"), "carol@other.test") {
		t.Error("the deleted user is still listed")
	}
	for path, form := range map[string]url.Values{
		"/admin/global/users/update": {"target": {"other"}, "id": {id}, "role": {"admin"}},
		"/admin/global/users/delete": {"target": {"other"}, "id": {id}},
	} {
		resp := b.post(path, form)
		if resp.StatusCode == http.StatusSeeOther {
			t.Errorf("POST %s for a deleted user succeeded", path)
		}
		_ = resp.Body.Close()
	}
}

func TestCredentialSecretsIgnoreAMangledCookie(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.makeGlobalAdmin()
	b := f.as("alice")
	b.page("/admin/global")
	for _, value := range []string{"!!!not-base64", "bm90LWpzb24"} {
		u, _ := url.Parse(f.server.URL + "/admin/global")
		b.client.Jar.SetCookies(u, []*http.Cookie{{Name: "tix_initial_credentials", Value: value, Path: "/admin/global"}})
		if strings.Contains(b.page("/admin/global"), `<section class="secret"`) {
			t.Errorf("a cookie holding %q drew a passwords section", value)
		}
	}
}
