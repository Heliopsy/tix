// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// patSession is pat signed in to a tenant in the browser: her actor there,
// with no token.
func patSession(scope core.TenantScope, actorID string) context.Context {
	return authContext(&core.Actor{ID: actorID, TenantID: scope.TenantID, Kind: core.ActorUser,
		Role: core.RoleMember})
}

// actionsOf lists the entries' actions with their tenant, oldest first.
func actionsOf(entries []core.AccountLogEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, string(e.Action)+"@"+e.TenantKey)
	}
	return out
}

func TestAnAccountsLogFollowsItAcrossTenants(t *testing.T) {
	f := newTwoTenants(t, WithTOTPKey(totpTestKey))
	role := core.RoleAdmin
	if _, err := f.l.UpdateUser(authContext(f.otherAdmin), f.otherPat.ID, core.UpdateUserInput{Role: &role}); err != nil {
		t.Fatalf("the other tenant's admin changing pat's role: %v", err)
	}
	secret := "a-password-pat-must-never-see-logged"
	if _, err := f.l.UpdateTenantUser(f.global, "other", f.otherPat.ID, core.UpdateUserInput{Password: &secret}); err != nil {
		t.Fatalf("a global admin setting pat's password: %v", err)
	}

	own, _, err := f.l.GetAccountLog(patSession(f.home, f.homePat.ID), core.Page{})
	if err != nil {
		t.Fatalf("pat reading her own log: %v", err)
	}
	want := []string{
		"account.created@acme", "account.membership_added@other",
		"account.membership_role_changed@other", "account.password_set@other",
	}
	if got := actionsOf(own); !slices.Equal(got, want) {
		t.Fatalf("pat's log = %v, want %v", got, want)
	}
	added, set := own[1], own[3]
	if !added.ByGlobalAdmin || added.ByEmail != "root@acme.test" {
		t.Errorf("the membership entry = %+v, want it made by root as a global admin", added)
	}
	if !set.ByGlobalAdmin || set.ByEmail != "root@acme.test" {
		t.Errorf("the password entry = %+v, want it made by root as a global admin", set)
	}
	if own[2].ByGlobalAdmin {
		t.Errorf("the role change by the other tenant's admin is marked as a global admin's: %+v", own[2])
	}

	fromOther, _, err := f.l.GetAccountLog(patSession(f.other, f.otherPat.ID), core.Page{})
	if err != nil || len(fromOther) != len(own) {
		t.Fatalf("pat reading her log from the other tenant = %d entries, %v; want the same %d", len(fromOther), err, len(own))
	}
	theirs, _, err := f.l.GetAccountLogFor(f.global, patEmail, core.Page{})
	if err != nil || len(theirs) != len(own) {
		t.Fatalf("a global admin reading pat's log = %d entries, %v; want %d", len(theirs), err, len(own))
	}

	raw, err := json.Marshal(own)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), secret) || strings.Contains(string(raw), "$argon2") {
		t.Fatalf("the account log carries a password or its hash:\n%s", raw)
	}
}

// A tenant admin never reads anybody's account log, because it names the
// person's other tenants, and an agent's token learns nothing about them.
func TestOnlyThePersonAndGlobalAdminsReadAnAccountLog(t *testing.T) {
	f := newTwoTenants(t)

	if _, _, err := f.l.GetAccountLogFor(authContext(f.otherAdmin), patEmail, core.Page{}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("a tenant admin reading pat's log = %v, want forbidden", err)
	}
	agent := &core.Actor{ID: f.homePat.ID, TenantID: f.home.TenantID, Kind: core.ActorUser,
		TokenID: "tok", Scopes: []core.Scope{core.ScopeAll}}
	if _, _, err := f.l.GetAccountLog(authContext(agent), core.Page{}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("a token with * reading pat's log = %v, want forbidden", err)
	}
	agent.Scopes = []core.Scope{core.ScopeAccountRead}
	if _, _, err := f.l.GetAccountLog(authContext(agent), core.Page{}); err != nil {
		t.Errorf("a token with account:read reading pat's log = %v, want allowed", err)
	}
	if _, _, err := f.l.GetAccountLog(authContext(f.homeAdmin), core.Page{}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("an actor with no account reading a log = %v, want invalid", err)
	}
}

// A refused change records nothing: the entry and the change are one write.
func TestARefusedChangeLeavesNoLogEntry(t *testing.T) {
	f := newTwoTenants(t)
	before, _, err := f.l.GetAccountLogFor(f.global, patEmail, core.Page{})
	if err != nil {
		t.Fatal(err)
	}
	password := "a-password-the-admin-chose"
	if _, err := f.l.UpdateUser(authContext(f.otherAdmin), f.otherPat.ID, core.UpdateUserInput{Password: &password}); err == nil {
		t.Fatal("the other tenant's admin set pat's password")
	}
	after, _, err := f.l.GetAccountLogFor(f.global, patEmail, core.Page{})
	if err != nil || len(after) != len(before) {
		t.Fatalf("pat's log after a refused change = %d entries, %v; want %d", len(after), err, len(before))
	}
}

func TestTheLogRecordsSecondFactorAndMembershipChanges(t *testing.T) {
	f := newTwoTenants(t, WithTOTPKey(totpTestKey))
	pat := &core.Actor{ID: f.homePat.ID, TenantID: f.home.TenantID, Kind: core.ActorUser, Role: core.RoleMember}
	enrolWith(t, f.l, pat)
	if err := f.l.ClearUserTOTP(authContext(f.homeAdmin), f.homePat.ID); !core.IsKind(err, core.KindForbidden) {
		t.Fatalf("the home admin clearing pat's factor = %v, want forbidden while she is in two tenants", err)
	}
	if err := f.l.DeleteUser(authContext(f.otherAdmin), f.otherPat.ID); err != nil {
		t.Fatalf("removing pat from the other tenant: %v", err)
	}
	if err := f.l.ClearUserTOTP(authContext(f.homeAdmin), f.homePat.ID); err != nil {
		t.Fatalf("the home admin clearing pat's factor once she is only at home: %v", err)
	}
	got, _, err := f.l.GetAccountLog(patSession(f.home, f.homePat.ID), core.Page{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"account.second_factor_enrolled@acme", "account.recovery_codes_issued@acme",
		"account.membership_removed@other", "account.second_factor_cleared@acme",
	} {
		if !slices.Contains(actionsOf(got), want) {
			t.Errorf("pat's log %v lacks %s", actionsOf(got), want)
		}
	}

	account := accountID(t, f.l, f.home, f.homePat.ID)
	if err := f.l.DeleteUser(authContext(f.homeAdmin), f.homePat.ID); err != nil {
		t.Fatalf("removing pat from her last tenant: %v", err)
	}
	kept := actionsOf(storedLog(t, f.l, f.home, account))
	if !slices.Contains(kept, "account.deleted@acme") || !slices.Contains(kept, "account.created@acme") {
		t.Errorf("the log of a deleted account = %v, want its history kept with the deletion", kept)
	}
}

func TestTheLogPagesOnItsSequence(t *testing.T) {
	f := newTwoTenants(t)
	first, next, err := f.l.GetAccountLogFor(f.global, patEmail, core.Page{Limit: 1})
	if err != nil || len(first) != 1 || next == "" {
		t.Fatalf("first page = %v, %q, %v; want one entry and a cursor", first, next, err)
	}
	second, _, err := f.l.GetAccountLogFor(f.global, patEmail, core.Page{Limit: 1, Cursor: next})
	if err != nil || len(second) != 1 || second[0].Seq <= first[0].Seq {
		t.Fatalf("second page = %v, %v; want the entry after %d", second, err, first[0].Seq)
	}
	newest, _, err := f.l.GetAccountLogFor(f.global, patEmail, core.Page{Limit: 1, Direction: core.Descending})
	if err != nil || len(newest) != 1 || newest[0].Seq != second[0].Seq {
		t.Fatalf("newest first = %v, %v; want %d", newest, err, second[0].Seq)
	}
}

// An account:read token is minted only for its owner's own actor, so an
// administrator cannot mint one that reads somebody else's account.
func TestAnAccountReadTokenIsMintedOnlyForItsOwner(t *testing.T) {
	f := newTwoTenants(t)
	in := core.CreateTokenInput{Name: "reader", ActorID: f.homePat.ID, Scopes: []core.Scope{core.ScopeAccountRead}}
	if _, err := f.l.CreateToken(authContext(f.homeAdmin), in); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("an admin minting account:read for pat = %v, want forbidden", err)
	}
	pat := &core.Actor{ID: f.homePat.ID, TenantID: f.home.TenantID, Kind: core.ActorUser, Role: core.RoleAdmin}
	in.ActorID = ""
	if _, err := f.l.CreateToken(authContext(pat), in); err != nil {
		t.Errorf("pat minting account:read for herself = %v, want allowed", err)
	}
	all := &core.Actor{ID: f.homePat.ID, TenantID: f.home.TenantID, Kind: core.ActorUser, TokenID: "t",
		Scopes: []core.Scope{core.ScopeAll}}
	in.Name = "escalate"
	if _, err := f.l.CreateToken(authContext(all), in); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("a * token minting account:read = %v, want forbidden", err)
	}
}

// enrolWith takes a person through enrolment with the service's own clock.
func enrolWith(t *testing.T, l *Local, actor *core.Actor) {
	t.Helper()
	ctx := authContext(actor)
	enrolment, err := l.BeginTOTPEnrolment(ctx)
	if err != nil {
		t.Fatalf("beginning enrolment: %v", err)
	}
	if _, err := l.ConfirmTOTPEnrolment(ctx, codeAt(t, enrolment.Secret, l)); err != nil {
		t.Fatalf("confirming enrolment: %v", err)
	}
}

// codeAt produces the code an authenticator holding secret shows at the
// service's current time.
func codeAt(t *testing.T, secret string, l *Local) string {
	t.Helper()
	code, err := auth.TOTPCode(secret, auth.TOTPStep(l.clock.Now()))
	if err != nil {
		t.Fatal(err)
	}
	return code
}

// storedLog reads an account's log straight from the store.
func storedLog(t *testing.T, l *Local, scope core.TenantScope, userID string) []core.AccountLogEntry {
	t.Helper()
	var out []core.AccountLogEntry
	if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		var err error
		out, err = tx.ListAccountLog(context.Background(), userID, core.Page{Limit: 100, Sort: "seq", Direction: core.Ascending})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return out
}
