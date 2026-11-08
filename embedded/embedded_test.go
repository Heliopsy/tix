// SPDX-License-Identifier: AGPL-3.0-or-later

package embedded_test

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/embedded"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/onboard"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/wire"
)

// TestMain drops password hashing to test cost, as every package building a
// service does.
func TestMain(m *testing.M) {
	service.UseCheapHashingInTests()
	os.Exit(m.Run())
}

func start(t *testing.T, path string, init bool) *embedded.Instance {
	t.Helper()
	in, err := embedded.Start(context.Background(), embedded.Options{DBPath: path, InitIfMissing: init})
	if err != nil {
		t.Fatalf("start %s: %v", path, err)
	}
	t.Cleanup(func() { _ = in.Stop(context.Background()) })
	return in
}

// get calls the instance with a bearer token and decodes the JSON answer.
func get(t *testing.T, in *embedded.Instance, token, route string, out any) int {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, in.URL()+route, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", route, err)
	}
	defer resp.Body.Close()
	if out != nil && resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decoding %s: %v", route, err)
		}
	}
	return resp.StatusCode
}

// post sends a JSON body with a bearer token and returns the status.
func post(t *testing.T, in *embedded.Instance, token, route, body string) int {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, in.URL()+route, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", route, err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

type whoami struct {
	ID       string       `json:"id"`
	TenantID string       `json:"tenant_id"`
	Handle   string       `json:"handle"`
	Scopes   []core.Scope `json:"scopes"`
}

func keys(ts []embedded.Tenant) []string {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		out = append(out, t.Key)
	}
	slices.Sort(out)
	return out
}

func TestStartRefusesAnythingButALoopbackIP(t *testing.T) {
	for _, listen := range []string{
		"0.0.0.0:8080", ":8080", "example.com:80", "localhost:0", "192.168.1.10:80", "[::]:0", "127.0.0.1",
	} {
		t.Run(listen, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "tix.db")
			_, err := embedded.Start(context.Background(),
				embedded.Options{DBPath: path, Listen: listen, InitIfMissing: true})
			if !core.IsKind(err, core.KindInvalid) {
				t.Fatalf("listen %q: got %v, want an invalid error", listen, err)
			}
			if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("listen %q opened the database before refusing: %v", listen, err)
			}
		})
	}
}

func TestStartListensOnTheLoopbackAddressGiven(t *testing.T) {
	in, err := embedded.Start(context.Background(), embedded.Options{
		DBPath: filepath.Join(t.TempDir(), "tix.db"), Listen: "[::1]:0", InitIfMissing: true,
	})
	if err != nil {
		t.Skipf("no ipv6 loopback here: %v", err)
	}
	defer func() { _ = in.Stop(context.Background()) }()
	if !strings.HasPrefix(in.URL(), "http://[::1]:") {
		t.Fatalf("URL %q is not on [::1]", in.URL())
	}
	if code := get(t, in, "", wire.RouteHealth, nil); code != http.StatusOK {
		t.Fatalf("healthz: %d", code)
	}
}

func TestStartRefusesAMissingFileUnlessAskedToCreateIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.db")
	_, err := embedded.Start(context.Background(), embedded.Options{DBPath: path})
	if !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("got %v, want not found", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a refused start created the file: %v", err)
	}
	if _, err := embedded.Start(context.Background(), embedded.Options{}); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("no path: got %v, want invalid", err)
	}
}

func TestAFreshFileIsSeededLikeTixInit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "fresh.db")
	in := start(t, path, true)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database not created: %v", err)
	}
	if !strings.HasPrefix(in.URL(), "http://127.0.0.1:") {
		t.Fatalf("URL %q is not on the loopback default", in.URL())
	}
	if code := get(t, in, "", wire.RouteHealth, nil); code != http.StatusOK {
		t.Fatalf("healthz: %d", code)
	}
	tenants, err := in.Tenants(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := keys(tenants); !slices.Equal(got, []string{service.DefaultTenantKey}) {
		t.Fatalf("tenants %v, want only the default", got)
	}
	token, err := in.Token(context.Background(), embedded.TokenRequest{Name: "app"})
	if err != nil {
		t.Fatal(err)
	}
	var projects struct {
		Items []struct {
			Key string `json:"key"`
		} `json:"items"`
	}
	if code := get(t, in, token, wire.RouteProjects, &projects); code != http.StatusOK {
		t.Fatalf("projects: %d", code)
	}
	if len(projects.Items) == 0 {
		t.Fatal("a fresh database has no starter projects")
	}
}

func TestTokensActAsTheLocalOwnerOfTheirTenant(t *testing.T) {
	in := start(t, filepath.Join(t.TempDir(), "tix.db"), true)
	ctx := context.Background()
	created, err := in.CreateTenant(ctx, "work", "Work")
	if err != nil {
		t.Fatal(err)
	}
	if created != (embedded.Tenant{Key: "work", Name: "Work"}) {
		t.Fatalf("created %+v", created)
	}
	tenants, err := in.Tenants(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(tenants, embedded.Tenant{Key: "work", Name: "Work"}) || len(tenants) != 2 {
		t.Fatalf("tenants %+v, want default and work", tenants)
	}

	work, err := in.Token(ctx, embedded.TokenRequest{Tenant: "work", Name: "app"})
	if err != nil {
		t.Fatal(err)
	}
	home, err := in.Token(ctx, embedded.TokenRequest{Name: "app", Scopes: []string{"task:read"}, TTL: -1})
	if err != nil {
		t.Fatal(err)
	}
	again, err := in.Token(ctx, embedded.TokenRequest{Tenant: "work", Name: "again"})
	if err != nil {
		t.Fatal(err)
	}
	var w, h, a whoami
	if code := get(t, in, again, wire.RouteWhoAmI, &a); code != http.StatusOK {
		t.Fatalf("whoami with the second work token: %d", code)
	}
	if code := get(t, in, work, wire.RouteWhoAmI, &w); code != http.StatusOK {
		t.Fatalf("whoami with the work token: %d", code)
	}
	if code := get(t, in, home, wire.RouteWhoAmI, &h); code != http.StatusOK {
		t.Fatalf("whoami with the default token: %d", code)
	}
	if w.ID == "" || w.ID != a.ID {
		t.Fatalf("two tokens of one tenant act as %q and %q, want the one local actor", w.ID, a.ID)
	}
	if w.ID == h.ID {
		t.Fatalf("the work and default tokens both act as %q", w.ID)
	}
	if w.TenantID == "" || w.TenantID == h.TenantID {
		t.Fatalf("work token in tenant %q, default token in %q: want two tenants", w.TenantID, h.TenantID)
	}
	if !slices.Equal(w.Scopes, []core.Scope{core.ScopeAll}) {
		t.Fatalf("a token asked for no scopes carries %v, want every scope", w.Scopes)
	}
	if !slices.Equal(h.Scopes, []core.Scope{"task:read"}) {
		t.Fatalf("a token asked for task:read carries %v", h.Scopes)
	}
	if code := post(t, in, work, wire.RouteProjects, `{"key":"inbox","name":"Inbox"}`); code != http.StatusCreated {
		t.Fatalf("the work token could not create a project: %d", code)
	}
	if code := post(t, in, work, wire.RouteTasks, `{"project_ref":"inbox","title":"first"}`); code != http.StatusCreated {
		t.Fatalf("the work token could not create a task: %d", code)
	}
}

func TestTokenLifetimes(t *testing.T) {
	in := start(t, filepath.Join(t.TempDir(), "tix.db"), true)
	ctx := context.Background()
	admin, err := in.Token(ctx, embedded.TokenRequest{Name: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	for name, ttl := range map[string]time.Duration{"short": time.Hour, "never": -1} {
		if _, err := in.Token(ctx, embedded.TokenRequest{Name: name, TTL: ttl}); err != nil {
			t.Fatal(err)
		}
	}
	var listed struct {
		Items []struct {
			Name      string     `json:"name"`
			ExpiresAt *time.Time `json:"expires_at"`
		} `json:"items"`
	}
	if code := get(t, in, admin, wire.RouteTokens, &listed); code != http.StatusOK {
		t.Fatalf("tokens: %d", code)
	}
	tokens := listed.Items
	want := map[string]time.Duration{"admin": embedded.DefaultTokenTTL, "short": time.Hour}
	seen := map[string]bool{}
	for _, tok := range tokens {
		seen[tok.Name] = true
		ttl, ok := want[tok.Name]
		switch {
		case tok.Name == "never" && tok.ExpiresAt != nil:
			t.Errorf("a token asked never to expire expires at %v", tok.ExpiresAt)
		case ok && tok.ExpiresAt == nil:
			t.Errorf("token %s never expires", tok.Name)
		case ok && time.Until(*tok.ExpiresAt) > ttl || ok && time.Until(*tok.ExpiresAt) < ttl-time.Minute:
			t.Errorf("token %s expires at %v, want about %v from now", tok.Name, tok.ExpiresAt, ttl)
		}
	}
	if len(seen) != 3 {
		t.Fatalf("listed %v, want admin, short and never", seen)
	}
}

func TestDefaultTokenLifetimeIsTheFormsDefault(t *testing.T) {
	for _, e := range onboard.TokenExpiries {
		if e.Key == onboard.DefaultTokenExpiry {
			if got := time.Duration(e.Days) * 24 * time.Hour; got != embedded.DefaultTokenTTL {
				t.Fatalf("the forms default to %v, the library to %v", got, embedded.DefaultTokenTTL)
			}
			return
		}
	}
	t.Fatalf("no token expiry named %q", onboard.DefaultTokenExpiry)
}

func TestTokenAndTenantRefusals(t *testing.T) {
	in := start(t, filepath.Join(t.TempDir(), "tix.db"), true)
	ctx := context.Background()
	if _, err := in.Token(ctx, embedded.TokenRequest{Tenant: "nope", Name: "x"}); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("token in a missing tenant: %v", err)
	}
	if _, err := in.Token(ctx, embedded.TokenRequest{}); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("token without a name: %v", err)
	}
	if _, err := in.CreateTenant(ctx, "Bad Key!", "x"); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("bad tenant key: %v", err)
	}
	if _, err := in.CreateTenant(ctx, "default", "Again"); !core.IsKind(err, core.KindConflict) {
		t.Fatalf("duplicate tenant: %v", err)
	}
}

func TestTwoInstancesRunSideBySide(t *testing.T) {
	dir := t.TempDir()
	a := start(t, filepath.Join(dir, "a.db"), true)
	b := start(t, filepath.Join(dir, "b.db"), true)
	ctx := context.Background()
	if a.URL() == b.URL() {
		t.Fatalf("both instances on %s", a.URL())
	}
	if _, err := a.CreateTenant(ctx, "only-a", "Only A"); err != nil {
		t.Fatal(err)
	}
	if err := a.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if code := get(t, b, "", wire.RouteHealth, nil); code != http.StatusOK {
		t.Fatalf("b stopped answering when a stopped: %d", code)
	}
	tenants, err := b.Tenants(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := keys(tenants); !slices.Equal(got, []string{service.DefaultTenantKey}) {
		t.Fatalf("b sees tenants %v", got)
	}
}

func TestStopReleasesThePortAndTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tix.db")
	ctx := context.Background()
	in, err := embedded.Start(ctx, embedded.Options{DBPath: path, InitIfMissing: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := in.CreateTenant(ctx, "kept", "Kept"); err != nil {
		t.Fatal(err)
	}
	url := in.URL()
	stopped := make(chan error, 1)
	go func() { stopped <- in.Stop(ctx) }()
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatalf("stop: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("stop did not return")
	}
	if err := in.Stop(ctx); err != nil {
		t.Fatalf("second stop: %v", err)
	}
	client := http.Client{Timeout: 2 * time.Second}
	if resp, err := client.Get(url + wire.RouteHealth); err == nil {
		resp.Body.Close()
		t.Fatalf("a stopped instance still answers on %s", url)
	}

	again := start(t, path, false)
	tenants, err := again.Tenants(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := keys(tenants); !slices.Equal(got, []string{"default", "kept"}) {
		t.Fatalf("reopened database holds %v", got)
	}
}

func TestStopHonoursItsDeadline(t *testing.T) {
	in := start(t, filepath.Join(t.TempDir(), "tix.db"), true)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := in.Stop(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("stop with an ended context: %v", err)
	}
	if again := in.Stop(context.Background()); !errors.Is(again, err) {
		t.Fatalf("second stop %v differs from the first %v", again, err)
	}
}
