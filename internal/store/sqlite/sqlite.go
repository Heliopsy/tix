// SPDX-License-Identifier: AGPL-3.0-or-later

// Package sqlite implements the store on SQLite through a pure Go driver.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"runtime"
	"strings"

	// The pure Go driver keeps every build CGO_ENABLED=0.
	_ "modernc.org/sqlite"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/migrations"
	"github.com/heliopsy/tix/internal/storeloc"
)

// BusyTimeoutMillis is how long a blocked writer waits for the lock before failing.
const BusyTimeoutMillis = 5000

// driverName is the name modernc.org/sqlite registers itself under.
const driverName = "sqlite"

// Store is a SQLite-backed store with a single-connection writer pool beside a
// multi-connection reader pool.
type Store struct {
	reader *sql.DB
	writer *sql.DB
	clock  clock.Clock
	path   string
}

// openConfig holds the options Open accepts.
type openConfig struct {
	allowNetworkFS bool
}

// Option configures Open.
type Option func(*openConfig)

// WithAllowNetworkFS overrides the refusal to open a database that has been
// detected on a network filesystem, such as NFS, SMB/CIFS or a network FUSE
// mount. The default is to refuse, because such placement corrupts a SQLite
// database rather than merely degrading its performance. Only pass true when
// the operator has explicitly accepted that risk, for example through a
// dedicated flag or configuration key.
func WithAllowNetworkFS(allow bool) Option {
	return func(c *openConfig) { c.allowNetworkFS = allow }
}

// Open opens, creating if needed, a SQLite database at path.
//
// Before opening anything, it refuses a path detected on a network
// filesystem: NFS, SMB/CIFS, a network FUSE mount such as sshfs, WebDAV, or,
// on Windows, a UNC path or a mapped network drive. This refusal happens
// here, not only in a diagnostic command, so it cannot be bypassed by simply
// not running one. Pass WithAllowNetworkFS(true) to override it. When the
// filesystem cannot be classified at all, on an unsupported platform or
// because the check itself failed, Open allows the database rather than
// refusing it: a detection that cannot speak should not overrule one that
// can.
func Open(path string, c clock.Clock, opts ...Option) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, core.Invalid("sqlite path must not be empty")
	}
	var cfg openConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	if dec := storeloc.CheckNetworkFS(path, cfg.allowNetworkFS); dec.Refuse {
		return nil, core.Precondition(
			"sqlite database %q is on a %s, which corrupts a SQLite database rather than merely running it slowly; "+
				"keep the database on local disk, or use PostgreSQL for a shared deployment; "+
				"pass --allow-network-fs, or set database.allow_network_fs, if you have accepted this risk",
			path, dec.Kind)
	}
	if c == nil {
		c = clock.New()
	}

	writer, err := sql.Open(driverName, dsn(path))
	if err != nil {
		return nil, core.Internal("opening sqlite writer at %q", path).Wrap(err)
	}
	writer.SetMaxOpenConns(1)
	writer.SetMaxIdleConns(1)

	reader, err := sql.Open(driverName, dsn(path))
	if err != nil {
		_ = writer.Close()
		return nil, core.Internal("opening sqlite reader at %q", path).Wrap(err)
	}
	reader.SetMaxOpenConns(readerConns())
	reader.SetMaxIdleConns(readerConns())

	s := &Store{reader: reader, writer: writer, clock: c, path: path}
	if err := s.verifyJournalMode(context.Background()); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

// dsn builds a connection string whose pragmas are applied to every connection
// the pool opens, not only the first.
func dsn(path string) string {
	return "file:" + path +
		"?_pragma=busy_timeout(" + fmt.Sprint(BusyTimeoutMillis) + ")" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=foreign_keys(1)" +
		"&_pragma=synchronous(NORMAL)"
}

func readerConns() int {
	if n := runtime.NumCPU(); n > 4 {
		return n
	}
	return 4
}

// verifyJournalMode reports read-only media clearly instead of letting a later
// write fail with an opaque driver error.
func (s *Store) verifyJournalMode(ctx context.Context) error {
	var mode string
	if err := s.writer.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil {
		return core.Internal("reading journal mode for %q", s.path).Wrap(err)
	}
	if !strings.EqualFold(mode, "wal") {
		return core.Precondition(
			"sqlite at %q is in %q journal mode; write-ahead logging could not be established, which usually means the file or its directory is not writable",
			s.path, mode)
	}
	return nil
}

// Dialect names the engine in use.
func (s *Store) Dialect() store.Dialect { return store.SQLite }

// Close releases both pools.
func (s *Store) Close() error {
	var first error
	if err := s.reader.Close(); err != nil {
		first = err
	}
	if err := s.writer.Close(); err != nil && first == nil {
		first = err
	}
	if first != nil {
		return core.Internal("closing sqlite store at %q", s.path).Wrap(first)
	}
	return nil
}

// Health reports whether both pools can reach the database.
func (s *Store) Health(ctx context.Context) error {
	if err := s.reader.PingContext(ctx); err != nil {
		return core.Internal("sqlite reader at %q is unreachable", s.path).Wrap(err)
	}
	if err := s.writer.PingContext(ctx); err != nil {
		return core.Internal("sqlite writer at %q is unreachable", s.path).Wrap(err)
	}
	return nil
}

// Migrate applies every pending migration.
func (s *Store) Migrate(ctx context.Context) error {
	if _, err := migrations.Run(ctx, s.writer); err != nil {
		return core.Internal("migrating sqlite at %q", s.path).Wrap(err)
	}
	return nil
}

// SchemaVersion returns the highest applied migration version.
func (s *Store) SchemaVersion(ctx context.Context) (int, error) {
	v, err := migrations.Current(ctx, s.writer)
	if err != nil {
		return 0, core.Internal("reading schema version of %q", s.path).Wrap(err)
	}
	return v, nil
}

// Begin starts a read-write transaction scoped to one tenant. The writer pool
// holds one connection, so a write transaction must be finished before another
// is opened on the same goroutine.
func (s *Store) Begin(ctx context.Context, scope core.TenantScope) (store.Tx, error) {
	if !scope.Valid() {
		return nil, core.Invalid("refusing to open a transaction without a tenant scope")
	}
	return s.beginWrite(ctx, scope)
}

// View runs fn inside a read transaction served by the reader pool.
func (s *Store) View(ctx context.Context, scope core.TenantScope, fn func(store.Tx) error) error {
	if !scope.Valid() {
		return core.Invalid("refusing to open a transaction without a tenant scope")
	}
	t, err := s.beginRead(ctx, scope)
	if err != nil {
		return err
	}
	// Rollback is a no-op once the transaction is finished, so this releases
	// the connection however fn leaves: returning, or panicking. Without it a
	// panic recovered upstream drained the reader pool one connection at a
	// time until every read blocked.
	defer func() { _ = t.Rollback() }()
	if err := fn(t); err != nil {
		return err
	}
	return t.Commit()
}

// Update runs fn inside a write transaction, committing on success.
func (s *Store) Update(ctx context.Context, scope core.TenantScope, fn func(store.Tx) error) error {
	if !scope.Valid() {
		return core.Invalid("refusing to open a transaction without a tenant scope")
	}
	t, err := s.beginWrite(ctx, scope)
	if err != nil {
		return err
	}
	// Rollback is a no-op once the transaction is finished, so this releases
	// the connection however fn leaves: returning, or panicking. Without it a
	// panic recovered upstream left the single writer connection checked out
	// for the life of the process, and every later write blocked on it.
	defer func() { _ = t.Rollback() }()
	if err := fn(t); err != nil {
		return err
	}
	return t.Commit()
}

// Unscoped runs fn in a write transaction that reaches across tenants.
func (s *Store) Unscoped(ctx context.Context, fn func(store.UnscopedTx) error) error {
	t, err := s.beginWrite(ctx, core.TenantScope{})
	if err != nil {
		return err
	}
	defer func() { _ = t.Rollback() }()
	if err := fn(t); err != nil {
		return err
	}
	return t.Commit()
}

var _ store.Store = (*Store)(nil)
