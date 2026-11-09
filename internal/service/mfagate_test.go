// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// setMFAPolicy turns the second factor policy on or off for the fixture's
// tenant, as the fixture's administrator, who has no account and so is never
// refused by the lockout guard.
func setMFAPolicy(t *testing.T, l *Local, admin *core.Actor, policy core.MFAPolicy, grace time.Duration) *core.Tenant {
	t.Helper()
	g := core.Duration(grace)
	out, err := l.UpdateTenant(authContext(admin), "", core.UpdateTenantInput{MFAPolicy: &policy, MFAGrace: &g})
	if err != nil {
		t.Fatalf("setting the policy to %s: %v", policy, err)
	}
	return out
}

// isGated reports whether err is the policy's refusal.
func isGated(err error) bool {
	var domain *core.Error
	return errors.As(err, &domain) && domain.Kind == core.KindForbidden &&
		domain.Details["reason"] == core.ReasonSecondFactorEnrollmentRequired
}

// listProjects is the ordinary operation the gate is tested against.
func listProjects(l *Local, actor *core.Actor) error {
	_, _, err := l.ListProjects(authContext(actor), core.ProjectFilter{})
	return err
}

// personToken is the actor a token minted for a person resolves to.
func personToken(person *core.Actor) *core.Actor {
	return &core.Actor{ID: person.ID, TenantID: person.TenantID, Kind: core.ActorAgent,
		Handle: "laptop", Scopes: []core.Scope{core.ScopeAll}, TokenID: "tok-person", TokenName: "laptop"}
}

func TestThePolicyGatesAPersonWithoutAFactor(t *testing.T) {
	l, clk, _, admin := newTOTPLocal(t)
	ada := userActor(t, l, seedUser(t, l, admin, "ada@example.com"), core.RoleMember)

	if err := listProjects(l, ada); err != nil {
		t.Fatalf("before the policy: %v", err)
	}
	setMFAPolicy(t, l, admin, core.MFAPolicyRequired, 0)

	cases := []struct {
		name  string
		actor *core.Actor
	}{
		{"session", ada},
		{"token the person owns", personToken(ada)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := listProjects(l, c.actor); !isGated(err) {
				t.Fatalf("listing projects = %v, want %s", err, core.ReasonSecondFactorEnrollmentRequired)
			}
		})
	}

	ctx := authContext(ada)
	if _, err := l.WhoAmI(ctx); err != nil {
		t.Errorf("who am I is refused to a gated person: %v", err)
	}
	if err := l.Logout(WithSessionToken(ctx, "x")); isGated(err) {
		t.Errorf("signing out is refused to a gated person: %v", err)
	}
	if _, _, err := l.GetAccountLog(ctx, core.Page{}); err != nil {
		t.Errorf("the account log is refused to a gated person: %v", err)
	}

	enrol(t, l, clk, ada)
	for _, c := range cases {
		if err := listProjects(l, c.actor); err != nil {
			t.Errorf("%s after enrolling: %v", c.name, err)
		}
	}
}

func TestAnAgentTokenIsExemptFromThePolicy(t *testing.T) {
	l, _, scope, admin := newTOTPLocal(t)
	agent := core.Actor{Kind: core.ActorAgent, Handle: "builder"}
	ctx := context.Background()
	if err := l.store.Update(ctx, scope, func(tx store.Tx) error { return tx.CreateActor(ctx, &agent) }); err != nil {
		t.Fatalf("creating the agent: %v", err)
	}
	agent.TenantID = scope.TenantID
	agent.Scopes = []core.Scope{core.ScopeAll}
	agent.TokenID = "tok-agent"
	setMFAPolicy(t, l, admin, core.MFAPolicyRequired, 0)

	if err := listProjects(l, &agent); err != nil {
		t.Fatalf("an agent token under the policy = %v, want it to work", err)
	}
}

func TestThePolicyGraceLetsAPersonWorkUntilTheDeadline(t *testing.T) {
	l, clk, _, admin := newTOTPLocal(t)
	ada := userActor(t, l, seedUser(t, l, admin, "ada@example.com"), core.RoleMember)
	start := clk.Now()
	setMFAPolicy(t, l, admin, core.MFAPolicyRequired, 7*24*time.Hour)

	clk.Advance(24 * time.Hour)
	if err := listProjects(l, ada); err != nil {
		t.Fatalf("inside the grace = %v, want it to work", err)
	}
	me, err := l.WhoAmI(authContext(ada))
	if err != nil {
		t.Fatal(err)
	}
	want := start.Add(7 * 24 * time.Hour)
	if me.SecondFactorDue == nil || !me.SecondFactorDue.Equal(want) || me.SecondFactorEnrollmentRequired {
		t.Errorf("who am I inside the grace = due %v, gated %v; want due %v, not gated",
			me.SecondFactorDue, me.SecondFactorEnrollmentRequired, want)
	}

	g := core.Duration(3 * 24 * time.Hour)
	if _, err := l.UpdateTenant(authContext(admin), "", core.UpdateTenantInput{MFAGrace: &g}); err != nil {
		t.Fatal(err)
	}
	clk.Advance(2*24*time.Hour + time.Minute)
	if err := listProjects(l, ada); !isGated(err) {
		t.Fatalf("after a shortened grace ran out = %v, want the gate", err)
	}
	status, err := l.TOTPStatus(authContext(ada))
	if err != nil {
		t.Fatal(err)
	}
	if !status.EnrollmentRequired || status.Policy != core.MFAPolicyRequired {
		t.Errorf("status after the grace = %+v, want enrollment required", status)
	}
}

func TestTurningThePolicyOnCannotLockOutTheCaller(t *testing.T) {
	l, clk, _, admin := newTOTPLocal(t)
	user, err := l.CreateUser(authContext(admin), core.CreateUserInput{
		Email: "root@example.com", Password: testPassword, Role: core.RoleAdmin,
	})
	if err != nil {
		t.Fatal(err)
	}
	root := userActor(t, l, user, core.RoleAdmin)
	required := core.MFAPolicyRequired

	_, err = l.UpdateTenant(authContext(root), "", core.UpdateTenantInput{MFAPolicy: &required})
	if !core.IsKind(err, core.KindPrecondition) {
		t.Fatalf("an unenrolled admin turning the policy on = %v, want a precondition failure", err)
	}
	tenant, err := l.GetTenant(authContext(root), "")
	if err != nil {
		t.Fatal(err)
	}
	if tenant.MFARequired() {
		t.Fatal("the refused change was written")
	}

	enrol(t, l, clk, root)
	tenant, err = l.UpdateTenant(authContext(root), "", core.UpdateTenantInput{MFAPolicy: &required})
	if err != nil {
		t.Fatalf("an enrolled admin turning the policy on: %v", err)
	}
	if !tenant.MFARequired() || tenant.MFARequiredAt == nil {
		t.Errorf("tenant = %+v, want required with the time it was turned on", tenant)
	}
}

func TestThePolicyNeedsASealingKey(t *testing.T) {
	l, _, _, admin := newLocal(t)
	required := core.MFAPolicyRequired
	_, err := l.UpdateTenant(authContext(admin), "", core.UpdateTenantInput{MFAPolicy: &required})
	if !core.IsKind(err, core.KindPrecondition) {
		t.Fatalf("requiring a factor with no key = %v, want a precondition failure", err)
	}
}

func TestClearingAFactorUnderThePolicyGatesThePersonAgain(t *testing.T) {
	l, clk, _, admin := newTOTPLocal(t)
	ada := userActor(t, l, seedUser(t, l, admin, "ada@example.com"), core.RoleMember)
	enrol(t, l, clk, ada)
	setMFAPolicy(t, l, admin, core.MFAPolicyRequired, 0)
	if err := listProjects(l, ada); err != nil {
		t.Fatalf("enrolled under the policy: %v", err)
	}
	if err := l.RemoveTOTP(authContext(ada)); !core.IsKind(err, core.KindPrecondition) {
		t.Errorf("removing one's own factor under the policy = %v, want a precondition failure", err)
	}
	if err := l.ClearUserTOTP(authContext(admin), ada.ID); err != nil {
		t.Fatal(err)
	}
	if err := listProjects(l, ada); !isGated(err) {
		t.Fatalf("after an administrator cleared the factor = %v, want the gate", err)
	}
}

func TestGateRefusalsAreAuditedOnceAnHour(t *testing.T) {
	l, clk, _, admin := newTOTPLocal(t)
	ada := userActor(t, l, seedUser(t, l, admin, "ada@example.com"), core.RoleMember)
	setMFAPolicy(t, l, admin, core.MFAPolicyRequired, 0)

	count := func() int {
		l.gates.pending.Wait()
		n := 0
		for _, e := range auditEntries(t, l, admin) {
			if e.Action == auditTOTPGate {
				n++
			}
		}
		return n
	}
	for range 3 {
		_ = listProjects(l, ada)
	}
	if got := count(); got != 1 {
		t.Fatalf("three refusals in a minute wrote %d gate entries, want 1", got)
	}
	clk.Advance(time.Hour)
	_ = listProjects(l, ada)
	if got := count(); got != 2 {
		t.Errorf("a refusal an hour later wrote %d entries in all, want 2", got)
	}
}

func TestPolicyChangesNameThePolicyInTheAudit(t *testing.T) {
	l, _, _, admin := newTOTPLocal(t)
	setMFAPolicy(t, l, admin, core.MFAPolicyRequired, 0)
	for _, e := range auditEntries(t, l, admin) {
		if e.Action == auditTenantUpdate {
			return
		}
	}
	t.Fatal("turning the policy on recorded no tenant update")
}

func TestUsersWithoutAFactorCanBeListed(t *testing.T) {
	l, clk, _, admin := newTOTPLocal(t)
	ada := userActor(t, l, seedUser(t, l, admin, "ada@example.com"), core.RoleMember)
	seedUser(t, l, admin, "bob@example.com")
	enrol(t, l, clk, ada)

	users, _, err := l.ListUsers(authContext(admin), core.UserFilter{WithoutTOTP: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].Email != "bob@example.com" {
		t.Fatalf("users without a factor = %+v, want bob alone", users)
	}
}

func TestSignInUnderThePolicy(t *testing.T) {
	l, clk, scope, admin := newTOTPLocal(t)
	seedUser(t, l, admin, "ada@example.com")
	setMFAPolicy(t, l, admin, core.MFAPolicyRequired, 0)

	web, err := l.Login(loginContext(scope), core.LoginInput{
		Email: "ada@example.com", Password: testPassword, Surface: core.SurfaceWeb,
	})
	if err != nil {
		t.Fatal(err)
	}
	if web.Session == nil || web.Challenge != nil {
		t.Errorf("a browser sign-in = %+v, want a session the gate confines", web)
	}
	cli, err := l.Login(loginContext(scope), core.LoginInput{
		Email: "ada@example.com", Password: testPassword, Surface: core.SurfaceCLI,
	})
	if err != nil {
		t.Fatal(err)
	}
	if cli.Challenge == nil || cli.Challenge.Kind != core.ChallengeEnrol {
		t.Errorf("a command line sign-in = %+v, want enrolment", cli)
	}

	setMFAPolicy(t, l, admin, core.MFAPolicyOff, 0)
	setMFAPolicy(t, l, admin, core.MFAPolicyRequired, time.Hour)
	clk.Advance(time.Minute)
	inGrace, err := l.Login(loginContext(scope), core.LoginInput{
		Email: "ada@example.com", Password: testPassword, Surface: core.SurfaceCLI,
	})
	if err != nil {
		t.Fatal(err)
	}
	if inGrace.Session == nil {
		t.Errorf("a command line sign-in inside the grace = %+v, want a session", inGrace)
	}
}
