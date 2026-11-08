// SPDX-License-Identifier: AGPL-3.0-or-later

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/wire"
)

// Hosts the shared host scenarios arrive on. homeHost is claimed by the
// harness's own tenant, testHost by tenant test; the loopback address the
// server listens on is claimed by nobody.
const (
	homeHost = "home.example.test"
	testHost = "test.example.test"
	sharedPW = "shared-host-password-1"
)

// sharedHostWorld is the harness with a second tenant, test, and three
// people: pat in both tenants, olly in test alone, and hal in home alone.
type sharedHostWorld struct {
	h        *matrixHarness
	testID   string
	homeProj string
	testProj string
}

func newSharedHostWorld(t *testing.T) *sharedHostWorld {
	t.Helper()
	h := newMatrixHarness(t)
	ctx := h.adminCtx
	olly := ""
	tenant, err := h.local.CreateTenant(ctx, core.CreateTenantInput{Key: "test", Name: "Test"})
	if err != nil {
		t.Fatalf("creating tenant test: %v", err)
	}
	for _, p := range []struct {
		email  string
		tenant string
	}{{"pat@example.test", "default"}, {"hal@example.test", "default"}, {"olly@example.test", "test"}} {
		u, err := h.local.CreateTenantUser(ctx, p.tenant, core.CreateUserInput{
			Email: p.email, Password: sharedPW, Role: core.RoleAdmin,
		})
		if err != nil {
			t.Fatalf("creating %s in %s: %v", p.email, p.tenant, err)
		}
		if p.tenant == "test" {
			olly = u.ID
		}
	}
	if _, err := h.local.AddTenantMember(ctx, "test", core.AddTenantMemberInput{
		Email: "pat@example.test", Role: core.RoleMember,
	}); err != nil {
		t.Fatalf("adding pat to test: %v", err)
	}
	if _, err := h.local.AddDomain(ctx, core.AddDomainInput{Hostname: homeHost}); err != nil {
		t.Fatalf("claiming %s: %v", homeHost, err)
	}
	w := &sharedHostWorld{h: h, testID: tenant.ID}
	w.homeProj = w.project(t, ctx, "homep")
	w.testProj = w.project(t, core.WithActor(context.Background(), &core.Actor{
		ID: olly, TenantID: tenant.ID, Kind: core.ActorUser, Role: core.RoleAdmin,
	}), "testp")
	return w
}

// project creates a project as the actor in ctx, so each tenant holds a row
// the other must never see.
func (w *sharedHostWorld) project(t *testing.T, ctx context.Context, key string) string {
	t.Helper()
	p, err := w.h.local.CreateProject(ctx, core.CreateProjectInput{Key: key, Name: strings.ToUpper(key)})
	if err != nil {
		t.Fatalf("creating project %s: %v", key, err)
	}
	return p.Key
}

// setShared marks the home tenant's domain shared or not, as a global admin
// would.
func (w *sharedHostWorld) setShared(t *testing.T, shared bool) {
	t.Helper()
	st := openStore(t, w.h.dbPath, w.h.clk)
	if err := st.Update(context.Background(), core.TenantScope{TenantID: w.h.tenantID}, func(tx store.Tx) error {
		return tx.SetDomainShared(context.Background(), homeHost, shared)
	}); err != nil {
		t.Fatalf("marking %s shared=%v: %v", homeHost, shared, err)
	}
}

// claimTest gives tenant test an address of its own.
func (w *sharedHostWorld) claimTest(t *testing.T) {
	t.Helper()
	st := openStore(t, w.h.dbPath, w.h.clk)
	if err := st.Update(context.Background(), core.TenantScope{TenantID: w.testID}, func(tx store.Tx) error {
		return tx.AddDomain(context.Background(), &core.Domain{Hostname: testHost})
	}); err != nil {
		t.Fatalf("claiming %s: %v", testHost, err)
	}
}

// request is one HTTP call on a host, with an optional tenant header and
// session cookie.
type request struct {
	method, path, host, tenant, cookie string
	body                               any
}

// answer is a response's status and body.
type answer struct {
	status int
	body   []byte
}

func (w *sharedHostWorld) do(t *testing.T, r request) answer {
	t.Helper()
	var body io.Reader
	if r.body != nil {
		raw, err := json.Marshal(r.body)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(r.method, w.h.baseURL+r.path, body)
	if err != nil {
		t.Fatal(err)
	}
	if r.host != "" {
		req.Host = r.host
	}
	if r.tenant != "" {
		req.Header.Set(core.HeaderTenant, r.tenant)
	}
	if r.cookie != "" {
		req.AddCookie(&http.Cookie{Name: wire.SessionCookieName, Value: r.cookie})
	}
	if r.body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	return answer{status: resp.StatusCode, body: raw}
}

// login signs in over the API and answers the session's token and tenant.
func (w *sharedHostWorld) login(t *testing.T, host, tenant, email string) (string, string, answer) {
	t.Helper()
	got := w.do(t, request{method: http.MethodPost, path: wire.RouteLogin, host: host, tenant: tenant,
		body: map[string]string{"email": email, "password": sharedPW, "surface": "web"}})
	var out core.LoginResult
	if got.status != http.StatusOK || json.Unmarshal(got.body, &out) != nil || out.Session == nil {
		return "", "", got
	}
	return out.Session.Token, out.Session.TenantID, got
}

// whoami answers the tenant key a session is served as on a host.
func (w *sharedHostWorld) whoami(t *testing.T, host, tenant, cookie string) (core.Actor, int) {
	t.Helper()
	got := w.do(t, request{method: http.MethodGet, path: wire.RouteWhoAmI, host: host, tenant: tenant, cookie: cookie})
	var me core.Actor
	_ = json.Unmarshal(got.body, &me)
	return me, got.status
}

// projectKeys lists the project keys a session reads on a host.
func (w *sharedHostWorld) projectKeys(t *testing.T, host, cookie string) (string, int) {
	t.Helper()
	got := w.do(t, request{method: http.MethodGet, path: wire.RouteProjects, host: host, cookie: cookie})
	return string(got.body), got.status
}

func TestABoundHostServesItsOwnTenantAlone(t *testing.T) {
	w := newSharedHostWorld(t)

	if _, _, got := w.login(t, homeHost, "test", "olly@example.test"); got.status != http.StatusForbidden {
		t.Errorf("sign-in on a bound host naming another tenant = %d %s, want 403", got.status, got.body)
	}
	if _, _, got := w.login(t, homeHost, "", "olly@example.test"); got.status != http.StatusUnauthorized {
		t.Errorf("sign-in on a bound host by a member of another tenant only = %d, want 401", got.status)
	}
	testSession, tenant, got := w.login(t, "127.0.0.1", "test", "olly@example.test")
	if testSession == "" || tenant != w.testID {
		t.Fatalf("sign-in on the unclaimed host naming test = %d %s, want a session in test", got.status, got.body)
	}
	if _, status := w.whoami(t, homeHost, "", testSession); status != http.StatusUnauthorized {
		t.Errorf("test's session on home's bound host = %d, want 401", status)
	}
	homeSession, _, _ := w.login(t, homeHost, "", "hal@example.test")
	if me, status := w.whoami(t, homeHost, "", homeSession); status != http.StatusOK || me.TenantKey != "default" {
		t.Errorf("home's session on its own host = %d %+v, want served as default", status, me)
	}
	if _, status := w.whoami(t, homeHost, "test", homeSession); status != http.StatusForbidden {
		t.Errorf("a header naming test on home's bound host = %d, want 403", status)
	}
}

func TestASharedHostServesTheTenantTheCredentialOrHeaderNames(t *testing.T) {
	w := newSharedHostWorld(t)
	w.setShared(t, true)

	for _, host := range []string{homeHost, "127.0.0.1"} {
		t.Run(host, func(t *testing.T) {
			testSession, tenant, got := w.login(t, host, "", "olly@example.test")
			if testSession == "" || tenant != w.testID {
				t.Fatalf("account-first sign-in by a member of test only = %d %s, want a session in test", got.status, got.body)
			}
			if me, status := w.whoami(t, host, "", testSession); status != http.StatusOK || me.TenantKey != "test" {
				t.Errorf("test's session with no header = %d %+v, want served as test", status, me)
			}
			keys, status := w.projectKeys(t, host, testSession)
			if status != http.StatusOK || !strings.Contains(keys, w.testProj) || strings.Contains(keys, w.homeProj) {
				t.Errorf("test's session reads projects %d %s, want test's and never home's", status, keys)
			}

			homeSession, tenant, _ := w.login(t, host, "", "hal@example.test")
			if tenant != w.h.tenantID {
				t.Fatalf("hal signed in to %q, want the host's own tenant", tenant)
			}
			keys, _ = w.projectKeys(t, host, homeSession)
			if !strings.Contains(keys, w.homeProj) || strings.Contains(keys, w.testProj) {
				t.Errorf("home's session reads projects %s, want home's and never test's", keys)
			}
			if _, status := w.whoami(t, host, "test", homeSession); status != http.StatusUnauthorized {
				t.Errorf("a header naming test with home's session = %d, want 401", status)
			}
			if _, _, got := w.login(t, host, "default", "olly@example.test"); got.status != http.StatusUnauthorized {
				t.Errorf("sign-in naming home by a member of test only = %d, want 401", got.status)
			}
			if _, _, got := w.login(t, host, "nowhere", "olly@example.test"); got.status != http.StatusNotFound {
				t.Errorf("sign-in naming a tenant that does not exist = %d, want 404", got.status)
			}
		})
	}
}

func TestASwitchOnASharedHostMovesTheSession(t *testing.T) {
	w := newSharedHostWorld(t)
	w.setShared(t, true)

	from, tenant, _ := w.login(t, homeHost, "", "pat@example.test")
	if tenant != w.h.tenantID {
		t.Fatalf("pat signed in to %q, want home first", tenant)
	}
	me, _ := w.whoami(t, homeHost, "", from)
	if len(me.Memberships) != 2 || me.AccountID == "" {
		t.Fatalf("pat's whoami = %+v, want two memberships and the account", me)
	}

	got := w.do(t, request{method: http.MethodPost, path: wire.RouteSwitch, host: homeHost, cookie: from,
		body: core.SwitchTenantInput{Tenant: "test"}})
	var result core.LoginResult
	if got.status != http.StatusOK || json.Unmarshal(got.body, &result) != nil || result.Session == nil {
		t.Fatalf("switch = %d %s, want a session", got.status, got.body)
	}
	if result.Session.TenantID != w.testID {
		t.Errorf("switched into %q, want test", result.Session.TenantID)
	}
	if _, status := w.whoami(t, homeHost, "", from); status != http.StatusUnauthorized {
		t.Errorf("the session switched from = %d, want 401", status)
	}
	if me, status := w.whoami(t, homeHost, "", result.Session.Token); status != http.StatusOK || me.TenantKey != "test" {
		t.Errorf("the switched session = %d %+v, want served as test", status, me)
	}

	got = w.do(t, request{method: http.MethodPost, path: wire.RouteSwitch, host: homeHost,
		body: core.SwitchTenantInput{Tenant: "test"}})
	if got.status != http.StatusUnauthorized {
		t.Errorf("a switch with no session = %d, want 401", got.status)
	}
	got = w.do(t, request{method: http.MethodPost, path: wire.RouteSwitch, host: homeHost, cookie: result.Session.Token,
		body: core.SwitchTenantInput{Tenant: "default"}})
	if got.status != http.StatusOK {
		t.Errorf("switching back = %d %s, want 200", got.status, got.body)
	}
}

func TestATenantAtItsOwnAddressIsNotServedOnTheSharedHost(t *testing.T) {
	w := newSharedHostWorld(t)
	w.setShared(t, true)
	testSession, _, _ := w.login(t, "127.0.0.1", "test", "olly@example.test")
	w.claimTest(t)

	if _, _, got := w.login(t, homeHost, "test", "olly@example.test"); got.status != http.StatusForbidden {
		t.Errorf("naming test on the shared host once it has its own address = %d %s, want 403", got.status, got.body)
	}
	if _, status := w.whoami(t, homeHost, "", testSession); status != http.StatusUnauthorized {
		t.Errorf("test's session on the shared host once test has its own address = %d, want 401", status)
	}
	if me, status := w.whoami(t, testHost, "", testSession); status != http.StatusOK || me.TenantKey != "test" {
		t.Errorf("test's session at test's own address = %d %+v, want served as test", status, me)
	}
}
