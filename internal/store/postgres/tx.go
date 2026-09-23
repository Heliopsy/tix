// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// dialect is the placeholder form every statement in this package is built with.
const dialect = sqlb.Postgres

// embedded renders a subquery with ? markers, because a subquery that is spliced
// into another builder's statement is renumbered by that outer statement.
const embedded = sqlb.SQLite

// executor is the subset of database/sql shared by *sql.DB, *sql.Conn and *sql.Tx.
type executor interface {
	ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
}

// tx is one transaction. The tenant it is scoped to is pushed into a
// transaction-local setting that the row-level security policies read, so a bug
// in the query builder still cannot cross tenants.
type tx struct {
	store  *Store
	scope  core.TenantScope
	inner  *sql.Tx
	ex     executor
	done   bool
	notify bool
}

func (s *Store) begin(ctx context.Context, scope core.TenantScope, readOnly bool) (*tx, error) {
	inner, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: readOnly})
	if err != nil {
		return nil, mapErr(err, "beginning transaction")
	}
	if _, err := inner.ExecContext(ctx,
		`SELECT set_config($1, $2, true)`, tenantSetting, scope.TenantID); err != nil {
		_ = inner.Rollback()
		return nil, mapErr(err, "scoping the transaction to tenant %q", scope.TenantID)
	}
	if s.appRole {
		if _, err := inner.ExecContext(ctx, "SET LOCAL ROLE "+appRoleName); err != nil {
			_ = inner.Rollback()
			return nil, mapErr(err, "entering the %q role", appRoleName)
		}
	}
	return &tx{store: s, scope: scope, inner: inner, ex: inner}, nil
}

// Scope returns the tenant this transaction is confined to.
func (t *tx) Scope() core.TenantScope { return t.scope }

// Commit makes the transaction's writes durable, waking event listeners.
func (t *tx) Commit() error {
	if t.done {
		return core.Internal("transaction is already finished")
	}
	t.done = true
	if t.notify {
		if _, err := t.ex.ExecContext(context.Background(),
			`SELECT pg_notify($1, $2)`, eventsChannel, t.scope.TenantID); err != nil {
			_ = t.inner.Rollback()
			return mapErr(err, "announcing new events")
		}
	}
	if err := t.inner.Commit(); err != nil {
		return mapErr(err, "committing transaction")
	}
	return nil
}

// Rollback discards the transaction's writes.
func (t *tx) Rollback() error {
	if t.done {
		return nil
	}
	t.done = true
	if err := t.inner.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return mapErr(err, "rolling back transaction")
	}
	return nil
}

func (t *tx) now() time.Time { return t.store.clock.Now().UTC() }

func (t *tx) builder(table string) *sqlb.Builder {
	return sqlb.MustNew(dialect, t.scope, table)
}

// embed starts a subquery whose placeholders the enclosing statement renumbers.
func (t *tx) embed(table string) *sqlb.Builder {
	return sqlb.MustNew(embedded, t.scope, table)
}

func (t *tx) insert(table string) *sqlb.Insert {
	in, err := sqlb.NewInsert(dialect, t.scope, table)
	if err != nil {
		panic(err)
	}
	return in
}

// statementSavepoint keeps one failed statement from poisoning the whole
// transaction. PostgreSQL aborts a transaction on any error, where SQLite lets
// the caller carry on, so every write is bracketed to keep the two engines
// behaving the same from the caller's side.
const statementSavepoint = "tix_stmt"

func (t *tx) exec(ctx context.Context, q string, args []any, what string, whatArgs ...any) (sql.Result, error) {
	if err := t.savepoint(ctx, what, whatArgs...); err != nil {
		return nil, err
	}
	res, err := t.ex.ExecContext(ctx, q, args...)
	if err != nil {
		t.rollbackToSavepoint(ctx)
		return nil, mapErr(err, what, whatArgs...)
	}
	t.releaseSavepoint(ctx)
	return res, nil
}

func (t *tx) savepoint(ctx context.Context, what string, whatArgs ...any) error {
	if _, err := t.ex.ExecContext(ctx, "SAVEPOINT "+statementSavepoint); err != nil {
		return mapErr(err, what, whatArgs...)
	}
	return nil
}

func (t *tx) rollbackToSavepoint(ctx context.Context) {
	_, _ = t.ex.ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+statementSavepoint)
}

func (t *tx) releaseSavepoint(ctx context.Context) {
	_, _ = t.ex.ExecContext(ctx, "RELEASE SAVEPOINT "+statementSavepoint)
}

func (t *tx) execInsert(ctx context.Context, in *sqlb.Insert, what string, whatArgs ...any) (sql.Result, error) {
	q, args, err := in.Query()
	if err != nil {
		return nil, err
	}
	return t.exec(ctx, q, args, what, whatArgs...)
}

// insertReturning inserts one row and reads a generated column back, which is
// how a sequence is read here; there is no last insert identifier.
func (t *tx) insertReturning(ctx context.Context, in *sqlb.Insert, column string, dest any, what string, whatArgs ...any) error {
	q, args, err := in.Query()
	if err != nil {
		return err
	}
	if err := t.savepoint(ctx, what, whatArgs...); err != nil {
		return err
	}
	if err := t.ex.QueryRowContext(ctx, q+" RETURNING "+column, args...).Scan(dest); err != nil {
		t.rollbackToSavepoint(ctx)
		return mapErr(err, what, whatArgs...)
	}
	t.releaseSavepoint(ctx)
	return nil
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

// pickLocked reads identifiers with a row lock that a competing worker steps
// over, which is how two processes divide a queue without overlapping.
func (t *tx) pickLocked(ctx context.Context, b *sqlb.Builder, what string) ([]string, error) {
	q, args := b.SelectQuery()
	rows, err := t.ex.QueryContext(ctx, q+skipLocked, args...)
	if err != nil {
		return nil, mapErr(err, "%s", what)
	}
	defer func() { _ = rows.Close() }()

	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, mapErr(err, "%s", what)
		}
		out = append(out, id)
	}
	return out, mapRowsErr(rows, what)
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

// SQLSTATE codes this engine maps onto core error kinds.
const (
	codeUniqueViolation       = "23505"
	codeForeignKeyViolation   = "23503"
	codeNotNullViolation      = "23502"
	codeCheckViolation        = "23514"
	codeSerializationFailed   = "40001"
	codeDeadlockDetected      = "40P01"
	codeInsufficientPrivilege = "42501"
)

// mapErr turns a driver error into a core error kind.
func mapErr(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	msg := fmt.Sprintf(format, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return core.NotFound("%s", msg).Wrap(err)
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case codeUniqueViolation:
			return core.Conflict("%s", msg).Wrap(err)
		case codeForeignKeyViolation:
			return core.Precondition("%s: referenced row is missing", msg).Wrap(err)
		case codeSerializationFailed, codeDeadlockDetected:
			return core.Conflict("%s: the transaction conflicted with another", msg).Wrap(err)
		case codeNotNullViolation, codeCheckViolation:
			return core.Invalid("%s: %s", msg, pg.Message).Wrap(err)
		case codeInsufficientPrivilege:
			return core.Precondition("%s: %s", msg, pg.Message).Wrap(err)
		}
	}
	return core.Internal("%s", msg).Wrap(err)
}

func mapRowsErr(rows *sql.Rows, what string) error {
	if err := rows.Err(); err != nil {
		return mapErr(err, "%s", what)
	}
	return nil
}

var (
	_ store.Tx         = (*tx)(nil)
	_ store.UnscopedTx = (*tx)(nil)
)
