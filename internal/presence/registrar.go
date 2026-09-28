// SPDX-License-Identifier: AGPL-3.0-or-later

// Package presence keeps a running server's row in the installation's registry
// of server processes fresh, so a reader anywhere can tell how many servers are
// up.
//
// Nothing here is tenant state. A server answers requests for every tenant the
// database holds, so its row belongs to none of them and is reached through
// store.UnscopedTx. confinement_test.go names the files that may do so.
//
// A heartbeat writes no audit entry and no outbox event. Both of those tables
// are tenant-scoped, and there is no tenant to attribute process liveness to;
// choosing one would put infrastructure a tenant does not own into that
// tenant's own audit trail. The volume argument stands behind that and is not
// small either: two servers beating every thirty seconds is nearly six thousand
// rows a day each, against a task table that may hold a few thousand for its
// whole life.
package presence

import (
	"context"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/id"
	"github.com/heliopsy/tix/internal/store"
)

// Config describes the process a registrar speaks for.
type Config struct {
	Store store.Store
	// Address is read when the row is written rather than when the registrar
	// is built, because a server told to listen on port zero does not know its
	// own address until the listener is open.
	Address func() string
	Version string
	// Surfaces are the surfaces this process actually serves, which is a
	// property of how it was configured rather than of the build.
	Surfaces []core.ServerSurface
}

// Registrar keeps one process's row fresh for as long as the process runs.
type Registrar struct {
	store    store.Store
	address  func() string
	clock    clock.Clock
	interval time.Duration
	forget   time.Duration
	onError  func(error)

	server core.Server
}

// Option configures a Registrar.
type Option func(*Registrar)

// WithClock sets the time source.
func WithClock(c clock.Clock) Option {
	return func(r *Registrar) {
		if c != nil {
			r.clock = c
		}
	}
}

// WithInterval sets how often Run heartbeats.
func WithInterval(d time.Duration) Option {
	return func(r *Registrar) {
		if d > 0 {
			r.interval = d
		}
	}
}

// WithForgetAfter sets how long an unseen row is kept before a registration
// deletes it.
func WithForgetAfter(d time.Duration) Option {
	return func(r *Registrar) {
		if d > 0 {
			r.forget = d
		}
	}
}

// WithErrorHandler sets what a running registrar does with a failed beat. A
// beat that fails is not fatal: the next one may succeed, and until it does a
// reader correctly calls this server stale.
func WithErrorHandler(fn func(error)) Option { return func(r *Registrar) { r.onError = fn } }

// New builds a registrar for one process. The identifier is generated here
// rather than taken from the host, because two servers on one machine are the
// case the whole table exists to tell apart.
func New(cfg Config, opts ...Option) *Registrar {
	r := &Registrar{
		store:    cfg.Store,
		address:  cfg.Address,
		clock:    clock.New(),
		interval: core.ServerHeartbeatInterval,
		forget:   core.ServerForgetAfter,
		server: core.Server{
			ID:       id.New(),
			Version:  cfg.Version,
			Surfaces: cfg.Surfaces,
		},
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// ServerID names the process this registrar speaks for.
func (r *Registrar) ServerID() string { return r.server.ID }

// Interval reports how often Run heartbeats.
func (r *Registrar) Interval() time.Duration { return r.interval }

// Register writes the row and forgets rows nobody has seen for a long time.
//
// The purge rides along here rather than running on its own ticker because it
// is hygiene and nothing a reader sees depends on it: a row that survives it
// still reads as stale, and an installation that never restarts a server never
// needs it.
func (r *Registrar) Register(ctx context.Context) error {
	now := r.clock.Now()
	// The first attempt fixes the start. A registration that had to be retried
	// still belongs to a process that started when it started, and reporting
	// the retry instead would understate every uptime that followed a busy
	// database.
	if r.server.StartedAt.IsZero() {
		r.server.StartedAt = now
	}
	r.server.LastSeenAt = now
	if r.address != nil {
		r.server.Address = r.address()
	}
	return r.store.Unscoped(ctx, func(tx store.UnscopedTx) error {
		if _, err := tx.ForgetServersBefore(ctx, now.Add(-r.forget)); err != nil {
			return err
		}
		return tx.RegisterServer(ctx, &r.server)
	})
}

// Beat refreshes the row's last seen instant.
func (r *Registrar) Beat(ctx context.Context) error {
	at := r.clock.Now()
	return r.store.Unscoped(ctx, func(tx store.UnscopedTx) error {
		return tx.HeartbeatServer(ctx, r.server.ID, at)
	})
}

// Deregister removes the row, for a shutdown this process is running itself.
//
// It deliberately takes its own context. By the time this is called the
// server's context has usually been cancelled, and a removal that gave up
// because of that would leave a row behind that reads as a crash, which is the
// one thing a graceful shutdown must not look like.
func (r *Registrar) Deregister(ctx context.Context) error {
	return r.store.Unscoped(ctx, func(tx store.UnscopedTx) error {
		return tx.DeregisterServer(ctx, r.server.ID)
	})
}

// Run registers, heartbeats until the context ends, and removes the row on the
// way out.
func (r *Registrar) Run(ctx context.Context) error {
	// A registration that cannot be written is not fatal, and returning here
	// was. The database being busy is the ordinary contended case rather than
	// a fault: two servers on one SQLite file serialize their writes, and one
	// of them meets SQLITE_BUSY on the way in. Giving up left a server running
	// correctly and absent from `tix status` for as long as it lived, with no
	// further attempt, which is the one failure a status command must not
	// have: under-reporting what is running reads as "that server is down".
	//
	// So it retries on the same ticker the heartbeat uses. Beat only updates a
	// row, so it cannot stand in for the registration that never landed.
	err := r.Register(ctx)
	registered := err == nil
	if !registered && r.onError != nil {
		r.onError(err)
	}
	defer r.removeOnStop()

	ticker := r.clock.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C():
			if !registered {
				if err := r.Register(ctx); err != nil {
					if r.onError != nil {
						r.onError(err)
					}
					continue
				}
				registered = true
				continue
			}
			if err := r.Beat(ctx); err != nil && r.onError != nil {
				r.onError(err)
			}
		}
	}
}

// removeOnStop takes the row out on a context of its own, bounded so a
// shutdown cannot hang on an unreachable database.
func (r *Registrar) removeOnStop() {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(context.Background()), shutdownBudget)
	defer cancel()
	if err := r.Deregister(ctx); err != nil && r.onError != nil {
		r.onError(err)
	}
}

// shutdownBudget bounds the removal a stopping server performs.
const shutdownBudget = 5 * time.Second
