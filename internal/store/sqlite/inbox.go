// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/id"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

var inboxColumns = []string{
	"inbox_items.id", "inbox_items.recipient_id", "inbox_items.kind", "inbox_items.task_id",
	"inbox_items.actor_id", "inbox_items.comment_id", "inbox_items.from_status",
	"inbox_items.to_status", "inbox_items.excerpt", "inbox_items.created_at", "inbox_items.read_at",
}

// maxStarrers bounds the watchers one comment notifies.
const maxStarrers = 5000

func scanInboxItem(s scanner) (core.InboxItem, error) {
	var (
		it                core.InboxItem
		kind              string
		comment, from, to sql.NullString
		created, read     sql.NullString
	)
	if err := s.Scan(&it.ID, &it.RecipientID, &kind, &it.TaskID, &it.ActorID, &comment,
		&from, &to, &it.Excerpt, &created, &read); err != nil {
		return core.InboxItem{}, mapErr(err, "scanning inbox item")
	}
	it.Kind = core.InboxKind(kind)
	it.CommentID, it.FromStatus, it.ToStatus = sqlb.Text(comment), sqlb.Text(from), sqlb.Text(to)
	var err error
	if it.CreatedAt, err = sqlb.ScanTime(created); err != nil {
		return core.InboxItem{}, err
	}
	if it.ReadAt, err = sqlb.ScanNullTime(read); err != nil {
		return core.InboxItem{}, err
	}
	return it, nil
}

// CreateInboxItem inserts an item, filling its identifier when empty.
func (t *tx) CreateInboxItem(ctx context.Context, it *core.InboxItem) error {
	if it.ID == "" {
		it.ID = id.New()
	}
	if it.CreatedAt.IsZero() {
		it.CreatedAt = t.store.clock.Now()
	}
	ins := t.insert("inbox_items").
		Set("id", it.ID).
		Set("recipient_id", it.RecipientID).
		Set("kind", string(it.Kind)).
		Set("task_id", it.TaskID).
		Set("actor_id", it.ActorID).
		Set("comment_id", sqlb.NullText(it.CommentID)).
		Set("from_status", sqlb.NullText(it.FromStatus)).
		Set("to_status", sqlb.NullText(it.ToStatus)).
		Set("excerpt", it.Excerpt).
		Set("created_at", sqlb.TimeText(it.CreatedAt)).
		Set("read_at", sqlb.NullTimeText(it.ReadAt))
	_, err := t.execInsert(ctx, ins, "creating inbox item for %q", it.RecipientID)
	return err
}

// ListInbox returns one page of a recipient's items, newest first.
func (t *tx) ListInbox(ctx context.Context, recipientID string, f core.InboxFilter) ([]core.InboxItem, error) {
	spec, err := resolvePage(f.Page, core.SortCreatedAt, map[string]string{
		core.SortCreatedAt: "inbox_items.created_at",
	})
	if err != nil {
		return nil, err
	}
	b := t.builder("inbox_items").Select(inboxColumns...).
		Where("inbox_items.recipient_id = ?", recipientID)
	if f.Unread {
		b.Where("inbox_items.read_at IS NULL")
	}
	rows, err := t.query(ctx, spec.apply(b, "inbox_items.id"), "listing inbox")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []core.InboxItem{}
	for rows.Next() {
		it, err := scanInboxItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, mapRowsErr(rows, "listing inbox")
}

// CountUnreadInbox counts a recipient's unread items.
func (t *tx) CountUnreadInbox(ctx context.Context, recipientID string) (int, error) {
	b := t.builder("inbox_items").
		Where("inbox_items.recipient_id = ?", recipientID).
		Where("inbox_items.read_at IS NULL")
	return t.count(ctx, b, "counting unread inbox items")
}

// GetInboxItem returns one of a recipient's items.
func (t *tx) GetInboxItem(ctx context.Context, recipientID, itemID string) (*core.InboxItem, error) {
	b := t.inboxRow(recipientID, itemID).Select(inboxColumns...).Limit(1)
	q, args := b.SelectQuery()
	it, err := scanInboxItem(t.ex.QueryRowContext(ctx, q, args...))
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound("inbox item %q", itemID)
		}
		return nil, err
	}
	return &it, nil
}

// SetInboxRead sets or clears when an item was read.
func (t *tx) SetInboxRead(ctx context.Context, recipientID, itemID string, at *time.Time) (bool, error) {
	b := t.inboxRow(recipientID, itemID).Set("read_at", sqlb.NullTimeText(at))
	if at == nil {
		b.Where("inbox_items.read_at IS NOT NULL")
	} else {
		b.Where("inbox_items.read_at IS NULL")
	}
	n, err := t.execUpdate(ctx, b, "marking inbox item %q", itemID)
	return n > 0, err
}

// ReadAllInbox marks every unread item of a recipient read.
func (t *tx) ReadAllInbox(ctx context.Context, recipientID string, at time.Time) (int64, error) {
	b := t.builder("inbox_items").
		Set("read_at", sqlb.TimeText(at)).
		Where("inbox_items.recipient_id = ?", recipientID).
		Where("inbox_items.read_at IS NULL")
	return t.execUpdate(ctx, b, "marking every inbox item read")
}

// InboxMutesOf returns the kinds each of the given actors muted.
func (t *tx) InboxMutesOf(ctx context.Context, actorIDs []string) (map[string][]core.InboxKind, error) {
	out := map[string][]core.InboxKind{}
	if len(actorIDs) == 0 {
		return out, nil
	}
	b := t.builder("inbox_mutes").Select("inbox_mutes.actor_id", "inbox_mutes.kind").
		OrderBy("inbox_mutes.actor_id", core.Ascending).
		OrderBy("inbox_mutes.kind", core.Ascending)
	b.WhereIn("inbox_mutes.actor_id", actorIDs)
	err := t.eachPair(ctx, b, "reading inbox mutes", func(actor, kind string) {
		out[actor] = append(out[actor], core.InboxKind(kind))
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SetInboxMutes replaces the kinds an actor muted.
func (t *tx) SetInboxMutes(ctx context.Context, actorID string, kinds []core.InboxKind) error {
	del := t.builder("inbox_mutes").Where("inbox_mutes.actor_id = ?", actorID)
	if _, err := t.execDelete(ctx, del, "clearing inbox mutes"); err != nil {
		return err
	}
	for _, k := range kinds {
		ins := t.insert("inbox_mutes").Set("actor_id", actorID).Set("kind", string(k))
		if _, err := t.execInsert(ctx, ins, "muting %q", k); err != nil {
			return err
		}
	}
	return nil
}

// TaskStarrers lists the actors who starred a task.
func (t *tx) TaskStarrers(ctx context.Context, taskID string) ([]string, error) {
	b := t.builder("task_stars").Select("task_stars.actor_id", "task_stars.task_id").
		Where("task_stars.task_id = ?", taskID).
		OrderBy("task_stars.actor_id", core.Ascending).
		Limit(maxStarrers)
	var out []string
	err := t.eachPair(ctx, b, "reading who starred a task", func(actor, _ string) { out = append(out, actor) })
	return out, err
}

func (t *tx) inboxRow(recipientID, itemID string) *sqlb.Builder {
	return t.builder("inbox_items").
		Where("inbox_items.recipient_id = ?", recipientID).
		Where("inbox_items.id = ?", itemID)
}
