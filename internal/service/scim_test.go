// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// standingLookup resolves a token the way the server does, refusing one whose
// actor is disabled.
type standingLookup struct {
	l     *Local
	scope core.TenantScope
}

func (s standingLookup) TokenByHash(ctx context.Context, hash string) (*core.APIToken, error) {
	var out *core.APIToken
	err := s.l.store.View(ctx, s.scope, func(tx store.Tx) error {
		tok, err := tx.GetTokenByHash(ctx, hash)
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if gone, err := StandingWithdrawn(ctx, tx, tok.ActorID); err != nil || gone {
			return err
		}
		out = tok
		return nil
	})
	return out, err
}

// scimFixture is a tenant with SCIM enabled and a provider's context.
type scimFixture struct {
	l        *Local
	clk      *clock.Fake
	scope    core.TenantScope
	admin    context.Context
	provider context.Context
	token    string
}

func newSCIMFixture(t *testing.T) *scimFixture {
	t.Helper()
	l, clk, scope, admin := newLocal(t)
	f := &scimFixture{l: l, clk: clk, scope: scope, admin: authContext(admin)}
	s, err := l.EnableSCIM(f.admin)
	if err != nil {
		t.Fatalf("EnableSCIM: %v", err)
	}
	f.token = s.Token
	f.provider = f.as(t, s.Token)
	return f
}

// as authenticates a token as a request under /scim/v2 does.
func (f *scimFixture) as(t *testing.T, token string) context.Context {
	t.Helper()
	ctx := core.WithSCIMRequest(context.Background())
	actor, err := auth.NewTokenVerifier(standingLookup{l: f.l, scope: f.scope}, f.clk).Verify(ctx, token)
	if err != nil {
		t.Fatalf("verifying a provisioning token: %v", err)
	}
	return core.WithActor(ctx, actor)
}

func (f *scimFixture) create(t *testing.T, in core.SCIMUser) *core.SCIMUser {
	t.Helper()
	u, err := f.l.SCIMCreateUser(f.provider, in)
	if err != nil {
		t.Fatalf("SCIMCreateUser(%s): %v", in.UserName, err)
	}
	return u
}

func (f *scimFixture) role(t *testing.T, actorID string) core.Role {
	t.Helper()
	var role core.Role
	if err := f.l.store.View(context.Background(), f.scope, func(tx store.Tx) error {
		var err error
		role, err = userRole(context.Background(), tx, actorID)
		return err
	}); err != nil {
		t.Fatalf("reading role: %v", err)
	}
	return role
}

func TestEnableSCIMMintsItsTokenOnceAndConfinesIt(t *testing.T) {
	f := newSCIMFixture(t)
	if f.token == "" {
		t.Fatal("enabling SCIM printed no token")
	}
	again, err := f.l.EnableSCIM(f.admin)
	if err != nil {
		t.Fatalf("enabling again: %v", err)
	}
	if again.Token != "" || len(again.Tokens) != 1 || !again.Enabled {
		t.Fatalf("second enable: token %q, %d live tokens, enabled %v; want none, 1, true",
			again.Token, len(again.Tokens), again.Enabled)
	}
	if again.Path != core.SCIMPath || again.AgentID == "" {
		t.Errorf("settings path %q agent %q", again.Path, again.AgentID)
	}
	_, err = auth.NewTokenVerifier(standingLookup{l: f.l, scope: f.scope}, f.clk).Verify(context.Background(), f.token)
	if !core.IsKind(err, core.KindForbidden) {
		t.Fatalf("a provisioning token outside /scim/v2 = %v, want forbidden", err)
	}
	rotated, err := f.l.RotateSCIMToken(f.admin)
	if err != nil {
		t.Fatalf("RotateSCIMToken: %v", err)
	}
	if rotated.Token == "" || rotated.Token == f.token || len(rotated.Tokens) != 1 {
		t.Fatalf("rotation gave token %q with %d live", rotated.Token, len(rotated.Tokens))
	}
	_, err = auth.NewTokenVerifier(standingLookup{l: f.l, scope: f.scope}, f.clk).
		Verify(core.WithSCIMRequest(context.Background()), f.token)
	if !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("the rotated-out token = %v, want unauthenticated", err)
	}
}

func TestSCIMOffAnswersNotFound(t *testing.T) {
	f := newSCIMFixture(t)
	if _, err := f.l.DisableSCIM(f.admin); err != nil {
		t.Fatalf("DisableSCIM: %v", err)
	}
	if _, _, err := f.l.SCIMListUsers(f.provider, core.SCIMQuery{Count: 10}); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("listing while off = %v, want not found", err)
	}
	if err := f.l.SCIMReady(f.provider); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("SCIMReady while off = %v, want not found", err)
	}
	s, err := f.l.GetSCIM(f.admin)
	if err != nil || len(s.Tokens) != 1 {
		t.Fatalf("after disabling: %v, %d tokens; the token is kept", err, len(s.Tokens))
	}
}

func TestSCIMProvisionScopeIsNotAWildcard(t *testing.T) {
	f := newSCIMFixture(t)
	if err := f.l.SCIMReady(f.admin); !core.IsKind(err, core.KindForbidden) {
		t.Fatalf("an admin's session provisioning = %v, want forbidden", err)
	}
	tests := []struct {
		name   string
		in     core.CreateTokenInput
		member bool
		want   core.Kind
	}{
		{"mixed with another scope", core.CreateTokenInput{Name: "a",
			Scopes: []core.Scope{core.ScopeSCIMProvision, core.ScopeTaskRead}}, false, core.KindInvalid},
		{"with a claim requirement", core.CreateTokenInput{Name: "b",
			Scopes: []core.Scope{core.ScopeSCIMProvision}, RequireClaim: true}, false, core.KindInvalid},
		{"by a member", core.CreateTokenInput{Name: "c",
			Scopes: []core.Scope{core.ScopeSCIMProvision}}, true, core.KindForbidden},
		{"alone by an admin", core.CreateTokenInput{Name: "okta-scim",
			Scopes: []core.Scope{core.ScopeSCIMProvision}}, false, ""},
	}
	member := &core.Actor{ID: "m", TenantID: f.scope.TenantID, Kind: core.ActorUser, Role: core.RoleMember,
		Scopes: []core.Scope{core.ScopeTokenAdmin}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := f.admin
			if tc.member {
				ctx = authContext(member)
			}
			_, err := f.l.CreateToken(ctx, tc.in)
			if tc.want == "" && err != nil || tc.want != "" && !core.IsKind(err, tc.want) {
				t.Fatalf("CreateToken = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestSCIMCreatesListsFiltersAndPages(t *testing.T) {
	f := newSCIMFixture(t)
	ann := f.create(t, core.SCIMUser{UserName: "Ann@Example.com", ExternalID: "ext-ann", GivenName: "Ann",
		FamilyName: "Lee", Active: true})
	f.create(t, core.SCIMUser{UserName: "bob", Email: "bob@example.com", Active: true})
	f.create(t, core.SCIMUser{UserName: "cy@example.com", Active: false})

	if ann.DisplayName != "Ann Lee" || ann.Email != "ann@example.com" || !ann.Active {
		t.Errorf("created %+v", ann)
	}
	if hash := storedPasswordHash(t, f.l, "ann@example.com"); hash != "" {
		t.Error("a provisioned account was given a password")
	}
	if role := f.role(t, ann.ID); role != core.RoleViewer {
		t.Errorf("a new provisioned person is %q, want viewer", role)
	}

	tests := []struct {
		name  string
		q     core.SCIMQuery
		total int
		ids   int
	}{
		{"everybody", core.SCIMQuery{StartIndex: 1, Count: 100}, 3, 3},
		{"userName ignoring case", core.SCIMQuery{Conditions: []core.SCIMCondition{
			{Attr: core.SCIMAttrUserName, Value: "ann@EXAMPLE.com"}}, StartIndex: 1, Count: 10}, 1, 1},
		{"externalId", core.SCIMQuery{Conditions: []core.SCIMCondition{
			{Attr: core.SCIMAttrExternalID, Value: "ext-ann"}}, StartIndex: 1, Count: 10}, 1, 1},
		{"email", core.SCIMQuery{Conditions: []core.SCIMCondition{
			{Attr: core.SCIMAttrEmail, Value: "bob@example.com"}}, StartIndex: 1, Count: 10}, 1, 1},
		{"id and userName that disagree", core.SCIMQuery{Conditions: []core.SCIMCondition{
			{Attr: core.SCIMAttrID, Value: ann.ID}, {Attr: core.SCIMAttrUserName, Value: "bob"}},
			StartIndex: 1, Count: 10}, 0, 0},
		{"nobody", core.SCIMQuery{Conditions: []core.SCIMCondition{
			{Attr: core.SCIMAttrUserName, Value: "zed"}}, StartIndex: 1, Count: 10}, 0, 0},
		{"second page of one", core.SCIMQuery{StartIndex: 2, Count: 1}, 3, 1},
		{"past the end", core.SCIMQuery{StartIndex: 9, Count: 5}, 3, 0},
		{"count zero", core.SCIMQuery{StartIndex: 1, Count: 0}, 3, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, total, err := f.l.SCIMListUsers(f.provider, tc.q)
			if err != nil {
				t.Fatal(err)
			}
			if total != tc.total || len(got) != tc.ids {
				t.Fatalf("total %d with %d on the page, want %d with %d", total, len(got), tc.total, tc.ids)
			}
		})
	}

	_, err := f.l.SCIMCreateUser(f.provider, core.SCIMUser{UserName: "ANN@example.com", Email: "x@example.com", Active: true})
	if !core.IsKind(err, core.KindConflict) {
		t.Fatalf("a taken userName = %v, want conflict", err)
	}
	if _, err := f.l.SCIMCreateUser(f.provider, core.SCIMUser{UserName: "noaddress", Active: true}); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("a user with no address = %v, want invalid", err)
	}
}

func TestSCIMCreateLinksAMemberOfThisTenantOnly(t *testing.T) {
	f := newSCIMFixture(t)
	member, err := f.l.CreateUser(f.admin, core.CreateUserInput{Email: "dee@example.com", Role: core.RoleMember})
	if err != nil {
		t.Fatal(err)
	}
	linked := f.create(t, core.SCIMUser{UserName: "dee@example.com", ExternalID: "ext-dee", Active: true})
	if linked.ID != member.ID || linked.ExternalID != "ext-dee" {
		t.Fatalf("linked %+v, want the member %s", linked, member.ID)
	}
	if role := f.role(t, member.ID); role != core.RoleMember {
		t.Errorf("linking changed the role to %q", role)
	}
	if _, err := f.l.SCIMCreateUser(f.provider, core.SCIMUser{UserName: "dee2", Email: "dee@example.com",
		Active: true}); !core.IsKind(err, core.KindConflict) {
		t.Fatalf("provisioning an already provisioned account = %v, want conflict", err)
	}

	other := otherTenantActor(t, f.l)
	if _, err := f.l.CreateUser(authContext(other), core.CreateUserInput{Email: "eve@example.com"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.l.SCIMCreateUser(f.provider, core.SCIMUser{UserName: "eve@example.com",
		Active: true}); !core.IsKind(err, core.KindConflict) {
		t.Fatalf("an account of another tenant only = %v, want conflict", err)
	}
}

func TestSCIMDeactivationSuspendsCredentialsAndKeepsThem(t *testing.T) {
	f := newSCIMFixture(t)
	u := f.create(t, core.SCIMUser{UserName: "fay@example.com", Active: true})
	issued, err := f.l.CreateToken(f.admin, core.CreateTokenInput{Name: "fay", ActorID: u.ID,
		Scopes: []core.Scope{core.ScopeTaskRead}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := f.l.store.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.CreateSession(ctx, u.ID, "fay-session", f.clk.Now().Add(time.Hour), core.SessionOrigin{})
	}); err != nil {
		t.Fatal(err)
	}
	verifier := auth.NewTokenVerifier(standingLookup{l: f.l, scope: f.scope}, f.clk)
	setActive := func(on bool) {
		t.Helper()
		got, err := f.l.SCIMUpdateUser(f.provider, u.ID, func(s *core.SCIMUser) error {
			s.Active = on
			return nil
		})
		if err != nil || got.Active != on {
			t.Fatalf("setting active %v: %v, %+v", on, err, got)
		}
	}

	setActive(false)
	if _, err := verifier.Verify(ctx, issued.Token); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("a deactivated person's token = %v, want refused", err)
	}
	if err := f.l.store.View(ctx, f.scope, func(tx store.Tx) error {
		_, _, err := tx.GetSessionByHash(ctx, "fay-session")
		return err
	}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("a deactivated person's session = %v, want deleted", err)
	}
	tokens, err := f.l.ListTokens(f.admin, u.ID)
	if err != nil || len(tokens) != 1 || tokens[0].RevokedAt != nil {
		t.Fatalf("tokens while inactive = %+v, %v; want one, not revoked", tokens, err)
	}

	setActive(true)
	if _, err := verifier.Verify(ctx, issued.Token); err != nil {
		t.Errorf("a reactivated person's token = %v, want it working again", err)
	}
}

func TestSCIMDeleteRemovesMembershipAndKeepsTheAccount(t *testing.T) {
	f := newSCIMFixture(t)
	u := f.create(t, core.SCIMUser{UserName: "gus@example.com", Active: true})
	if err := f.l.SCIMDeleteUser(f.provider, u.ID); err != nil {
		t.Fatalf("SCIMDeleteUser: %v", err)
	}
	if _, err := f.l.SCIMGetUser(f.provider, u.ID); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("reading a deleted user = %v, want not found", err)
	}
	if role := f.role(t, u.ID); role != "" {
		t.Errorf("membership after delete = %q, want none", role)
	}
	if hash := storedPasswordHash(t, f.l, "gus@example.com"); hash != "" {
		t.Error("unexpected password")
	}
	ctx := context.Background()
	if err := f.l.store.View(ctx, f.scope, func(tx store.Tx) error {
		_, err := tx.GetActor(ctx, u.ID)
		return err
	}); err != nil {
		t.Errorf("the actor is gone after delete: %v", err)
	}
	again := f.create(t, core.SCIMUser{UserName: "gus@example.com", Active: true})
	if again.ID != u.ID {
		t.Errorf("provisioning again made a new person %s, want %s", again.ID, u.ID)
	}
}

func TestSCIMPatchUpdatesNamesAndRefusesTakenUserNames(t *testing.T) {
	f := newSCIMFixture(t)
	a := f.create(t, core.SCIMUser{UserName: "hal@example.com", Active: true})
	f.create(t, core.SCIMUser{UserName: "ida@example.com", Active: true})
	got, err := f.l.SCIMUpdateUser(f.provider, a.ID, func(s *core.SCIMUser) error {
		s.DisplayName, s.ExternalID, s.Email = "Hal B", "ext-hal", "hal.b@example.com"
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.DisplayName != "Hal B" || got.ExternalID != "ext-hal" || got.Email != "hal.b@example.com" {
		t.Errorf("updated %+v", got)
	}
	if _, err := f.l.SCIMUpdateUser(f.provider, a.ID, func(s *core.SCIMUser) error {
		s.UserName = "IDA@example.com"
		return nil
	}); !core.IsKind(err, core.KindConflict) {
		t.Fatalf("taking another's userName = %v, want conflict", err)
	}
}

func TestSCIMGroupsMapRoles(t *testing.T) {
	f := newSCIMFixture(t)
	if _, err := f.l.MapSCIMGroup(f.admin, core.SCIMGroupRoleInput{Group: "engineering", Role: core.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.l.MapSCIMGroup(f.admin, core.SCIMGroupRoleInput{Group: "Staff", Role: core.RoleMember}); err != nil {
		t.Fatal(err)
	}
	jo := f.create(t, core.SCIMUser{UserName: "jo@example.com", Active: true})
	manual, err := f.l.CreateUser(f.admin, core.CreateUserInput{Email: "kim@example.com", Role: core.RoleMember})
	if err != nil {
		t.Fatal(err)
	}

	eng, err := f.l.SCIMCreateGroup(f.provider, core.SCIMGroup{DisplayName: "Engineering",
		Members: []core.SCIMRef{{ID: jo.ID}, {ID: manual.ID}}})
	if err != nil {
		t.Fatalf("SCIMCreateGroup: %v", err)
	}
	staff, err := f.l.SCIMCreateGroup(f.provider, core.SCIMGroup{DisplayName: "Staff", Members: []core.SCIMRef{{ID: jo.ID}}})
	if err != nil {
		t.Fatal(err)
	}
	if role := f.role(t, jo.ID); role != core.RoleAdmin {
		t.Fatalf("in Engineering and Staff jo is %q, want admin", role)
	}
	if role := f.role(t, manual.ID); role != core.RoleMember {
		t.Errorf("a person never provisioned changed to %q", role)
	}
	if _, err := f.l.SCIMCreateGroup(f.provider, core.SCIMGroup{DisplayName: "ENGINEERING"}); !core.IsKind(err, core.KindConflict) {
		t.Errorf("a duplicate group name = %v, want conflict", err)
	}
	if _, err := f.l.SCIMCreateGroup(f.provider, core.SCIMGroup{DisplayName: "x",
		Members: []core.SCIMRef{{ID: "nobody"}}}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("a member who is nobody = %v, want invalid", err)
	}

	removeJo := func(g *core.SCIMGroup) error {
		g.Members = slices.DeleteFunc(g.Members, func(r core.SCIMRef) bool { return r.ID == jo.ID })
		return nil
	}
	if _, err := f.l.SCIMUpdateGroup(f.provider, eng.ID, removeJo); err != nil {
		t.Fatal(err)
	}
	if role := f.role(t, jo.ID); role != core.RoleMember {
		t.Fatalf("out of Engineering jo is %q, want member from Staff", role)
	}
	if err := f.l.SCIMDeleteGroup(f.provider, staff.ID); err != nil {
		t.Fatal(err)
	}
	if role := f.role(t, jo.ID); role != core.RoleViewer {
		t.Fatalf("in no mapped group jo is %q, want viewer", role)
	}
	user, err := f.l.SCIMGetUser(f.provider, jo.ID)
	if err != nil || len(user.Groups) != 0 {
		t.Fatalf("jo's groups = %+v, %v", user, err)
	}

	if _, err := f.l.SCIMUpdateGroup(f.provider, eng.ID, func(g *core.SCIMGroup) error {
		g.Members = append(g.Members, core.SCIMRef{ID: jo.ID})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.l.UnmapSCIMGroup(f.admin, "ENGINEERING"); err != nil {
		t.Fatal(err)
	}
	if role := f.role(t, jo.ID); role != core.RoleViewer {
		t.Errorf("after unmapping jo is %q, want viewer", role)
	}
	groups, total, err := f.l.SCIMListGroups(f.provider, core.SCIMQuery{Conditions: []core.SCIMCondition{
		{Attr: core.SCIMAttrMember, Value: jo.ID}}, StartIndex: 1, Count: 10})
	if err != nil || total != 1 || groups[0].ID != eng.ID {
		t.Errorf("groups of jo = %+v, %d, %v", groups, total, err)
	}
	if err := f.l.UnmapSCIMGroup(f.admin, "nope"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("unmapping nothing = %v, want not found", err)
	}
	s, err := f.l.GetSCIM(f.admin)
	if err != nil || s.Users != 1 || s.Groups != 1 || len(s.GroupRoles) != 1 || s.LastActivityAt == nil {
		t.Errorf("settings %+v, %v", s, err)
	}
}

func TestSCIMChangesAreAuditedOnTheSCIMChannel(t *testing.T) {
	f := newSCIMFixture(t)
	u := f.create(t, core.SCIMUser{UserName: "lou@example.com", Active: true})
	entries, _, err := f.l.ListAudit(f.admin, core.AuditFilter{Vias: []core.ViaKind{core.ViaSCIM},
		Page: core.Page{Limit: 50}})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range entries {
		if e.Action == auditSCIMUserCreate && e.SubjectID == u.ID {
			found = true
			if e.Via.Kind != core.ViaSCIM || e.Via.TokenName != core.SCIMTokenName {
				t.Errorf("via %+v, want scim with the token's name", e.Via)
			}
		}
	}
	if !found {
		t.Fatalf("no %s entry among %d scim entries", auditSCIMUserCreate, len(entries))
	}
}
