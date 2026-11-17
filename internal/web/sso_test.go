// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/sso/ssotest"
)

// addProvider creates provider kc against a fake identity provider through
// the Single sign-on screen.
func addProvider(t *testing.T, f *fixture) *ssotest.OIDC {
	t.Helper()
	idp := ssotest.NewOIDC(t, f.clock.Now)
	resp := f.as("alice").post("/admin/sso", url.Values{
		"key": {"kc"}, "kind": {"oidc"}, "display_name": {"Keycloak"}, "issuer": {idp.Issuer()},
		"client_id": {idp.ClientID}, "client_secret": {idp.ClientSecret}, "default_role": {"member"},
		"enabled": {"1"}, "auto_create": {"1"},
	})
	wantStatus(t, resp, http.StatusSeeOther)
	return idp
}

func TestSSOScreenListsAProviderWithoutItsSecret(t *testing.T) {
	f := newFixture(t)
	idp := addProvider(t, f)
	page := f.as("alice").page("/admin/sso")
	if !strings.Contains(page, "Keycloak") || !strings.Contains(page, idp.Issuer()) {
		t.Fatalf("the screen does not list the provider:\n%s", page)
	}
	if strings.Contains(page, idp.ClientSecret) {
		t.Error("the screen shows the client secret")
	}
	if !strings.Contains(page, "/login/sso/callback") {
		t.Error("the screen does not name the redirect URL to register")
	}
}

func TestSignInWithAProviderInTheBrowser(t *testing.T) {
	f := newFixture(t)
	idp := addProvider(t, f)
	idp.SetClaims(map[string]any{"sub": "n-1", "email": "newbie@acme.test", "email_verified": true})
	visitor := f.as("")
	login := visitor.page("/login")
	if !strings.Contains(login, `formaction="/login/sso/kc"`) || !strings.Contains(login, "Sign in with Keycloak") {
		t.Fatalf("the sign-in screen offers no Sign in with Keycloak button:\n%s", login)
	}

	start := visitor.get("/login/sso/kc?next=%2Ftasks")
	wantStatus(t, start, http.StatusSeeOther)
	if !strings.Contains(strings.Join(start.Header.Values("Set-Cookie"), "\n"), "tix_sso_flow=") {
		t.Fatal("starting a sign-in set no flow cookie")
	}
	// The provider signs the person in and sends the browser back.
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
	done := visitor.get(back.RequestURI())
	wantStatus(t, done, http.StatusSeeOther)
	if got := done.Header.Get("Location"); got != "/tasks" {
		t.Errorf("signed in and sent to %q, want /tasks", got)
	}
	if visitor.cookie("tix_session") == "" {
		t.Error("the callback set no session cookie")
	}

	replay := f.as("").get(back.RequestURI())
	if replay.StatusCode != http.StatusUnauthorized {
		t.Errorf("a callback in a browser that did not start the sign-in = %d, want 401", replay.StatusCode)
	}
}

func TestPasswordFormGoesWhenPasswordsAreOff(t *testing.T) {
	f := newFixture(t)
	addProvider(t, f)
	wantStatus(t, f.as("alice").post("/admin/sso/password", url.Values{"password_login": {"disabled"}}), http.StatusSeeOther)
	login := f.as("").page("/login")
	if strings.Contains(login, `name="password"`) {
		t.Error("the sign-in screen still draws a password field")
	}
	if !strings.Contains(f.as("").page("/login?password=1"), `name="password"`) {
		t.Error("the break-glass address draws no password field")
	}
}
