// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	gossh "golang.org/x/crypto/ssh"
)

// member makes the account an admin of a new tenant, creating the account the
// first time, and returns the person's actor there.
func member(t *testing.T, st store.Store, tenantKey string, account *core.User) *core.Actor {
	t.Helper()
	ctx := context.Background()
	tenant := &core.Tenant{Key: tenantKey, Name: tenantKey}
	if err := st.Unscoped(ctx, func(u store.UnscopedTx) error { return u.CreateTenant(ctx, tenant) }); err != nil {
		t.Fatalf("creating tenant %q: %v", tenantKey, err)
	}
	actor := &core.Actor{TenantID: tenant.ID, Kind: core.ActorUser, Handle: "pat", DisplayName: "Pat"}
	if err := st.Update(ctx, core.TenantScope{TenantID: tenant.ID}, func(tx store.Tx) error {
		if account.ID == "" {
			if err := tx.CreateUser(ctx, account, ""); err != nil {
				return err
			}
		}
		actor.UserID = account.ID
		if err := tx.CreateActor(ctx, actor); err != nil {
			return err
		}
		return tx.AddMember(ctx, &core.Membership{TenantID: tenant.ID, ActorID: actor.ID, Role: core.RoleAdmin})
	}); err != nil {
		t.Fatalf("adding pat to %q: %v", tenantKey, err)
	}
	actor.Role = core.RoleAdmin
	return actor
}

// The all-tenants enrolment writes one enrolment per tenant, each audited by
// the person's actor there, and the SSH username still picks the tenant.
func TestAKeyEnrolledInEveryTenantOpensEachByItsUsername(t *testing.T) {
	p, _, st := newProvisioner(t)
	ctx := context.Background()
	account := &core.User{Email: "pat@example.com"}
	ho := member(t, st, "ho", account)
	test := member(t, st, "test", account)
	signer, fingerprint := newSigner(t)
	pub := strings.TrimSpace(string(gossh.MarshalAuthorizedKey(signer.PublicKey())))

	session := *ho
	session.SessionID = "session"
	asPat := core.WithActor(core.WithTenant(ctx, core.TenantScope{TenantID: ho.TenantID}), &session)
	key, err := p.service.EnrolSSHKey(asPat, core.EnrolSSHKeyInput{PublicKey: pub, AllTenants: true})
	if err != nil {
		t.Fatalf("EnrolSSHKey all tenants = %v", err)
	}
	if !slices.Equal(key.EnrolledIn, []string{"ho", "test"}) {
		t.Fatalf("enrolled in %v, want ho and test", key.EnrolledIn)
	}

	for _, a := range []*core.Actor{ho, test} {
		var entries []core.AuditEntry
		if err := st.View(ctx, core.TenantScope{TenantID: a.TenantID}, func(tx store.Tx) error {
			entries, err = tx.ListAudit(ctx, core.AuditFilter{Actions: []string{"sshkey.enrol"}})
			return err
		}); err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].ActorID != a.ID {
			t.Errorf("enrolments audited in %s = %+v, want one by pat's actor there", a.TenantID, entries)
		}
	}

	e := &enrolled{store: st, clk: p.clk}
	for username, want := range map[string]*core.Actor{"ho": ho, "test": test} {
		got, err := e.ActorByFingerprint(asUser(username), fingerprint)
		if err != nil || got.ID != want.ID || got.TenantID != want.TenantID {
			t.Errorf("ssh %s@host = %+v, %v; want pat's actor in %s", username, got, err, username)
		}
	}
	if _, err := e.ActorByFingerprint(asUser(NeutralUser), fingerprint); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("the neutral username with the key in two tenants = %v, want the ambiguity refusal", err)
	}
}
