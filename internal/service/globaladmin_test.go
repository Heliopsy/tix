// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// globalAdmin creates a user in the actor's tenant, makes it a global admin
// from a shell holding the database, and returns a context signed in as it
// the way a browser session is: no token, the admin role in its own tenant.
func globalAdmin(t *testing.T, l *Local, actor *core.Actor, email string) (context.Context, *core.Actor) {
	t.Helper()
	shell := core.WithDirectStore(core.WithActor(context.Background(), actor))
	u, err := l.CreateUser(shell, core.CreateUserInput{Email: email, Role: core.RoleAdmin, Password: "correct-horse-battery"})
	if err != nil {
		t.Fatalf("CreateUser(%s): %v", email, err)
	}
	if _, err := l.GrantGlobalAdmin(shell, core.GlobalAdminInput{Email: email}); err != nil {
		t.Fatalf("GrantGlobalAdmin(%s): %v", email, err)
	}
	session := &core.Actor{ID: u.ID, TenantID: actor.TenantID, Kind: core.ActorUser, Handle: userHandle("", email), Role: core.RoleAdmin}
	return core.WithActor(context.Background(), session), session
}

// tenantAdmin is a signed-in admin of the actor's tenant who is not a global
// admin: what every tenant admin was before global admins existed.
func tenantAdmin(t *testing.T, l *Local, actor *core.Actor, email string) context.Context {
	t.Helper()
	shell := core.WithDirectStore(core.WithActor(context.Background(), actor))
	u, err := l.CreateUser(shell, core.CreateUserInput{Email: email, Role: core.RoleAdmin, Password: "correct-horse-battery"})
	if err != nil {
		t.Fatalf("CreateUser(%s): %v", email, err)
	}
	return core.WithActor(context.Background(), &core.Actor{ID: u.ID, TenantID: actor.TenantID,
		Kind: core.ActorUser, Handle: userHandle("", email), Role: core.RoleAdmin})
}

// signInAt signs in with a password. A generated first password has to be
// replaced on the way in, so a password_change challenge is answered with the
// same password and a suffix, which is what the person would do on the page.
func signInAt(t *testing.T, l *Local, tenantID, email, password string) (*core.Session, error) {
	t.Helper()
	ctx := core.WithTenant(context.Background(), core.TenantScope{TenantID: tenantID})
	in := core.LoginInput{Email: email, Password: password, Surface: core.SurfaceWeb}
	res, err := l.Login(ctx, in)
	if err != nil {
		return nil, err
	}
	if res.Challenge != nil && res.Challenge.Kind == core.ChallengePasswordChange {
		in.NewPassword = password + "-replaced"
		if res, err = l.Login(ctx, in); err != nil {
			return nil, err
		}
	}
	if res.Session == nil {
		t.Fatalf("signing in %s asked for a second factor", email)
	}
	return res.Session, nil
}

func TestCreateTenantWithAdminsTheyCanSignInAndAdministerOnlyIt(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	ctx, _ := globalAdmin(t, l, actor, "root@acme.test")
	membersBefore, err := l.ListMembers(ctx)
	if err != nil {
		t.Fatal(err)
	}

	created, err := l.CreateTenant(ctx, core.CreateTenantInput{Key: "test", Name: "Test", Admins: []core.TenantAdminInput{
		{Email: "Villy@Z09.test"},
		{Email: "icek@z09.test", DisplayName: "Icek"},
	}})
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if len(created.Admins) != 2 || len(created.InitialCredentials) != 2 {
		t.Fatalf("created = %+v, want two admins with generated passwords", created)
	}
	if created.InitialCredentials[0].Email != "villy@z09.test" || created.Admins[0].Handle != "villy" {
		t.Errorf("first admin = %+v / %+v, want the address lowercased and the handle from it",
			created.Admins[0], created.InitialCredentials[0])
	}

	newCtx := core.WithActor(context.Background(), core.SystemActor(created.ID))
	members, err := l.ListMembers(newCtx)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 2 {
		t.Fatalf("the new tenant has %d members, want the two admins and not the caller", len(members))
	}
	for _, m := range members {
		if m.Role != core.RoleAdmin {
			t.Errorf("member %s holds %q, want admin", m.ActorID, m.Role)
		}
	}

	for _, c := range created.InitialCredentials {
		sess, err := signInAt(t, l, created.ID, c.Email, c.Password)
		if err != nil {
			t.Fatalf("%s signing in to the new tenant with the returned password: %v", c.Email, err)
		}
		if _, err := signInAt(t, l, scope.TenantID, c.Email, c.Password); !core.IsKind(err, core.KindUnauthenticated) {
			t.Errorf("%s signing in to the caller's tenant = %v, want refused", c.Email, err)
		}
		admin := core.WithActor(context.Background(), &core.Actor{ID: sess.ActorID, TenantID: sess.TenantID,
			Kind: core.ActorUser, Role: core.RoleAdmin})
		if _, _, err := l.ListUsers(admin, core.UserFilter{}); err != nil {
			t.Errorf("%s administering the new tenant: %v", c.Email, err)
		}
		if _, err := l.GetTenant(admin, scope.TenantID); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("%s reading the caller's tenant = %v, want not found", c.Email, err)
		}
		if _, err := l.CreateTenant(admin, core.CreateTenantInput{Key: "more", Name: "More"}); !core.IsKind(err, core.KindForbidden) {
			t.Errorf("%s creating a tenant = %v, want forbidden: an admin of a tenant is not a global admin", c.Email, err)
		}
	}

	membersAfter, err := l.ListMembers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(membersAfter) != len(membersBefore) {
		t.Errorf("the caller's tenant went from %d members to %d", len(membersBefore), len(membersAfter))
	}
}

func TestCreateTenantKeepsASuppliedPasswordAndReturnsNoCredentialForIt(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx, _ := globalAdmin(t, l, actor, "root@acme.test")
	created, err := l.CreateTenant(ctx, core.CreateTenantInput{Key: "test", Name: "Test", Admins: []core.TenantAdminInput{
		{Email: "ada@z09.test", Password: "a-password-ada-chose"},
		{Email: "bob@z09.test"},
	}})
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if len(created.InitialCredentials) != 1 || created.InitialCredentials[0].Email != "bob@z09.test" {
		t.Fatalf("credentials = %+v, want only bob's generated one", created.InitialCredentials)
	}
	if created.Admins[0].PasswordGenerated || !created.Admins[1].PasswordGenerated {
		t.Errorf("admins = %+v, want only the second marked generated", created.Admins)
	}
	if _, err := signInAt(t, l, created.ID, "ada@z09.test", "a-password-ada-chose"); err != nil {
		t.Errorf("ada signing in with her own password: %v", err)
	}
}

func TestCreateTenantWithoutAdminsStillWorks(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx, _ := globalAdmin(t, l, actor, "root@acme.test")
	created, err := l.CreateTenant(ctx, core.CreateTenantInput{Key: "bare", Name: "Bare"})
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if len(created.Admins) != 0 || len(created.InitialCredentials) != 0 {
		t.Errorf("created = %+v, want no admins", created)
	}
}

// An address that already has an account becomes an admin of the new tenant
// with the password it has: nothing is generated for it, and its other tenant
// is untouched.
func TestCreateTenantMakesAnExistingAccountAnAdmin(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	ctx, _ := globalAdmin(t, l, actor, "root@acme.test")
	created, err := l.CreateTenant(ctx, core.CreateTenantInput{Key: "test", Name: "Test", Admins: []core.TenantAdminInput{
		{Email: "new@z09.test"}, {Email: "ROOT@acme.test"},
	}})
	if err != nil {
		t.Fatalf("CreateTenant naming an existing account = %v, want it made an admin", err)
	}
	if len(created.Admins) != 2 || len(created.InitialCredentials) != 1 ||
		created.InitialCredentials[0].Email != "new@z09.test" {
		t.Fatalf("created = %+v, want two admins and a credential for the new address alone", created)
	}
	if _, err := signInAt(t, l, created.ID, "root@acme.test", "correct-horse-battery"); err != nil {
		t.Fatalf("the existing account signing in to the new tenant with its own password = %v", err)
	}
	if _, err := signInAt(t, l, scope.TenantID, "root@acme.test", "correct-horse-battery"); err != nil {
		t.Fatalf("the existing account signing in to its first tenant = %v, want it unchanged", err)
	}
}

// A password supplied for an address that already has an account is refused
// rather than ignored, and nothing is created.
func TestCreateTenantRefusesAPasswordForAnExistingAccount(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx, _ := globalAdmin(t, l, actor, "root@acme.test")
	_, err := l.CreateTenant(ctx, core.CreateTenantInput{Key: "test", Name: "Test", Admins: []core.TenantAdminInput{
		{Email: "new@z09.test"}, {Email: "root@acme.test", Password: "another-password-entirely"},
	}})
	if !core.IsKind(err, core.KindInvalid) || !strings.Contains(err.Error(), "root@acme.test") {
		t.Fatalf("CreateTenant with a password for an existing account = %v, want invalid naming it", err)
	}
	assertNoTenant(t, l, "test")
	assertNoUser(t, l, "new@z09.test")
}
func TestCreateTenantAdminValidation(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx, _ := globalAdmin(t, l, actor, "root@acme.test")
	many := make([]core.TenantAdminInput, core.MaxTenantAdmins+1)
	for i := range many {
		many[i] = core.TenantAdminInput{Email: string(rune('a'+i)) + "@many.test"}
	}
	tests := []struct {
		name   string
		admins []core.TenantAdminInput
	}{
		{"no at sign", []core.TenantAdminInput{{Email: "villy.z09.test"}}},
		{"nothing before the at", []core.TenantAdminInput{{Email: "@z09.test"}}},
		{"blank", []core.TenantAdminInput{{Email: "  "}}},
		{"space inside", []core.TenantAdminInput{{Email: "vil ly@z09.test"}}},
		{"the same address twice, in two cases", []core.TenantAdminInput{{Email: "v@z09.test"}, {Email: "V@Z09.test"}}},
		{"two admins sharing a handle", []core.TenantAdminInput{{Email: "v@a.test"}, {Email: "v@b.test"}}},
		{"a short password", []core.TenantAdminInput{{Email: "v@z09.test", Password: "short"}}},
		{"too many", many},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := l.CreateTenant(ctx, core.CreateTenantInput{Key: "test", Name: "Test", Admins: tc.admins})
			if !core.IsKind(err, core.KindInvalid) {
				t.Fatalf("CreateTenant = %v, want invalid", err)
			}
			assertNoTenant(t, l, "test")
		})
	}
}

// The generated password is returned to the caller and nowhere else: no audit
// entry and no event in either tenant carries it.
func TestGeneratedPasswordsAppearInNoAuditEntryOrEvent(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	ctx, _ := globalAdmin(t, l, actor, "root@acme.test")
	created, err := l.CreateTenant(ctx, core.CreateTenantInput{Key: "test", Name: "Test", Admins: []core.TenantAdminInput{
		{Email: "villy@z09.test"}, {Email: "icek@z09.test", Password: "icek-chose-this-one"},
	}})
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	secrets := []string{"icek-chose-this-one"}
	for _, c := range created.InitialCredentials {
		secrets = append(secrets, c.Password)
	}
	recorded := map[string]int{}
	for _, tenantID := range []string{scope.TenantID, created.ID} {
		if err := l.store.View(context.Background(), core.TenantScope{TenantID: tenantID}, func(tx store.Tx) error {
			entries, err := tx.ListAudit(context.Background(), core.AuditFilter{Page: core.Page{Limit: 1000}})
			if err != nil {
				return err
			}
			events, err := tx.ReadEvents(context.Background(), 0, 1000)
			if err != nil {
				return err
			}
			for _, e := range entries {
				recorded[e.Action]++
				raw, _ := json.Marshal(e)
				assertCarriesNone(t, "audit entry "+e.Action, string(raw), secrets)
			}
			for _, e := range events {
				raw, _ := json.Marshal(e)
				assertCarriesNone(t, "event "+string(e.Type), string(raw), secrets)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if recorded[auditUserCreate] < 2 || recorded[auditTenantCreate] != 1 {
		t.Errorf("recorded %v, want the tenant creation and both users", recorded)
	}
}

func assertCarriesNone(t *testing.T, what, text string, secrets []string) {
	t.Helper()
	for _, s := range secrets {
		if strings.Contains(text, s) {
			t.Errorf("%s carries a password: %s", what, text)
		}
	}
}

func assertNoTenant(t *testing.T, l *Local, key string) {
	t.Helper()
	if err := l.store.ViewUnscoped(context.Background(), func(u store.UnscopedTx) error {
		_, err := u.GetTenantByKey(context.Background(), key)
		if !core.IsKind(err, core.KindNotFound) {
			t.Errorf("tenant %q = %v, want not found", key, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func assertNoUser(t *testing.T, l *Local, email string) {
	t.Helper()
	if err := l.store.ViewUnscoped(context.Background(), func(u store.UnscopedTx) error {
		_, _, err := u.GetUserByEmail(context.Background(), email)
		if !core.IsKind(err, core.KindNotFound) {
			t.Errorf("user %q = %v, want not found", email, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// failingSeed is a store whose transactions in any tenant but one fail, which
// is how the seeding of a new tenant is made to fail after its row exists.
type failingSeed struct {
	store.Store
	allowed string
}

func (f *failingSeed) Update(ctx context.Context, scope core.TenantScope, fn func(store.Tx) error) error {
	if scope.TenantID != f.allowed {
		return errors.New("the disk filled up")
	}
	return f.Store.Update(ctx, scope, fn)
}

func TestCreateTenantRemovesTheTenantWhenSettingItUpFails(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	ctx, _ := globalAdmin(t, l, actor, "root@acme.test")
	broken := New(&failingSeed{Store: l.store, allowed: scope.TenantID}, WithClock(l.clock), WithHasher(l.hasher))

	_, err := broken.CreateTenant(ctx, core.CreateTenantInput{Key: "test", Name: "Test",
		Admins: []core.TenantAdminInput{{Email: "villy@z09.test"}}})
	if err == nil {
		t.Fatal("CreateTenant succeeded while the new tenant could not be written")
	}
	assertNoTenant(t, l, "test")
	assertNoUser(t, l, "villy@z09.test")

	var actions []string
	if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		entries, err := tx.ListAudit(context.Background(), core.AuditFilter{Page: core.Page{Limit: 1000}})
		for _, e := range entries {
			actions = append(actions, e.Action)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(actions, " ")
	if !strings.Contains(joined, auditTenantCreate) || !strings.Contains(joined, auditTenantDelete) {
		t.Errorf("the caller's audit holds %v, want the creation and the removal that undid it", actions)
	}

	if _, err := l.CreateTenant(ctx, core.CreateTenantInput{Key: "test", Name: "Test",
		Admins: []core.TenantAdminInput{{Email: "villy@z09.test"}}}); err != nil {
		t.Errorf("retrying after the failure: %v", err)
	}
}

func TestOnlyAGlobalAdminCreatesATenant(t *testing.T) {
	l, _, _, actor := newLocal(t)
	admin := tenantAdmin(t, l, actor, "admin@acme.test")
	if _, err := l.CreateTenant(admin, core.CreateTenantInput{Key: "test", Name: "Test"}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("a tenant admin creating a tenant = %v, want forbidden", err)
	}
	wildcard := core.WithActor(context.Background(), actor)
	if _, err := l.CreateTenant(wildcard, core.CreateTenantInput{Key: "test", Name: "Test"}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("an actor holding * creating a tenant = %v, want forbidden", err)
	}
	assertNoTenant(t, l, "test")
}

func TestGlobalAdminGrantRevokeAndTheLastOne(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	ctx, _ := globalAdmin(t, l, actor, "root@acme.test")
	tenantAdmin(t, l, actor, "second@acme.test")

	if err := l.RevokeGlobalAdmin(ctx, "root@acme.test"); !core.IsKind(err, core.KindPrecondition) {
		t.Fatalf("revoking the last global admin = %v, want refused", err)
	}
	if _, err := l.GrantGlobalAdmin(ctx, core.GlobalAdminInput{Email: "Second@acme.test"}); err != nil {
		t.Fatalf("GrantGlobalAdmin: %v", err)
	}
	listed, err := l.ListGlobalAdmins(ctx)
	if err != nil || len(listed) != 2 {
		t.Fatalf("ListGlobalAdmins = %d, %v, want 2", len(listed), err)
	}
	if err := l.RevokeGlobalAdmin(ctx, "second@acme.test"); err != nil {
		t.Fatalf("RevokeGlobalAdmin: %v", err)
	}
	if err := l.RevokeGlobalAdmin(ctx, "second@acme.test"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("revoking somebody who is not one = %v, want not found", err)
	}
	if _, err := l.GrantGlobalAdmin(ctx, core.GlobalAdminInput{Email: "nobody@acme.test"}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("granting to an unknown address = %v, want not found", err)
	}

	recorded := map[string]int{}
	if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		entries, err := tx.ListAudit(context.Background(), core.AuditFilter{Page: core.Page{Limit: 1000}})
		for _, e := range entries {
			recorded[e.Action]++
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if recorded[auditGlobalAdminGrant] != 2 || recorded[auditGlobalAdminRevoke] != 1 {
		t.Errorf("audit recorded %v, want two grants and one revoke", recorded)
	}
}

func TestATenantAdminCannotGrantGlobalAdmin(t *testing.T) {
	l, _, _, actor := newLocal(t)
	globalAdmin(t, l, actor, "root@acme.test")
	admin := tenantAdmin(t, l, actor, "admin@acme.test")
	if _, err := l.GrantGlobalAdmin(admin, core.GlobalAdminInput{Email: "admin@acme.test"}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("a tenant admin granting themselves global admin = %v, want forbidden", err)
	}
	if _, err := l.ListGlobalAdmins(admin); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("a tenant admin listing global admins = %v, want forbidden", err)
	}
}

// A revoke reaches a session already signed in, because the flag is read from
// the store on every call rather than carried on the session.
func TestRevokingGlobalAdminTakesEffectOnALiveSession(t *testing.T) {
	l, _, _, actor := newLocal(t)
	root, _ := globalAdmin(t, l, actor, "root@acme.test")
	other, _ := globalAdmin(t, l, actor, "other@acme.test")
	if err := l.RevokeGlobalAdmin(root, "other@acme.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := l.CreateTenant(other, core.CreateTenantInput{Key: "test", Name: "Test"}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("a revoked global admin creating a tenant = %v, want forbidden", err)
	}
}

func TestGlobalAdminTokenScopeCannotBeEscalated(t *testing.T) {
	l, _, _, actor := newLocal(t)
	root, self := globalAdmin(t, l, actor, "root@acme.test")
	admin := tenantAdmin(t, l, actor, "admin@acme.test")
	adminActor, _ := core.ActorFrom(admin)
	scopes := []core.Scope{core.ScopeAll, core.ScopeGlobalAdmin}

	if _, err := l.CreateToken(admin, core.CreateTokenInput{Name: "mine", Scopes: scopes}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("a tenant admin minting a global:admin token = %v, want forbidden", err)
	}
	if _, err := l.CreateToken(admin, core.CreateTokenInput{Name: "borrowed", ActorID: self.ID, Scopes: scopes}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("a tenant admin minting a global:admin token acting as a global admin = %v, want forbidden", err)
	}
	if _, err := l.CreateToken(root, core.CreateTokenInput{Name: "theirs", ActorID: adminActor.ID, Scopes: scopes}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("a global admin minting a global:admin token for a tenant admin = %v, want forbidden", err)
	}
	issued, err := l.CreateToken(root, core.CreateTokenInput{Name: "root", ActorID: self.ID, Scopes: scopes})
	if err != nil {
		t.Fatalf("a global admin minting their own global:admin token: %v", err)
	}
	asToken := core.WithActor(context.Background(), &core.Actor{ID: self.ID, TenantID: self.TenantID,
		Kind: core.ActorAgent, TokenID: issued.ID, Scopes: issued.Scopes})
	if _, err := l.CreateTenant(asToken, core.CreateTenantInput{Key: "bytoken", Name: "By token"}); err != nil {
		t.Errorf("creating a tenant with that token: %v", err)
	}
	wildcard := core.WithActor(context.Background(), &core.Actor{ID: self.ID, TenantID: self.TenantID,
		Kind: core.ActorAgent, TokenID: "t-wild", Scopes: []core.Scope{core.ScopeAll}})
	if _, err := l.CreateTenant(wildcard, core.CreateTenantInput{Key: "wild", Name: "Wild"}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("a global admin's * token creating a tenant = %v, want forbidden", err)
	}
}

func TestGlobalAdminManagesUsersOfAnotherTenant(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	root, _ := globalAdmin(t, l, actor, "root@acme.test")
	other, err := l.CreateTenant(root, core.CreateTenantInput{Key: "other", Name: "Other"})
	if err != nil {
		t.Fatal(err)
	}
	u, err := l.CreateTenantUser(root, "other", core.CreateUserInput{Email: "carol@other.test", Role: core.RoleViewer})
	if err != nil {
		t.Fatalf("CreateTenantUser: %v", err)
	}
	role := core.RoleAdmin
	if _, err := l.UpdateTenantUser(root, "other", u.ID, core.UpdateUserInput{Role: &role}); err != nil {
		t.Fatalf("UpdateTenantUser: %v", err)
	}
	users, _, err := l.ListTenantUsers(root, "other", core.UserFilter{})
	if err != nil || len(users) != 1 || users[0].Role != core.RoleAdmin {
		t.Fatalf("ListTenantUsers = %+v, %v, want carol as admin", users, err)
	}
	if _, err := l.UpdateTenantUser(root, scope.TenantID, u.ID, core.UpdateUserInput{Role: &role}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("reaching carol through the wrong tenant = %v, want not found", err)
	}
	if err := l.DeleteTenantUser(root, other.Key, u.ID); err != nil {
		t.Fatalf("DeleteTenantUser: %v", err)
	}

	all, _, err := l.ListTenants(root, core.Page{})
	if err != nil || len(all) != 2 {
		t.Errorf("a global admin listing tenants = %d, %v, want both", len(all), err)
	}
	renamed := "Renamed"
	if _, err := l.UpdateTenant(root, "other", core.UpdateTenantInput{Name: &renamed}); err != nil {
		t.Errorf("a global admin renaming another tenant: %v", err)
	}
	if err := l.DeleteTenant(root, "other"); err != nil {
		t.Errorf("a global admin deleting another tenant: %v", err)
	}
}

func TestATenantAdminCannotReachAnotherTenant(t *testing.T) {
	l, _, _, actor := newLocal(t)
	root, _ := globalAdmin(t, l, actor, "root@acme.test")
	if _, err := l.CreateTenant(root, core.CreateTenantInput{Key: "other", Name: "Other"}); err != nil {
		t.Fatal(err)
	}
	admin := tenantAdmin(t, l, actor, "admin@acme.test")
	for _, ref := range []string{"other", "nosuch"} {
		if _, _, err := l.ListTenantUsers(admin, ref, core.UserFilter{}); !core.IsKind(err, core.KindForbidden) {
			t.Errorf("ListTenantUsers(%s) = %v, want forbidden whether or not it exists", ref, err)
		}
		if _, err := l.CreateTenantUser(admin, ref, core.CreateUserInput{Email: "x@y.test"}); !core.IsKind(err, core.KindForbidden) {
			t.Errorf("CreateTenantUser(%s) = %v, want forbidden", ref, err)
		}
		if _, err := l.GetTenant(admin, ref); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("GetTenant(%s) = %v, want not found", ref, err)
		}
		name := "Mine now"
		if _, err := l.UpdateTenant(admin, ref, core.UpdateTenantInput{Name: &name}); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("UpdateTenant(%s) = %v, want not found", ref, err)
		}
		if err := l.DeleteTenant(admin, ref); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("DeleteTenant(%s) = %v, want not found", ref, err)
		}
	}
	listed, _, err := l.ListTenants(admin, core.Page{})
	if err != nil || len(listed) != 1 {
		t.Errorf("a tenant admin listing tenants = %d, %v, want only their own", len(listed), err)
	}
}

func TestWhoAmIReportsGlobalAdmin(t *testing.T) {
	l, _, _, actor := newLocal(t)
	root, _ := globalAdmin(t, l, actor, "root@acme.test")
	admin := tenantAdmin(t, l, actor, "admin@acme.test")
	if me, err := l.WhoAmI(root); err != nil || !me.GlobalAdmin {
		t.Errorf("WhoAmI for a global admin = %+v, %v", me, err)
	}
	if me, err := l.WhoAmI(admin); err != nil || me.GlobalAdmin {
		t.Errorf("WhoAmI for a tenant admin = %+v, %v", me, err)
	}
}

func TestAGlobalAdminPagesThroughEveryLiveTenant(t *testing.T) {
	l, _, _, actor := newLocal(t)
	root, _ := globalAdmin(t, l, actor, "root@acme.test")
	for _, key := range []string{"bravo", "charlie", "delta"} {
		if _, err := l.CreateTenant(root, core.CreateTenantInput{Key: key, Name: strings.ToUpper(key)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.DeleteTenant(root, "charlie"); err != nil {
		t.Fatal(err)
	}
	for _, sort := range []string{"", "key", "name"} {
		var keys []string
		page := core.Page{Limit: 1, Sort: sort}
		for range 10 {
			got, next, err := l.ListTenants(root, page)
			if err != nil {
				t.Fatalf("ListTenants(sort %q): %v", sort, err)
			}
			for _, tn := range got {
				keys = append(keys, tn.Key)
			}
			if next == "" {
				break
			}
			page.Cursor = next
		}
		if len(keys) != 3 || strings.Contains(strings.Join(keys, " "), "charlie") {
			t.Errorf("paging by %q gave %v, want acme, bravo and delta once each", sort, keys)
		}
	}
}

func TestGlobalAdminRefusesBadAddresses(t *testing.T) {
	l, _, _, actor := newLocal(t)
	root, _ := globalAdmin(t, l, actor, "root@acme.test")
	if _, err := l.GrantGlobalAdmin(root, core.GlobalAdminInput{Email: "nope"}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("granting a malformed address = %v, want invalid", err)
	}
	if err := l.RevokeGlobalAdmin(root, "nope"); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("revoking a malformed address = %v, want invalid", err)
	}
	if err := l.RevokeGlobalAdmin(root, "ghost@acme.test"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("revoking an unknown address = %v, want not found", err)
	}
	if _, err := l.UpdateTenantUser(root, "nosuch", "x", core.UpdateUserInput{}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("reaching an unknown tenant = %v, want not found", err)
	}
	if err := l.DeleteTenantUser(root, "nosuch", "x"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("deleting in an unknown tenant = %v, want not found", err)
	}
	if _, err := l.CreateTenantUser(root, "nosuch", core.CreateUserInput{Email: "x@y.test"}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("creating in an unknown tenant = %v, want not found", err)
	}
	if _, _, err := l.ListTenantUsers(root, "acme", core.UserFilter{Page: core.Page{Limit: -1}}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("a bad page = %v, want invalid", err)
	}
}

func TestAGeneratedFirstAdminPasswordMustBeChanged(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx, _ := globalAdmin(t, l, actor, "root@acme.test")
	created, err := l.CreateTenant(ctx, core.CreateTenantInput{Key: "test", Name: "Test", Admins: []core.TenantAdminInput{
		{Email: "villy@z09.test"}, {Email: "icek@z09.test", Password: "icek-chose-this-one"},
	}})
	if err != nil || len(created.InitialCredentials) != 1 {
		t.Fatalf("CreateTenant = %+v, %v; want one generated password", created, err)
	}
	login := core.WithTenant(context.Background(), core.TenantScope{TenantID: created.ID})
	generated, err := l.Login(login, core.LoginInput{Email: "villy@z09.test", Password: created.InitialCredentials[0].Password})
	if err != nil || generated.Challenge == nil || generated.Challenge.Kind != core.ChallengePasswordChange {
		t.Fatalf("signing in with the generated password = %+v, %v; want a password_change challenge", generated, err)
	}
	if _, err := signInAt(t, l, created.ID, "icek@z09.test", "icek-chose-this-one"); err != nil {
		t.Fatalf("a supplied password was flagged too: %v", err)
	}
}
