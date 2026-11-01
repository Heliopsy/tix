// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/heliopsy/tix/internal/core"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

var calendarFeedColumns = []string{"id", "tenant_id", "actor_id", "filter", "created_at", "revoked_at"}

func scanCalendarFeed(s scanner) (*core.CalendarFeed, error) {
	var (
		f       core.CalendarFeed
		created sql.NullString
		revoked sql.NullString
	)
	if err := s.Scan(&f.ID, &f.TenantID, &f.ActorID, &f.Filter, &created, &revoked); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, core.NotFound("calendar feed not found")
		}
		return nil, mapErr(err, "scanning calendar feed")
	}
	var err error
	if f.CreatedAt, err = sqlb.ScanTime(created); err != nil {
		return nil, err
	}
	if f.RevokedAt, err = sqlb.ScanNullTime(revoked); err != nil {
		return nil, err
	}
	return &f, nil
}

// CreateCalendarFeed stores a feed and the hash of its secret.
func (t *tx) CreateCalendarFeed(ctx context.Context, f *core.CalendarFeed, tokenHash string) error {
	f.TenantID = t.scope.TenantID
	ins := t.insert("calendar_feeds").
		Set("id", f.ID).
		Set("actor_id", f.ActorID).
		Set("token_hash", tokenHash).
		Set("filter", f.Filter).
		Set("created_at", sqlb.TimeText(f.CreatedAt))
	_, err := t.execInsert(ctx, ins, "creating calendar feed")
	return err
}

// LiveCalendarFeed returns an actor's unrevoked feed.
func (t *tx) LiveCalendarFeed(ctx context.Context, actorID string) (*core.CalendarFeed, error) {
	return t.oneCalendarFeed(ctx, t.builder("calendar_feeds").Where("actor_id = ?", actorID))
}

// CalendarFeedByHash returns the unrevoked feed behind a secret's hash.
func (t *tx) CalendarFeedByHash(ctx context.Context, tokenHash string) (*core.CalendarFeed, error) {
	return t.oneCalendarFeed(ctx, t.builder("calendar_feeds").Where("token_hash = ?", tokenHash))
}

func (t *tx) oneCalendarFeed(ctx context.Context, b *sqlb.Builder) (*core.CalendarFeed, error) {
	q, args := b.Select(calendarFeedColumns...).Where("revoked_at IS NULL").Limit(1).SelectQuery()
	return scanCalendarFeed(t.ex.QueryRowContext(ctx, q, args...))
}

// SetCalendarFeedFilter replaces a live feed's filter.
func (t *tx) SetCalendarFeedFilter(ctx context.Context, id, filter string) error {
	b := t.builder("calendar_feeds").Where("id = ?", id).Where("revoked_at IS NULL").Set("filter", filter)
	return t.oneCalendarRow(ctx, b, "setting the filter of calendar feed %q", id)
}

// RevokeCalendarFeed marks a live feed unusable from the given instant.
func (t *tx) RevokeCalendarFeed(ctx context.Context, id string, at time.Time) error {
	b := t.builder("calendar_feeds").Where("id = ?", id).Where("revoked_at IS NULL").Set("revoked_at", sqlb.TimeText(at))
	return t.oneCalendarRow(ctx, b, "revoking calendar feed %q", id)
}

func (t *tx) oneCalendarRow(ctx context.Context, b *sqlb.Builder, what, id string) error {
	n, err := t.execUpdate(ctx, b, what, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("calendar feed not found")
	}
	return nil
}
