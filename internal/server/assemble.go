package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/thereisnotime/tix/internal/clock"
	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/httpapi"
	"github.com/thereisnotime/tix/internal/lease"
	"github.com/thereisnotime/tix/internal/retention"
	"github.com/thereisnotime/tix/internal/store"
	"github.com/thereisnotime/tix/internal/store/migrations"
	"github.com/thereisnotime/tix/internal/webhook"
)

// Options describe the process the serve command starts.
type Options struct {
	Service core.Service
	Store   store.Store
	Logger  *slog.Logger
	Clock   clock.Clock

	// TenantID serves requests whose Host maps to no tenant. Leaving it empty
	// answers an unknown host with not found instead.
	TenantID string

	Addr     string
	CertFile string
	KeyFile  string

	// EventPollInterval is how often a tenant's reader checks for new events.
	// Zero uses a sensible default.
	EventPollInterval time.Duration

	// WebHandler serves the browser interface. Leaving it nil serves the API only.
	WebHandler http.Handler

	AllowInsecure bool

	MaxBodyBytes    int64
	RequestTimeout  time.Duration
	ShutdownTimeout time.Duration

	SweepInterval    time.Duration
	PruneInterval    time.Duration
	DispatchInterval time.Duration
	DisableSweep     bool
	DisablePrune     bool
	DisableDispatch  bool
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
	pump := newPumps(context.Background(), hub, log, opts.EventPollInterval)
	hub.SetTenantHooks(pump.start, pump.stop)

	router, err := httpapi.New(httpapi.Config{
		Service:         opts.Service,
		EventHandler:    httpapi.NewEventStream(hub, log),
		WebHandler:      opts.WebHandler,
		Authenticator:   NewAuthenticator(opts.Store, opts.Clock),
		Logger:          opts.Logger,
		DefaultTenantID: opts.TenantID,
		MaxBodyBytes:    opts.MaxBodyBytes,
		RequestTimeout:  opts.RequestTimeout,
		SecureCookies:   opts.CertFile != "",
		Probe:           opts.Store,
		ExpectedSchema:  latest,
	})
	if err != nil {
		return nil, err
	}

	return New(Config{
		Addr:            opts.Addr,
		Handler:         router,
		Logger:          opts.Logger,
		CertFile:        opts.CertFile,
		KeyFile:         opts.KeyFile,
		AllowInsecure:   opts.AllowInsecure,
		ShutdownTimeout: opts.ShutdownTimeout,
		Workers:         workersFor(opts),
	})
}

// workersFor builds the enabled background workers.
func workersFor(opts Options) []Worker {
	var workers []Worker

	if !opts.DisableSweep {
		sweeper := lease.NewSweeper(
			func(ctx context.Context, limit int) (int, error) {
				return opts.Service.SweepLeases(systemContext(ctx, opts.TenantID), limit)
			},
			lease.WithClock(opts.Clock),
			lease.WithInterval(opts.SweepInterval),
			lease.WithErrorHandler(func(err error) {
				opts.Logger.Error("lease sweep failed", "error", err.Error())
			}),
		)
		workers = append(workers, SweeperWorker(sweeper))
	}

	if !opts.DisablePrune && opts.PruneInterval > 0 {
		pruner := retention.NewPruner(
			systemRunner{service: opts.Service, tenantID: opts.TenantID},
			opts.Clock,
			core.Duration(opts.PruneInterval),
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
		)
		workers = append(workers, DispatcherWorker(dispatcher))
	}
	return workers
}

// systemContext gives background work the identity it has no request to
// borrow from.
func systemContext(ctx context.Context, tenantID string) context.Context {
	return core.WithSource(core.WithActor(ctx, core.SystemActor(tenantID)), core.SourceSystem)
}

// systemRunner prunes as the system actor.
type systemRunner struct {
	service  core.Service
	tenantID string
}

// Prune removes records past their retention window.
func (r systemRunner) Prune(ctx context.Context, in core.PruneInput) (*core.PruneResult, error) {
	return r.service.Prune(systemContext(ctx, r.tenantID), in)
}
