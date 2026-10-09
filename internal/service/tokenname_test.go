// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// liveToken is the input for a token that does nothing but exist.
func liveToken(name string) core.CreateTokenInput {
	return core.CreateTokenInput{Name: name, Scopes: []core.Scope{core.ScopeTaskRead}}
}

// TestTokenNamesAreUniqueWithinATenant covers the rule where it lives, which
// is the service, so the command line and the HTTP API are held to it as well
// as the browser.
func TestTokenNamesAreUniqueWithinATenant(t *testing.T) {
	l, _, _, admin := newLocal(t)
	ctx := authContext(admin)

	if _, err := l.CreateToken(ctx, liveToken("ci")); err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	_, err := l.CreateToken(ctx, liveToken("ci"))
	if !core.IsKind(err, core.KindConflict) {
		t.Fatalf("a second token named ci = %v, want a conflict", err)
	}
	if got := core.FieldOf(err); got != "name" {
		t.Errorf("the refusal names field %q, want name, so a form cannot place it", got)
	}
	tokens, err := l.ListTokens(ctx, "")
	if err != nil {
		t.Fatalf("ListTokens: %v", err)
	}
	if len(tokens) != 1 {
		t.Errorf("the tenant holds %d tokens, want 1", len(tokens))
	}
}

// TestRevokingATokenFreesItsName is the rotation case: revoke the credential
// that leaked, then mint its replacement under the name everything already
// refers to.
func TestRevokingATokenFreesItsName(t *testing.T) {
	l, _, _, admin := newLocal(t)
	ctx := authContext(admin)

	first, err := l.CreateToken(ctx, liveToken("ci"))
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	if err := l.RevokeToken(ctx, first.ID); err != nil {
		t.Fatalf("RevokeToken: %v", err)
	}
	second, err := l.CreateToken(ctx, liveToken("ci"))
	if err != nil {
		t.Fatalf("reissuing a revoked name: %v", err)
	}
	if second.ID == first.ID {
		t.Error("reissuing returned the revoked token")
	}
}

// TestTokenNamesAreUniquePerTenantNotGlobally pins the scope of the rule. One
// tenant must not be able to learn, or constrain, what another has named its
// credentials.
func TestTokenNamesAreUniquePerTenantNotGlobally(t *testing.T) {
	l, _, _, admin := newLocal(t)
	ctx := authContext(admin)

	if _, err := l.CreateToken(ctx, liveToken("ci")); err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	other, err := l.CreateTenant(ctx, core.CreateTenantInput{Key: "beta", Name: "Beta"})
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	theirs := core.Actor{Kind: core.ActorUser, Handle: "beta-admin",
		Role: core.RoleAdmin, Scopes: []core.Scope{core.ScopeAll}}
	seedActors(t, l, other.ID, theirs)
	their := core.Actor{ID: actorID(t, l, other.ID, "beta-admin"), TenantID: other.ID,
		Kind: core.ActorUser, Handle: "beta-admin", Role: core.RoleAdmin,
		Scopes: []core.Scope{core.ScopeAll}}
	if _, err := l.CreateToken(authContext(&their), liveToken("ci")); err != nil {
		t.Fatalf("a token named ci in another tenant was refused: %v", err)
	}
}

// actorID resolves a seeded actor's identifier by handle.
func actorID(t *testing.T, l *Local, tenantID, handle string) string {
	t.Helper()
	ctx := context.Background()
	var id string
	if err := l.store.View(ctx, core.TenantScope{TenantID: tenantID}, func(tx store.Tx) error {
		a, err := tx.GetActorByHandle(ctx, handle)
		if err != nil {
			return err
		}
		id = a.ID
		return nil
	}); err != nil {
		t.Fatalf("resolving actor %q: %v", handle, err)
	}
	return id
}

// TestTokenValidationNamesTheFieldItRefused is what lets a form put a refusal
// beside the control that caused it instead of on a page of its own.
func TestTokenValidationNamesTheFieldItRefused(t *testing.T) {
	for _, tc := range []struct {
		name  string
		in    core.CreateTokenInput
		field string
	}{
		{"no name", core.CreateTokenInput{Scopes: []core.Scope{core.ScopeTaskRead}}, "name"},
		{"no scope", core.CreateTokenInput{Name: "agent"}, "scopes"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.in.Validate()
			if err == nil {
				t.Fatalf("Validate accepted %+v", tc.in)
			}
			if got := core.FieldOf(err); got != tc.field {
				t.Errorf("the refusal names field %q, want %q", got, tc.field)
			}
		})
	}
}
