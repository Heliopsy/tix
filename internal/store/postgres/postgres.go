// SPDX-License-Identifier: AGPL-3.0-or-later

// Package postgres implements the store on PostgreSQL through pgx.
//
// pgx is driven through database/sql rather than through its native pool, so the
// shared migration runner, the shared query builder and the executor shape the
// SQLite engine uses all apply unchanged; the pgx connection underneath is still
// reached through Conn.Raw where LISTEN/NOTIFY needs it.
package postgres

import (
	"context"
	"database/sql"
	"net/url"
	"strings"
	"time"

	// The pgx driver is pure Go, so builds stay CGO_ENABLED=0.
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/migrations"
)

// driverName is the name pgx/v5/stdlib registers itself under.
const driverName = "pgx"

// Pool defaults. A caller that needs different limits sets them on the DSN.
const (
	MaxOpenConns    = 16
	MaxIdleConns    = 8
	ConnMaxLifetime = 30 * time.Minute
	ConnMaxIdleTime = 5 * time.Minute
)

// DefaultConnectTimeout bounds the startup reachability check when a caller
// passes no timeout of its own.
const DefaultConnectTimeout = 15 * time.Second

// openConfig holds the options Open accepts.
type openConfig struct {
	connectTimeout time.Duration
}

// Option configures Open.
type Option func(*openConfig)

// WithConnectTimeout bounds how long Open waits for the database to answer its
// startup check before calling it unreachable. A non-positive value keeps
// DefaultConnectTimeout, so a caller that has not decided cannot accidentally
// ask for no wait at all.
func WithConnectTimeout(d time.Duration) Option {
	return func(c *openConfig) {
		if d > 0 {
			c.connectTimeout = d
		}
	}
}

// Store is a PostgreSQL-backed store.
type Store struct {
	db      *sql.DB
	clock   clock.Clock
	dsn     string
	appRole bool
}

// Open connects to the PostgreSQL database named by dsn. It refuses to return
// a store it could not reach within the connect timeout, which
// WithConnectTimeout sets and database.connect_timeout carries.
func Open(dsn string, c clock.Clock, opts ...Option) (*Store, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, core.Invalid("postgres dsn must not be empty")
	}
	if c == nil {
		c = clock.New()
	}
	cfg := openConfig{connectTimeout: DefaultConnectTimeout}
	for _, opt := range opts {
		opt(&cfg)
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, core.Internal("opening postgres at %q", redact(dsn)).Wrap(err)
	}
	db.SetMaxOpenConns(MaxOpenConns)
	db.SetMaxIdleConns(MaxIdleConns)
	db.SetConnMaxLifetime(ConnMaxLifetime)
	db.SetConnMaxIdleTime(ConnMaxIdleTime)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.connectTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, core.Internal("postgres at %q is unreachable within %s", redact(dsn), cfg.connectTimeout).Wrap(err)
	}

	s := &Store{db: db, clock: c, dsn: dsn}
	s.appRole = s.roleUsable(ctx)
	return s, nil
}

// Dialect names the engine in use.
func (s *Store) Dialect() store.Dialect { return store.Postgres }

// Close releases the pool.
func (s *Store) Close() error {
	if err := s.db.Close(); err != nil {
		return core.Internal("closing postgres at %q", redact(s.dsn)).Wrap(err)
	}
	return nil
}

// Health reports whether the pool can reach the database.
func (s *Store) Health(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return core.Internal("postgres at %q is unreachable", redact(s.dsn)).Wrap(err)
	}
	return nil
}

// Migrate applies every pending migration and the engine-specific adjustments.
func (s *Store) Migrate(ctx context.Context) error {
	if err := runMigrations(ctx, s.db, s.clock); err != nil {
		return err
	}
	s.grantAppRole(ctx)
	s.appRole = s.roleUsable(ctx)
	return nil
}

// SchemaVersion returns the highest applied migration version.
func (s *Store) SchemaVersion(ctx context.Context) (int, error) {
	v, err := migrations.Current(ctx, s.db)
	if err != nil {
		return 0, core.Internal("reading schema version of %q", redact(s.dsn)).Wrap(err)
	}
	return v, nil
}

// Begin starts a read-write transaction scoped to one tenant.
func (s *Store) Begin(ctx context.Context, scope core.TenantScope) (store.Tx, error) {
	if !scope.Valid() {
		return nil, core.Invalid("refusing to open a transaction without a tenant scope")
	}
	return s.begin(ctx, scope, false)
}

// View runs fn in a read-only transaction.
func (s *Store) View(ctx context.Context, scope core.TenantScope, fn func(store.Tx) error) error {
	if !scope.Valid() {
		return core.Invalid("refusing to open a transaction without a tenant scope")
	}
	t, err := s.begin(ctx, scope, true)
	if err != nil {
		return err
	}
	// Rollback is a no-op once the transaction is finished, so this releases
	// the connection however fn leaves: returning, or panicking.
	defer func() { _ = t.Rollback() }()
	if err := fn(t); err != nil {
		return err
	}
	return t.Commit()
}

// Update runs fn in a read-write transaction, committing on success.
func (s *Store) Update(ctx context.Context, scope core.TenantScope, fn func(store.Tx) error) error {
	if !scope.Valid() {
		return core.Invalid("refusing to open a transaction without a tenant scope")
	}
	t, err := s.begin(ctx, scope, false)
	if err != nil {
		return err
	}
	// Rollback is a no-op once the transaction is finished, so this releases
	// the connection however fn leaves: returning, or panicking.
	defer func() { _ = t.Rollback() }()
	if err := fn(t); err != nil {
		return err
	}
	return t.Commit()
}

// Unscoped runs fn in a transaction that reaches across tenants.
func (s *Store) Unscoped(ctx context.Context, fn func(store.UnscopedTx) error) error {
	t, err := s.begin(ctx, core.TenantScope{}, false)
	if err != nil {
		return err
	}
	defer func() { _ = t.Rollback() }()
	if err := fn(t); err != nil {
		return err
	}
	return t.Commit()
}

// redact removes the password from a DSN so it is safe to put in an error.
func redact(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil || u.User == nil {
		return dsn
	}
	u.User = url.User(u.User.Username())
	return u.String()
}

var _ store.Store = (*Store)(nil)
