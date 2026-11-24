// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// davObjectChunk bounds how many task identifiers one IN list carries.
const davObjectChunk = 500

// InvalidateDAVSyncStates spends the short cut of an actor's snapshots.
func (t *tx) InvalidateDAVSyncStates(ctx context.Context, actorID string) error {
	upd := t.builder("dav_sync_states").Where("actor_id = ?", actorID).Set("event_seq", -1).Set("event_mark", -1)
	_, err := t.execUpdate(ctx, upd, "invalidating sync states")
	return err
}

var davObjectColumns = []string{"task_id", "uid", "name", "created_at"}

func scanDAVObject(s scanner) (*store.DAVObjectName, error) {
	var (
		o       store.DAVObjectName
		created sql.NullString
	)
	if err := s.Scan(&o.TaskID, &o.UID, &o.Name, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, core.NotFound("client object not found")
		}
		return nil, mapErr(err, "scanning client object")
	}
	var err error
	if o.CreatedAt, err = sqlb.ScanTime(created); err != nil {
		return nil, err
	}
	return &o, nil
}

// CreateDAVObject records a client's UID and name for a task.
func (t *tx) CreateDAVObject(ctx context.Context, o *store.DAVObjectName) error {
	ins := t.insert("dav_objects").Set("task_id", o.TaskID).Set("uid", o.UID).Set("name", o.Name).
		Set("created_at", sqlb.TimeText(o.CreatedAt))
	_, err := t.execInsert(ctx, ins, "recording client object %q", o.Name)
	return err
}

// GetDAVObjectByName finds a client-created task by its resource name.
func (t *tx) GetDAVObjectByName(ctx context.Context, name string) (*store.DAVObjectName, error) {
	q, args := t.builder("dav_objects").Select(davObjectColumns...).Where("name = ?", name).Limit(1).SelectQuery()
	return scanDAVObject(t.ex.QueryRowContext(ctx, q, args...))
}

// GetDAVObjectByUID finds a client-created task by its UID.
func (t *tx) GetDAVObjectByUID(ctx context.Context, uid string) (*store.DAVObjectName, error) {
	q, args := t.builder("dav_objects").Select(davObjectColumns...).Where("uid = ?", uid).Limit(1).SelectQuery()
	return scanDAVObject(t.ex.QueryRowContext(ctx, q, args...))
}

// ListDAVObjects returns the client names of those of the tasks that have one.
func (t *tx) ListDAVObjects(ctx context.Context, taskIDs []string) ([]store.DAVObjectName, error) {
	out := []store.DAVObjectName{}
	for start := 0; start < len(taskIDs); start += davObjectChunk {
		ids := taskIDs[start:min(start+davObjectChunk, len(taskIDs))]
		rows, err := t.query(ctx, t.builder("dav_objects").Select(davObjectColumns...).WhereIn("task_id", ids),
			"listing client objects")
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			o, err := scanDAVObject(rows)
			if err != nil {
				_ = rows.Close()
				return nil, err
			}
			out = append(out, *o)
		}
		err = mapRowsErr(rows, "listing client objects")
		_ = rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// GetDAVProps returns what is kept for an actor, by task.
func (t *tx) GetDAVProps(ctx context.Context, actorID string, taskIDs []string) (map[string]string, error) {
	out := map[string]string{}
	for start := 0; start < len(taskIDs); start += davObjectChunk {
		ids := taskIDs[start:min(start+davObjectChunk, len(taskIDs))]
		b := t.builder("dav_object_props").Select("task_id", "props").Where("actor_id = ?", actorID).
			WhereIn("task_id", ids)
		rows, err := t.query(ctx, b, "reading kept properties")
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id, props string
			if err := rows.Scan(&id, &props); err != nil {
				_ = rows.Close()
				return nil, mapErr(err, "reading kept properties")
			}
			out[id] = props
		}
		err = mapRowsErr(rows, "reading kept properties")
		_ = rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// PutDAVProps replaces what is kept for an actor and a task.
func (t *tx) PutDAVProps(ctx context.Context, actorID, taskID, props string, at time.Time) error {
	row := t.builder("dav_object_props").Where("actor_id = ?", actorID).Where("task_id = ?", taskID)
	if props == "" {
		_, err := t.execDelete(ctx, row, "removing kept properties")
		return err
	}
	n, err := t.execUpdate(ctx, row.Set("props", props).Set("updated_at", sqlb.TimeText(at)), "updating kept properties")
	if err != nil || n > 0 {
		return err
	}
	ins := t.insert("dav_object_props").Set("actor_id", actorID).Set("task_id", taskID).Set("props", props).
		Set("updated_at", sqlb.TimeText(at))
	_, err = t.execInsert(ctx, ins, "keeping properties")
	return err
}
