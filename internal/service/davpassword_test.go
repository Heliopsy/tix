// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// davFixture is a tenant with one project, an administrator, and a member
// who owns app passwords.
type davFixture struct {
	l       *Local
	clk     *clock.Fake
	scope   core.TenantScope
	admin   context.Context
	adminID *core.Actor
	member  context.Context
	actor   *core.Actor
	project *core.Project
}

// davTestWorkflow reaches every state from every other, and has a cancelled state.
func davTestWorkflow() core.WorkflowDefinition {
	keys := []string{"todo", "doing", "blocked", "done", "cancelled"}
	def := core.WorkflowDefinition{Initial: "todo", States: []core.State{
		{Key: "todo", Label: "To do", Category: core.CategoryTodo},
		{Key: "doing", Label: "Doing", Category: core.CategoryInProgress},
		{Key: "blocked", Label: "Blocked", Category: core.CategoryBlocked},
		{Key: "done", Label: "Done", Terminal: true, Category: core.CategoryDone},
		{Key: "cancelled", Label: "Cancelled", Terminal: true, Category: core.CategoryCancelled},
	}}
	for _, from := range keys {
		for _, to := range keys {
			if from != to {
				def.Transitions = append(def.Transitions, core.Transition{From: from, To: to})
			}
		}
	}
	return def
}

func newDAVFixture(t *testing.T, opts ...Option) davFixture {
	t.Helper()
	l, clk, scope, admin := newLocalWith(t, opts...)
	project := seedTaskProjectWith(t, l, scope, "infra", davTestWorkflow())
	user := seedUser(t, l, admin, "bob@example.com")
	member := &core.Actor{ID: user.ID, TenantID: scope.TenantID, Kind: core.ActorUser, Handle: "bob", Role: core.RoleMember}
	return davFixture{l: l, clk: clk, scope: scope, admin: taskContext(admin), adminID: admin,
		member: taskContext(member), actor: member, project: project}
}

// person adds another member and returns a context acting as them.
func (f davFixture) person(t *testing.T, email, handle string) (context.Context, *core.Actor) {
	t.Helper()
	user := seedUser(t, f.l, f.adminID, email)
	a := &core.Actor{ID: user.ID, TenantID: f.scope.TenantID, Kind: core.ActorUser, Handle: handle, Role: core.RoleMember}
	return taskContext(a), a
}

// as is the context a DAV request carries for an app password: the token's
// actor as the verifier builds it.
func (f davFixture) as(p *core.IssuedDAVPassword) context.Context {
	return taskContext(&core.Actor{ID: p.ActorID, TenantID: f.scope.TenantID, Kind: core.ActorAgent,
		Handle: p.TokenName, Scopes: []core.Scope{core.ScopeCaldav}, TokenID: p.ID, TokenName: p.TokenName})
}

func (f davFixture) create(t *testing.T, ctx context.Context, in core.CreateDAVPasswordInput) *core.IssuedDAVPassword {
	t.Helper()
	p, err := f.l.CreateDAVPassword(ctx, in)
	if err != nil {
		t.Fatalf("creating app password %q: %v", in.Name, err)
	}
	return p
}

func TestAMemberCreatesAnAppPasswordWithoutTokenAdmin(t *testing.T) {
	f := newDAVFixture(t)
	if _, err := f.l.CreateToken(f.member, core.CreateTokenInput{Name: "x", Scopes: []core.Scope{core.ScopeTaskRead}}); !core.IsKind(err, core.KindForbidden) {
		t.Fatalf("the member could create an ordinary token: %v", err)
	}
	p := f.create(t, f.member, core.CreateDAVPasswordInput{Name: "iPhone"})
	if p.ActorID != f.actor.ID || !slices.Equal(p.Scopes, []core.Scope{core.ScopeCaldav}) ||
		p.Collections != core.DAVCollectionsAssigned || p.ReadOnly || p.ExpiresAt != nil {
		t.Errorf("issued %+v", p.DAVPassword)
	}
	if !strings.HasPrefix(p.Password, "tix_pat_") || p.Username != "bob@example.com" || p.Path != "/dav/" {
		t.Errorf("issued password %q, username %q, path %q", p.Password, p.Username, p.Path)
	}
	if p.TokenName != "dav-bob-iphone" {
		t.Errorf("token name %q", p.TokenName)
	}
	list, err := f.l.ListDAVPasswords(f.member)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != p.ID || list[0].Name != "iPhone" || list[0].RevokedAt != nil {
		t.Fatalf("listed %+v", list)
	}
	raw, _ := json.Marshal(list)
	if strings.Contains(string(raw), p.Password) || strings.Contains(string(raw), `"password"`) {
		t.Errorf("the list carries the secret: %s", raw)
	}
	tokens, err := f.l.ListTokens(f.admin, f.actor.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 1 || tokens[0].Name != "dav-bob-iphone" {
		t.Errorf("an admin sees tokens %+v", tokens)
	}
}

func TestAppPasswordCreationIsAuditedAsATokenWithoutItsSecret(t *testing.T) {
	f := newDAVFixture(t)
	p := f.create(t, f.member, core.CreateDAVPasswordInput{Name: "Mac", Collections: core.DAVCollectionsAll})
	if err := f.l.RevokeDAVPassword(f.member, p.ID); err != nil {
		t.Fatal(err)
	}
	var events []core.Event
	var audits []core.AuditEntry
	if err := f.l.store.View(context.Background(), f.scope, func(tx store.Tx) error {
		var err error
		if events, err = tx.ReadEvents(context.Background(), 0, 500); err != nil {
			return err
		}
		audits, err = tx.ListAudit(context.Background(), core.AuditFilter{Page: core.Page{Limit: 500}})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var types []core.EventType
	for _, e := range events {
		if e.SubjectID == p.ID {
			types = append(types, e.Type)
			raw, _ := json.Marshal(e.Payload)
			if !strings.Contains(string(raw), `"caldav"`) {
				t.Errorf("%s payload names no caldav scope: %s", e.Type, raw)
			}
			if strings.Contains(string(raw), p.Password) {
				t.Errorf("%s payload carries the secret", e.Type)
			}
		}
	}
	if !slices.Equal(types, []core.EventType{eventTokenCreated, eventTokenRevoked}) {
		t.Errorf("events %v", types)
	}
	var actions []string
	for _, a := range audits {
		if a.SubjectID == p.ID {
			actions = append(actions, a.Action)
			raw, _ := json.Marshal(a)
			if strings.Contains(string(raw), p.Password) {
				t.Errorf("%s audit entry carries the secret", a.Action)
			}
		}
	}
	slices.Sort(actions)
	if !slices.Equal(actions, []string{auditTokenCreate, auditTokenRevoke}) {
		t.Errorf("audit actions %v", actions)
	}
}

func TestAppPasswordNamesAreUniquePerPersonWhileLive(t *testing.T) {
	f := newDAVFixture(t)
	first := f.create(t, f.member, core.CreateDAVPasswordInput{Name: "iPhone"})
	if _, err := f.l.CreateDAVPassword(f.member, core.CreateDAVPasswordInput{Name: "iphone"}); !core.IsKind(err, core.KindConflict) {
		t.Fatalf("a second live iPhone: %v, want conflict", err)
	}
	other, _ := f.person(t, "ana@example.com", "ana")
	theirs := f.create(t, other, core.CreateDAVPasswordInput{Name: "iPhone"})
	if theirs.TokenName == first.TokenName {
		t.Errorf("two people's token names collide: %q", theirs.TokenName)
	}
	if err := f.l.RevokeDAVPassword(f.member, first.ID); err != nil {
		t.Fatal(err)
	}
	again := f.create(t, f.member, core.CreateDAVPasswordInput{Name: "iPhone"})
	if again.ID == first.ID {
		t.Error("the revoked password came back")
	}
}

func TestOnlyAPersonCreatesAnAppPassword(t *testing.T) {
	f := newDAVFixture(t)
	agent := &core.Actor{Kind: core.ActorAgent, Handle: "ci", Role: core.RoleMember}
	if err := f.l.store.Update(context.Background(), f.scope, func(tx store.Tx) error {
		if err := tx.CreateActor(context.Background(), agent); err != nil {
			return err
		}
		return tx.AddMember(context.Background(), &core.Membership{ActorID: agent.ID, Role: core.RoleMember})
	}); err != nil {
		t.Fatal(err)
	}
	agent.TenantID = f.scope.TenantID
	for _, call := range []func(ctx context.Context) error{
		func(ctx context.Context) error {
			_, err := f.l.CreateDAVPassword(ctx, core.CreateDAVPasswordInput{Name: "x"})
			return err
		},
		func(ctx context.Context) error { _, err := f.l.ListDAVPasswords(ctx); return err },
		func(ctx context.Context) error { _, err := f.l.GetDAVInfo(ctx); return err },
	} {
		if err := call(taskContext(agent)); !core.IsKind(err, core.KindForbidden) {
			t.Errorf("an agent: %v, want forbidden", err)
		}
	}
}

func TestAnSSOOnlyPersonCreatesAndUsesAnAppPassword(t *testing.T) {
	f := newDAVFixture(t)
	user, err := f.l.CreateUser(f.admin, core.CreateUserInput{Email: "sso@example.com", Role: core.RoleMember})
	if err != nil {
		t.Fatalf("creating an account with no password: %v", err)
	}
	if hash := storedPasswordHash(t, f.l, "sso@example.com"); hash != "" {
		t.Fatalf("the account has a password hash %q", hash)
	}
	ctx := taskContext(&core.Actor{ID: user.ID, TenantID: f.scope.TenantID, Kind: core.ActorUser,
		Handle: "sso", Role: core.RoleMember, SessionID: "s1"})
	p := f.create(t, ctx, core.CreateDAVPasswordInput{Name: "Phone"})
	who, err := f.l.DAVPrincipal(f.as(p))
	if err != nil {
		t.Fatal(err)
	}
	if who.Email != "sso@example.com" || who.ActorID != strings.ToLower(user.ID) {
		t.Errorf("principal %+v", who)
	}
}

func TestRevokingAnAppPasswordIsImmediateAndOnlyYourOwn(t *testing.T) {
	f := newDAVFixture(t)
	p := f.create(t, f.member, core.CreateDAVPasswordInput{Name: "iPhone"})
	other, _ := f.person(t, "ana@example.com", "ana")
	if err := f.l.RevokeDAVPassword(other, p.ID); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("somebody else revoked it: %v", err)
	}
	if _, err := f.l.DAVPrincipal(f.as(p)); err != nil {
		t.Fatalf("before revocation: %v", err)
	}
	if err := f.l.RevokeDAVPassword(f.member, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.l.RevokeDAVPassword(f.member, p.ID); err != nil {
		t.Fatalf("revoking twice: %v", err)
	}
	list, err := f.l.ListDAVPasswords(f.member)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].RevokedAt == nil {
		t.Fatalf("listed %+v", list)
	}
}

func TestTokenCreateRefusesTheCaldavScope(t *testing.T) {
	f := newDAVFixture(t)
	_, err := f.l.CreateToken(f.admin, core.CreateTokenInput{Name: "x", ActorID: f.actor.ID,
		Scopes: []core.Scope{core.ScopeCaldav}})
	if !core.IsKind(err, core.KindInvalid) || !strings.Contains(err.Error(), "tix dav password create") {
		t.Fatalf("err = %v, want invalid pointing at tix dav password create", err)
	}
	tokens, _ := f.l.ListTokens(f.admin, f.actor.ID)
	if len(tokens) != 0 {
		t.Errorf("a token was created: %+v", tokens)
	}
}

func TestDAVInfoNamesTheUsernameAndIsReadOnly(t *testing.T) {
	f := newDAVFixture(t)
	info, err := f.l.GetDAVInfo(f.member)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Enabled || info.Write || info.Username != "bob@example.com" || info.Path != "/dav/" ||
		!slices.Equal(info.Collections, core.AllDAVCollections) || info.Reason != "" {
		t.Errorf("info %+v", info)
	}
	off := newDAVFixture(t, WithCalDAV(false, 0, 0))
	info, err = off.l.GetDAVInfo(off.member)
	if err != nil {
		t.Fatal(err)
	}
	if info.Enabled || info.Reason == "" {
		t.Errorf("disabled info %+v", info)
	}
}

func TestAppPasswordInputIsChecked(t *testing.T) {
	f := newDAVFixture(t)
	past := f.clk.Now().Add(-time.Hour)
	for name, in := range map[string]core.CreateDAVPasswordInput{
		"no name":         {},
		"a long name":     {Name: strings.Repeat("x", core.MaxDAVPasswordName+1)},
		"a control":       {Name: "a\nb"},
		"bad collections": {Name: "x", Collections: "some"},
		"expired":         {Name: "x", ExpiresAt: &past},
	} {
		if _, err := f.l.CreateDAVPassword(f.member, in); !core.IsKind(err, core.KindInvalid) {
			t.Errorf("%s: %v, want invalid", name, err)
		}
	}
}
