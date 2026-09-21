package service

import (
	"context"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// testPassword is long enough to satisfy core.MinPasswordLength.
const testPassword = "correct-horse-battery-staple"

// authContext returns a context carrying the actor.
func authContext(actor *core.Actor) context.Context {
	return core.WithActor(context.Background(), actor)
}

// otherTenantActor builds a second tenant with an administrator of its own, so
// that cross-tenant access can be exercised against a real actor.
func otherTenantActor(t *testing.T, l *Local) *core.Actor {
	t.Helper()
	ctx := context.Background()

	tenant := core.Tenant{Key: "other", Name: "Other"}
	if err := l.store.Unscoped(ctx, func(u store.UnscopedTx) error {
		return u.CreateTenant(ctx, &tenant)
	}); err != nil {
		t.Fatalf("creating the second tenant: %v", err)
	}
	scope := core.TenantScope{TenantID: tenant.ID}
	actor := core.Actor{Kind: core.ActorUser, Handle: "bob", Scopes: []core.Scope{core.ScopeAll}}
	if err := l.store.Update(ctx, scope, func(tx store.Tx) error {
		return tx.CreateActor(ctx, &actor)
	}); err != nil {
		t.Fatalf("creating the second tenant's actor: %v", err)
	}
	actor.TenantID = tenant.ID
	return &actor
}

// storedPasswordHash returns what the database holds for an email address.
func storedPasswordHash(t *testing.T, l *Local, email string) string {
	t.Helper()
	var hash string
	if err := l.store.Unscoped(context.Background(), func(u store.UnscopedTx) error {
		_, h, err := u.GetUserByEmail(context.Background(), email)
		hash = h
		return err
	}); err != nil {
		t.Fatalf("reading the stored hash: %v", err)
	}
	return hash
}

// A user without an actor cannot be assigned work, so creating one must create
// its actor and its membership in the same transaction.
func TestCreateUserCreatesActorAndMembership(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	ctx := authContext(admin)

	user, err := l.CreateUser(ctx, core.CreateUserInput{
		Email: "Ada@Example.com", Password: testPassword,
		DisplayName: "Ada", Role: core.RoleMember,
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if user.Email != "ada@example.com" {
		t.Errorf("email = %q, want it normalized", user.Email)
	}

	if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		a, err := tx.GetActor(context.Background(), user.ID)
		if err != nil {
			return err
		}
		if a.Handle != "ada" {
			t.Errorf("actor handle = %q, want %q", a.Handle, "ada")
		}
		if a.Kind != core.ActorUser {
			t.Errorf("actor kind = %q, want %q", a.Kind, core.ActorUser)
		}
		mem, err := tx.GetMember(context.Background(), user.ID)
		if err != nil {
			return err
		}
		if mem.Role != core.RoleMember {
			t.Errorf("membership role = %q, want %q", mem.Role, core.RoleMember)
		}
		return nil
	}); err != nil {
		t.Fatalf("reading back the actor: %v", err)
	}
}

func TestCreateUserDefaultsToTheWeakestRole(t *testing.T) {
	l, _, scope, admin := newLocal(t)

	user, err := l.CreateUser(authContext(admin), core.CreateUserInput{Email: "grace@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		mem, err := tx.GetMember(context.Background(), user.ID)
		if err != nil {
			return err
		}
		if mem.Role != core.RoleViewer {
			t.Errorf("default role = %q, want %q", mem.Role, core.RoleViewer)
		}
		return nil
	}); err != nil {
		t.Fatalf("reading back the membership: %v", err)
	}
}

func TestCreateUserStoresOnlyAHash(t *testing.T) {
	l, _, _, admin := newLocal(t)

	if _, err := l.CreateUser(authContext(admin), core.CreateUserInput{
		Email: "ada@example.com", Password: testPassword,
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	hash := storedPasswordHash(t, l, "ada@example.com")
	if hash == testPassword || strings.Contains(hash, testPassword) {
		t.Fatal("the plaintext password reached the database")
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Errorf("stored hash = %q, want an argon2id encoding", hash)
	}
	if err := auth.Verify(hash, testPassword); err != nil {
		t.Errorf("the stored hash does not verify the password: %v", err)
	}
}

func TestCreateUserRejectsBadInput(t *testing.T) {
	l, _, _, admin := newLocal(t)
	ctx := authContext(admin)

	if _, err := l.CreateUser(ctx, core.CreateUserInput{Email: "ada@example.com", Password: "short"}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("a short password = %v, want invalid", err)
	}
	if _, err := l.CreateUser(ctx, core.CreateUserInput{Email: "   "}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("an empty email = %v, want invalid", err)
	}
	if _, err := l.CreateUser(ctx, core.CreateUserInput{Email: "ada@example.com", Role: "emperor"}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("an unknown role = %v, want invalid", err)
	}
	if _, err := l.CreateUser(ctx, core.CreateUserInput{Email: "@example.com"}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("an email with no local part = %v, want invalid", err)
	}
}

func TestCreateUserRejectsDuplicates(t *testing.T) {
	l, _, _, admin := newLocal(t)
	ctx := authContext(admin)

	if _, err := l.CreateUser(ctx, core.CreateUserInput{Email: "ada@example.com"}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if _, err := l.CreateUser(ctx, core.CreateUserInput{Email: "ada@example.com", Handle: "ada2"}); !core.IsKind(err, core.KindConflict) {
		t.Errorf("a duplicate email = %v, want conflict", err)
	}
	if _, err := l.CreateUser(ctx, core.CreateUserInput{Email: "other@example.com", Handle: "ada"}); !core.IsKind(err, core.KindConflict) {
		t.Errorf("a duplicate handle = %v, want conflict", err)
	}
}

func TestGetUpdateDeleteUser(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	ctx := authContext(admin)

	user, err := l.CreateUser(ctx, core.CreateUserInput{Email: "ada@example.com", Role: core.RoleViewer})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	got, err := l.GetUser(ctx, user.ID)
	if err != nil || got.ID != user.ID {
		t.Fatalf("GetUser = %+v, %v", got, err)
	}

	name, role, disabled := "Ada Lovelace", core.RoleAdmin, true
	updated, err := l.UpdateUser(ctx, user.ID, core.UpdateUserInput{
		DisplayName: &name, Role: &role, Disabled: &disabled,
	})
	if err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	if updated.DisplayName != name {
		t.Errorf("display name = %q, want %q", updated.DisplayName, name)
	}
	if updated.DisabledAt == nil {
		t.Error("the user was not disabled")
	}
	if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		mem, err := tx.GetMember(context.Background(), user.ID)
		if err != nil {
			return err
		}
		if mem.Role != core.RoleAdmin {
			t.Errorf("role after update = %q, want %q", mem.Role, core.RoleAdmin)
		}
		return nil
	}); err != nil {
		t.Fatalf("reading the membership: %v", err)
	}

	enabled := false
	if updated, err = l.UpdateUser(ctx, user.ID, core.UpdateUserInput{Disabled: &enabled}); err != nil {
		t.Fatalf("re-enabling: %v", err)
	}
	if updated.DisabledAt != nil {
		t.Error("the user was not re-enabled")
	}

	if err := l.DeleteUser(ctx, user.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if _, err := l.GetUser(ctx, user.ID); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("GetUser after delete = %v, want not found", err)
	}
	if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		if _, err := tx.GetMember(context.Background(), user.ID); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("membership after delete = %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("reading the membership: %v", err)
	}
}

func TestUpdateUserRejectsBadInput(t *testing.T) {
	l, _, _, admin := newLocal(t)
	ctx := authContext(admin)

	user, err := l.CreateUser(ctx, core.CreateUserInput{Email: "ada@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	bad := core.Role("emperor")
	if _, err := l.UpdateUser(ctx, user.ID, core.UpdateUserInput{Role: &bad}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("an unknown role = %v, want invalid", err)
	}
	short := "nope"
	if _, err := l.UpdateUser(ctx, user.ID, core.UpdateUserInput{Password: &short}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("a short password = %v, want invalid", err)
	}
	if _, err := l.UpdateUser(ctx, "", core.UpdateUserInput{}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("an empty identifier = %v, want invalid", err)
	}
}

func TestUpdateUserReplacesThePasswordHash(t *testing.T) {
	l, _, _, admin := newLocal(t)
	ctx := authContext(admin)

	user, err := l.CreateUser(ctx, core.CreateUserInput{Email: "ada@example.com", Password: testPassword})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	before := storedPasswordHash(t, l, "ada@example.com")

	next := "an-entirely-different-secret"
	if _, err := l.UpdateUser(ctx, user.ID, core.UpdateUserInput{Password: &next}); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	after := storedPasswordHash(t, l, "ada@example.com")
	if after == before {
		t.Fatal("the password hash did not change")
	}
	if strings.Contains(after, next) {
		t.Fatal("the plaintext password reached the database")
	}
	if err := auth.Verify(after, next); err != nil {
		t.Errorf("the new hash does not verify the new password: %v", err)
	}
}

func TestListUsersIsTenantScoped(t *testing.T) {
	l, _, _, admin := newLocal(t)
	ctx := authContext(admin)

	for _, email := range []string{"ada@example.com", "grace@example.com"} {
		if _, err := l.CreateUser(ctx, core.CreateUserInput{Email: email}); err != nil {
			t.Fatalf("CreateUser %q: %v", email, err)
		}
	}
	intruder := otherTenantActor(t, l)
	if _, err := l.CreateUser(authContext(intruder), core.CreateUserInput{Email: "eve@example.com"}); err != nil {
		t.Fatalf("CreateUser in the other tenant: %v", err)
	}

	users, _, err := l.ListUsers(ctx, core.Page{})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("ListUsers returned %d users, want 2", len(users))
	}
	for _, u := range users {
		if u.Email == "eve@example.com" {
			t.Error("ListUsers disclosed another tenant's user")
		}
	}
}

func TestListUsersPaginates(t *testing.T) {
	l, _, _, admin := newLocal(t)
	ctx := authContext(admin)

	for _, email := range []string{"a@example.com", "b@example.com", "c@example.com"} {
		if _, err := l.CreateUser(ctx, core.CreateUserInput{Email: email}); err != nil {
			t.Fatalf("CreateUser %q: %v", email, err)
		}
	}

	first, next, err := l.ListUsers(ctx, core.Page{Limit: 2, Sort: "email"})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(first) != 2 || next == "" {
		t.Fatalf("first page = %d users, cursor %q", len(first), next)
	}
	rest, last, err := l.ListUsers(ctx, core.Page{Limit: 2, Sort: "email", Cursor: next})
	if err != nil {
		t.Fatalf("ListUsers page two: %v", err)
	}
	if len(rest) != 1 || last != "" {
		t.Fatalf("second page = %d users, cursor %q", len(rest), last)
	}
	if _, _, err := l.ListUsers(ctx, core.Page{Limit: -1}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("a negative limit = %v, want invalid", err)
	}
}

func TestUserAdminIsDeniedToNonAdmins(t *testing.T) {
	l, _, _, admin := newLocal(t)
	member := &core.Actor{ID: "m1", TenantID: admin.TenantID, Kind: core.ActorUser, Role: core.RoleMember}
	ctx := authContext(member)

	if _, err := l.CreateUser(ctx, core.CreateUserInput{Email: "ada@example.com"}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("CreateUser as a member = %v, want forbidden", err)
	}
	if _, err := l.GetUser(ctx, "whatever"); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("GetUser as a member = %v, want forbidden", err)
	}
	if _, _, err := l.ListUsers(ctx, core.Page{}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("ListUsers as a member = %v, want forbidden", err)
	}
	if _, err := l.UpdateUser(ctx, "whatever", core.UpdateUserInput{}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("UpdateUser as a member = %v, want forbidden", err)
	}
	if err := l.DeleteUser(ctx, "whatever"); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("DeleteUser as a member = %v, want forbidden", err)
	}
}

// A user of another tenant must be indistinguishable from one that never
// existed, or the API confirms accounts it should not.
func TestUserOfAnotherTenantIsNotFound(t *testing.T) {
	l, _, _, admin := newLocal(t)

	user, err := l.CreateUser(authContext(admin), core.CreateUserInput{Email: "ada@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	intruder := authContext(otherTenantActor(t, l))

	if _, err := l.GetUser(intruder, user.ID); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("GetUser across tenants = %v, want not found", err)
	}
	if _, err := l.UpdateUser(intruder, user.ID, core.UpdateUserInput{}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("UpdateUser across tenants = %v, want not found", err)
	}
	if err := l.DeleteUser(intruder, user.ID); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("DeleteUser across tenants = %v, want not found", err)
	}
}

func TestUserMutationsRecordAuditAndEvent(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	ctx := authContext(admin)

	beforeEvents, beforeAudits := countRows(t, l, scope)
	user, err := l.CreateUser(ctx, core.CreateUserInput{Email: "ada@example.com", Password: testPassword})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := l.DeleteUser(ctx, user.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	afterEvents, afterAudits := countRows(t, l, scope)

	if afterEvents-beforeEvents != 2 {
		t.Errorf("events grew by %d, want 2", afterEvents-beforeEvents)
	}
	if afterAudits-beforeAudits != 2 {
		t.Errorf("audit entries grew by %d, want 2", afterAudits-beforeAudits)
	}
}

// Removing a user must cut off access immediately. Leaving a live session or a
// valid token behind would mean a revoked account kept working until the
// credential happened to expire.
func TestDeleteUserEndsSessionsAndRevokesTokens(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	ctx := adminCtx(admin)

	user := seedUser(t, l, admin, "ada@example.com")
	session, err := l.Login(loginContext(scope), "ada@example.com", testPassword)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	subject := &core.Actor{
		ID: user.ID, TenantID: scope.TenantID, Kind: core.ActorUser,
		Handle: "ada", Scopes: []core.Scope{core.ScopeAll},
	}
	issued, err := l.CreateToken(core.WithActor(context.Background(), subject),
		core.CreateTokenInput{Name: "ada-bot", Scopes: []core.Scope{core.ScopeTaskRead}})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	if err := l.DeleteUser(ctx, user.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	assertSessionGone(t, l, scope, session.Token)
	assertTokenRevoked(t, l, scope, issued.Token)
}

func TestDisablingAUserEndsSessionsAndRevokesTokens(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	ctx := adminCtx(admin)

	user := seedUser(t, l, admin, "grace@example.com")
	session, err := l.Login(loginContext(scope), "grace@example.com", testPassword)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	subject := &core.Actor{
		ID: user.ID, TenantID: scope.TenantID, Kind: core.ActorUser,
		Handle: "grace", Scopes: []core.Scope{core.ScopeAll},
	}
	issued, err := l.CreateToken(core.WithActor(context.Background(), subject),
		core.CreateTokenInput{Name: "grace-bot", Scopes: []core.Scope{core.ScopeTaskRead}})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	disabled := true
	if _, err := l.UpdateUser(ctx, user.ID, core.UpdateUserInput{Disabled: &disabled}); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}

	assertSessionGone(t, l, scope, session.Token)
	assertTokenRevoked(t, l, scope, issued.Token)
}

func assertSessionGone(t *testing.T, l *Local, scope core.TenantScope, token string) {
	t.Helper()
	ctx := context.Background()
	if err := l.store.View(ctx, scope, func(tx store.Tx) error {
		_, _, err := tx.GetSessionByHash(ctx, auth.HashToken(token))
		if err == nil {
			t.Error("a session survived the account being removed or disabled")
		}
		return nil
	}); err != nil {
		t.Fatalf("checking session: %v", err)
	}
}

func assertTokenRevoked(t *testing.T, l *Local, scope core.TenantScope, token string) {
	t.Helper()
	ctx := context.Background()
	if err := l.store.View(ctx, scope, func(tx store.Tx) error {
		stored, err := tx.GetTokenByHash(ctx, auth.HashToken(token))
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if stored.RevokedAt == nil {
			t.Error("an api token survived the account being removed or disabled")
		}
		return nil
	}); err != nil {
		t.Fatalf("checking token: %v", err)
	}
}
