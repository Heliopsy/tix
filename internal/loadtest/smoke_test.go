// SPDX-License-Identifier: AGPL-3.0-or-later

package loadtest

import (
	"context"
	"io"
	"log/slog"
	"net/netip"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/egress"
	"github.com/heliopsy/tix/internal/server"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/store/sqlite"
	"github.com/heliopsy/tix/internal/store/sqlite/sqlitetest"
	"github.com/heliopsy/tix/internal/web"
)

// inProcessServer serves a scratch database on a loopback port and returns its
// base url and one account with a password and a token.
func inProcessServer(t *testing.T) (string, Account) {
	t.Helper()
	ctx := context.Background()
	clk := clock.New()
	st, err := sqlite.Open(sqlitetest.Path(t), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	loopback := egress.Policy{Allow: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}}
	svc := service.New(st, service.WithClock(clk), service.WithEgress(loopback),
		service.WithHasher(auth.NewHasherWithParams(auth.TestParams())))
	tenant, err := svc.EnsureDefaults(ctx)
	if err != nil {
		t.Fatal(err)
	}
	admin := core.WithSource(core.WithActor(ctx, core.SystemActor(tenant.ID)), core.SourceCLI)
	const password = "smoke-test-password"
	u, err := svc.CreateUser(admin, core.CreateUserInput{Email: "load@example.test", Handle: "load", Role: core.RoleAdmin, Password: password})
	if err != nil {
		t.Fatal(err)
	}
	tok, err := svc.CreateToken(admin, core.CreateTokenInput{Name: "load", ActorID: u.ID, Scopes: []core.Scope{core.ScopeAll}})
	if err != nil {
		t.Fatal(err)
	}
	srv, err := server.Assemble(server.Options{
		Egress: loopback, Service: svc, Store: st, Clock: clk,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		TenantID: tenant.ID, WebHandler: web.Handler(svc), Addr: "127.0.0.1:0",
		EventPollInterval: 50 * time.Millisecond, DisableSweep: true, DisablePrune: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Listen(); err != nil {
		t.Fatal(err)
	}
	serveCtx, stop := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Go(func() { _ = srv.Serve(serveCtx) })
	t.Cleanup(func() { stop(); wg.Wait() })
	return "http://" + srv.Addr(), Account{Email: "load@example.test", Password: password, Token: tok.Token}
}

func TestSmokeOneUserForThreeSecondsProducesAReport(t *testing.T) {
	if testing.Short() {
		t.Skip("runs real load for three seconds")
	}
	base, acct := inProcessServer(t)
	ctx := context.Background()
	seed := Seed{Projects: 2, MinBody: 2 << 10, MaxBody: 6 << 10}
	pool, err := SeedTenant(ctx, base, acct, seed, 6, 1, nil)
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	if len(pool.Tasks) != 6 || len(pool.Projects) != 2 {
		t.Fatalf("seeded %d tasks in %d projects, want 6 in 2", len(pool.Tasks), len(pool.Projects))
	}

	mix, _ := ParseMix("browser=1")
	cfg := Config{Users: 1, Duration: 3 * time.Second, Mix: mix, Think: 200 * time.Millisecond,
		Sample: 500 * time.Millisecond, Warmup: 0.25, MaxRSS: 64 << 30}
	probe := Probe{Why: "not linux"}
	if runtime.GOOS == "linux" {
		probe = Probe{PID: os.Getpid()}
	}
	tgt := Target{Base: base, AllowWrites: true, Accounts: []Account{acct}}
	r, err := Run(ctx, cfg, tgt, probe, Report{Mode: "local", Target: base}, t.Logf)
	if err != nil {
		t.Fatal(err)
	}

	routes := map[string]RouteStats{}
	for _, st := range r.Routes {
		routes[st.Route] = st
	}
	for _, want := range []string{"POST /auth/login", "GET /projects", "WS /events (open)", "GET /assets/*"} {
		if routes[want].Count == 0 {
			t.Errorf("the report has no %s: %v", want, r.Routes)
		}
	}
	if routes["POST /auth/login"].Statuses["200"] != 1 {
		t.Errorf("sign in: %+v, want one 200", routes["POST /auth/login"])
	}
	if r.Errors != 0 {
		t.Errorf("errors = %d in %+v", r.Errors, r.Routes)
	}
	if r.Duration < 2.5 || r.Duration > 30 {
		t.Errorf("the run took %.1fs, want about 3s", r.Duration)
	}
	if runtime.GOOS == "linux" && (!r.Memory.Available || r.Memory.Peak <= 0 || !r.Budget.Checked || !r.Budget.Pass) {
		t.Errorf("memory %+v budget %+v, want measured and within a 64 GiB budget", r.Memory.Available, r.Budget)
	}
	var text strings.Builder
	if err := r.WriteText(&text); err != nil || !strings.Contains(text.String(), "tix load report") {
		t.Fatalf("text report: %v\n%s", err, text.String())
	}
}
