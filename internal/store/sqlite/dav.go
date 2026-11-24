// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

var davPasswordColumns = []string{"token_id", "actor_id", "name", "collections", "read_only", "created_at"}

func scanDAVPassword(s scanner) (*core.DAVPassword, error) {
	var (
		p        core.DAVPassword
		readOnly int64
		created  sql.NullString
	)
	if err := s.Scan(&p.ID, &p.ActorID, &p.Name, &p.Collections, &readOnly, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, core.NotFound("app password not found")
		}
		return nil, mapErr(err, "scanning app password")
	}
	p.ReadOnly = sqlb.Bool(readOnly)
	var err error
	if p.CreatedAt, err = sqlb.ScanTime(created); err != nil {
		return nil, err
	}
	return &p, nil
}

// CreateDAVPassword stores what an app password exposes.
func (t *tx) CreateDAVPassword(ctx context.Context, p *core.DAVPassword) error {
	ins := t.insert("dav_passwords").
		Set("token_id", p.ID).
		Set("actor_id", p.ActorID).
		Set("name", p.Name).
		Set("collections", string(p.Collections)).
		Set("read_only", sqlb.BoolInt(p.ReadOnly)).
		Set("created_at", sqlb.TimeText(p.CreatedAt))
	_, err := t.execInsert(ctx, ins, "creating app password %q", p.Name)
	return err
}

// GetDAVPassword returns the settings of the app password behind a token.
func (t *tx) GetDAVPassword(ctx context.Context, tokenID string) (*core.DAVPassword, error) {
	q, args := t.builder("dav_passwords").Select(davPasswordColumns...).Where("token_id = ?", tokenID).
		Limit(1).SelectQuery()
	return scanDAVPassword(t.ex.QueryRowContext(ctx, q, args...))
}

// ListDAVPasswords returns an actor's app password settings, oldest first.
func (t *tx) ListDAVPasswords(ctx context.Context, actorID string) ([]core.DAVPassword, error) {
	b := t.builder("dav_passwords").Select(davPasswordColumns...).Where("actor_id = ?", actorID).
		OrderBy("created_at", core.Ascending).OrderBy("token_id", core.Ascending)
	rows, err := t.query(ctx, b, "listing app passwords")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []core.DAVPassword{}
	for rows.Next() {
		p, err := scanDAVPassword(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, mapRowsErr(rows, "listing app passwords")
}

var davSyncColumns = []string{"token_id", "actor_id", "collection", "set_hash", "members", "event_seq",
	"event_mark", "config", "valid_until", "seen_at"}

// GetDAVSyncState returns the snapshot a sync token names.
func (t *tx) GetDAVSyncState(ctx context.Context, tokenID, collection, setHash string) (*store.DAVSyncState, error) {
	q, args := t.davSyncRow(tokenID, collection, setHash).Select(davSyncColumns...).Limit(1).SelectQuery()
	var (
		s       store.DAVSyncState
		members string
		seen    sql.NullString
		until   sql.NullString
	)
	err := t.ex.QueryRowContext(ctx, q, args...).Scan(&s.TokenID, &s.ActorID, &s.Collection, &s.SetHash, &members,
		&s.EventSeq, &s.EventMark, &s.Config, &until, &seen)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, core.NotFound("sync state not found")
	}
	if err != nil {
		return nil, mapErr(err, "reading sync state")
	}
	if err := json.Unmarshal([]byte(members), &s.Members); err != nil {
		return nil, core.Internal("reading the members of a sync state").Wrap(err)
	}
	if s.ValidUntil, err = sqlb.ScanTime(until); err != nil {
		return nil, err
	}
	if s.SeenAt, err = sqlb.ScanTime(seen); err != nil {
		return nil, err
	}
	return &s, nil
}

func (t *tx) davSyncRow(tokenID, collection, setHash string) *sqlb.Builder {
	return t.builder("dav_sync_states").Where("token_id = ?", tokenID).Where("collection = ?", collection).
		Where("set_hash = ?", setHash)
}

// PutDAVSyncState stores a snapshot, replacing the one of the same set.
func (t *tx) PutDAVSyncState(ctx context.Context, s *store.DAVSyncState) error {
	members, err := json.Marshal(s.Members)
	if err != nil {
		return core.Internal("encoding the members of a sync state").Wrap(err)
	}
	upd := t.davSyncRow(s.TokenID, s.Collection, s.SetHash).
		Set("event_seq", s.EventSeq).Set("event_mark", s.EventMark).Set("config", s.Config).
		Set("valid_until", sqlb.TimeText(s.ValidUntil)).Set("seen_at", sqlb.TimeText(s.SeenAt))
	n, err := t.execUpdate(ctx, upd, "updating sync state")
	if err != nil || n > 0 {
		return err
	}
	ins := t.insert("dav_sync_states").
		Set("token_id", s.TokenID).Set("actor_id", s.ActorID).Set("collection", s.Collection).
		Set("set_hash", s.SetHash).Set("members", string(members)).
		Set("event_seq", s.EventSeq).Set("event_mark", s.EventMark).Set("config", s.Config).
		Set("valid_until", sqlb.TimeText(s.ValidUntil)).Set("seen_at", sqlb.TimeText(s.SeenAt))
	_, err = t.execInsert(ctx, ins, "storing sync state")
	return err
}

// PruneDAVSyncStates removes snapshots seen before an instant, except the
// keep most recently seen.
func (t *tx) PruneDAVSyncStates(ctx context.Context, tokenID, collection string, keep int, before time.Time) (int64, error) {
	b := t.builder("dav_sync_states").Select("set_hash").Where("token_id = ?", tokenID).
		Where("collection = ?", collection).OrderBy("seen_at", core.Descending).OrderBy("set_hash", core.Ascending).
		Limit(keep)
	rows, err := t.query(ctx, b, "listing sync states")
	if err != nil {
		return 0, err
	}
	var kept []string
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			_ = rows.Close()
			return 0, mapErr(err, "listing sync states")
		}
		kept = append(kept, h)
	}
	if err := mapRowsErr(rows, "listing sync states"); err != nil {
		_ = rows.Close()
		return 0, err
	}
	_ = rows.Close()
	del := t.builder("dav_sync_states").Where("token_id = ?", tokenID).Where("collection = ?", collection).
		Where("seen_at < ?", sqlb.TimeText(before))
	if len(kept) > 0 {
		del = del.WhereNotIn("set_hash", kept)
	}
	return t.execDelete(ctx, del, "pruning sync states")
}
