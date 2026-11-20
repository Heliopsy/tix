// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"errors"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/egress"
	"github.com/heliopsy/tix/internal/sso/ssotest"
)

const (
	appRedirect = "red.tix.mobile:/sso"
	appVerifier = "verifier-0123456789-0123456789-0123456789-abcdef"
)

// appBegin is a begin as the app's browser asks for it.
func appBegin(provider string) core.BeginSSOInput {
	return core.BeginSSOInput{Provider: provider, RedirectURI: ssoCallback, Surface: core.SurfaceWeb,
		AppRedirect: appRedirect, CodeChallenge: auth.PKCEChallenge(appVerifier), CodeChallengeMethod: "S256",
		AppState: "opaque-1"}
}

// appSignInSSO runs an app's sign-in through the fixture's OIDC provider as
// far as the browser goes.
func (f ssoFixture) appSignInSSO(t *testing.T, claims map[string]any) (*core.LoginResult, error) {
	t.Helper()
	f.idp.SetClaims(claims)
	ctx := loginContext(f.scope)
	begin, err := f.l.BeginSSOLogin(ctx, appBegin("kc"))
	if err != nil {
		t.Fatalf("BeginSSOLogin: %v", err)
	}
	code := f.idp.Grant(begin.AuthURL)
	return f.l.CompleteSSOLogin(ctx, core.CompleteSSOInput{Flow: begin.Flow, Code: code, State: begin.State})
}

// handedCode reads the code and state out of the app redirect a sign-in
// ended with.
func handedCode(t *testing.T, result *core.LoginResult) string {
	t.Helper()
	if result == nil || result.Session != nil || result.Challenge != nil || result.AppRedirect == "" {
		t.Fatalf("result = %+v, want the app redirect and no session", result)
	}
	u, err := url.Parse(result.AppRedirect)
	if err != nil || u.Scheme != "red.tix.mobile" || u.Opaque != "/sso" && u.Path != "/sso" {
		t.Fatalf("handed to %q, want red.tix.mobile:/sso", result.AppRedirect)
	}
	q := u.Query()
	if q.Get("state") != "opaque-1" || len(q.Get("code")) < 43 || q.Get("error") != "" {
		t.Fatalf("handed %q, want a code of 32 bytes or more and the app's state", result.AppRedirect)
	}
	return q.Get("code")
}

func (f ssoFixture) exchange(code, verifier string) (*core.LoginResult, error) {
	return f.l.ExchangeSSOCode(loginContext(f.scope), core.SSOExchangeInput{Code: code, CodeVerifier: verifier})
}

func TestAppSignInHandsACodeThatTheAppExchangesOnce(t *testing.T) {
	f := newSSOFixture(t, func(in *core.CreateSSOProviderInput) { in.AutoCreate = true })
	result, err := f.appSignInSSO(t, person("sub-1", "ada@example.com", true))
	if err != nil {
		t.Fatalf("app sign-in: %v", err)
	}
	code := handedCode(t, result)
	if got := f.audits(t, auditUserLogin); len(got) != 0 {
		t.Fatalf("user.login entries before the exchange = %d, want 0: no session yet", len(got))
	}
	out, err := f.exchange(code, appVerifier)
	if err != nil || out.Session == nil || out.Session.Token == "" {
		t.Fatalf("exchange = %+v, %v; want a session", out, err)
	}
	if !out.Session.Remembered {
		t.Error("an app session is not remembered")
	}
	logins := f.audits(t, auditUserLogin)
	if len(logins) != 1 || !strings.Contains(string(logins[0].After), `"method":"sso:kc"`) {
		t.Errorf("user.login entries = %+v, want one with method sso:kc", logins)
	}
	if _, err := f.exchange(code, appVerifier); !core.IsKind(err, core.KindUnauthenticated) ||
		!strings.Contains(err.Error(), "invalid credentials") {
		t.Errorf("a reused code = %v, want invalid credentials", err)
	}
}

func TestAppSignInRefusesAWrongVerifierAndBurnsTheCode(t *testing.T) {
	f := newSSOFixture(t, func(in *core.CreateSSOProviderInput) { in.AutoCreate = true })
	result, err := f.appSignInSSO(t, person("sub-1", "ada@example.com", true))
	if err != nil {
		t.Fatal(err)
	}
	code := handedCode(t, result)
	ctx, note := auth.WithRefusalNote(loginContext(f.scope))
	_, err = f.l.ExchangeSSOCode(ctx, core.SSOExchangeInput{Code: code, CodeVerifier: appVerifier + "x"})
	if !core.IsKind(err, core.KindUnauthenticated) {
		t.Fatalf("a wrong verifier = %v, want unauthenticated", err)
	}
	if r, ok := note.Get(); !ok || r.Reason != auth.RefusedSSOCode {
		t.Errorf("refusal noted = %+v, want %s", r, auth.RefusedSSOCode)
	}
	if _, err := f.exchange(code, appVerifier); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("the right verifier after a wrong one = %v, want refused: the code is spent", err)
	}
}

func TestAppSignInCodeExpires(t *testing.T) {
	f := newSSOFixture(t, func(in *core.CreateSSOProviderInput) { in.AutoCreate = true })
	result, err := f.appSignInSSO(t, person("sub-1", "ada@example.com", true))
	if err != nil {
		t.Fatal(err)
	}
	code := handedCode(t, result)
	f.clk.Advance(auth.SSOHandoffTTL + time.Second)
	if _, err := f.exchange(code, appVerifier); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("an expired code = %v, want invalid credentials", err)
	}
}

func TestAppSignInCodeIsTakenOnceUnderARace(t *testing.T) {
	f := newSSOFixture(t, func(in *core.CreateSSOProviderInput) { in.AutoCreate = true })
	result, err := f.appSignInSSO(t, person("sub-1", "ada@example.com", true))
	if err != nil {
		t.Fatal(err)
	}
	code := handedCode(t, result)
	const racers = 8
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		sessions int
		refused  int
	)
	start := make(chan struct{})
	for range racers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			out, err := f.exchange(code, appVerifier)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil && out.Session != nil:
				sessions++
			case core.IsKind(err, core.KindUnauthenticated):
				refused++
			default:
				t.Errorf("a racing exchange = %+v, %v", out, err)
			}
		}()
	}
	close(start)
	wg.Wait()
	if sessions != 1 || refused != racers-1 {
		t.Errorf("racing exchanges minted %d sessions and refused %d, want 1 and %d", sessions, refused, racers-1)
	}
}

func TestExchangeRefusesAMalformedBodyAndAnUnknownCode(t *testing.T) {
	f := newSSOFixture(t, nil)
	if _, err := f.exchange("", appVerifier); !core.IsKind(err, core.KindInvalid) || core.FieldOf(err) != "code" {
		t.Errorf("no code = %v, want invalid on code", err)
	}
	if _, err := f.exchange("abc", ""); !core.IsKind(err, core.KindInvalid) || core.FieldOf(err) != "code_verifier" {
		t.Errorf("no verifier = %v, want invalid on code_verifier", err)
	}
	for _, code := range []string{"never-handed", strings.Repeat("x", 300)} {
		if _, err := f.exchange(code, appVerifier); !core.IsKind(err, core.KindUnauthenticated) {
			t.Errorf("code %.10q = %v, want invalid credentials", code, err)
		}
	}
}

func TestAppSignInBeginRefusesBadAppParameters(t *testing.T) {
	f := newSSOFixture(t, nil)
	cases := []struct {
		name  string
		edit  func(*core.BeginSSOInput)
		field string
	}{
		{"not listed", func(in *core.BeginSSOInput) { in.AppRedirect = "evil.app:/sso" }, "app_redirect"},
		{"a prefix of a listed one", func(in *core.BeginSSOInput) { in.AppRedirect = "red.tix.mobile:/ss" }, "app_redirect"},
		{"longer than a listed one", func(in *core.BeginSSOInput) { in.AppRedirect = appRedirect + "/x" }, "app_redirect"},
		{"https not listed", func(in *core.BeginSSOInput) { in.AppRedirect = "https://evil.example/sso" }, "app_redirect"},
		{"no redirect", func(in *core.BeginSSOInput) { in.AppRedirect = "" }, "app_redirect"},
		{"no challenge", func(in *core.BeginSSOInput) { in.CodeChallenge = "" }, "code_challenge"},
		{"plain", func(in *core.BeginSSOInput) { in.CodeChallengeMethod = "plain" }, "code_challenge_method"},
		{"no method", func(in *core.BeginSSOInput) { in.CodeChallengeMethod = "" }, "code_challenge_method"},
		{"not a digest", func(in *core.BeginSSOInput) { in.CodeChallenge = "short" }, "code_challenge"},
		{"long state", func(in *core.BeginSSOInput) { in.AppState = strings.Repeat("s", 257) }, "app_state"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := appBegin("kc")
			tc.edit(&in)
			_, err := f.l.BeginSSOLogin(loginContext(f.scope), in)
			if !core.IsKind(err, core.KindInvalid) || core.FieldOf(err) != tc.field {
				t.Errorf("begin = %v, want invalid on %s", err, tc.field)
			}
		})
	}
	in := appBegin("kc")
	in.AppState = strings.Repeat("ж", 256)
	if _, err := f.l.BeginSSOLogin(loginContext(f.scope), in); err != nil {
		t.Errorf("a 256 character state = %v, want accepted", err)
	}
}

func TestAppSignInListedHTTPSRedirect(t *testing.T) {
	l, clk, scope, admin := newLocalWith(t, WithTOTPKey(totpTestKey), WithEgress(egress.Permissive()),
		WithSSOAppRedirects([]string{"https://app.example/sso?x=1"}))
	idp := ssotest.NewOIDC(t, clk.Now)
	if _, err := l.CreateSSOProvider(authContext(admin), core.CreateSSOProviderInput{Key: "kc", Kind: core.SSOKindOIDC,
		Issuer: idp.Issuer(), ClientID: idp.ClientID, ClientSecret: idp.ClientSecret, AutoCreate: true}); err != nil {
		t.Fatal(err)
	}
	f := ssoFixture{l: l, clk: clk, scope: scope, admin: admin, idp: idp}
	in := appBegin("kc")
	if _, err := l.BeginSSOLogin(loginContext(scope), in); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("the default redirect on a server that lists another = %v, want refused", err)
	}
	in.AppRedirect = "https://app.example/sso?x=1"
	idp.SetClaims(person("sub-1", "ada@example.com", true))
	begin, err := l.BeginSSOLogin(loginContext(scope), in)
	if err != nil {
		t.Fatal(err)
	}
	result, err := l.CompleteSSOLogin(loginContext(scope), core.CompleteSSOInput{Flow: begin.Flow,
		Code: idp.Grant(begin.AuthURL), State: begin.State})
	if err != nil || !strings.HasPrefix(result.AppRedirect, "https://app.example/sso?x=1&") {
		t.Fatalf("result = %+v, %v; want the listed address with the code added to its query", result, err)
	}
	code, _ := url.Parse(result.AppRedirect)
	if _, err := f.exchange(code.Query().Get("code"), appVerifier); err != nil {
		t.Errorf("exchange = %v", err)
	}
}

func TestAppSignInOff(t *testing.T) {
	l, _, scope, admin := newLocalWith(t, WithTOTPKey(totpTestKey), WithEgress(egress.Permissive()),
		WithSSOAppRedirects(nil))
	idp := ssotest.NewOIDC(t, time.Now)
	if _, err := l.CreateSSOProvider(authContext(admin), core.CreateSSOProviderInput{Key: "kc", Kind: core.SSOKindOIDC,
		Issuer: idp.Issuer(), ClientID: idp.ClientID, ClientSecret: idp.ClientSecret}); err != nil {
		t.Fatal(err)
	}
	opts, err := l.SignInOptions(loginContext(scope))
	if err != nil || opts.AppSignIn || len(opts.Providers) != 1 {
		t.Errorf("options = %+v, %v; want the provider and app_sign_in false", opts, err)
	}
	_, err = l.BeginSSOLogin(loginContext(scope), appBegin("kc"))
	if !core.IsKind(err, core.KindInvalid) || !strings.Contains(err.Error(), "turned off") {
		t.Errorf("an app sign-in on a server with none = %v, want refused as turned off", err)
	}
}

func TestSignInOptionsOffersAppSignInByDefault(t *testing.T) {
	f := newSSOFixture(t, nil)
	opts, err := f.l.SignInOptions(loginContext(f.scope))
	if err != nil || !opts.AppSignIn || opts.PasswordLogin != core.PasswordLoginAllowed {
		t.Errorf("options = %+v, %v; want app_sign_in true beside the password policy", opts, err)
	}
}

// refusedTo is the app redirect a refused app sign-in names, parsed.
func refusedTo(t *testing.T, err error) url.Values {
	t.Helper()
	var e *core.Error
	if !errors.As(err, &e) {
		t.Fatalf("refusal = %v, want a domain error", err)
	}
	target, _ := e.Details[core.DetailAppRedirect].(string)
	if !strings.HasPrefix(target, appRedirect+"?") {
		t.Fatalf("refusal %v names app redirect %q, want %s?...", err, target, appRedirect)
	}
	u, _ := url.Parse(target)
	if u.Query().Get("code") != "" {
		t.Fatalf("a refusal handed a code: %q", target)
	}
	return u.Query()
}

func TestAppSignInRefusalsGoBackToTheApp(t *testing.T) {
	f := newSSOFixture(t, nil)
	_, err := f.appSignInSSO(t, person("sub-1", "nobody@example.com", true))
	if q := refusedTo(t, err); q.Get("error") != core.AppSignInNotLinked || q.Get("state") != "opaque-1" {
		t.Errorf("an unknown person = %v, want not_linked with the app's state", q)
	}

	ctx := loginContext(f.scope)
	begin, err := f.l.BeginSSOLogin(ctx, appBegin("kc"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.l.CompleteSSOLogin(ctx, core.CompleteSSOInput{Flow: begin.Flow, State: begin.State,
		Error: "access_denied"})
	if q := refusedTo(t, err); q.Get("error") != core.AppSignInAccessDenied {
		t.Errorf("the provider ending it = %v, want access_denied", q)
	}
	_, err = f.l.CompleteSSOLogin(ctx, core.CompleteSSOInput{Flow: begin.Flow, State: "other", Code: "x"})
	if q := refusedTo(t, err); q.Get("error") != core.AppSignInExpired {
		t.Errorf("a foreign state = %v, want expired", q)
	}
	f.clk.Advance(auth.SSOFlowTTL + time.Minute)
	_, err = f.l.CompleteSSOLogin(ctx, core.CompleteSSOInput{Flow: begin.Flow, State: begin.State, Code: "x"})
	if q := refusedTo(t, err); q.Get("error") != core.AppSignInExpired {
		t.Errorf("an expired flow = %v, want expired", q)
	}

	f.idp.SetTweak(ssotest.Tweak{ForeignKey: true})
	_, err = f.appSignInSSO(t, person("sub-1", "nobody@example.com", true))
	if q := refusedTo(t, err); q.Get("error") != core.AppSignInProviderError {
		t.Errorf("a bad token = %v, want provider_error", q)
	}
}

func TestAppSignInAsksForTheSecondFactorInTheBrowser(t *testing.T) {
	f := newSSOFixture(t, func(in *core.CreateSSOProviderInput) { in.AutoCreate = true })
	// Only the cli surface requires it: the app's session is the one the
	// app's password sign-in gets, so the cli requirement applies.
	requireTOTP(t, f.l, f.scope, core.SurfaceCLI)
	result, err := f.appSignInSSO(t, person("sub-1", "ada@example.com", true))
	if err != nil {
		t.Fatal(err)
	}
	if result.Challenge == nil || result.Challenge.Enrolment == nil || result.AppRedirect != "" {
		t.Fatalf("result = %+v, want an enrolment challenge and no code yet", result)
	}
	ctx := loginContext(f.scope)
	done, err := f.l.Login(ctx, core.LoginInput{Surface: core.SurfaceWeb, Ticket: result.Challenge.Ticket,
		Code: codeFor(t, result.Challenge.Enrolment.Secret, f.clk)})
	if err != nil {
		t.Fatalf("second factor step: %v", err)
	}
	if len(done.RecoveryCodes) == 0 {
		t.Error("the enrolment issued no recovery codes to show in the browser")
	}
	code := handedCode(t, done)
	out, err := f.exchange(code, appVerifier)
	if err != nil || out.Session == nil {
		t.Fatalf("exchange = %+v, %v", out, err)
	}
	sessions, _, err := f.l.ListSessions(authContext(&core.Actor{ID: out.Session.ActorID, TenantID: f.scope.TenantID,
		Kind: core.ActorUser, Role: core.RoleMember}), core.Page{Limit: 10})
	if err != nil || len(sessions) != 1 || sessions[0].Method != "sso:kc" || !sessions[0].SecondFactor {
		t.Errorf("sessions = %+v, %v; want one sso:kc session that passed a second factor", sessions, err)
	}
}

func TestAppSignInThroughSAML(t *testing.T) {
	f := newSAMLFixture(t, nil)
	ctx := loginContext(f.scope)
	begin, err := f.l.BeginSSOLogin(ctx, appBegin("okta"))
	if err != nil {
		t.Fatalf("BeginSSOLogin: %v", err)
	}
	req := ssotest.ReadRequest(t, begin.AuthURL)
	response := f.idp.Response(t, req, ssotest.Assertion{NameID: "00u9",
		Attributes: map[string][]string{"email": {"newbie@acme.test"}}}, ssotest.SignAssertion)
	result, err := f.post(begin, req.RelayState, response)
	if err != nil {
		t.Fatalf("ACS: %v", err)
	}
	code := handedCode(t, result)
	sf := ssoFixture{l: f.l, clk: f.clk, scope: f.scope, admin: f.admin}
	out, err := sf.exchange(code, appVerifier)
	if err != nil || out.Session == nil {
		t.Fatalf("exchange = %+v, %v", out, err)
	}
	_, err = f.post(begin, req.RelayState, response)
	if q := refusedTo(t, err); q.Get("error") != core.AppSignInExpired {
		t.Errorf("the response posted again = %v, want expired", q)
	}
}

func TestExchangeRefusesAPersonDisabledSinceTheHandoff(t *testing.T) {
	f := newSSOFixture(t, func(in *core.CreateSSOProviderInput) { in.AutoCreate = true })
	result, err := f.appSignInSSO(t, person("sub-1", "ada@example.com", true))
	if err != nil {
		t.Fatal(err)
	}
	code := handedCode(t, result)
	off := false
	if _, err := f.l.UpdateSSOProvider(authContext(f.admin), "kc", core.UpdateSSOProviderInput{Enabled: &off}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.exchange(code, appVerifier); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("exchange after the provider was switched off = %v, want invalid credentials", err)
	}
}
