// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/id"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

var idempotencyCols = []string{
	"id", "actor_id", "idem_key", "fingerprint", "state", "status", "content_type", "body",
	"created_at", "lease_expires_at", "expires_at",
}

// GetIdempotencyKey returns an actor's row for a key.
func (t *tx) GetIdempotencyKey(ctx context.Context, actorID, key string) (*store.IdempotencyRecord, error) {
	b := t.builder("idempotency_keys").
		Select(idempotencyCols...).
		Where("actor_id = ?", actorID).
		Where("idem_key = ?", key).
		Limit(1)
	q, args := b.SelectQuery()
	var (
		r                       store.IdempotencyRecord
		status                  int64
		created, lease, expires sql.NullString
	)
	err := t.ex.QueryRowContext(ctx, q, args...).Scan(&r.ID, &r.ActorID, &r.Key, &r.Fingerprint, &r.State,
		&status, &r.ContentType, &r.Body, &created, &lease, &expires)
	if err != nil {
		return nil, mapErr(err, "reading idempotency key")
	}
	r.Status = int(status)
	for _, f := range []struct {
		dst *time.Time
		src sql.NullString
	}{{&r.CreatedAt, created}, {&r.LeaseExpiresAt, lease}, {&r.ExpiresAt, expires}} {
		if *f.dst, err = sqlb.ScanTime(f.src); err != nil {
			return nil, err
		}
	}
	return &r, nil
}

// CreateIdempotencyKey inserts a row, assigning its identifier when empty.
func (t *tx) CreateIdempotencyKey(ctx context.Context, r *store.IdempotencyRecord) error {
	if r.ID == "" {
		r.ID = id.New()
	}
	ins := t.insert("idempotency_keys").
		Set("id", r.ID).
		Set("actor_id", r.ActorID).
		Set("idem_key", r.Key).
		Set("fingerprint", r.Fingerprint).
		Set("state", r.State).
		Set("status", r.Status).
		Set("content_type", r.ContentType).
		Set("body", r.Body).
		Set("created_at", sqlb.TimeText(r.CreatedAt)).
		Set("lease_expires_at", sqlb.TimeText(r.LeaseExpiresAt)).
		Set("expires_at", sqlb.TimeText(r.ExpiresAt))
	_, err := t.execInsert(ctx, ins, "reserving idempotency key")
	return err
}

// MarkIdempotencyExecuted moves a reserved row to executed.
func (t *tx) MarkIdempotencyExecuted(ctx context.Context, rowID string) error {
	up := t.builder("idempotency_keys").
		Where("id = ?", rowID).
		Where("state = ?", store.IdempotencyReserved).
		Set("state", store.IdempotencyExecuted)
	_, err := t.execUpdate(ctx, up, "marking idempotency key executed")
	return err
}

// FinishIdempotencyKey writes a row's state, response and expiry.
func (t *tx) FinishIdempotencyKey(ctx context.Context, r *store.IdempotencyRecord) error {
	up := t.builder("idempotency_keys").
		Where("id = ?", r.ID).
		Set("state", r.State).
		Set("status", r.Status).
		Set("content_type", r.ContentType).
		Set("body", r.Body).
		Set("expires_at", sqlb.TimeText(r.ExpiresAt))
	n, err := t.execUpdate(ctx, up, "keeping idempotent response")
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("idempotency key %q", r.ID)
	}
	return nil
}

// DeleteIdempotencyKey removes one row.
func (t *tx) DeleteIdempotencyKey(ctx context.Context, rowID string) error {
	b := t.builder("idempotency_keys").Where("id = ?", rowID)
	_, err := t.execDelete(ctx, b, "releasing idempotency key")
	return err
}

// PruneIdempotencyKeys deletes expired rows, oldest expiry first.
func (t *tx) PruneIdempotencyKeys(ctx context.Context, before time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = core.MaxPageLimit
	}
	inner := t.builder("idempotency_keys").
		Select("id").
		Where("expires_at < ?", sqlb.TimeText(before)).
		OrderBy("expires_at", core.Ascending).
		Limit(limit)
	sub, subArgs := inner.SelectQuery()
	b := t.builder("idempotency_keys").Where("id IN ("+sub+")", subArgs...)
	return t.execDelete(ctx, b, "pruning idempotency keys")
}

// CountExpiredIdempotencyKeys counts the rows that expired before the cutoff.
func (t *tx) CountExpiredIdempotencyKeys(ctx context.Context, before time.Time) (int, error) {
	b := t.builder("idempotency_keys").Where("expires_at < ?", sqlb.TimeText(before))
	return t.count(ctx, b, "counting expired idempotency keys")
}
