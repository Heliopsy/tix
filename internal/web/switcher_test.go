// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/server"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/sqlite"
	"github.com/heliopsy/tix/internal/store/sqlite/sqlitetest"
	"github.com/heliopsy/tix/internal/web"
)

const switchPassword = "switch-password-long-enough"

// switchWorld is the browser interface behind the real API middleware, so a
// sign-in mints a real session and the tenant is resolved from the host and
// the cookie as in production. pat belongs to home, test and away, which has
// an address of its own; solo belongs to home alone.
type switchWorld struct {
	t      *testing.T
	srv    *httptest.Server
	local  *service.Local
	store  *sqlite.Store
	clk    *clock.Fake
	home   core.Tenant
	test   core.Tenant
	patIDs map[string]string
}

func newSwitchWorld(t *testing.T) *switchWorld {
	t.Helper()
	ctx := context.Background()
	clk := clock.NewFake(time.Now())
	st, err := sqlite.Open(sqlitetest.Path(t), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	local := service.New(st, service.WithClock(clk), service.WithHooks(service.HookOff),
		service.WithHasher(auth.NewHasherWithParams(auth.TestParams())), service.WithTOTPKey(testTOTPKey),
		service.WithoutStarterProjects())
	w := &switchWorld{t: t, local: local, store: st, clk: clk, patIDs: map[string]string{}}
	w.home = seedTenant(t, st, "home", "Home")
	w.test = seedTenant(t, st, "test", "Test")
	away := seedTenant(t, st, "away", "Away")
	if err := st.Update(ctx, core.TenantScope{TenantID: away.ID}, func(tx store.Tx) error {
		return tx.AddDomain(ctx, &core.Domain{Hostname: "away.example.test"})
	}); err != nil {
		t.Fatal(err)
	}
	shell := core.WithDirectStore(core.WithActor(ctx, core.SystemActor(w.home.ID)))
	for _, p := range []struct{ email, tenant string }{{"pat@example.test", "home"}, {"solo@example.test", "home"}} {
		u, err := local.CreateTenantUser(shell, p.tenant, core.CreateUserInput{Email: p.email, Password: switchPassword, Role: core.RoleAdmin})
		if err != nil {
			t.Fatalf("creating %s: %v", p.email, err)
		}
		if p.email == "pat@example.test" {
			w.patIDs["home"] = u.ID
		}
	}
	for _, key := range []string{"test", "away"} {
		u, err := local.AddTenantMember(shell, key, core.AddTenantMemberInput{Email: "pat@example.test", Role: core.RoleMember})
		if err != nil {
			t.Fatalf("adding pat to %s: %v", key, err)
		}
		w.patIDs[key] = u.ID
	}
	router, err := httpapi.New(httpapi.Config{
		Service: local, Authenticator: server.NewAuthenticator(st, clk), DefaultTenantID: w.home.ID,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), WebHandler: web.Handler(local),
	})
	if err != nil {
		t.Fatal(err)
	}
	w.srv = httptest.NewServer(router)
	t.Cleanup(w.srv.Close)
	return w
}

// visitor is one browser: its own cookie jar, and no redirect followed.
type visitor struct {
	w      *switchWorld
	client *http.Client
}

func (w *switchWorld) visitor() *visitor {
	jar, _ := cookiejar.New(nil)
	return &visitor{w: w, client: &http.Client{Jar: jar, Transport: w.srv.Client().Transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (v *visitor) do(method, path string, form url.Values) (*http.Response, string) {
	v.w.t.Helper()
	var body io.Reader
	if form != nil {
		form.Set("csrf_token", v.cookie(web.CSRFCookieName))
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequest(method, v.w.srv.URL+path, body)
	if err != nil {
		v.w.t.Fatal(err)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := v.client.Do(req)
	if err != nil {
		v.w.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	return resp, string(raw)
}

func (v *visitor) cookie(name string) string {
	u, _ := url.Parse(v.w.srv.URL)
	for _, c := range v.client.Jar.Cookies(u) {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}

// signIn signs in through the sign-in form and answers where it went.
func (v *visitor) signIn(email string) string {
	v.w.t.Helper()
	v.do(http.MethodGet, web.RouteLogin, nil)
	resp, page := v.do(http.MethodPost, web.RouteLogin, url.Values{"email": {email}, "password": {switchPassword}})
	if resp.StatusCode != http.StatusSeeOther {
		v.w.t.Fatalf("signing in %s = %d\n%s", email, resp.StatusCode, page)
	}
	return resp.Header.Get("Location")
}

// switcherRowOf is the switcher row naming one tenant key.
func switcherRowOf(t *testing.T, page, key string) string {
	t.Helper()
	for _, row := range strings.Split(page, `<li class="tenantswitch-row">`)[1:] {
		row, _, _ = strings.Cut(row, "</li>")
		if strings.Contains(row, `<span class="mono muted">`+key+`</span>`) {
			return row
		}
	}
	t.Fatalf("no switcher row for %q in\n%s", key, page)
	return ""
}

func TestSignInOnASharedHostOffersTheReadersTenants(t *testing.T) {
	w := newSwitchWorld(t)
	pat := w.visitor()
	where := pat.signIn("pat@example.test")
	if !strings.HasPrefix(where, web.RouteSwitch+"?") {
		t.Fatalf("pat's sign-in went to %q, want Choose a tenant", where)
	}
	_, page := pat.do(http.MethodGet, where, nil)
	home, test, away := switcherRowOf(t, page, "home"), switcherRowOf(t, page, "test"), switcherRowOf(t, page, "away")
	if !strings.Contains(home, "aria-current") || strings.Contains(home, "<form") {
		t.Errorf("the current tenant's row = %s, want it marked and with no Switch", home)
	}
	if !strings.Contains(test, `action="/switch"`) || !strings.Contains(test, `name="to" value="test"`) {
		t.Errorf("test's row = %s, want a Switch form posting tenant test", test)
	}
	if !strings.Contains(away, `href="http://away.example.test/login"`) || strings.Contains(away, "<form") {
		t.Errorf("away's row = %s, want a link to its own sign-in page and no Switch", away)
	}

	solo := w.visitor()
	if where := solo.signIn("solo@example.test"); where != web.RouteProjects {
		t.Errorf("a sign-in with one tenant went to %q, want straight to projects", where)
	}
	if _, page := solo.do(http.MethodGet, web.RouteProjects, nil); strings.Contains(page, "nav-tenant") ||
		strings.Contains(page, `id="tenants"`) {
		t.Error("a reader in one tenant is offered a tenant switcher")
	}
}

func TestTheHeaderSwitcherWorksWithoutScripting(t *testing.T) {
	w := newSwitchWorld(t)
	pat := w.visitor()
	pat.signIn("pat@example.test")

	_, page := pat.do(http.MethodGet, web.RouteProjects, nil)
	open := between(t, page, `class="nav-tenant"`, "</a>")
	if !strings.Contains(page, `href="/projects?dialog=tenants#tenants" class="nav-tenant"`) || !strings.Contains(open, "Home") {
		t.Fatalf("the sidebar's tenant entry = %q, want Home linking to ?dialog=tenants", open)
	}
	dialog := between(t, page, `<dialog id="tenants"`, "</dialog>")
	if strings.Contains(dialog, "<details") || strings.Contains(between(t, page, `<dialog id="tenants"`, ">"), " open") {
		t.Errorf("the switcher dialog = %q, want closed and with no disclosure", dialog)
	}
	_, page = pat.do(http.MethodGet, web.RouteProjects+"?dialog=tenants", nil)
	if head := between(t, page, `<dialog id="tenants"`, ">"); !strings.Contains(head, "open") {
		t.Errorf("?dialog=tenants drew the dialog as %q, want it open in place", head)
	}
	if !strings.Contains(switcherRowOf(t, page, "test"), `action="/switch"`) {
		t.Error("the in-place switcher has no Switch form for test")
	}
}

func TestASwitchReplacesTheSessionCookie(t *testing.T) {
	w := newSwitchWorld(t)
	pat := w.visitor()
	pat.signIn("pat@example.test")
	before := pat.cookie(auth.SessionCookieName)

	resp, page := pat.do(http.MethodPost, web.RouteSwitch, url.Values{"to": {"test"}})
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != web.RouteProjects {
		t.Fatalf("switch = %d to %q\n%s", resp.StatusCode, resp.Header.Get("Location"), page)
	}
	after := pat.cookie(auth.SessionCookieName)
	if after == "" || after == before {
		t.Fatal("the switch left the session cookie as it was")
	}
	if _, page := pat.do(http.MethodGet, web.RouteProjects, nil); !strings.Contains(between(t, page, `class="nav-tenant"`, "</a>"), "Test") {
		t.Error("after the switch the sidebar does not name Test as the current tenant")
	}

	old := w.visitor()
	u, _ := url.Parse(w.srv.URL)
	old.client.Jar.SetCookies(u, []*http.Cookie{{Name: auth.SessionCookieName, Value: before}})
	if resp, _ := old.do(http.MethodGet, web.RouteProjects, nil); resp.StatusCode != http.StatusSeeOther ||
		!strings.HasPrefix(resp.Header.Get("Location"), web.RouteLogin) {
		t.Errorf("the old session after a switch = %d to %q, want sent to sign in", resp.StatusCode, resp.Header.Get("Location"))
	}

	if resp, _ := pat.do(http.MethodPost, web.RouteSwitch, url.Values{"to": {"away"}}); resp.StatusCode == http.StatusSeeOther {
		t.Error("a switch to a tenant served at its own address went through")
	}
}

func TestASwitchAsksForACodeOnTheWeb(t *testing.T) {
	w := newSwitchWorld(t)
	pat := w.visitor()
	pat.signIn("pat@example.test")

	actor := &core.Actor{ID: w.patIDs["home"], TenantID: w.home.ID, Kind: core.ActorUser, Role: core.RoleAdmin}
	ctx := core.WithActor(context.Background(), actor)
	enrolment, err := w.local.BeginTOTPEnrolment(ctx)
	if err != nil {
		t.Fatal(err)
	}
	code := func() string {
		c, err := auth.TOTPCode(enrolment.Secret, auth.TOTPStep(w.clk.Now()))
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	if _, err := w.local.ConfirmTOTPEnrolment(ctx, code()); err != nil {
		t.Fatal(err)
	}
	if err := w.store.Unscoped(context.Background(), func(u store.UnscopedTx) error {
		w.test.RequireTOTPWeb = true
		return u.UpdateTenant(context.Background(), &w.test)
	}); err != nil {
		t.Fatal(err)
	}

	resp, page := pat.do(http.MethodPost, web.RouteSwitch, url.Values{"to": {"test"}})
	form := between(t, page, `<form method="post" action="/switch"`, "</form>")
	if resp.StatusCode != http.StatusOK || !strings.Contains(form, `name="code"`) || !strings.Contains(form, `value="test"`) {
		t.Fatalf("switch to a tenant asking for a code = %d, form %q", resp.StatusCode, form)
	}
	if resp, page := pat.do(http.MethodPost, web.RouteSwitch, url.Values{"to": {"test"}, "code": {"000000"}}); resp.StatusCode == http.StatusSeeOther ||
		!strings.Contains(page, `id="code-error"`) {
		t.Errorf("a wrong code = %d, want the code step again with its refusal", resp.StatusCode)
	}
	w.clk.Advance(auth.TOTPPeriod)
	if resp, _ := pat.do(http.MethodPost, web.RouteSwitch, url.Values{"to": {"test"}, "code": {code()}}); resp.StatusCode != http.StatusSeeOther {
		t.Errorf("the right code = %d, want the switch done", resp.StatusCode)
	}
}
