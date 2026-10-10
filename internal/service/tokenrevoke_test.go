// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// revocationActions returns the audit actions recorded against one token, so
// an assertion reads the action column of that token's entries and not every
// entry the test happened to write.
func revocationActions(t *testing.T, l *Local, ctx context.Context, tokenID string) []string {
	t.Helper()
	entries, _, err := l.ListAudit(ctx, core.AuditFilter{SubjectType: "api_token", SubjectID: tokenID})
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	var out []string
	for _, e := range entries {
		if e.Action == auditTokenCreate {
			continue
		}
		out = append(out, e.Action)
	}
	return out
}

// seedActorFor creates a second actor of this tenant, so a token can belong to
// somebody other than the caller.
func seedActorFor(t *testing.T, l *Local, scope core.TenantScope, handle string) core.Actor {
	t.Helper()
	ctx := context.Background()
	actor := core.Actor{Kind: core.ActorUser, Handle: handle, Role: core.RoleMember}
	if err := l.store.Update(ctx, scope, func(tx store.Tx) error {
		return tx.CreateActor(ctx, &actor)
	}); err != nil {
		t.Fatalf("seeding actor %q: %v", handle, err)
	}
	return actor
}

// TestRevokingYourOwnTokenIsAuditedAsYourOwn is the baseline the next test is
// distinguished from. Without it, "revoke_other was recorded" proves nothing:
// a trail that recorded revoke_other for every revocation would pass.
func TestRevokingYourOwnTokenIsAuditedAsYourOwn(t *testing.T) {
	l, _, _, admin := newLocal(t)
	ctx := authContext(admin)

	issued, err := l.CreateToken(ctx, core.CreateTokenInput{
		Name: "mine", Scopes: []core.Scope{core.ScopeTaskRead}})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	if err := l.RevokeToken(ctx, issued.ID); err != nil {
		t.Fatalf("RevokeToken: %v", err)
	}

	got := revocationActions(t, l, ctx, issued.ID)
	if len(got) != 1 || got[0] != auditTokenRevoke {
		t.Fatalf("revoking your own token records %v, want [%s]", got, auditTokenRevoke)
	}
}

// TestRevokingAnotherActorsTokenIsAuditedDistinguishably is the whole point of
// letting an administrator reach somebody else's credential: afterwards an
// operator reading the trail has to be able to tell who ended what, and
// whether it was theirs to end.
func TestRevokingAnotherActorsTokenIsAuditedDistinguishably(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	ctx := authContext(admin)
	other := seedActorFor(t, l, scope, "carol")

	issued, err := l.CreateToken(ctx, core.CreateTokenInput{
		Name: "carols-agent", ActorID: other.ID, Scopes: []core.Scope{core.ScopeTaskRead}})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	if issued.ActorID != other.ID {
		t.Fatalf("the seeded token belongs to %q, want %q", issued.ActorID, other.ID)
	}
	if err := l.RevokeToken(ctx, issued.ID); err != nil {
		t.Fatalf("RevokeToken: %v", err)
	}

	got := revocationActions(t, l, ctx, issued.ID)
	if len(got) != 1 {
		t.Fatalf("revoking another actor's token records %v, want exactly one entry", got)
	}
	if got[0] == auditTokenRevoke {
		t.Fatalf("revoking another actor's token is recorded as %q, the same action as revoking "+
			"your own; an operator reading the trail cannot tell the two apart", got[0])
	}
	if got[0] != auditTokenRevokeOther {
		t.Fatalf("revoking another actor's token records %q, want %q", got[0], auditTokenRevokeOther)
	}

	// Who performed it. The entry's own actor, not the token's owner.
	entries, _, err := l.ListAudit(ctx, core.AuditFilter{Actions: []string{auditTokenRevokeOther}})
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("the trail holds %d %s entries, want 1", len(entries), auditTokenRevokeOther)
	}
	if entries[0].ActorID != admin.ID {
		t.Errorf("the entry names %q as the actor, want the administrator %q",
			entries[0].ActorID, admin.ID)
	}
}
