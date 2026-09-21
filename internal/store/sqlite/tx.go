package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// dialect is the placeholder form every statement in this package is built with.
const dialect = sqlb.SQLite

// executor is the subset of database/sql shared by *sql.Conn and *sql.Tx.
type executor interface {
	ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
}

// tx is one transaction. A write transaction owns a connection from the
// single-connection writer pool and drives BEGIN IMMEDIATE by hand, so the write
// lock is taken up front rather than upgraded mid-transaction.
type tx struct {
	store *Store
	scope core.TenantScope
	conn  *sql.Conn
	inner *sql.Tx
	ex    executor
	write bool
	done  bool
}

func (s *Store) beginWrite(ctx context.Context, scope core.TenantScope) (*tx, error) {
	conn, err := s.writer.Conn(ctx)
	if err != nil {
		return nil, mapErr(err, "acquiring sqlite writer connection")
	}
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		_ = conn.Close()
		return nil, mapErr(err, "beginning immediate transaction")
	}
	return &tx{store: s, scope: scope, conn: conn, ex: conn, write: true}, nil
}

func (s *Store) beginRead(ctx context.Context, scope core.TenantScope) (*tx, error) {
	inner, err := s.reader.BeginTx(ctx, nil)
	if err != nil {
		return nil, mapErr(err, "beginning read transaction")
	}
	return &tx{store: s, scope: scope, inner: inner, ex: inner}, nil
}

// Scope returns the tenant this transaction is confined to.
func (t *tx) Scope() core.TenantScope { return t.scope }

// Commit makes the transaction's writes durable.
func (t *tx) Commit() error {
	if t.done {
		return core.Internal("transaction is already finished")
	}
	t.done = true
	if !t.write {
		if err := t.inner.Commit(); err != nil {
			return mapErr(err, "committing read transaction")
		}
		return nil
	}
	_, err := t.conn.ExecContext(context.Background(), "COMMIT")
	if err != nil {
		// A COMMIT can fail with the transaction still open, a violated
		// deferred constraint being the usual cause. Handing that connection
		// back to the single-slot writer pool leaves every later BEGIN
		// IMMEDIATE facing a transaction that is already running.
		_, _ = t.conn.ExecContext(context.Background(), "ROLLBACK")
	}
	closeErr := t.conn.Close()
	if err != nil {
		return mapErr(err, "committing transaction")
	}
	if closeErr != nil {
		return mapErr(closeErr, "releasing writer connection")
	}
	return nil
}

// Rollback discards the transaction's writes.
func (t *tx) Rollback() error {
	if t.done {
		return nil
	}
	t.done = true
	if !t.write {
		if err := t.inner.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			return mapErr(err, "rolling back read transaction")
		}
		return nil
	}
	_, err := t.conn.ExecContext(context.Background(), "ROLLBACK")
	closeErr := t.conn.Close()
	if err != nil {
		return mapErr(err, "rolling back transaction")
	}
	if closeErr != nil {
		return mapErr(closeErr, "releasing writer connection")
	}
	return nil
}

func (t *tx) now() string { return sqlb.TimeText(t.store.clock.Now()) }

func (t *tx) builder(table string) *sqlb.Builder {
	return sqlb.MustNew(dialect, t.scope, table)
}

func (t *tx) insert(table string) *sqlb.Insert {
	in, err := sqlb.NewInsert(dialect, t.scope, table)
	if err != nil {
		panic(err)
	}
	return in
}

func (t *tx) exec(ctx context.Context, q string, args []any, what string, whatArgs ...any) (sql.Result, error) {
	res, err := t.ex.ExecContext(ctx, q, args...)
	if err != nil {
		return nil, mapErr(err, what, whatArgs...)
	}
	return res, nil
}

func (t *tx) execInsert(ctx context.Context, in *sqlb.Insert, what string, whatArgs ...any) (sql.Result, error) {
	q, args, err := in.Query()
	if err != nil {
		return nil, err
	}
	return t.exec(ctx, q, args, what, whatArgs...)
}

func (t *tx) execUpdate(ctx context.Context, b *sqlb.Builder, what string, whatArgs ...any) (int64, error) {
	q, args, err := b.UpdateQuery()
	if err != nil {
		return 0, err
	}
	res, err := t.exec(ctx, q, args, what, whatArgs...)
	if err != nil {
		return 0, err
	}
	return rowsAffected(res, what)
}

func (t *tx) execDelete(ctx context.Context, b *sqlb.Builder, what string, whatArgs ...any) (int64, error) {
	q, args := b.DeleteQuery()
	res, err := t.exec(ctx, q, args, what, whatArgs...)
	if err != nil {
		return 0, err
	}
	return rowsAffected(res, what)
}

func (t *tx) query(ctx context.Context, b *sqlb.Builder, what string, whatArgs ...any) (*sql.Rows, error) {
	q, args := b.SelectQuery()
	rows, err := t.ex.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, mapErr(err, what, whatArgs...)
	}
	return rows, nil
}

func (t *tx) count(ctx context.Context, b *sqlb.Builder, what string, whatArgs ...any) (int, error) {
	q, args := b.CountQuery()
	var n int
	if err := t.ex.QueryRowContext(ctx, q, args...).Scan(&n); err != nil {
		return 0, mapErr(err, what, whatArgs...)
	}
	return n, nil
}

func rowsAffected(res sql.Result, what string) (int64, error) {
	n, err := res.RowsAffected()
	if err != nil {
		return 0, mapErr(err, "%s", what)
	}
	return n, nil
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface{ Scan(dest ...any) error }

// mapErr turns a driver error into a core error kind.
func mapErr(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	msg := fmt.Sprintf(format, args...)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return core.NotFound("%s", msg).Wrap(err)
	case isUnique(err):
		return core.Conflict("%s", msg).Wrap(err)
	case isBusy(err):
		return core.Conflict("%s: database is busy", msg).Wrap(err)
	case isForeignKey(err):
		return core.Precondition("%s: referenced row is missing", msg).Wrap(err)
	}
	return core.Internal("%s", msg).Wrap(err)
}

func isUnique(err error) bool {
	s := err.Error()
	return strings.Contains(s, "UNIQUE constraint failed") ||
		strings.Contains(s, "PRIMARY KEY constraint failed")
}

func isBusy(err error) bool {
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "database is locked") ||
		strings.Contains(s, "database table is locked") ||
		strings.Contains(s, "sqlite_busy")
}

func isForeignKey(err error) bool {
	return strings.Contains(err.Error(), "FOREIGN KEY constraint failed")
}

var (
	_ store.Tx         = (*tx)(nil)
	_ store.UnscopedTx = (*tx)(nil)
)
