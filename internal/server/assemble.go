// SPDX-License-Identifier: AGPL-3.0-or-later

package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/connections"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/egress"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/lease"
	"github.com/heliopsy/tix/internal/presence"
	"github.com/heliopsy/tix/internal/retention"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/migrations"
	"github.com/heliopsy/tix/internal/version"
	"github.com/heliopsy/tix/internal/webhook"
)

// Options describe the process the serve command starts.
type Options struct {
	Service core.Service
	Store   store.Store

	// TrustedProxies lists the proxies whose forwarded headers are believed.
	// Empty means none, so a forwarded scheme or client address from an
	// untrusted peer is ignored rather than taken at its word.
	TrustedProxies []string
	// CookieSecurity overrides how the Secure attribute is decided.
	CookieSecurity string
	// PublicURL is server.public_url: the address links drawn from a request
	// on a shared host, and channel messages, are built under. Empty keeps
	// the request's own host.
	PublicURL string
	// AllowPrivateWebhookTargets lets webhook deliveries reach loopback and
	// private network addresses. Off by default: a tenant-supplied URL
	// reaching internal infrastructure is the classic request-forgery shape.
	AllowPrivateWebhookTargets bool
	// Egress is the operator's policy for tenant-supplied urls, strict when
	// zero. AllowPrivateWebhookTargets widens it for deliveries only.
	Egress egress.Policy
	Logger *slog.Logger
	Clock  clock.Clock

	// TenantID serves requests whose Host maps to no tenant. Leaving it empty
	// answers an unknown host with not found instead.
	TenantID string

	Addr     string
	CertFile string
	KeyFile  string

	// EventPollInterval is the floor under a tenant's reader: a commit wakes it
	// directly, and this is how long it waits when no wake arrives. Zero uses a
	// sensible default.
	EventPollInterval time.Duration

	// WebHandler serves the browser interface. Leaving it nil serves the API only.
	WebHandler http.Handler

	// SSH runs the terminal interface beside the HTTP surface. Nil starts no
	// listener and binds no port.
	SSH SSHListener

	AllowInsecure bool

	MaxBodyBytes   int64
	RequestTimeout time.Duration
	// MaxImportBytes caps a task import in place of MaxBodyBytes, on the API's
	// import route and on ImportRoutes, the web handler's own.
	MaxImportBytes int64
	ImportRoutes   []string
	// MaxArtifactBytes is artifacts.max_bytes. The API's artifact route and
	// ArtifactRoutes, the web handler's own, take a body sized to carry it.
	MaxArtifactBytes int64
	ArtifactRoutes   []string
	// Sessions are the server's session settings; the idle timeout and
	// renewal are applied as each request authenticates.
	Sessions        core.SessionPolicy
	ShutdownTimeout time.Duration

	SweepInterval    time.Duration
	PruneInterval    time.Duration
	DispatchInterval time.Duration
	DisableSweep     bool
	DisablePrune     bool
	DisableDispatch  bool

	// HeartbeatInterval is how often this process refreshes its registration.
	// Zero uses the interval the staleness threshold is derived from, so the
	// two cannot drift apart by accident.
	HeartbeatInterval time.Duration
	// DisableRegistry stops this process registering itself at all, which
	// makes it invisible to `tix status`. It exists for tests and for a
	// process an operator deliberately does not want counted.
	DisableRegistry bool
}

// Assemble builds the router over the service and returns a server with its
// background workers attached.
func Assemble(opts Options) (*Server, error) {
	if opts.Service == nil {
		return nil, core.Invalid("serving requires a service")
	}
	if opts.Store == nil {
		return nil, core.Invalid("serving requires a store")
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Clock == nil {
		opts.Clock = clock.New()
	}

	latest, err := migrations.Latest()
	if err != nil {
		return nil, err
	}
	// One event reader per tenant that has a live connection. Events are only
	// readable inside a tenant-scoped transaction, so this preserves the
	// isolation boundary rather than reading every tenant at once.
	hub := httpapi.NewHub()
	log := eventLog{store: opts.Store}
	pump := newPumps(context.Background(), hub, log, opts.EventPollInterval, opts.Logger)
	hub.SetTenantHooks(pump.start, pump.stop)

	router, err := httpapi.New(httpapi.Config{
		Service:          opts.Service,
		EventHandler:     httpapi.NewEventStream(hub, log),
		WebHandler:       opts.WebHandler,
		Authenticator:    NewAuthenticator(opts.Store, opts.Clock, opts.Sessions),
		Logger:           opts.Logger,
		DefaultTenantID:  opts.TenantID,
		MaxBodyBytes:     opts.MaxBodyBytes,
		MaxImportBytes:   opts.MaxImportBytes,
		ImportRoutes:     opts.ImportRoutes,
		MaxArtifactBytes: opts.MaxArtifactBytes,
		ArtifactRoutes:   opts.ArtifactRoutes,
		RequestTimeout:   opts.RequestTimeout,
		SecureCookies:    opts.CertFile != "",
		TrustedProxies:   opts.TrustedProxies,
		CookieSecurity:   opts.CookieSecurity,
		PublicURL:        opts.PublicURL,
		Probe:            opts.Store,
		ExpectedSchema:   latest,
	})
	if err != nil {
		return nil, err
	}

	var srv *Server
	workers := workersFor(opts)
	if registrar := newRegistrar(opts, func() string {
		if srv != nil {
			if addr := srv.Addr(); addr != "" {
				return addr
			}
		}
		return opts.Addr
	}); registrar != nil {
		// The connection registry answers under the identifier this process
		// registered, so a status report can say which listed server the
		// connection count it holds belongs to. Nothing has connected yet.
		connections.Default.SetServerID(registrar.ServerID())
		workers = append(workers, RegistrarWorker(registrar))
	}

	srv, err = New(Config{
		Addr:            opts.Addr,
		Handler:         router,
		Logger:          opts.Logger,
		CertFile:        opts.CertFile,
		KeyFile:         opts.KeyFile,
		AllowInsecure:   opts.AllowInsecure,
		ShutdownTimeout: opts.ShutdownTimeout,
		SSH:             opts.SSH,
		Workers:         append(workers, EventPumpWorker(pump)),
	})
	return srv, err
}

// newRegistrar builds the worker that keeps this process's row in the server
// registry fresh, or nothing when registration is off.
//
// The address is a function rather than a value because a server told to listen
// on port zero does not know its own address until the listener is open, and
// registering the address it was asked for would advertise a port nothing is
// on.
func newRegistrar(opts Options, addr func() string) *presence.Registrar {
	if opts.DisableRegistry {
		return nil
	}
	return presence.New(
		presence.Config{
			Store:    opts.Store,
			Address:  addr,
			Version:  version.Version,
			Surfaces: surfacesOf(opts),
		},
		presence.WithClock(opts.Clock),
		presence.WithInterval(opts.HeartbeatInterval),
		presence.WithErrorHandler(func(err error) {
			if errors.Is(err, context.Canceled) {
				return
			}
			opts.Logger.Error("server registration failed", "error", err.Error())
		}),
	)
}

// surfacesOf names what this process actually serves, which is decided by how
// it was configured rather than by what the build can do.
func surfacesOf(opts Options) []core.ServerSurface {
	out := []core.ServerSurface{core.ServerSurfaceAPI, core.ServerSurfaceWS}
	if opts.WebHandler != nil {
		out = append(out, core.ServerSurfaceWeb)
	}
	if opts.SSH != nil {
		out = append(out, core.ServerSurfaceSSH)
	}
	return out
}

// workersFor builds the enabled background workers.
func workersFor(opts Options) []Worker {
	var workers []Worker

	if !opts.DisableSweep {
		sweeper := lease.NewSweeper(
			func(ctx context.Context, limit int) (int, error) {
				return opts.Service.SweepLeases(systemContext(ctx, opts.TenantID, JobLeaseSweep), limit)
			},
			lease.WithClock(opts.Clock),
			lease.WithInterval(opts.SweepInterval),
			lease.WithErrorHandler(func(err error) {
				opts.Logger.Error("lease sweep failed", "error", err.Error())
			}),
		)
		workers = append(workers, SweeperWorker(sweeper))

		waker := lease.NewSweeper(
			func(ctx context.Context, limit int) (int, error) {
				return opts.Service.WakeSnoozed(systemContext(ctx, opts.TenantID, JobSnoozeWake), limit)
			},
			lease.WithClock(opts.Clock),
			lease.WithInterval(opts.SweepInterval),
			lease.WithErrorHandler(func(err error) {
				opts.Logger.Error("snooze wake failed", "error", err.Error())
			}),
		)
		workers = append(workers, FuncWorker{WorkerName: "snooze-waker", Fn: waker.Run})
	}

	if !opts.DisablePrune && opts.PruneInterval > 0 {
		pruner := retention.NewPruner(
			systemRunner{service: opts.Service, tenantID: opts.TenantID},
			opts.Clock,
			core.Duration(opts.PruneInterval),
			retention.WithHousekeeper(systemRunner{service: opts.Service, tenantID: opts.TenantID}),
			retention.WithErrorHandler(func(err error) {
				opts.Logger.Error("retention prune failed", "error", err.Error())
			}),
		)
		workers = append(workers, PrunerWorker(pruner))
	}

	if !opts.DisableDispatch {
		interval := opts.DispatchInterval
		if interval <= 0 {
			interval = webhook.DefaultInterval
		}
		dispatcher := webhook.NewDispatcher(
			opts.Store,
			core.TenantScope{TenantID: opts.TenantID},
			opts.Clock,
			webhook.WithInterval(interval),
			webhook.WithErrorHandler(func(err error) {
				if errors.Is(err, context.Canceled) {
					return
				}
				opts.Logger.Error("webhook dispatch failed", "error", err.Error())
			}),
			// The dispatcher refuses private network targets unless the
			// operator allowed them. Without this it would build its own
			// deny-by-default guard, which is safe but would also refuse a
			// target an operator deliberately permitted.
			webhook.WithGuard(webhookGuard(opts)),
			webhook.WithPublicURL(opts.PublicURL),
		)
		workers = append(workers, DispatcherWorker(dispatcher))
	}
	return workers
}

// Background jobs, as the access channel of what they change names them.
const (
	JobLeaseSweep   = "lease-sweep"
	JobSnoozeWake   = "snooze-wake"
	JobPrune        = "prune"
	JobHousekeeping = "housekeeping"
)

// systemContext gives background work the identity it has no request to
// borrow from, and names the job on the system channel.
func systemContext(ctx context.Context, tenantID, job string) context.Context {
	ctx = core.WithSource(core.WithActor(ctx, core.SystemActor(tenantID)), core.SourceSystem)
	return core.WithVia(ctx, core.Via{Kind: core.ViaSystem, Job: job})
}

// systemRunner prunes as the system actor.
type systemRunner struct {
	service  core.Service
	tenantID string
}

// Prune removes records past their retention window.
func (r systemRunner) Prune(ctx context.Context, in core.PruneInput) (*core.PruneResult, error) {
	return r.service.Prune(systemContext(ctx, r.tenantID, JobPrune), in)
}

// RunHousekeeping applies the housekeeping policy as the system actor.
func (r systemRunner) RunHousekeeping(ctx context.Context, in core.HousekeepingRunInput) (*core.HousekeepingResult, error) {
	return r.service.RunHousekeeping(systemContext(ctx, r.tenantID, JobHousekeeping), in)
}

// webhookGuard is the delivery guard the operator configured: the egress
// policy, widened to every private address when webhooks allow it.
func webhookGuard(opts Options) webhook.Guard {
	p := opts.Egress
	if opts.AllowPrivateWebhookTargets {
		p.AllowPrivate = true
	}
	return webhook.GuardFor(p)
}
