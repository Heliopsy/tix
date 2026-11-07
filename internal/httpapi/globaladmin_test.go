// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/sqlite"
	"github.com/heliopsy/tix/internal/wire"
)

// globalAdminToken gives tenant A a global admin, an account linked to an
// admin actor of its own, and mints a token for that actor naming
// global:admin.
func (f *apiFixture) globalAdminToken() string {
	f.t.Helper()
	if f.globalA.ID == "" {
		f.globalA = seedLinkedActor(f.t, f.store, f.tenantA.ID, "root", "root@acme.test", true)
	}
	return f.mintScoped(f.tenantA.ID, f.globalA.ID, core.ScopeAll, core.ScopeGlobalAdmin)
}

// seedLinkedActor writes an account and an admin actor of tenantID linked to
// it, as user administration does.
func seedLinkedActor(t *testing.T, st *sqlite.Store, tenantID, handle, email string, global bool) core.Actor {
	t.Helper()
	ctx := context.Background()
	user := core.User{Email: email}
	actor := core.Actor{Kind: core.ActorUser, Handle: handle, Role: core.RoleAdmin}
	if err := st.Update(ctx, core.TenantScope{TenantID: tenantID}, func(tx store.Tx) error {
		if err := tx.CreateUser(ctx, &user, ""); err != nil {
			return err
		}
		actor.UserID = user.ID
		if err := tx.CreateActor(ctx, &actor); err != nil {
			return err
		}
		if err := tx.AddMember(ctx, &core.Membership{ActorID: actor.ID, Role: core.RoleAdmin}); err != nil {
			return err
		}
		return tx.SetUserGlobalAdmin(ctx, user.ID, global)
	}); err != nil {
		t.Fatalf("creating account %q: %v", email, err)
	}
	actor.TenantID = tenantID
	return actor
}

// mintScoped mints a token with exactly these scopes.
func (f *apiFixture) mintScoped(tenantID, actorID string, scopes ...core.Scope) string {
	f.t.Helper()
	minted, err := auth.MintAPIToken(f.clock, tenantID, core.CreateTokenInput{
		Name: "scoped-" + string(scopes[len(scopes)-1]), ActorID: actorID, Scopes: scopes,
	})
	if err != nil {
		f.t.Fatalf("minting token: %v", err)
	}
	stored := minted.Issued.APIToken
	f.lookup.byHash[minted.Hash] = &stored
	return minted.Issued.Token
}

func TestCreateTenantOverHTTPWithAdmins(t *testing.T) {
	f := newFixture(t)
	token := f.globalAdminToken()

	resp := f.do(http.MethodPost, wire.RouteTenants, f.hostA, token, map[string]any{
		"key": "test", "name": "Test",
		"admins": []map[string]any{{"email": "villy@z09.test"}, {"email": "icek@z09.test", "handle": "icek"}},
	})
	mustStatus(t, resp, http.StatusCreated)
	var got core.CreatedTenant
	decodeBody(t, resp, &got)
	if got.Key != "test" || len(got.Admins) != 2 || len(got.InitialCredentials) != 2 {
		t.Fatalf("created = %+v, want tenant test with two admins and two credentials", got)
	}
	for _, c := range got.InitialCredentials {
		if len(strings.ReplaceAll(c.Password, "-", "")) < 20 {
			t.Errorf("generated password for %s is %q, want at least 20 characters", c.Email, c.Password)
		}
		if strings.Contains(f.logs.String(), c.Password) {
			t.Errorf("the server log carries %s's generated password", c.Email)
		}
	}

	listed := f.do(http.MethodGet, "/api/v1/tenants/test/users", f.hostA, token, nil)
	mustStatus(t, listed, http.StatusOK)
	var page wire.Page[core.TenantUser]
	decodeBody(t, listed, &page)
	if len(page.Items) != 2 {
		t.Fatalf("tenant test lists %d users, want 2", len(page.Items))
	}
	for _, u := range page.Items {
		if u.Role != core.RoleAdmin {
			t.Errorf("%s holds %q in the new tenant, want admin", u.Email, u.Role)
		}
	}
}

// A tenant admin, which is what tenant:admin and "*" make a token, may no
// longer create a tenant, nor reach another tenant's users by naming it.
func TestTenantAdminIsRefusedTheGlobalRoutes(t *testing.T) {
	f := newFixture(t)

	created := f.call(http.MethodPost, wire.RouteTenants, core.CreateTenantInput{Key: "third", Name: "Third"})
	mustStatus(t, created, http.StatusForbidden)
	_ = created.Body.Close()

	for _, path := range []string{"/api/v1/tenants/other/users", "/api/v1/tenants/nosuch/users"} {
		resp := f.call(http.MethodGet, path, nil)
		mustStatus(t, resp, http.StatusForbidden)
		_ = resp.Body.Close()
	}
	granted := f.call(http.MethodPost, wire.RouteGlobalAdmins, core.GlobalAdminInput{Email: "bob@other.test"})
	mustStatus(t, granted, http.StatusForbidden)
	_ = granted.Body.Close()
}

// A token naming global:admin is still refused when its actor is not a
// global admin: the scope alone carries nothing.
func TestGlobalAdminScopeWithoutTheFlagIsRefused(t *testing.T) {
	f := newFixture(t)
	token := f.mintScoped(f.tenantA.ID, f.actorA.ID, core.ScopeAll, core.ScopeGlobalAdmin)
	resp := f.do(http.MethodPost, wire.RouteTenants, f.hostA, token, core.CreateTenantInput{Key: "third", Name: "Third"})
	mustStatus(t, resp, http.StatusForbidden)
	_ = resp.Body.Close()
}

func TestGlobalAdminManagesUsersOfAnotherTenantOverHTTP(t *testing.T) {
	f := newFixture(t)
	token := f.globalAdminToken()

	created := f.do(http.MethodPost, "/api/v1/tenants/other/users", f.hostA, token,
		core.CreateUserInput{Email: "carol@other.test", Password: "correct-horse-battery", Role: core.RoleMember})
	mustStatus(t, created, http.StatusCreated)
	var user core.User
	decodeBody(t, created, &user)

	role := core.RoleAdmin
	updated := f.do(http.MethodPatch, "/api/v1/tenants/other/users/"+user.ID, f.hostA, token,
		core.UpdateUserInput{Role: &role})
	mustStatus(t, updated, http.StatusOK)
	_ = updated.Body.Close()

	deleted := f.do(http.MethodDelete, "/api/v1/tenants/other/users/"+user.ID, f.hostA, token, nil)
	mustStatus(t, deleted, http.StatusNoContent)
	_ = deleted.Body.Close()
}

func TestGlobalAdminRoutesGrantListAndRevoke(t *testing.T) {
	f := newFixture(t)
	token := f.globalAdminToken()
	ctx := context.Background()
	if err := f.store.Update(ctx, core.TenantScope{TenantID: f.tenantB.ID}, func(tx store.Tx) error {
		return tx.CreateUser(ctx, &core.User{ID: f.actorB.ID, Email: "bob@other.test"}, "")
	}); err != nil {
		t.Fatal(err)
	}

	granted := f.do(http.MethodPost, wire.RouteGlobalAdmins, f.hostA, token, core.GlobalAdminInput{Email: "bob@other.test"})
	mustStatus(t, granted, http.StatusOK)
	var user core.User
	decodeBody(t, granted, &user)
	if !user.GlobalAdmin {
		t.Errorf("granted = %+v, want a global admin", user)
	}

	listed := f.do(http.MethodGet, wire.RouteGlobalAdmins, f.hostA, token, nil)
	mustStatus(t, listed, http.StatusOK)
	var page wire.Page[core.User]
	decodeBody(t, listed, &page)
	if len(page.Items) != 2 {
		t.Errorf("listing %d global admins, want 2", len(page.Items))
	}

	revoked := f.do(http.MethodDelete, "/api/v1/global-admins/bob@other.test", f.hostA, token, nil)
	mustStatus(t, revoked, http.StatusNoContent)
	_ = revoked.Body.Close()
	last := f.do(http.MethodDelete, "/api/v1/global-admins/root@acme.test", f.hostA, token, nil)
	mustStatus(t, last, http.StatusUnprocessableEntity)
	_ = last.Body.Close()

	refused := f.call(http.MethodGet, wire.RouteGlobalAdmins, nil)
	mustStatus(t, refused, http.StatusForbidden)
	_ = refused.Body.Close()
}

func TestTenantUserRoutesRefuseBadInput(t *testing.T) {
	f := newFixture(t)
	token := f.globalAdminToken()
	for _, tc := range []struct {
		method, path string
		body         any
		want         int
	}{
		{http.MethodPost, "/api/v1/tenants/other/users", "not an object", http.StatusBadRequest},
		{http.MethodPatch, "/api/v1/tenants/other/users/nosuch", "not an object", http.StatusBadRequest},
		{http.MethodPost, wire.RouteGlobalAdmins, "not an object", http.StatusBadRequest},
		{http.MethodPost, "/api/v1/tenants/nosuch/users", core.CreateUserInput{Email: "x@y.test"}, http.StatusNotFound},
		{http.MethodPatch, "/api/v1/tenants/other/users/nosuch", core.UpdateUserInput{}, http.StatusNotFound},
		{http.MethodDelete, "/api/v1/tenants/other/users/nosuch", nil, http.StatusNotFound},
		{http.MethodGet, "/api/v1/tenants/other/users?limit=-1", nil, http.StatusBadRequest},
	} {
		resp := f.do(tc.method, tc.path, f.hostA, token, tc.body)
		if resp.StatusCode != tc.want {
			t.Errorf("%s %s = %d, want %d: %s", tc.method, tc.path, resp.StatusCode, tc.want, readBody(t, resp))
			continue
		}
		_ = resp.Body.Close()
	}
}
