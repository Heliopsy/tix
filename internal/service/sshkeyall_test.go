// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// enrolments lists a tenant's sshkey.enrol audit entries, read by its admin.
func enrolments(t *testing.T, l *Local, admin *core.Actor) []core.AuditEntry {
	t.Helper()
	entries, _, err := l.ListAudit(authContext(admin), core.AuditFilter{Actions: []string{auditSSHKeyEnrol}})
	if err != nil {
		t.Fatalf("listing enrolments: %v", err)
	}
	return entries
}

// promotePat makes pat an admin of both tenants, since enrolling a key needs
// token:admin, which only an admin holds.
func (f twoTenants) promotePat(t *testing.T) {
	t.Helper()
	admin := core.RoleAdmin
	for _, p := range []struct {
		by *core.Actor
		id string
	}{{f.homeAdmin, f.homePat.ID}, {f.otherAdmin, f.otherPat.ID}} {
		if _, err := f.l.UpdateUser(authContext(p.by), p.id, core.UpdateUserInput{Role: &admin}); err != nil {
			t.Fatalf("promoting pat: %v", err)
		}
	}
}

func TestAnAllTenantsEnrolmentWritesOneAuditedEnrolmentPerTenant(t *testing.T) {
	f := newTwoTenants(t)
	f.promotePat(t)
	session, err := signIn(f.l, f.home, patEmail, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	ctx := core.WithVia(f.sessionContext(t, session, f.sharedHost()), core.Via{Kind: core.ViaWeb, Session: "home-session"})

	key, err := f.l.EnrolSSHKey(ctx, core.EnrolSSHKeyInput{PublicKey: pubKey, Label: "laptop", AllTenants: true})
	if err != nil {
		t.Fatalf("EnrolSSHKey all tenants = %v", err)
	}
	if key.TenantID != f.home.TenantID || key.ActorID != f.homePat.ID {
		t.Errorf("answer = %+v, want pat's enrolment in the tenant asked from", key)
	}
	if len(key.EnrolledIn) != 2 || !slices.Contains(key.EnrolledIn, f.otherKey) {
		t.Errorf("enrolled in %v, want both of pat's tenants", key.EnrolledIn)
	}

	home := enrolments(t, f.l, f.homeAdmin)
	if len(home) != 1 || home[0].ActorID != f.homePat.ID || home[0].Via.Session != "home-session" {
		t.Fatalf("home enrolments = %+v, want one by pat's home actor", home)
	}
	other := enrolments(t, f.l, f.otherAdmin)
	if len(other) != 1 || other[0].ActorID != f.otherPat.ID {
		t.Fatalf("other enrolments = %+v, want one by pat's actor there", other)
	}
	if strings.Contains(string(other[0].After), f.homePat.ID) || other[0].Via.Session != "" || other[0].Via.TokenID != "" {
		t.Errorf("the other tenant's entry names the tenant asked from: %s, via %+v", other[0].After, other[0].Via)
	}
	otherKeys, err := f.l.ListSSHKeys(authContext(f.otherAdmin), f.otherPat.ID)
	if err != nil || len(otherKeys) != 1 || otherKeys[0].Fingerprint != key.Fingerprint {
		t.Fatalf("pat's keys in the other tenant = %+v, %v; want the one enrolled", otherKeys, err)
	}

	if _, err := f.l.EnrolSSHKey(ctx, core.EnrolSSHKeyInput{PublicKey: pubKey, AllTenants: true}); !core.IsKind(err, core.KindConflict) {
		t.Errorf("enrolling again everywhere = %v, want conflict", err)
	}
}

// A tenant already holding the key is passed over, and the answer still names
// the enrolment in the tenant asked from.
func TestAnAllTenantsEnrolmentPassesOverATenantHoldingTheKey(t *testing.T) {
	f := newTwoTenants(t)
	f.promotePat(t)
	session, err := signIn(f.l, f.home, patEmail, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	ctx := f.sessionContext(t, session, f.sharedHost())
	first, err := f.l.EnrolSSHKey(ctx, core.EnrolSSHKeyInput{PublicKey: pubKey})
	if err != nil {
		t.Fatal(err)
	}
	key, err := f.l.EnrolSSHKey(ctx, core.EnrolSSHKeyInput{PublicKey: pubKey, AllTenants: true})
	if err != nil {
		t.Fatalf("EnrolSSHKey all tenants = %v", err)
	}
	if key.ID != first.ID || !slices.Equal(key.EnrolledIn, []string{f.otherKey}) {
		t.Errorf("answer = %+v, want the existing home enrolment and only %q enrolled", key, f.otherKey)
	}
}

// A tenant where the person may not enrol keys is passed over rather than
// written to.
func TestAnAllTenantsEnrolmentPassesOverATenantWhereThePersonIsNoAdmin(t *testing.T) {
	f := newTwoTenants(t)
	admin := core.RoleAdmin
	if _, err := f.l.UpdateUser(authContext(f.homeAdmin), f.homePat.ID, core.UpdateUserInput{Role: &admin}); err != nil {
		t.Fatal(err)
	}
	session, err := signIn(f.l, f.home, patEmail, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	key, err := f.l.EnrolSSHKey(f.sessionContext(t, session, f.sharedHost()),
		core.EnrolSSHKeyInput{PublicKey: pubKey, AllTenants: true})
	if err != nil {
		t.Fatalf("EnrolSSHKey all tenants = %v", err)
	}
	if slices.Contains(key.EnrolledIn, f.otherKey) || len(enrolments(t, f.l, f.otherAdmin)) != 0 {
		t.Errorf("enrolled in %v, want the tenant where pat is a member passed over", key.EnrolledIn)
	}
}

func TestAnAllTenantsEnrolmentIsRefusedWhereItWouldReachTooFar(t *testing.T) {
	f := newTwoTenants(t)
	f.promotePat(t)
	token := &core.Actor{
		ID: f.homePat.ID, TenantID: f.home.TenantID, Kind: core.ActorUser, Role: core.RoleAdmin,
		TokenID: "tok", Scopes: []core.Scope{core.ScopeAll},
	}
	if _, err := f.l.EnrolSSHKey(authContext(token), core.EnrolSSHKeyInput{PublicKey: pubKey, AllTenants: true}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("a token without account:read = %v, want forbidden", err)
	}
	if got := len(enrolments(t, f.l, f.otherAdmin)); got != 0 {
		t.Errorf("a refused enrolment wrote %d entries in the other tenant", got)
	}
	in := core.EnrolSSHKeyInput{PublicKey: pubKey, AllTenants: true, ActorID: f.homePat.ID}
	if _, err := f.l.EnrolSSHKey(authContext(f.homeAdmin), in); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("an all-tenants enrolment naming an actor = %v, want invalid", err)
	}
}
