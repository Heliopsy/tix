// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// seedProject creates a workflow and a project straight through the store, so
// that a token can be restricted to something that exists.
func seedProject(t *testing.T, l *Local, scope core.TenantScope, key string) *core.Project {
	t.Helper()
	ctx := context.Background()
	wf := core.Workflow{Key: key, Name: key, Definition: core.WorkflowDefinition{
		Initial: "todo",
		States:  []core.State{{Key: "todo"}, {Key: "done", Terminal: true}},
	}}
	p := core.Project{Key: key, Name: key}
	if err := l.store.Update(ctx, scope, func(tx store.Tx) error {
		if err := tx.PutWorkflow(ctx, &wf); err != nil {
			return err
		}
		p.WorkflowID = wf.ID
		return tx.CreateProject(ctx, &p)
	}); err != nil {
		t.Fatalf("seeding project %q: %v", key, err)
	}
	return &p
}

// scopedActor returns an actor of this tenant holding exactly these scopes.
func scopedActor(admin *core.Actor, scopes ...core.Scope) *core.Actor {
	return &core.Actor{
		ID: admin.ID, TenantID: admin.TenantID, Kind: core.ActorUser,
		Handle: admin.Handle, Scopes: scopes,
	}
}

func TestCreateTokenReturnsItsValueOnce(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	ctx := authContext(admin)

	issued, err := l.CreateToken(ctx, core.CreateTokenInput{
		Name: "agent", Scopes: []core.Scope{core.ScopeTaskRead, core.ScopeTaskClaim},
	})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	if !strings.HasPrefix(issued.Token, auth.TokenPrefix) {
		t.Errorf("token %q does not carry the tix prefix", issued.Token)
	}
	if issued.ActorID != admin.ID || issued.TenantID != scope.TenantID {
		t.Errorf("token = %+v, want the caller's actor and tenant", issued.APIToken)
	}

	bg := context.Background()
	if err := l.store.View(bg, scope, func(tx store.Tx) error {
		if _, err := tx.GetTokenByHash(bg, issued.Token); !core.IsKind(err, core.KindNotFound) {
			t.Error("the raw token value is stored in the database")
		}
		stored, err := tx.GetTokenByHash(bg, auth.HashToken(issued.Token))
		if err != nil {
			return err
		}
		if stored.ID != issued.ID {
			t.Errorf("stored token = %q, want %q", stored.ID, issued.ID)
		}
		return nil
	}); err != nil {
		t.Fatalf("reading the token back: %v", err)
	}

	listed, err := l.ListTokens(ctx, "")
	if err != nil {
		t.Fatalf("ListTokens: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != issued.ID {
		t.Fatalf("ListTokens = %+v, want the one token", listed)
	}
	raw, err := json.Marshal(listed)
	if err != nil {
		t.Fatalf("marshalling tokens: %v", err)
	}
	if strings.Contains(string(raw), issued.Token) {
		t.Error("a listed token still carries its secret value")
	}
}

// Minting a token with scopes the caller does not hold would be an escalation
// that leaves the caller's own permissions untouched.
func TestCreateTokenRefusesScopeEscalation(t *testing.T) {
	l, _, _, admin := newLocal(t)
	limited := scopedActor(admin, core.ScopeTokenAdmin, core.ScopeTaskRead)
	ctx := authContext(limited)

	cases := []struct {
		name   string
		scopes []core.Scope
	}{
		{"a scope the caller lacks", []core.Scope{core.ScopeTaskDelete}},
		{"every scope", []core.Scope{core.ScopeAll}},
		{"one held and one not", []core.Scope{core.ScopeTaskRead, core.ScopeUserAdmin}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := l.CreateToken(ctx, core.CreateTokenInput{Name: "agent", Scopes: tc.scopes}); !core.IsKind(err, core.KindForbidden) {
				t.Errorf("CreateToken = %v, want forbidden", err)
			}
		})
	}

	issued, err := l.CreateToken(ctx, core.CreateTokenInput{
		Name: "agent", Scopes: []core.Scope{core.ScopeTaskRead},
	})
	if err != nil {
		t.Fatalf("minting a token within the caller's own scopes: %v", err)
	}
	if len(issued.Scopes) != 1 || issued.Scopes[0] != core.ScopeTaskRead {
		t.Errorf("scopes = %v, want only task read", issued.Scopes)
	}
}

func TestCreateTokenValidatesItsInput(t *testing.T) {
	l, _, _, admin := newLocal(t)
	ctx := authContext(admin)

	cases := []struct {
		name string
		in   core.CreateTokenInput
	}{
		{"no name", core.CreateTokenInput{Scopes: []core.Scope{core.ScopeTaskRead}}},
		{"no scopes", core.CreateTokenInput{Name: "agent"}},
		{"unknown scope", core.CreateTokenInput{Name: "agent", Scopes: []core.Scope{"task:teleport"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := l.CreateToken(ctx, tc.in); !core.IsKind(err, core.KindInvalid) {
				t.Errorf("CreateToken = %v, want invalid", err)
			}
		})
	}
}

func TestCreateTokenRefusesUnknownSubjects(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	ctx := authContext(admin)
	other := otherTenantActor(t, l)

	if _, err := l.CreateToken(ctx, core.CreateTokenInput{
		Name: "agent", ActorID: other.ID, Scopes: []core.Scope{core.ScopeTaskRead},
	}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("an actor of another tenant = %v, want not found", err)
	}
	if _, err := l.CreateToken(ctx, core.CreateTokenInput{
		Name: "agent", ProjectID: "01J000000000000000000A", Scopes: []core.Scope{core.ScopeTaskRead},
	}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("an unknown project = %v, want not found", err)
	}

	p := seedProject(t, l, scope, "alpha")
	issued, err := l.CreateToken(ctx, core.CreateTokenInput{
		Name: "agent", ProjectID: p.ID, Scopes: []core.Scope{core.ScopeTaskRead},
	})
	if err != nil {
		t.Fatalf("restricting a token to a project: %v", err)
	}
	if issued.ProjectID != p.ID {
		t.Errorf("token project = %q, want %q", issued.ProjectID, p.ID)
	}
}

// --project used to be passed straight through as the project's identifier,
// so a key like "infra" fell all the way to a foreign key violation instead
// of a clean "not found". CreateToken now resolves a key exactly as
// task-facing commands already do, by key, by identifier or refusing
// something that is neither.
func TestCreateTokenResolvesProjectByKeyOrID(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	ctx := authContext(admin)
	p := seedProject(t, l, scope, "infra")

	cases := []struct {
		name     string
		project  string
		wantErr  bool
		wantKind core.Kind
	}{
		{name: "by key", project: p.Key},
		{name: "by key, mixed case", project: strings.ToUpper(p.Key)},
		{name: "by id", project: p.ID},
		{name: "unknown key", project: "no-such-project", wantErr: true, wantKind: core.KindNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			issued, err := l.CreateToken(ctx, core.CreateTokenInput{
				Name: "agent-" + tc.name, ProjectID: tc.project, Scopes: []core.Scope{core.ScopeTaskRead},
			})
			if tc.wantErr {
				if !core.IsKind(err, tc.wantKind) {
					t.Fatalf("CreateToken(%q) = %v, want kind %v", tc.project, err, tc.wantKind)
				}
				if core.IsKind(err, core.KindPrecondition) {
					t.Fatalf("CreateToken(%q) leaked a precondition error instead of not found", tc.project)
				}
				return
			}
			if err != nil {
				t.Fatalf("CreateToken(%q): %v", tc.project, err)
			}
			if issued.ProjectID != p.ID {
				t.Errorf("token project = %q, want the resolved id %q", issued.ProjectID, p.ID)
			}
		})
	}
}

// --actor on token create used to accept only an identifier. It now resolves
// a handle too, mirroring GetActor.
func TestCreateTokenResolvesActorByHandleOrID(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	ctx := authContext(admin)

	agent := core.Actor{Kind: core.ActorAgent, Handle: "ci-bot", Scopes: []core.Scope{core.ScopeAll}}
	if err := l.store.Update(context.Background(), scope, func(tx store.Tx) error {
		return tx.CreateActor(context.Background(), &agent)
	}); err != nil {
		t.Fatalf("seeding actor: %v", err)
	}

	cases := []struct {
		name string
		ref  string
	}{
		{"by id", agent.ID},
		{"by handle", "ci-bot"},
		{"by handle, mixed case", "CI-BOT"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			issued, err := l.CreateToken(ctx, core.CreateTokenInput{
				Name: "agent-" + tc.name, ActorID: tc.ref, Scopes: []core.Scope{core.ScopeTaskRead},
			})
			if err != nil {
				t.Fatalf("CreateToken(actor=%q): %v", tc.ref, err)
			}
			if issued.ActorID != agent.ID {
				t.Errorf("token actor = %q, want %q", issued.ActorID, agent.ID)
			}
		})
	}

	if _, err := l.CreateToken(ctx, core.CreateTokenInput{
		Name: "agent-bad", ActorID: "no-such-actor", Scopes: []core.Scope{core.ScopeTaskRead},
	}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("CreateToken for an unknown actor = %v, want not found", err)
	}
}

// A token restricted to one project cannot mint a token for another.
func TestCreateTokenHonoursTheCallersProject(t *testing.T) {
	l, _, _, admin := newLocal(t)
	bound := scopedActor(admin, core.ScopeTokenAdmin, core.ScopeTaskRead)
	bound.ProjectID = "project-one"
	ctx := authContext(bound)

	if _, err := l.CreateToken(ctx, core.CreateTokenInput{
		Name: "agent", ProjectID: "project-two", Scopes: []core.Scope{core.ScopeTaskRead},
	}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("CreateToken for another project = %v, want forbidden", err)
	}
}

func TestRevokeTokenStopsItAuthenticatingAndIsIdempotent(t *testing.T) {
	l, clk, scope, admin := newLocal(t)
	ctx := authContext(admin)

	issued, err := l.CreateToken(ctx, core.CreateTokenInput{
		Name: "agent", Scopes: []core.Scope{core.ScopeTaskRead},
	})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	if err := l.RevokeToken(ctx, issued.ID); err != nil {
		t.Fatalf("RevokeToken: %v", err)
	}
	if err := l.RevokeToken(ctx, issued.ID); err != nil {
		t.Errorf("a second revocation = %v, want it to be idempotent", err)
	}

	bg := context.Background()
	if err := l.store.View(bg, scope, func(tx store.Tx) error {
		stored, err := tx.GetTokenByHash(bg, auth.HashToken(issued.Token))
		if err != nil {
			return err
		}
		if stored.RevokedAt == nil {
			t.Fatal("the token was not marked revoked")
		}
		if stored.Active(clk.Now()) {
			t.Error("a revoked token is still active")
		}
		return nil
	}); err != nil {
		t.Fatalf("reading the token back: %v", err)
	}
}

func TestRevokeTokenOfAnotherTenantIsNotFound(t *testing.T) {
	l, _, _, admin := newLocal(t)
	other := otherTenantActor(t, l)

	issued, err := l.CreateToken(authContext(other), core.CreateTokenInput{
		Name: "agent", Scopes: []core.Scope{core.ScopeTaskRead},
	})
	if err != nil {
		t.Fatalf("CreateToken in the other tenant: %v", err)
	}

	ctx := authContext(admin)
	foreign := l.RevokeToken(ctx, issued.ID)
	unknown := l.RevokeToken(ctx, "01J000000000000000000A")
	if !core.IsKind(foreign, core.KindNotFound) {
		t.Errorf("revoking another tenant's token = %v, want not found", foreign)
	}
	if !core.IsKind(unknown, core.KindNotFound) {
		t.Errorf("revoking an unknown token = %v, want not found", unknown)
	}
	shape := func(err error, id string) string { return strings.ReplaceAll(err.Error(), id, "ID") }
	if shape(foreign, issued.ID) != shape(unknown, "01J000000000000000000A") {
		t.Errorf("another tenant's token answers %q but an unknown one answers %q", foreign, unknown)
	}
	if err := l.RevokeToken(ctx, "  "); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("an empty identifier = %v, want invalid", err)
	}
}

func TestListTokensIsTenantScoped(t *testing.T) {
	l, _, _, admin := newLocal(t)
	other := otherTenantActor(t, l)

	if _, err := l.ListTokens(authContext(admin), other.ID); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("listing another tenant's actor = %v, want not found", err)
	}
	listed, err := l.ListTokens(authContext(admin), admin.ID)
	if err != nil {
		t.Fatalf("ListTokens: %v", err)
	}
	if len(listed) != 0 {
		t.Errorf("ListTokens = %d tokens, want none", len(listed))
	}
}

func TestTokenAdminIsDeniedToNonAdmins(t *testing.T) {
	l, _, _, admin := newLocal(t)
	member := &core.Actor{ID: admin.ID, TenantID: admin.TenantID, Kind: core.ActorUser, Role: core.RoleMember}
	ctx := authContext(member)

	if _, err := l.CreateToken(ctx, core.CreateTokenInput{Name: "agent", Scopes: []core.Scope{core.ScopeTaskRead}}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("CreateToken as a member = %v, want forbidden", err)
	}
	if _, err := l.ListTokens(ctx, ""); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("ListTokens as a member = %v, want forbidden", err)
	}
	if err := l.RevokeToken(ctx, "whatever"); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("RevokeToken as a member = %v, want forbidden", err)
	}
}

func TestTokenMutationsRecordAuditAndEvent(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	ctx := authContext(admin)

	beforeEvents, beforeAudits := countRows(t, l, scope)
	issued, err := l.CreateToken(ctx, core.CreateTokenInput{
		Name: "agent", Scopes: []core.Scope{core.ScopeTaskRead},
	})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	if err := l.RevokeToken(ctx, issued.ID); err != nil {
		t.Fatalf("RevokeToken: %v", err)
	}
	afterEvents, afterAudits := countRows(t, l, scope)

	if afterEvents-beforeEvents != 2 {
		t.Errorf("events grew by %d, want 2", afterEvents-beforeEvents)
	}
	if afterAudits-beforeAudits != 2 {
		t.Errorf("audit entries grew by %d, want 2", afterAudits-beforeAudits)
	}
}
