package integration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/client"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/server"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/store/sqlite"
	"github.com/heliopsy/tix/internal/web"
)

// matrixHarness boots exactly one service.Local, driven by a clock the test
// controls, and serves it over one real HTTP+WebSocket server. "direct" and
// "remote" access below are two ways to reach the SAME brain, which is the
// whole point of the transport-equivalence suite: a divergence here is a
// divergence between service.Local and client.Client, not between two
// installations.
type matrixHarness struct {
	t *testing.T

	clk      *clock.Fake
	tenantID string

	local      *service.Local
	baseURL    string
	adminActor *core.Actor
	adminCtx   context.Context
	adminToken string
}

// newMatrixHarness boots a fresh tenant, database and server. Each scenario
// invocation gets its own, so advancing the fake clock or exhausting a scope
// in one scenario can never bleed into another.
func newMatrixHarness(t *testing.T) *matrixHarness {
	t.Helper()
	ctx := context.Background()
	// Started near the real wall clock, not clock.NewFakeAt's fixed 2026-01-01
	// epoch: a cookie's Expires attribute is set from this clock but checked
	// by the real net/http cookiejar against real wall time, so a fake clock
	// anchored in the past makes every session cookie look already expired to
	// the browser client the web-transport scenarios drive.
	clk := clock.NewFake(time.Now())
	path := filepath.Join(t.TempDir(), "tix.db")

	st, err := sqlite.Open(path, clk)
	if err != nil {
		t.Fatalf("opening the store at %q: %v", path, err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrating %q: %v", path, err)
	}

	svc := service.New(st,
		service.WithClock(clk),
		service.WithHasher(auth.NewHasherWithParams(auth.TestParams())),
		// The starter lists (work, homelab, house) exist only to make a
		// brand new human installation not look empty; they only add noise
		// to a scenario table that creates its own project per run, and to
		// the terminal-interface scenarios, which navigate the project
		// picker by position.
		service.WithoutStarterProjects())

	tenant, err := svc.EnsureDefaults(ctx)
	if err != nil {
		t.Fatalf("bootstrapping defaults: %v", err)
	}
	bootstrapCtx := core.WithSource(core.WithActor(ctx, core.SystemActor(tenant.ID)), core.SourceCLI)

	user, err := svc.CreateUser(bootstrapCtx, core.CreateUserInput{
		Email: "matrix@example.test", Handle: "matrix", Role: core.RoleAdmin,
	})
	if err != nil {
		t.Fatalf("creating the admin actor: %v", err)
	}
	issued, err := svc.CreateToken(bootstrapCtx, core.CreateTokenInput{
		Name: "matrix-admin", ActorID: user.ID, Scopes: []core.Scope{core.ScopeAll},
	})
	if err != nil {
		t.Fatalf("minting the admin token: %v", err)
	}

	adminActor := &core.Actor{
		ID: user.ID, TenantID: tenant.ID, Kind: core.ActorUser,
		Handle: "matrix", Role: core.RoleAdmin, Scopes: []core.Scope{core.ScopeAll},
	}
	adminCtx := core.WithSource(core.WithActor(ctx, adminActor), core.SourceCLI)

	srv, err := server.Assemble(server.Options{
		Service:         svc,
		Store:           st,
		Clock:           clk,
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		TenantID:        tenant.ID,
		WebHandler:      web.Handler(svc),
		Addr:            "127.0.0.1:0",
		DisableSweep:    true,
		DisablePrune:    true,
		DisableDispatch: true,
	})
	if err != nil {
		t.Fatalf("assembling the server: %v", err)
	}
	if err := srv.Listen(); err != nil {
		t.Fatalf("binding the server: %v", err)
	}

	serveCtx, stop := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = srv.Serve(serveCtx)
	}()
	t.Cleanup(func() {
		stop()
		wg.Wait()
	})

	return &matrixHarness{
		t: t, clk: clk, tenantID: tenant.ID,
		local: svc, baseURL: "http://" + srv.Addr(),
		adminActor: adminActor, adminCtx: adminCtx, adminToken: issued.Token,
	}
}

// target is one way of reaching the harness's service: straight at the Local,
// or over HTTP through a client.Client fronting the same Local.
type target struct {
	name   string
	remote bool
	svc    core.Service
	ctx    context.Context
}

// localTarget drives the service.Local directly, as a CLI process talking
// straight to the database would.
func (h *matrixHarness) localTarget() target {
	return target{name: "direct service.Local", svc: h.local, ctx: h.adminCtx}
}

// remoteTarget drives the same Local over HTTP via client.Client.
func (h *matrixHarness) remoteTarget() target {
	c, err := client.New(h.baseURL, h.adminToken)
	if err != nil {
		h.t.Fatalf("building the remote client: %v", err)
	}
	h.t.Cleanup(func() { _ = c.Close() })
	return target{name: "client.Client over HTTP", svc: c, remote: true, ctx: context.Background()}
}

// scoped returns a service handle and context restricted to exactly the
// given scopes, on whichever transport tg names, for exercising permission
// denial identically on both sides.
func (h *matrixHarness) scoped(tg target, scopes ...core.Scope) (core.Service, context.Context) {
	h.t.Helper()
	if !tg.remote {
		restricted := &core.Actor{
			ID: h.adminActor.ID, TenantID: h.tenantID, Kind: core.ActorUser,
			Handle: "restricted", Scopes: scopes,
		}
		return h.local, core.WithSource(core.WithActor(context.Background(), restricted), core.SourceCLI)
	}
	issued, err := h.local.CreateToken(h.adminCtx, core.CreateTokenInput{
		Name: "scoped-" + randSuffix(), ActorID: h.adminActor.ID, Scopes: scopes,
	})
	if err != nil {
		h.t.Fatalf("minting a scoped token: %v", err)
	}
	c, err := client.New(h.baseURL, issued.Token)
	if err != nil {
		h.t.Fatalf("building the scoped client: %v", err)
	}
	h.t.Cleanup(func() { _ = c.Close() })
	return c, context.Background()
}

// newProject creates a project isolated to this scenario and target, so two
// runs sharing a harness never see each other's tasks.
func (h *matrixHarness) newProject(t *testing.T, tg target) core.Project {
	t.Helper()
	key := "m" + randSuffix()
	p, err := tg.svc.CreateProject(tg.ctx, core.CreateProjectInput{Key: key, Name: key})
	if err != nil {
		t.Fatalf("[%s] creating a scenario project: %v", tg.name, err)
	}
	return *p
}

// randSuffix is a short, collision-resistant lowercase suffix for keys that
// must be unique per scenario run but need not be a UUID.
func randSuffix() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// sig is the normalized, comparable shape of a scenario's outcome. Only
// semantic fields belong here: identifiers, refs and timestamps are
// necessarily different between two independent runs even when the two
// transports agree in every way that matters.
type sig map[string]any

func mustf(t *testing.T, tg target, err error, format string, args ...any) {
	t.Helper()
	if err != nil {
		t.Fatalf("[%s] %s: %v", tg.name, fmt.Sprintf(format, args...), err)
	}
}
