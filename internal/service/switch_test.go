// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// sharedHost is the host the switch tests arrive on: one no domain claims,
// whose own tenant is the fixture's.
func (f twoTenants) sharedHost() core.ServedHost {
	return core.ServedHost{Host: "tix.example.test", Shared: true, Owner: f.home.TenantID}
}

// sessionContext is what the transport hands the service for a request that
// presented a session cookie on host: the actor the session verifies as, its
// token, and the host.
func (f twoTenants) sessionContext(t *testing.T, session *core.Session, host core.ServedHost) context.Context {
	t.Helper()
	ctx := context.Background()
	scope := core.TenantScope{TenantID: session.TenantID}
	var role core.Role
	if err := f.l.store.View(ctx, scope, func(tx store.Tx) error {
		var err error
		role, err = userRole(ctx, tx, session.ActorID)
		return err
	}); err != nil {
		t.Fatalf("reading the session's role: %v", err)
	}
	actor := &core.Actor{ID: session.ActorID, TenantID: session.TenantID, Kind: core.ActorUser, Role: role,
		SessionID: auth.SessionRef(auth.HashToken(session.Token))}
	ctx = core.WithActor(core.WithTenant(ctx, scope), actor)
	return core.WithServedHost(WithSessionToken(ctx, session.Token), host)
}

// sessionLives reports whether a session token still resolves in its tenant.
func (f twoTenants) sessionLives(t *testing.T, session *core.Session) bool {
	t.Helper()
	found := false
	if err := f.l.store.View(context.Background(), core.TenantScope{TenantID: session.TenantID}, func(tx store.Tx) error {
		_, _, err := tx.GetSessionByHash(context.Background(), auth.HashToken(session.Token))
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		found = err == nil
		return err
	}); err != nil {
		t.Fatalf("reading a session: %v", err)
	}
	return found
}

// origin reads how a session was minted.
func (f twoTenants) origin(t *testing.T, session *core.Session) core.SessionOrigin {
	t.Helper()
	var out core.SessionOrigin
	if err := f.l.store.View(context.Background(), core.TenantScope{TenantID: session.TenantID}, func(tx store.Tx) error {
		var err error
		out, err = tx.GetSessionOrigin(context.Background(), auth.HashToken(session.Token))
		return err
	}); err != nil {
		t.Fatalf("reading a session's origin: %v", err)
	}
	return out
}

// logins lists a tenant's sign-in audit entries, read by its admin.
func logins(t *testing.T, l *Local, admin *core.Actor) []core.AuditEntry {
	t.Helper()
	entries, _, err := l.ListAudit(authContext(admin), core.AuditFilter{Actions: []string{auditUserLogin}})
	if err != nil {
		t.Fatalf("listing sign-ins: %v", err)
	}
	return entries
}

func TestASwitchMintsASessionInTheTargetAndEndsTheOldOne(t *testing.T) {
	f := newTwoTenants(t)
	from, err := signIn(f.l, f.home, patEmail, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	homeLogins := len(logins(t, f.l, f.homeAdmin))

	result, err := f.l.SwitchTenant(f.sessionContext(t, from, f.sharedHost()), core.SwitchTenantInput{Tenant: "OTHER"})
	if err != nil {
		t.Fatalf("SwitchTenant = %v", err)
	}
	to := result.Session
	if to == nil || to.TenantID != f.other.TenantID || to.ActorID != f.otherPat.ID {
		t.Fatalf("switched session = %+v, want pat's actor %q in the other tenant", to, f.otherPat.ID)
	}
	if f.sessionLives(t, from) {
		t.Error("the session switched from still authenticates")
	}
	if !f.sessionLives(t, to) {
		t.Error("the switched session does not resolve in the target tenant")
	}
	if got := f.origin(t, to); got.Method != core.LoginMethodSwitch || got.SecondFactor {
		t.Errorf("switched session origin = %+v, want method switch with no second factor", got)
	}

	entries := logins(t, f.l, f.otherAdmin)
	if len(entries) != 1 || entries[0].ActorID != f.otherPat.ID || !strings.Contains(string(entries[0].After), core.LoginMethodSwitch) {
		t.Fatalf("the target's sign-ins = %+v, want one by pat's actor there with method switch", entries)
	}
	if strings.Contains(string(entries[0].After), f.home.TenantID) || strings.Contains(string(entries[0].After), f.homePat.ID) {
		t.Errorf("the target's entry names the tenant left: %s", entries[0].After)
	}
	if entries[0].Via.Session != "" {
		t.Errorf("the target's entry carries the old session's reference %q", entries[0].Via.Session)
	}
	if got := len(logins(t, f.l, f.homeAdmin)); got != homeLogins {
		t.Errorf("the tenant left recorded %d new sign-ins, want none", got-homeLogins)
	}
}

func TestASwitchToATenantThePersonIsNotInIsNotFound(t *testing.T) {
	f := newTwoTenants(t)
	other := seedUser(t, f.l, f.homeAdmin, "solo@example.com")
	_ = other
	solo, err := signIn(f.l, f.home, "solo@example.com", testPassword)
	if err != nil {
		t.Fatal(err)
	}
	ctx := f.sessionContext(t, solo, f.sharedHost())

	_, notMember := f.l.SwitchTenant(ctx, core.SwitchTenantInput{Tenant: "other"})
	_, missing := f.l.SwitchTenant(ctx, core.SwitchTenantInput{Tenant: "nowhere"})
	for name, err := range map[string]error{"a tenant without a membership": notMember, "a tenant that does not exist": missing} {
		if !core.IsKind(err, core.KindNotFound) {
			t.Errorf("switching to %s = %v, want not found", name, err)
		}
	}
	if !f.sessionLives(t, solo) {
		t.Error("a refused switch ended the session")
	}
}

func TestOnlyASessionOnASharedHostSwitches(t *testing.T) {
	f := newTwoTenants(t)
	from, err := signIn(f.l, f.home, patEmail, testPassword)
	if err != nil {
		t.Fatal(err)
	}

	bound := core.ServedHost{Host: "acme.example.test", Owner: f.home.TenantID}
	if _, err := f.l.SwitchTenant(f.sessionContext(t, from, bound), core.SwitchTenantInput{Tenant: "other"}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("switching on a host bound to one tenant = %v, want invalid", err)
	}

	tokenCtx := f.sessionContext(t, from, f.sharedHost())
	actor, _ := core.ActorFrom(tokenCtx)
	asToken := *actor
	asToken.SessionID, asToken.TokenID = "", "tok"
	if _, err := f.l.SwitchTenant(core.WithActor(tokenCtx, &asToken), core.SwitchTenantInput{Tenant: "other"}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("a token switching = %v, want invalid", err)
	}
	if !f.sessionLives(t, from) {
		t.Error("a refused switch ended the session")
	}
}

func TestASwitchSkipsATenantServedAtItsOwnAddress(t *testing.T) {
	f := newTwoTenants(t)
	if err := f.l.store.Update(context.Background(), f.other, func(tx store.Tx) error {
		return tx.AddDomain(context.Background(), &core.Domain{Hostname: "other.example.test"})
	}); err != nil {
		t.Fatal(err)
	}
	from, err := signIn(f.l, f.home, patEmail, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.l.SwitchTenant(f.sessionContext(t, from, f.sharedHost()), core.SwitchTenantInput{Tenant: "other"})
	if !core.IsKind(err, core.KindInvalid) || !strings.Contains(err.Error(), "other.example.test") {
		t.Fatalf("switching to a tenant with its own address = %v, want invalid naming the address", err)
	}
	if !f.sessionLives(t, from) {
		t.Error("a refused switch ended the session")
	}
}

func TestASwitchAsksForTheFactorTheTargetRequires(t *testing.T) {
	f := newTwoTenants(t, WithTOTPKey(totpTestKey))
	requireTOTP(t, f.l, f.other, core.SurfaceWeb)

	t.Run("an account with no factor is told to set one up", func(t *testing.T) {
		from, err := signIn(f.l, f.home, patEmail, testPassword)
		if err != nil {
			t.Fatal(err)
		}
		_, err = f.l.SwitchTenant(f.sessionContext(t, from, f.sharedHost()), core.SwitchTenantInput{Tenant: "other"})
		var e *core.Error
		if !core.IsKind(err, core.KindForbidden) || !asError(err, &e) || e.Details["reason"] != ReasonSecondFactorRequired {
			t.Fatalf("switch without an enrolment = %v, want forbidden with %s", err, ReasonSecondFactorRequired)
		}
	})

	homeActor := &core.Actor{ID: f.homePat.ID, TenantID: f.home.TenantID, Kind: core.ActorUser, Role: core.RoleMember}
	secret, _ := enrol(t, f.l, f.clk, homeActor)

	t.Run("a session without a factor is challenged, and a wrong code mints nothing", func(t *testing.T) {
		f.clk.Advance(auth.TOTPPeriod)
		result, err := f.l.Login(core.WithTenant(context.Background(), f.home),
			core.LoginInput{Email: patEmail, Password: testPassword, Surface: core.SurfaceCLI})
		if err != nil || result.Challenge == nil {
			t.Fatalf("password step = %+v, %v; want a code challenge", result, err)
		}
		result, err = f.l.Login(core.WithTenant(context.Background(), f.home),
			core.LoginInput{Ticket: result.Challenge.Ticket, Code: codeFor(t, secret, f.clk), Surface: core.SurfaceCLI})
		if err != nil {
			t.Fatal(err)
		}
		withFactor := result.Session
		if got := f.origin(t, withFactor); !got.SecondFactor || got.Method != core.LoginMethodPassword {
			t.Fatalf("a sign-in that passed a code recorded %+v, want password with a second factor", got)
		}

		noFactor := mintPasswordSession(t, f)
		ctx := f.sessionContext(t, noFactor, f.sharedHost())
		challenged, err := f.l.SwitchTenant(ctx, core.SwitchTenantInput{Tenant: "other"})
		if err != nil || challenged.Challenge == nil || challenged.Challenge.Kind != core.ChallengeCode || challenged.Session != nil {
			t.Fatalf("switch from a session without a factor = %+v, %v; want a code challenge and no session", challenged, err)
		}
		if _, err := f.l.SwitchTenant(ctx, core.SwitchTenantInput{Tenant: "other", Code: "000000"}); err == nil {
			t.Fatal("a wrong code completed the switch")
		}
		if !f.sessionLives(t, noFactor) {
			t.Fatal("a refused code ended the session")
		}
		f.clk.Advance(auth.TOTPPeriod)
		done, err := f.l.SwitchTenant(ctx, core.SwitchTenantInput{Tenant: "other", Code: codeFor(t, secret, f.clk)})
		if err != nil || done.Session == nil {
			t.Fatalf("switch with the right code = %+v, %v", done, err)
		}
		if got := f.origin(t, done.Session); !got.SecondFactor || got.Method != core.LoginMethodSwitch {
			t.Errorf("a switch that passed a code recorded %+v, want switch with a second factor", got)
		}

		f.clk.Advance(auth.TOTPPeriod)
		again, err := f.l.SwitchTenant(f.sessionContext(t, withFactor, f.sharedHost()), core.SwitchTenantInput{Tenant: "other"})
		if err != nil || again.Session == nil {
			t.Fatalf("switch from a session that passed a factor = %+v, %v; want it done without asking", again, err)
		}
	})
}

// mintPasswordSession writes a session for pat in her home tenant as a
// password sign-in with no second factor, which is how every session minted
// before sessions recorded it reads.
func mintPasswordSession(t *testing.T, f twoTenants) *core.Session {
	t.Helper()
	actor := &core.Actor{ID: f.homePat.ID, TenantID: f.home.TenantID, Kind: core.ActorUser, Role: core.RoleMember}
	result, err := f.l.mintSessionIf(context.Background(), f.home, actor, passwordOrigin,
		func(*mutation) (bool, error) { return true, nil })
	if err != nil {
		t.Fatal(err)
	}
	return result.Session
}

func TestWhoAmIListsMembershipsToASessionAndToAccountRead(t *testing.T) {
	f := newTwoTenants(t)
	from, err := signIn(f.l, f.home, patEmail, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	me, err := f.l.WhoAmI(f.sessionContext(t, from, f.sharedHost()))
	if err != nil {
		t.Fatal(err)
	}
	if me.AccountID == "" || me.Email != patEmail || me.TenantKey == "" {
		t.Errorf("whoami = %+v, want the account, its email and the tenant key", me)
	}
	if len(me.Memberships) != 2 || me.Memberships[1].TenantKey != "other" || me.Memberships[1].ActorID != f.otherPat.ID {
		t.Fatalf("a session's memberships = %+v, want both tenants with pat's actor in each", me.Memberships)
	}

	token := &core.Actor{ID: f.homePat.ID, TenantID: f.home.TenantID, Kind: core.ActorUser, TokenID: "t",
		Scopes: []core.Scope{core.ScopeAll}}
	me, err = f.l.WhoAmI(authContext(token))
	if err != nil {
		t.Fatal(err)
	}
	if len(me.Memberships) != 0 || me.AccountID == "" {
		t.Errorf("a * token's whoami = %+v, want the account and no memberships", me)
	}
	token.Scopes = append(token.Scopes, core.ScopeAccountRead)
	if me, err = f.l.WhoAmI(authContext(token)); err != nil || len(me.Memberships) != 2 {
		t.Errorf("an account:read token's memberships = %+v, %v; want both", me, err)
	}
}

func TestSignInOnASharedHostGoesByTheAccount(t *testing.T) {
	f := newTwoTenants(t)
	ctx := context.Background()
	if _, err := f.l.CreateUser(authContext(f.otherAdmin), core.CreateUserInput{
		Email: "olly@example.com", Password: testPassword, Role: core.RoleMember,
	}); err != nil {
		t.Fatal(err)
	}
	at := func(host core.ServedHost) (*core.LoginResult, error) {
		return f.l.Login(core.WithServedHost(core.WithTenant(ctx, f.home), host),
			core.LoginInput{Email: "olly@example.com", Password: testPassword, Surface: core.SurfaceWeb})
	}

	result, err := at(f.sharedHost())
	if err != nil || result.Session == nil || result.Session.TenantID != f.other.TenantID {
		t.Fatalf("sign-in on a shared host by a member of the other tenant only = %+v, %v; want a session there", result, err)
	}
	named := f.sharedHost()
	named.Named = true
	if _, err := at(named); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("sign-in where the header named this tenant = %v, want invalid credentials", err)
	}
	if _, err := at(core.ServedHost{Host: "acme.example.test", Owner: f.home.TenantID}); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("sign-in on a host bound to this tenant = %v, want invalid credentials", err)
	}
	if _, err := f.l.Login(core.WithServedHost(core.WithTenant(ctx, f.home), f.sharedHost()),
		core.LoginInput{Email: "olly@example.com", Password: "wrong-password-entirely", Surface: core.SurfaceWeb}); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("a wrong password on a shared host = %v, want invalid credentials", err)
	}
}
