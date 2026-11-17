// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/egress"
	"github.com/heliopsy/tix/internal/sso/ssotest"
	"github.com/heliopsy/tix/internal/store"
)

const ssoCallback = "https://tix.example" + core.SSOCallbackPath

type ssoFixture struct {
	l     *Local
	clk   *clock.Fake
	scope core.TenantScope
	admin *core.Actor
	idp   *ssotest.OIDC
}

func newSSOFixture(t *testing.T, edit func(*core.CreateSSOProviderInput)) ssoFixture {
	t.Helper()
	l, clk, scope, admin := newLocalWith(t, WithTOTPKey(totpTestKey), WithEgress(egress.Permissive()))
	idp := ssotest.NewOIDC(t, clk.Now)
	in := core.CreateSSOProviderInput{Key: "kc", Kind: core.SSOKindOIDC, DisplayName: "Keycloak",
		Issuer: idp.Issuer(), ClientID: idp.ClientID, ClientSecret: idp.ClientSecret}
	if edit != nil {
		edit(&in)
	}
	if _, err := l.CreateSSOProvider(authContext(admin), in); err != nil {
		t.Fatalf("CreateSSOProvider: %v", err)
	}
	return ssoFixture{l: l, clk: clk, scope: scope, admin: admin, idp: idp}
}

// person is the claims of somebody the fake provider signs in.
func person(sub, email string, verified bool) map[string]any {
	return map[string]any{"sub": sub, "email": email, "email_verified": verified, "name": "Ada Lovelace",
		"preferred_username": "ada"}
}

// signInSSO runs a browser sign-in through the fixture's provider as the
// person the claims describe.
func (f ssoFixture) signInSSO(t *testing.T, claims map[string]any) (*core.LoginResult, error) {
	t.Helper()
	f.idp.SetClaims(claims)
	ctx := loginContext(f.scope)
	begin, err := f.l.BeginSSOLogin(ctx, core.BeginSSOInput{Provider: "kc", RedirectURI: ssoCallback, Next: "/tasks"})
	if err != nil {
		t.Fatalf("BeginSSOLogin: %v", err)
	}
	code := f.idp.Grant(begin.AuthURL)
	return f.l.CompleteSSOLogin(ctx, core.CompleteSSOInput{Flow: begin.Flow, Code: code, State: begin.State})
}

func (f ssoFixture) audits(t *testing.T, action string) []core.AuditEntry {
	t.Helper()
	entries, _, err := f.l.ListAudit(authContext(f.admin), core.AuditFilter{Page: core.Page{Limit: 200}})
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	var out []core.AuditEntry
	for _, e := range entries {
		if e.Action == action {
			out = append(out, e)
		}
	}
	return out
}

func TestSSOCreatesAPersonJustInTimeThenFindsThemBySubject(t *testing.T) {
	f := newSSOFixture(t, func(in *core.CreateSSOProviderInput) { in.AutoCreate = true })
	first, err := f.signInSSO(t, person("sub-1", "Ada@Example.com", true))
	if err != nil {
		t.Fatalf("first sign-in: %v", err)
	}
	if first.Session == nil || first.Next != "/tasks" {
		t.Fatalf("first sign-in = %+v, want a session returning to /tasks", first)
	}
	if got := f.audits(t, auditSSOUserCreated); len(got) != 1 {
		t.Fatalf("sso.user_created entries = %d, want 1", len(got))
	}
	second, err := f.signInSSO(t, person("sub-1", "ada@example.com", true))
	if err != nil {
		t.Fatalf("second sign-in: %v", err)
	}
	if second.Session.ActorID != first.Session.ActorID {
		t.Errorf("second sign-in is actor %s, want %s", second.Session.ActorID, first.Session.ActorID)
	}
	if got := f.audits(t, auditSSOLink); len(got) != 1 {
		t.Errorf("sso.link entries = %d, want 1: the second sign-in found the identity by subject", len(got))
	}
	if got := f.audits(t, auditSSOLogin); len(got) != 2 {
		t.Errorf("sso.login entries = %d, want 2", len(got))
	}
	sessions, _, err := f.l.ListSessions(authContext(&core.Actor{ID: first.Session.ActorID, TenantID: f.scope.TenantID,
		Kind: core.ActorUser, Role: core.RoleMember}), core.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) == 0 || sessions[0].Method != "sso:kc" {
		t.Errorf("sessions = %+v, want method sso:kc", sessions)
	}
	user, err := f.l.GetUser(authContext(f.admin), first.Session.ActorID)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if user.Email != "ada@example.com" || roleOf(t, f, user.ID) != core.RoleMember {
		t.Errorf("created user = %+v, want ada@example.com as member", user)
	}
}

func TestSSORefusesATokenThatFailsAnyCheck(t *testing.T) {
	cases := []struct {
		name  string
		tweak ssotest.Tweak
		want  string
	}{
		{"bad signature", ssotest.Tweak{ForeignKey: true}, "signature"},
		{"wrong audience", ssotest.Tweak{Audience: "someone-else"}, "another client"},
		{"wrong issuer", ssotest.Tweak{Issuer: "https://evil.example"}, "another issuer"},
		{"expired", ssotest.Tweak{ExpiresIn: -time.Minute}, "expired"},
		{"bad nonce", ssotest.Tweak{Nonce: "replayed"}, "nonce"},
		{"no id token", ssotest.Tweak{NoIDToken: true}, "no id token"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newSSOFixture(t, func(in *core.CreateSSOProviderInput) { in.AutoCreate = true })
			f.idp.SetTweak(tc.tweak)
			result, err := f.signInSSO(t, person("sub-1", "ada@example.com", true))
			if !core.IsKind(err, core.KindUnauthenticated) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("sign-in = %+v, %v; want unauthenticated naming %q", result, err, tc.want)
			}
			if got := f.audits(t, auditSSOUserCreated); len(got) != 0 {
				t.Errorf("a refused token created %d accounts", len(got))
			}
		})
	}
}

func TestSSORefusesAForeignState(t *testing.T) {
	f := newSSOFixture(t, func(in *core.CreateSSOProviderInput) { in.AutoCreate = true })
	f.idp.SetClaims(person("sub-1", "ada@example.com", true))
	ctx := loginContext(f.scope)
	begin, err := f.l.BeginSSOLogin(ctx, core.BeginSSOInput{Provider: "kc", RedirectURI: ssoCallback})
	if err != nil {
		t.Fatalf("BeginSSOLogin: %v", err)
	}
	code := f.idp.Grant(begin.AuthURL)
	_, err = f.l.CompleteSSOLogin(ctx, core.CompleteSSOInput{Flow: begin.Flow, Code: code, State: "forged"})
	if !core.IsKind(err, core.KindUnauthenticated) || !strings.Contains(err.Error(), "state") {
		t.Fatalf("forged state = %v, want an unauthenticated refusal naming the state", err)
	}
	if _, err := f.l.CompleteSSOLogin(ctx, core.CompleteSSOInput{Flow: begin.Flow + "x", Code: code,
		State: begin.State}); !core.IsKind(err, core.KindUnauthenticated) {
		t.Fatalf("tampered flow = %v, want unauthenticated", err)
	}
	f.clk.Advance(11 * time.Minute)
	if _, err := f.l.CompleteSSOLogin(ctx, core.CompleteSSOInput{Flow: begin.Flow, Code: code,
		State: begin.State}); !core.IsKind(err, core.KindUnauthenticated) {
		t.Fatalf("expired flow = %v, want unauthenticated", err)
	}
}

func TestSSOLinkingRules(t *testing.T) {
	cases := []struct {
		name       string
		autoCreate bool
		domains    []string
		claims     map[string]any
		reason     string
	}{
		{"unverified email", true, nil, person("s", "new@example.com", false), "sso_email_unverified"},
		{"unknown without auto create", false, nil, person("s", "new@example.com", true), "sso_not_member"},
		{"no email", true, nil, map[string]any{"sub": "s"}, "sso_no_email"},
		{"domain not allowed", true, []string{"example.org"}, person("s", "new@example.com", true), "sso_email_domain"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newSSOFixture(t, func(in *core.CreateSSOProviderInput) {
				in.AutoCreate, in.AllowedDomains = tc.autoCreate, tc.domains
			})
			_, err := f.signInSSO(t, tc.claims)
			var domain *core.Error
			if !core.IsKind(err, core.KindForbidden) || !asError(err, &domain) || domain.Details["reason"] != tc.reason {
				t.Fatalf("sign-in = %v, want forbidden with reason %s", err, tc.reason)
			}
			denied := f.audits(t, auditSSODenied)
			if len(denied) != 1 || !strings.Contains(string(denied[0].After), strings.TrimPrefix(tc.reason, "sso_")) {
				t.Errorf("sso.denied = %+v, want one naming %s", denied, tc.reason)
			}
		})
	}
}

func mustJSON(v any) []byte {
	out, _ := json.Marshal(v)
	return out
}

func TestSSOLinksAVerifiedEmailOfAMember(t *testing.T) {
	f := newSSOFixture(t, nil)
	member := seedUser(t, f.l, f.admin, "grace@example.com")
	result, err := f.signInSSO(t, person("sub-g", "grace@example.com", true))
	if err != nil {
		t.Fatalf("sign-in: %v", err)
	}
	if result.Session.ActorID != member.ID {
		t.Errorf("signed in as %s, want the member %s", result.Session.ActorID, member.ID)
	}
	if got := f.audits(t, auditSSOLink); len(got) != 1 {
		t.Errorf("sso.link entries = %d, want 1", len(got))
	}
}

func TestSSONeverAdoptsAnAccountOfAnotherTenant(t *testing.T) {
	f := newSSOFixture(t, func(in *core.CreateSSOProviderInput) { in.AutoCreate = true })
	ctx := context.Background()
	other := core.Tenant{Key: "other", Name: "Other"}
	if err := f.l.store.Unscoped(ctx, func(u store.UnscopedTx) error { return u.CreateTenant(ctx, &other) }); err != nil {
		t.Fatal(err)
	}
	if err := f.l.store.Update(ctx, core.TenantScope{TenantID: other.ID}, func(tx store.Tx) error {
		u := &core.User{ID: "u-elsewhere", Email: "lin@example.com"}
		if err := tx.CreateUser(ctx, u, ""); err != nil {
			return err
		}
		return tx.CreateActor(ctx, &core.Actor{ID: u.ID, UserID: u.ID, Kind: core.ActorUser, Handle: "lin"})
	}); err != nil {
		t.Fatal(err)
	}
	_, err := f.signInSSO(t, person("sub-l", "lin@example.com", true))
	var domain *core.Error
	if !asError(err, &domain) || domain.Details["reason"] != "sso_account_elsewhere" {
		t.Fatalf("sign-in = %v, want a refusal naming the account elsewhere", err)
	}
}

func TestSSOMapsGroupsToRoles(t *testing.T) {
	f := newSSOFixture(t, func(in *core.CreateSSOProviderInput) {
		in.AutoCreate = true
		in.RoleMapping = map[string]core.Role{"tix-admins": core.RoleAdmin, "tix-viewers": core.RoleViewer}
	})
	member := seedUser(t, f.l, f.admin, "grace@example.com")
	claims := person("sub-g", "grace@example.com", true)
	claims["groups"] = []string{"staff", "tix-viewers", "tix-admins"}
	if _, err := f.signInSSO(t, claims); err != nil {
		t.Fatalf("sign-in: %v", err)
	}
	if got := roleOf(t, f, member.ID); got != core.RoleAdmin {
		t.Errorf("role = %s, want admin from tix-admins", got)
	}
	claims["groups"] = []string{"staff"}
	if _, err := f.signInSSO(t, claims); err != nil {
		t.Fatalf("second sign-in: %v", err)
	}
	if got := roleOf(t, f, member.ID); got != core.RoleAdmin {
		t.Errorf("role = %s after a sign-in with no mapped group, want it kept", got)
	}
}

func TestSSORefusesAGlobalAdmin(t *testing.T) {
	f := newSSOFixture(t, nil)
	boss := seedUser(t, f.l, f.admin, "boss@example.com")
	makeGlobalAdmin(t, f.l, f.scope, boss.ID)
	_, err := f.signInSSO(t, person("sub-b", "boss@example.com", true))
	var domain *core.Error
	if !asError(err, &domain) || domain.Details["reason"] != "sso_global_admin" {
		t.Fatalf("sign-in = %v, want the global admin refused", err)
	}
}

func makeGlobalAdmin(t *testing.T, l *Local, scope core.TenantScope, userID string) {
	t.Helper()
	ctx := context.Background()
	if err := l.store.Update(ctx, scope, func(tx store.Tx) error {
		return tx.SetUserGlobalAdmin(ctx, userID, true)
	}); err != nil {
		t.Fatal(err)
	}
}

func TestSSOStillAsksForTheTenantsSecondFactor(t *testing.T) {
	f := newSSOFixture(t, func(in *core.CreateSSOProviderInput) { in.AutoCreate = true })
	on := true
	if _, err := f.l.UpdateTenant(authContext(f.admin), "", core.UpdateTenantInput{RequireTOTPWeb: &on}); err != nil {
		t.Fatal(err)
	}
	claims := person("sub-1", "ada@example.com", true)
	claims["amr"] = []string{"pwd", "mfa"}
	result, err := f.signInSSO(t, claims)
	if err != nil {
		t.Fatalf("sign-in: %v", err)
	}
	if result.Session != nil || result.Challenge == nil {
		t.Fatalf("result = %+v, want a second factor challenge and no session", result)
	}
	trust := true
	if _, err := f.l.UpdateSSOProvider(authContext(f.admin), "kc", core.UpdateSSOProviderInput{TrustMFA: &trust}); err != nil {
		t.Fatal(err)
	}
	if result, err = f.signInSSO(t, claims); err != nil || result.Session == nil {
		t.Fatalf("trusted sign-in = %+v, %v; want a session", result, err)
	}
	claims["amr"] = []string{"pwd"}
	if result, err = f.signInSSO(t, claims); err != nil || result.Challenge == nil {
		t.Fatalf("trusted provider without mfa asserted = %+v, %v; want a challenge", result, err)
	}
}

func TestPasswordLoginPolicy(t *testing.T) {
	l, _, scope, admin := newLocalWith(t, WithTOTPKey(totpTestKey), WithEgress(egress.Permissive()))
	member := seedUser(t, l, admin, "m@example.com")
	boss := seedUser(t, l, admin, "boss@example.com")
	makeGlobalAdmin(t, l, scope, boss.ID)
	off := core.PasswordLoginDisabled
	if _, err := l.UpdateTenant(authContext(admin), "", core.UpdateTenantInput{PasswordLogin: &off}); !core.IsKind(err, core.KindPrecondition) {
		t.Fatalf("disabling with no provider = %v, want a precondition refusal", err)
	}
	idp := ssotest.NewOIDC(t, time.Now)
	if _, err := l.CreateSSOProvider(authContext(admin), core.CreateSSOProviderInput{Key: "kc", Kind: core.SSOKindOIDC,
		Issuer: idp.Issuer(), ClientID: idp.ClientID}); err != nil {
		t.Fatal(err)
	}
	tenant, err := l.UpdateTenant(authContext(admin), "", core.UpdateTenantInput{PasswordLogin: &off})
	if err != nil || tenant.PasswordLogin != off {
		t.Fatalf("disabling with a provider = %+v, %v", tenant, err)
	}
	if _, err := signIn(l, scope, member.Email, testPassword); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("member password sign-in = %v, want forbidden", err)
	}
	if _, err := signIn(l, scope, member.Email, "wrong-password-entirely"); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("wrong password = %v, want the usual invalid credentials", err)
	}
	if s, err := signIn(l, scope, boss.Email, testPassword); err != nil || s == nil {
		t.Errorf("global admin break-glass = %v, want a session", err)
	}
	disable := false
	if _, err := l.UpdateSSOProvider(authContext(admin), "kc", core.UpdateSSOProviderInput{Enabled: &disable}); !core.IsKind(err, core.KindPrecondition) {
		t.Errorf("disabling the last provider = %v, want refused", err)
	}
	if err := l.DeleteSSOProvider(authContext(admin), "kc"); !core.IsKind(err, core.KindPrecondition) {
		t.Errorf("deleting the last provider = %v, want refused", err)
	}
	options, err := l.SignInOptions(loginContext(scope))
	if err != nil || options.PasswordLogin != off || len(options.Providers) != 1 {
		t.Errorf("SignInOptions = %+v, %v", options, err)
	}
}

func TestSSOProviderSecretIsSealedAndAdminOnly(t *testing.T) {
	f := newSSOFixture(t, nil)
	got, err := f.l.GetSSOProvider(authContext(f.admin), "kc")
	if err != nil {
		t.Fatal(err)
	}
	raw := string(mustJSON(got))
	if strings.Contains(raw, f.idp.ClientSecret) || !got.SecretSet || got.SecretHint != "alue" {
		t.Errorf("provider = %s, want the secret only as set with its hint", raw)
	}
	for _, e := range f.audits(t, auditSSOProviderCreate) {
		if strings.Contains(string(mustJSON(e)), f.idp.ClientSecret) || strings.Contains(string(mustJSON(e)), got.SecretHint+"\"") {
			t.Errorf("audit entry carries the secret: %s", mustJSON(e))
		}
	}
	member := seedUser(t, f.l, f.admin, "m@example.com")
	if _, err := f.l.ListSSOProviders(authContext(userActor(t, f.l, member, core.RoleMember))); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("member listing providers = %v, want forbidden", err)
	}
	if _, err := f.l.CreateSSOProvider(authContext(f.admin), core.CreateSSOProviderInput{Key: "saml1", Kind: core.SSOKindSAML,
		ClientID: "x"}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("saml provider = %v, want refused", err)
	}
}

func TestBeginSSORefusesAForeignRedirect(t *testing.T) {
	f := newSSOFixture(t, nil)
	for _, uri := range []string{"https://evil.example/steal", "http://evil.example:8080/callback", "ftp://127.0.0.1/x"} {
		if _, err := f.l.BeginSSOLogin(loginContext(f.scope), core.BeginSSOInput{Provider: "kc", RedirectURI: uri}); !core.IsKind(err, core.KindInvalid) {
			t.Errorf("redirect %s = %v, want refused", uri, err)
		}
	}
	begin, err := f.l.BeginSSOLogin(loginContext(f.scope), core.BeginSSOInput{Provider: "kc",
		RedirectURI: "http://127.0.0.1:43123/callback"})
	if err != nil || !strings.Contains(begin.AuthURL, "code_challenge_method=S256") {
		t.Fatalf("loopback begin = %+v, %v; want a PKCE auth url", begin, err)
	}
}

func roleOf(t *testing.T, f ssoFixture, actorID string) core.Role {
	t.Helper()
	ctx := context.Background()
	var role core.Role
	if err := f.l.store.View(ctx, f.scope, func(tx store.Tx) error {
		var err error
		role, err = userRole(ctx, tx, actorID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return role
}
