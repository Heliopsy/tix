// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"context"
	"html"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/sso/ssotest"
)

const appVerifier = "web-app-verifier-0123456789-0123456789-0123456789"

// appStart is the address the app opens in the system browser.
func appStart(key, redirect string) string {
	return "/login/sso/" + key + "?" + url.Values{"app_redirect": {redirect},
		"code_challenge": {auth.PKCEChallenge(appVerifier)}, "code_challenge_method": {"S256"},
		"app_state": {"st-9"}}.Encode()
}

// atProvider follows the redirect to the fake OIDC provider and answers the
// path of the callback it sends the browser back to.
func atProvider(t *testing.T, start *http.Response) string {
	t.Helper()
	plain := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	atIdP, err := plain.Get(start.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	_ = atIdP.Body.Close()
	back, err := url.Parse(atIdP.Header.Get("Location"))
	if err != nil || back.Path != "/login/sso/callback" {
		t.Fatalf("the provider sent the browser to %q, want the callback", atIdP.Header.Get("Location"))
	}
	return back.RequestURI()
}

// appCode reads the code out of an app redirect, checking the state came back.
func appCode(t *testing.T, target string) string {
	t.Helper()
	u, err := url.Parse(target)
	if err != nil || u.Scheme != "red.tix.mobile" || u.Query().Get("state") != "st-9" || u.Query().Get("code") == "" {
		t.Fatalf("handed to %q, want red.tix.mobile:/sso with a code and state st-9", target)
	}
	return u.Query().Get("code")
}

func (f *fixture) exchange(t *testing.T, code string) *core.LoginResult {
	t.Helper()
	ctx := core.WithTenant(context.Background(), core.TenantScope{TenantID: f.tenantA.ID})
	out, err := f.svc.ExchangeSSOCode(ctx, core.SSOExchangeInput{Code: code, CodeVerifier: appVerifier})
	if err != nil || out.Session == nil {
		t.Fatalf("exchange = %+v, %v; want a session", out, err)
	}
	return out
}

func TestAnAppSignsInThroughTheBrowserWithOIDC(t *testing.T) {
	f := newFixture(t)
	idp := addProvider(t, f)
	idp.SetClaims(map[string]any{"sub": "n-1", "email": "newbie@acme.test", "email_verified": true})
	visitor := f.as("")
	start := visitor.get(appStart("kc", "red.tix.mobile:/sso"))
	wantStatus(t, start, http.StatusSeeOther)
	done := visitor.get(atProvider(t, start))
	wantStatus(t, done, http.StatusFound)
	code := appCode(t, done.Header.Get("Location"))
	if visitor.cookie("tix_session") != "" {
		t.Error("an app sign-in set a browser session cookie")
	}
	if done.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("the handoff answered Cache-Control %q, want no-store", done.Header.Get("Cache-Control"))
	}
	if out := f.exchange(t, code); !out.Session.Remembered {
		t.Error("the app's session is not remembered")
	}
}

func TestAnAppSignInRefusedAtTheProviderGoesBackToTheApp(t *testing.T) {
	f := newFixture(t)
	idp := addProvider(t, f)
	idp.SetClaims(map[string]any{"sub": "n-1", "email": "newbie@acme.test", "email_verified": false})
	visitor := f.as("")
	start := visitor.get(appStart("kc", "red.tix.mobile:/sso"))
	done := visitor.get(atProvider(t, start))
	wantStatus(t, done, http.StatusFound)
	u, err := url.Parse(done.Header.Get("Location"))
	if err != nil || u.Scheme != "red.tix.mobile" || u.Query().Get("error") != "not_linked" ||
		u.Query().Get("state") != "st-9" || u.Query().Get("code") != "" {
		t.Errorf("a refused app sign-in went to %q, want red.tix.mobile:/sso?error=not_linked&state=st-9",
			done.Header.Get("Location"))
	}
}

func TestAnAppRedirectOffTheListGetsTheErrorScreen(t *testing.T) {
	f := newFixture(t)
	addProvider(t, f)
	for _, redirect := range []string{"evil.app:/sso", "red.tix.mobile:/sso/more", "https://evil.example/sso"} {
		start := f.as("").get(appStart("kc", redirect))
		body := readAll(t, start)
		if start.StatusCode != http.StatusBadRequest || start.Header.Get("Location") != "" ||
			!strings.Contains(html.UnescapeString(body), "auth.sso_app_redirects") {
			t.Errorf("start with %q = %d to %q, want the 400 error screen naming the setting",
				redirect, start.StatusCode, start.Header.Get("Location"))
		}
	}
	noChallenge := f.as("").get("/login/sso/kc?app_redirect=" + url.QueryEscape("red.tix.mobile:/sso"))
	_ = noChallenge.Body.Close()
	if noChallenge.StatusCode != http.StatusBadRequest || noChallenge.Header.Get("Location") != "" {
		t.Errorf("an app start without a challenge = %d to %q, want the 400 error screen",
			noChallenge.StatusCode, noChallenge.Header.Get("Location"))
	}
}

func TestAnAppSignsInThroughTheBrowserWithSAML(t *testing.T) {
	f := newFixture(t)
	idp := addSAMLProvider(t, f)
	visitor := f.as("")
	start := visitor.get(appStart("okta", "red.tix.mobile:/sso"))
	wantStatus(t, start, http.StatusSeeOther)
	req := ssotest.ReadRequest(t, start.Header.Get("Location"))
	response := idp.Response(t, req, ssotest.Assertion{NameID: "00u9",
		Attributes: map[string][]string{"email": {"newbie@acme.test"}}}, ssotest.SignAssertion)
	done := visitor.postRaw("/login/sso/saml/okta/acs", url.Values{"SAMLResponse": {response},
		"RelayState": {req.RelayState}})
	body := readAll(t, done)
	wantStatus(t, done, http.StatusOK)
	link := html.UnescapeString(between(t, body, `<a class="button" href="`, `"`))
	refresh := html.UnescapeString(between(t, body, `<meta http-equiv="refresh" content="0;url=`, `"`))
	if refresh != link {
		t.Errorf("the page navigates to %q but links %q", refresh, link)
	}
	code := appCode(t, link)
	if visitor.cookie("tix_session") != "" {
		t.Error("an app sign-in through SAML set a browser session cookie")
	}
	f.exchange(t, code)
}

// appEnrolment runs an app's sign-in under a second factor requirement to
// the code step and answers the browser and the enrolment form's fields.
func appEnrolment(t *testing.T, f *fixture) (*browser, url.Values) {
	t.Helper()
	f.requireTOTPWeb()
	idp := addProvider(t, f)
	idp.SetClaims(map[string]any{"sub": "n-1", "email": "newbie@acme.test", "email_verified": true})
	visitor := f.as("")
	start := visitor.get(appStart("kc", "red.tix.mobile:/sso"))
	challenge := visitor.get(atProvider(t, start))
	page := readAll(t, challenge)
	wantStatus(t, challenge, http.StatusOK)
	form := totpForm(t, page, "/login")
	ticket := html.UnescapeString(between(t, form, `name="ticket" value="`, `"`))
	uri := html.UnescapeString(between(t, form, `name="uri" value="`, `"`))
	secret, _ := url.Parse(uri)
	code, err := auth.TOTPCode(secret.Query().Get("secret"), auth.TOTPStep(f.clock.Now()))
	if err != nil || ticket == "" {
		t.Fatalf("the second factor step carries no ticket or enrolment: %v\n%s", err, form)
	}
	return visitor, url.Values{"ticket": {ticket}, "code": {code}, "uri": {uri}}
}

func TestAnAppSignInAsksTheSecondFactorInTheBrowser(t *testing.T) {
	f := newFixture(t)
	visitor, fields := appEnrolment(t, f)
	done := visitor.post("/login", fields)
	body := readAll(t, done)
	wantStatus(t, done, http.StatusOK)
	if visitor.cookie("tix_session") != "" {
		t.Error("the second factor step of an app sign-in set a browser session cookie")
	}
	if !strings.Contains(body, `id="recovery-codes"`) {
		t.Error("the page does not show the recovery codes the enrolment issued")
	}
	if strings.Contains(body, `http-equiv="refresh"`) {
		t.Error("the page navigates away before the reader has kept the recovery codes")
	}
	if strings.Contains(body, "red.tix.mobile:") {
		t.Error("the recovery codes page already carries the app's address, so its code expires while the codes are read")
	}
	open := formAt(t, body, "/login/sso/app")
	handoff := html.UnescapeString(between(t, open, `name="handoff" value="`, `"`))
	if !strings.Contains(open, `name="csrf_token"`) || !strings.Contains(open, "Open the app") {
		t.Fatalf("Open the app is not a CSRF protected form:\n%s", open)
	}
	// Keeping the codes takes longer than a handed code lives.
	f.clock.Advance(auth.SSOHandoffTTL + time.Minute)
	if refused := visitor.postRaw("/login/sso/app", url.Values{"handoff": {handoff}}); refused.StatusCode != http.StatusForbidden {
		t.Errorf("Open the app without the CSRF value = %d, want 403", refused.StatusCode)
	}
	opened := visitor.post("/login/sso/app", url.Values{"handoff": {handoff}})
	_ = opened.Body.Close()
	wantStatus(t, opened, http.StatusFound)
	if opened.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("Open the app answered Cache-Control %q, want no-store", opened.Header.Get("Cache-Control"))
	}
	if out := f.exchange(t, appCode(t, opened.Header.Get("Location"))); out.Session == nil {
		t.Fatal("no session")
	}
	if visitor.cookie("tix_session") != "" {
		t.Error("Open the app set a browser session cookie")
	}
}

func TestAnExpiredAppSecondFactorStepGoesBackToTheApp(t *testing.T) {
	f := newFixture(t)
	visitor, fields := appEnrolment(t, f)
	f.clock.Advance(auth.TicketTTL + time.Second)
	done := visitor.post("/login", fields)
	body := readAll(t, done)
	wantStatus(t, done, http.StatusOK)
	target, err := url.Parse(html.UnescapeString(between(t, body, `<meta http-equiv="refresh" content="0;url=`, `"`)))
	if err != nil || target.Scheme != "red.tix.mobile" || target.Query().Get("error") != "expired" ||
		target.Query().Get("state") != "st-9" {
		t.Errorf("an expired second factor step navigates to %v, want the app with error=expired", target)
	}
}

func TestAWrongCodeInAnAppSignInStaysOnTheForm(t *testing.T) {
	f := newFixture(t)
	visitor, fields := appEnrolment(t, f)
	fields.Set("code", "000000")
	done := visitor.post("/login", fields)
	body := readAll(t, done)
	if done.StatusCode != http.StatusUnauthorized || strings.Contains(body, "red.tix.mobile:") ||
		!strings.Contains(body, `name="ticket"`) {
		t.Errorf("a wrong code = %d, want the code form again without the app's address\n%s", done.StatusCode, body)
	}
}
