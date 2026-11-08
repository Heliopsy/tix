// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// twoTenants is a person, pat, created in the fixture's tenant and added by a
// global admin to a second tenant, with an admin of each tenant who is not a
// global admin.
type twoTenants struct {
	l           *Local
	clk         *clock.Fake
	home, other core.TenantScope
	homeAdmin   *core.Actor
	otherAdmin  *core.Actor
	global      context.Context
	homePat     *core.User
	otherPat    *core.User
	otherKey    string
}

const patEmail = "pat@example.com"

func newTwoTenants(t *testing.T, opts ...Option) twoTenants {
	t.Helper()
	l, clk, scope, admin := newLocalWith(t, opts...)
	other := otherTenantActor(t, l)
	global, _ := globalAdmin(t, l, admin, "root@acme.test")
	homePat, err := l.CreateUser(authContext(admin), core.CreateUserInput{
		Email: patEmail, Password: testPassword, Role: core.RoleMember,
	})
	if err != nil {
		t.Fatalf("creating pat: %v", err)
	}
	otherPat, err := l.AddTenantMember(global, "other", core.AddTenantMemberInput{Email: patEmail, Role: core.RoleMember})
	if err != nil {
		t.Fatalf("adding pat to the other tenant: %v", err)
	}
	return twoTenants{
		l: l, clk: clk, home: scope, other: core.TenantScope{TenantID: other.TenantID},
		homeAdmin: admin, otherAdmin: other, global: global,
		homePat: homePat, otherPat: otherPat, otherKey: "other",
	}
}

// canSignIn reports whether pat's password signs in to a tenant.
func (f twoTenants) canSignIn(t *testing.T, scope core.TenantScope) bool {
	t.Helper()
	session, err := signIn(f.l, scope, patEmail, testPassword)
	return err == nil && session != nil
}

func TestAGlobalAdminAddsAnExistingAccountToAnotherTenant(t *testing.T) {
	f := newTwoTenants(t)

	if f.otherPat.ID == f.homePat.ID {
		t.Fatalf("pat has one actor identifier in both tenants, %q; want one actor per tenant", f.otherPat.ID)
	}
	if !f.canSignIn(t, f.home) || !f.canSignIn(t, f.other) {
		t.Fatal("pat's one password does not sign in to both tenants")
	}
	found, err := f.l.membershipsOf(context.Background(), accountID(t, f.l, f.home, f.homePat.ID))
	if err != nil || len(found) != 2 {
		t.Fatalf("pat's memberships = %+v, %v; want two", found, err)
	}

	if _, err := f.l.AddTenantMember(f.global, "other", core.AddTenantMemberInput{Email: patEmail}); !core.IsKind(err, core.KindConflict) {
		t.Errorf("adding pat twice = %v, want conflict", err)
	}
	if _, err := f.l.AddTenantMember(f.global, "other", core.AddTenantMemberInput{Email: "nobody@example.com"}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("adding an address with no account = %v, want not found", err)
	}
	if _, err := f.l.AddTenantMember(authContext(f.homeAdmin), "other", core.AddTenantMemberInput{Email: patEmail}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("a tenant admin adding somebody to another tenant = %v, want forbidden", err)
	}
}

// The heart of the change: an administrator of one tenant cannot reach the
// credentials of a person who also works in another, because those
// credentials open the other tenant too.
func TestATenantAdminCannotChangeTheAccountOfAPersonInAnotherTenant(t *testing.T) {
	f := newTwoTenants(t, WithTOTPKey(totpTestKey))
	ctx := authContext(f.otherAdmin)
	password := "a-password-the-admin-chose"
	yes := true

	refusals := map[string]error{}
	_, refusals["password"] = f.l.UpdateUser(ctx, f.otherPat.ID, core.UpdateUserInput{Password: &password})
	_, refusals["require password change"] = f.l.UpdateUser(ctx, f.otherPat.ID, core.UpdateUserInput{RequirePasswordChange: &yes})
	refusals["second factor"] = f.l.ClearUserTOTP(ctx, f.otherPat.ID)
	for what, err := range refusals {
		var e *core.Error
		if !core.IsKind(err, core.KindForbidden) || !asError(err, &e) || e.Details["reason"] != ReasonAccountInOtherTenants {
			t.Errorf("the other tenant's admin changing pat's %s = %v, want forbidden with %s", what, err, ReasonAccountInOtherTenants)
		}
		if err != nil && strings.Contains(err.Error(), "acme") {
			t.Errorf("the refusal names pat's other tenant: %v", err)
		}
	}
	if !f.canSignIn(t, f.home) {
		t.Fatal("pat's password changed in her home tenant")
	}

	role := core.RoleAdmin
	name := "Pat in Other"
	got, err := f.l.UpdateUser(ctx, f.otherPat.ID, core.UpdateUserInput{Role: &role, DisplayName: &name})
	if err != nil {
		t.Fatalf("the other tenant's admin changing pat's role and name there = %v, want allowed", err)
	}
	if got.ID != f.otherPat.ID {
		t.Errorf("updated = %+v, want pat's actor in the other tenant", got)
	}
	home, err := f.l.GetUser(authContext(f.homeAdmin), f.homePat.ID)
	if err != nil || home.DisplayName == name {
		t.Errorf("pat's name at home = %+v, %v; want it untouched by the other tenant", home, err)
	}

	global, err := f.l.UpdateTenantUser(f.global, "other", f.otherPat.ID, core.UpdateUserInput{Password: &password})
	if err != nil || global == nil {
		t.Fatalf("a global admin setting pat's password = %v, want allowed", err)
	}
	if _, err := signIn(f.l, f.home, patEmail, password); err != nil {
		t.Errorf("the global admin's password does not sign pat in at home: %v", err)
	}
}

// A forced password change equal to the one the person already has, or
// already lacks, is not a change, so a client that sends every field of an
// edit form does not have the rest of the edit refused with it.
func TestAnUnchangedPasswordChangeIsNotRefusedForATwoTenantPerson(t *testing.T) {
	f := newTwoTenants(t)
	ctx := authContext(f.otherAdmin)
	no, yes := false, true
	role := core.RoleAdmin

	got, err := f.l.UpdateUser(ctx, f.otherPat.ID, core.UpdateUserInput{Role: &role, RequirePasswordChange: &no})
	if err != nil {
		t.Fatalf("changing pat's role with her forced change left off = %v, want allowed", err)
	}
	if got.RequirePasswordChange {
		t.Error("an unchanged forced password change was set")
	}
	if _, err := f.l.UpdateUser(ctx, f.otherPat.ID, core.UpdateUserInput{RequirePasswordChange: &yes}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("turning pat's forced change on = %v, want forbidden", err)
	}
}

func TestDisablingAPersonInOneTenantLeavesTheOther(t *testing.T) {
	f := newTwoTenants(t)
	yes, no := true, false
	ctx := authContext(f.otherAdmin)

	if _, err := f.l.UpdateUser(ctx, f.otherPat.ID, core.UpdateUserInput{Disabled: &yes}); err != nil {
		t.Fatalf("disabling pat in the other tenant: %v", err)
	}
	if f.canSignIn(t, f.other) {
		t.Error("pat signs in to the tenant she is disabled in")
	}
	if !f.canSignIn(t, f.home) {
		t.Error("disabling pat in the other tenant disabled her at home")
	}
	if !withdrawn(t, f.l, f.other, f.otherPat.ID) || withdrawn(t, f.l, f.home, f.homePat.ID) {
		t.Error("credentials: want pat's withdrawn in the other tenant alone")
	}
	listed, err := f.l.GetUser(ctx, f.otherPat.ID)
	if err != nil || listed.DisabledAt == nil {
		t.Errorf("pat in the other tenant's user admin = %+v, %v; want disabled", listed, err)
	}

	if _, err := f.l.UpdateUser(ctx, f.otherPat.ID, core.UpdateUserInput{Disabled: &no}); err != nil {
		t.Fatalf("enabling pat again: %v", err)
	}
	if !f.canSignIn(t, f.other) {
		t.Error("pat does not sign in once enabled again")
	}

	if _, err := f.l.UpdateTenantUser(f.global, "other", f.otherPat.ID, core.UpdateUserInput{Disabled: &yes}); err != nil {
		t.Fatalf("a global admin disabling pat's account: %v", err)
	}
	if !withdrawn(t, f.l, f.home, f.homePat.ID) || !withdrawn(t, f.l, f.other, f.otherPat.ID) {
		t.Error("a disabled account's credentials still work in one of its tenants")
	}
}

func TestRemovingAPersonFromOneTenantKeepsTheAccountUntilTheLast(t *testing.T) {
	f := newTwoTenants(t)
	issued, err := f.l.CreateToken(authContext(f.otherAdmin), core.CreateTokenInput{
		Name: "pat-agent", ActorID: f.otherPat.ID, Scopes: []core.Scope{core.ScopeTaskRead},
	})
	if err != nil {
		t.Fatalf("minting pat a token in the other tenant: %v", err)
	}

	if err := f.l.DeleteUser(authContext(f.otherAdmin), f.otherPat.ID); err != nil {
		t.Fatalf("removing pat from the other tenant: %v", err)
	}
	if f.canSignIn(t, f.other) {
		t.Error("pat signs in to the tenant she was removed from")
	}
	if !f.canSignIn(t, f.home) {
		t.Fatal("removing pat from the other tenant deleted her account")
	}
	if err := f.l.store.View(context.Background(), f.other, func(tx store.Tx) error {
		tok, err := tx.GetTokenByHash(context.Background(), auth.HashToken(issued.Token))
		if err == nil && tok.RevokedAt == nil {
			t.Error("pat's token in the other tenant survived her removal")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := f.l.DeleteUser(authContext(f.homeAdmin), f.homePat.ID); err != nil {
		t.Fatalf("removing pat from her last tenant: %v", err)
	}
	if err := f.l.store.ViewUnscoped(context.Background(), func(u store.UnscopedTx) error {
		_, _, err := u.GetUserByEmail(context.Background(), patEmail)
		if !core.IsKind(err, core.KindNotFound) {
			t.Errorf("pat's account after her last membership went = %v, want deleted", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestAnInviteToAnAccountWithASecondFactorAsksForACode(t *testing.T) {
	l, clk, scope, admin := newTOTPLocal(t)
	other := otherTenantActor(t, l)
	pat, err := l.CreateUser(authContext(other), core.CreateUserInput{Email: patEmail, Password: testPassword})
	if err != nil {
		t.Fatalf("creating pat in the other tenant: %v", err)
	}
	secret, _ := enrol(t, l, clk, &core.Actor{ID: pat.ID, TenantID: other.TenantID, Kind: core.ActorUser, Role: core.RoleMember})

	issued := invite(t, l, admin, core.CreateInviteInput{Email: patEmail})
	ctx := loginContext(scope)
	preview, err := l.PreviewInvite(ctx, issued.Token)
	if err != nil || !preview.CodeRequired {
		t.Fatalf("preview = %+v, %v; want a code asked for", preview, err)
	}
	if _, err := l.AcceptInvite(ctx, issued.Token, core.AcceptInviteInput{Password: testPassword}); core.FieldOf(err) != "code" {
		t.Fatalf("accepting with the password alone = %v, want a refusal naming the code field", err)
	}
	if _, err := l.AcceptInvite(ctx, issued.Token, core.AcceptInviteInput{Password: testPassword, Code: "000000"}); err == nil {
		t.Fatal("accepting with a wrong code succeeded")
	}
	if _, err := signIn(l, scope, patEmail, testPassword); err == nil {
		t.Fatal("a refused acceptance granted the membership")
	}
	clk.Advance(30 * time.Second)
	result, err := l.AcceptInvite(ctx, issued.Token, core.AcceptInviteInput{Password: testPassword, Code: codeFor(t, secret, clk)})
	if err != nil || result.Session == nil {
		t.Fatalf("accepting with password and code = %+v, %v; want a session", result, err)
	}
}

// An account with no password cannot confirm an invite, and accepting one
// would set its password; the administrator holding the link would then hold
// the account in its other tenant. Such an invite is refused when issued.
func TestAnInviteCannotSetThePasswordOfAnotherTenantsAccount(t *testing.T) {
	l, _, _, admin := newLocal(t)
	other := otherTenantActor(t, l)
	if _, err := l.CreateInvite(authContext(other), core.CreateInviteInput{Email: patEmail}); err != nil {
		t.Fatalf("the other tenant inviting pat: %v", err)
	}
	_, err := l.CreateInvite(authContext(admin), core.CreateInviteInput{Email: patEmail})
	if !core.IsKind(err, core.KindConflict) {
		t.Fatalf("inviting an account of another tenant that has no password = %v, want conflict", err)
	}
}

// accountID reads the account behind an actor.
func accountID(t *testing.T, l *Local, scope core.TenantScope, actorID string) string {
	t.Helper()
	var out string
	if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		u, err := accountOfActor(context.Background(), tx, actorID)
		if err == nil {
			out = u.ID
		}
		return err
	}); err != nil {
		t.Fatalf("reading the account of %q: %v", actorID, err)
	}
	return out
}

// withdrawn reports whether an actor's credentials are refused right now.
func withdrawn(t *testing.T, l *Local, scope core.TenantScope, actorID string) bool {
	t.Helper()
	var out bool
	if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		var err error
		out, err = StandingWithdrawn(context.Background(), tx, actorID)
		return err
	}); err != nil {
		t.Fatalf("reading the standing of %q: %v", actorID, err)
	}
	return out
}
