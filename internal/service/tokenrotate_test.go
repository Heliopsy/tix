// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// storeLookup reads tokens for the verifier straight from the service's store,
// so "the old value is refused" is decided by the authentication path itself.
type storeLookup struct {
	l     *Local
	scope core.TenantScope
}

func (s storeLookup) TokenByHash(ctx context.Context, hash string) (*core.APIToken, error) {
	var out *core.APIToken
	err := s.l.store.View(ctx, s.scope, func(tx store.Tx) error {
		tok, err := tx.GetTokenByHash(ctx, hash)
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		out = tok
		return err
	})
	return out, err
}

// verify presents a token value the way a request does.
func verify(t *testing.T, l *Local, clk clock.Clock, scope core.TenantScope, value string) (*core.Actor, error) {
	t.Helper()
	return auth.NewTokenVerifier(storeLookup{l: l, scope: scope}, clk).Verify(context.Background(), value)
}

// seedAgent creates an agent actor of this tenant.
func seedAgent(t *testing.T, l *Local, scope core.TenantScope, handle string) core.Actor {
	t.Helper()
	ctx := context.Background()
	agent := core.Actor{Kind: core.ActorAgent, Handle: handle, Scopes: []core.Scope{core.ScopeAll}}
	if err := l.store.Update(ctx, scope, func(tx store.Tx) error {
		return tx.CreateActor(ctx, &agent)
	}); err != nil {
		t.Fatalf("seeding agent %q: %v", handle, err)
	}
	return agent
}

func TestRotateTokenReplacesYourOwn(t *testing.T) {
	l, clk, scope, admin := newLocal(t)
	ctx := authContext(admin)
	project := seedProject(t, l, scope, "OPS")
	expires := clk.Now().Add(90 * 24 * time.Hour)
	old, err := l.CreateToken(ctx, core.CreateTokenInput{Name: "worker", ProjectID: project.Key,
		Scopes: []core.Scope{core.ScopeTaskRead, core.ScopeTaskClaim}, ExpiresAt: &expires})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	clk.Advance(80 * 24 * time.Hour)

	{
		issued, err := l.RotateToken(ctx, "worker", core.RotateTokenInput{})
		if err != nil {
			t.Fatalf("RotateToken by name: %v", err)
		}
		if issued.ID == old.ID || issued.Token == "" || issued.Token == old.Token {
			t.Fatalf("rotation returned %+v, want a new token with its own value", issued.APIToken)
		}
		if issued.Name != "worker" || issued.ActorID != admin.ID || issued.ProjectID != project.ID ||
			strings.Join(scopeStrings(issued.Scopes), ",") != "task:read,task:claim" {
			t.Errorf("rotated token = %+v, want the old name, actor, project and scopes", issued.APIToken)
		}
		if want := clk.Now().Add(90 * 24 * time.Hour); issued.ExpiresAt == nil || !issued.ExpiresAt.Equal(want) {
			t.Errorf("rotated token expires %v, want the old 90 day lifetime from now, %v", issued.ExpiresAt, want)
		}
		if _, err := verify(t, l, clk, scope, old.Token); !core.IsKind(err, core.KindUnauthenticated) {
			t.Errorf("the old value after an immediate rotation = %v, want unauthenticated", err)
		}
		if _, err := verify(t, l, clk, scope, issued.Token); err != nil {
			t.Errorf("the new value = %v, want it to authenticate", err)
		}
	}
}

func TestRotateTokenKeepsANonExpiringTokenNonExpiring(t *testing.T) {
	l, _, _, admin := newLocal(t)
	ctx := authContext(admin)
	old, err := l.CreateToken(ctx, core.CreateTokenInput{Name: "forever", Scopes: []core.Scope{core.ScopeTaskRead}})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	issued, err := l.RotateToken(ctx, old.ID, core.RotateTokenInput{})
	if err != nil {
		t.Fatalf("RotateToken: %v", err)
	}
	if issued.ExpiresAt != nil {
		t.Errorf("rotating a token without an expiry gave one expiring %v", issued.ExpiresAt)
	}
}

func TestAnAdministratorRotatesAnAgentsToken(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	ctx := authContext(admin)
	agent := seedAgent(t, l, scope, "ci-bot")
	_, err := l.CreateToken(ctx, core.CreateTokenInput{Name: "ci", ActorID: agent.ID,
		Scopes: []core.Scope{core.ScopeTaskRead}})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	if _, err := l.RotateToken(ctx, "ci", core.RotateTokenInput{ActorID: "someone-else"}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("rotating with an actor that does not exist = %v, want not found", err)
	}
	if _, err := l.RotateToken(ctx, "ci", core.RotateTokenInput{ActorID: admin.ID}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("rotating with an actor that does not hold the token = %v, want not found", err)
	}
	issued, err := l.RotateToken(ctx, "ci", core.RotateTokenInput{ActorID: "ci-bot"})
	if err != nil {
		t.Fatalf("RotateToken: %v", err)
	}
	if issued.ActorID != agent.ID {
		t.Errorf("rotated token acts as %q, want the agent %q", issued.ActorID, agent.ID)
	}

	member := &core.Actor{ID: admin.ID, TenantID: admin.TenantID, Kind: core.ActorUser, Role: core.RoleMember}
	_, err = l.RotateToken(authContext(member), issued.ID, core.RotateTokenInput{})
	if !core.IsKind(err, core.KindForbidden) {
		t.Fatalf("a member rotating an agent's token = %v, want forbidden", err)
	}
	var cerr *core.Error
	if !errors.As(err, &cerr) || cerr.Details["scope"] != string(core.ScopeTokenAdmin) {
		t.Errorf("the refusal %v does not name token:admin", err)
	}
}

// An agent holding nothing but its work scopes rotates the token it runs on,
// which is the point of the feature: no administrator has to be woken up.
func TestAnAgentRotatesItsOwnToken(t *testing.T) {
	l, clk, scope, admin := newLocal(t)
	agent := seedAgent(t, l, scope, "worker-bot")
	old, err := l.CreateToken(authContext(admin), core.CreateTokenInput{Name: "worker", ActorID: agent.ID,
		Scopes: []core.Scope{core.ScopeTaskRead, core.ScopeTaskClaim}})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	self, err := verify(t, l, clk, scope, old.Token)
	if err != nil {
		t.Fatalf("verifying the agent's token: %v", err)
	}
	issued, err := l.RotateToken(authContext(self), old.ID, core.RotateTokenInput{Grace: core.Duration(time.Minute)})
	if err != nil {
		t.Fatalf("the agent rotating its own token: %v", err)
	}
	if issued.ActorID != agent.ID {
		t.Errorf("rotated token acts as %q, want %q", issued.ActorID, agent.ID)
	}

	sibling, err := l.CreateToken(authContext(admin), core.CreateTokenInput{Name: "worker-admin", ActorID: agent.ID,
		Scopes: []core.Scope{core.ScopeTaskRead, core.ScopeExport}})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	self, err = verify(t, l, clk, scope, issued.Token)
	if err != nil {
		t.Fatalf("verifying the rotated token: %v", err)
	}
	_, err = l.RotateToken(authContext(self), sibling.ID, core.RotateTokenInput{})
	if !core.IsKind(err, core.KindForbidden) {
		t.Fatalf("an agent rotating a sibling carrying a scope it lacks = %v, want forbidden", err)
	}
	var cerr *core.Error
	if !errors.As(err, &cerr) || cerr.Details["scope"] != string(core.ScopeExport) {
		t.Errorf("the refusal %v does not name the scope it lacks", err)
	}
}

// An administrator token that does not hold a scope cannot use rotation to
// learn a fresh value for a token that does.
func TestRotateTokenRespectsTheScopeCeiling(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	agent := seedAgent(t, l, scope, "exporter")
	old, err := l.CreateToken(authContext(admin), core.CreateTokenInput{Name: "exports", ActorID: agent.ID,
		Scopes: []core.Scope{core.ScopeTaskRead, core.ScopeExport}})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	limited := scopedActor(admin, core.ScopeTokenAdmin, core.ScopeTaskRead)
	_, err = l.RotateToken(authContext(limited), old.ID, core.RotateTokenInput{})
	if !core.IsKind(err, core.KindForbidden) {
		t.Fatalf("rotating past the rotator's scopes = %v, want forbidden", err)
	}
	var cerr *core.Error
	if !errors.As(err, &cerr) || cerr.Details["scope"] != string(core.ScopeExport) {
		t.Errorf("the refusal %v does not name export", err)
	}
	listed, err := l.ListTokens(authContext(admin), agent.ID)
	if err != nil {
		t.Fatalf("ListTokens: %v", err)
	}
	if len(listed) != 1 || listed[0].RevokedAt != nil {
		t.Errorf("a refused rotation changed the tokens: %+v", listed)
	}
}

func TestRotateTokenHonoursTheGraceWindowAndThenEndsIt(t *testing.T) {
	l, clk, scope, admin := newLocal(t)
	ctx := authContext(admin)
	old, err := l.CreateToken(ctx, core.CreateTokenInput{Name: "swap", Scopes: []core.Scope{core.ScopeTaskRead}})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	issued, err := l.RotateToken(ctx, old.ID, core.RotateTokenInput{Grace: core.Duration(10 * time.Minute)})
	if err != nil {
		t.Fatalf("RotateToken: %v", err)
	}

	clk.Advance(5 * time.Minute)
	if _, err := verify(t, l, clk, scope, old.Token); err != nil {
		t.Errorf("the old value inside the window = %v, want it to authenticate", err)
	}
	listed, err := l.ListTokens(ctx, "")
	if err != nil {
		t.Fatalf("ListTokens: %v", err)
	}
	for _, tok := range listed {
		if tok.ID == old.ID && !tok.Retiring(clk.Now()) {
			t.Errorf("the old token inside its window is not retiring: %+v", tok)
		}
	}
	if _, err := l.RotateToken(ctx, old.ID, core.RotateTokenInput{}); !core.IsKind(err, core.KindConflict) {
		t.Errorf("rotating a retiring token again = %v, want conflict", err)
	}

	clk.Advance(6 * time.Minute)
	if _, err := verify(t, l, clk, scope, old.Token); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("the old value after the window = %v, want unauthenticated", err)
	}
	if _, err := verify(t, l, clk, scope, issued.Token); err != nil {
		t.Errorf("the new value after the window = %v, want it to authenticate", err)
	}
	if _, err := l.RotateToken(ctx, old.ID, core.RotateTokenInput{}); !core.IsKind(err, core.KindConflict) {
		t.Errorf("rotating a revoked token = %v, want conflict", err)
	}
}

func TestRotateTokenRefusesAGraceOutsideItsRange(t *testing.T) {
	l, _, _, admin := newLocal(t)
	ctx := authContext(admin)
	if _, err := l.CreateToken(ctx, core.CreateTokenInput{Name: "g", Scopes: []core.Scope{core.ScopeTaskRead}}); err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	for _, grace := range []time.Duration{-time.Minute, 25 * time.Hour} {
		_, err := l.RotateToken(ctx, "g", core.RotateTokenInput{Grace: core.Duration(grace)})
		var cerr *core.Error
		if !core.IsKind(err, core.KindInvalid) || !errors.As(err, &cerr) || cerr.Details[core.DetailField] != "grace" {
			t.Errorf("a grace of %s = %v, want invalid naming the grace field", grace, err)
		}
	}
	if _, err := l.RotateToken(ctx, "g", core.RotateTokenInput{Grace: core.Duration(24 * time.Hour)}); err != nil {
		t.Errorf("a grace of exactly 24h = %v, want it accepted", err)
	}
	if _, err := l.RotateToken(ctx, " ", core.RotateTokenInput{}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("an empty reference = %v, want invalid", err)
	}
	if _, err := l.RotateToken(ctx, "nosuch", core.RotateTokenInput{}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("an unknown token = %v, want not found", err)
	}
}

// A grace window is not a way round revocation: revoking the old token or
// disabling its user ends it at once.
func TestARetiringTokenStopsWhenRevokedOrItsUserIsDisabled(t *testing.T) {
	l, clk, scope, admin := newLocal(t)
	ctx := authContext(admin)
	old, err := l.CreateToken(ctx, core.CreateTokenInput{Name: "cut", Scopes: []core.Scope{core.ScopeTaskRead}})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	if _, err := l.RotateToken(ctx, old.ID, core.RotateTokenInput{Grace: core.Duration(time.Hour)}); err != nil {
		t.Fatalf("RotateToken: %v", err)
	}
	if err := l.RevokeToken(ctx, old.ID); err != nil {
		t.Fatalf("RevokeToken: %v", err)
	}
	if _, err := verify(t, l, clk, scope, old.Token); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("a retiring token after revocation = %v, want unauthenticated", err)
	}

	agent := seedAgent(t, l, scope, "doomed")
	second, err := l.CreateToken(ctx, core.CreateTokenInput{Name: "doomed", ActorID: agent.ID,
		Scopes: []core.Scope{core.ScopeTaskRead}})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	if _, err := l.RotateToken(ctx, second.ID, core.RotateTokenInput{Grace: core.Duration(time.Hour)}); err != nil {
		t.Fatalf("RotateToken: %v", err)
	}
	bg := context.Background()
	if err := l.store.Update(bg, scope, func(tx store.Tx) error {
		n, err := tx.RevokeActorTokens(bg, agent.ID, clk.Now())
		if err == nil && n != 2 {
			t.Errorf("revoking the actor's tokens ended %d, want the retiring one and its replacement", n)
		}
		return err
	}); err != nil {
		t.Fatalf("RevokeActorTokens: %v", err)
	}
	if _, err := verify(t, l, clk, scope, second.Token); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("a retiring token after its actor's tokens were revoked = %v, want unauthenticated", err)
	}
}

func TestRotateTokenIsAuditedWithoutItsSecret(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	ctx := authContext(admin)
	agent := seedAgent(t, l, scope, "audited")
	mine, err := l.CreateToken(ctx, core.CreateTokenInput{Name: "mine", Scopes: []core.Scope{core.ScopeTaskRead}})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	theirs, err := l.CreateToken(ctx, core.CreateTokenInput{Name: "theirs", ActorID: agent.ID,
		Scopes: []core.Scope{core.ScopeTaskRead}})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	beforeEvents, beforeAudits := countRows(t, l, scope)
	rotatedMine, err := l.RotateToken(ctx, mine.ID, core.RotateTokenInput{})
	if err != nil {
		t.Fatalf("RotateToken: %v", err)
	}
	rotatedTheirs, err := l.RotateToken(ctx, theirs.ID, core.RotateTokenInput{Grace: core.Duration(time.Minute)})
	if err != nil {
		t.Fatalf("RotateToken: %v", err)
	}
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents-beforeEvents != 2 || afterAudits-beforeAudits != 2 {
		t.Errorf("two rotations wrote %d events and %d audit entries, want 2 of each",
			afterEvents-beforeEvents, afterAudits-beforeAudits)
	}

	for _, tc := range []struct {
		old, issued *core.IssuedToken
		action      string
		holder      string
	}{
		{mine, rotatedMine, auditTokenRotate, admin.ID},
		{theirs, rotatedTheirs, auditTokenRotateOther, agent.ID},
	} {
		entries, _, err := l.ListAudit(ctx, core.AuditFilter{SubjectType: "api_token", SubjectID: tc.issued.ID})
		if err != nil {
			t.Fatalf("ListAudit: %v", err)
		}
		if len(entries) != 1 || entries[0].Action != tc.action {
			t.Fatalf("the rotation to %s recorded %+v, want one %s", tc.issued.Name, entries, tc.action)
		}
		e := entries[0]
		if e.ActorID != admin.ID {
			t.Errorf("%s recorded against %q, want the rotator %q", tc.action, e.ActorID, admin.ID)
		}
		raw, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("marshalling the entry: %v", err)
		}
		for _, want := range []string{tc.old.ID, tc.issued.ID, tc.holder} {
			if !strings.Contains(string(raw), want) {
				t.Errorf("%s entry does not name %q: %s", tc.action, want, raw)
			}
		}
		for _, secret := range []string{tc.old.Token, tc.issued.Token, auth.TokenPrefix} {
			if strings.Contains(string(raw), secret) {
				t.Errorf("%s entry carries a token value: %s", tc.action, raw)
			}
		}
	}
	filtered, _, err := l.ListAudit(ctx, core.AuditFilter{Actions: []string{auditTokenRotateOther}})
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	if len(filtered) != 1 || filtered[0].SubjectID != rotatedTheirs.ID {
		t.Errorf("filtering by %s returned %+v, want the one rotation of another actor's token",
			auditTokenRotateOther, filtered)
	}
}
