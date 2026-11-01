// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

func TestInboxPagesCountsMarksAndCascades(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	other := core.Actor{Kind: core.ActorUser, Handle: "other"}
	task := f.newTask(t, "t", core.PriorityNormal)
	kept := f.newTask(t, "kept", core.PriorityNormal)
	update := func(fn func(tx store.Tx) error) {
		t.Helper()
		if err := s.Update(ctx, f.scope, fn); err != nil {
			t.Fatal(err)
		}
	}
	update(func(tx store.Tx) error { return tx.CreateActor(ctx, &other) })

	var ids []string
	for i, kind := range []core.InboxKind{core.InboxAssigned, core.InboxComment, core.InboxStatus} {
		clk.Advance(time.Second)
		taskID := task.ID
		if i == 0 {
			taskID = kept.ID
		}
		it := &core.InboxItem{RecipientID: f.actor.ID, Kind: kind, TaskID: taskID, ActorID: other.ID,
			FromStatus: "todo", ToStatus: "doing", Excerpt: "x"}
		update(func(tx store.Tx) error { return tx.CreateInboxItem(ctx, it) })
		ids = append([]string{it.ID}, ids...)
	}
	update(func(tx store.Tx) error {
		return tx.CreateInboxItem(ctx, &core.InboxItem{RecipientID: other.ID, Kind: core.InboxMention, TaskID: task.ID, ActorID: f.actor.ID})
	})

	var paged []string
	cursor := ""
	for range 10 {
		var batch []core.InboxItem
		if err := s.View(ctx, f.scope, func(tx store.Tx) error {
			var err error
			batch, err = tx.ListInbox(ctx, f.actor.ID, core.InboxFilter{Page: core.Page{Limit: 2, Cursor: cursor,
				Sort: core.SortCreatedAt, Direction: core.Descending}})
			return err
		}); err != nil {
			t.Fatal(err)
		}
		for _, it := range batch {
			paged = append(paged, it.ID)
		}
		if len(batch) < 2 {
			break
		}
		last := batch[len(batch)-1]
		cursor = core.Cursor{SortValue: sqlb.TimeText(last.CreatedAt), ID: last.ID, Sort: core.SortCreatedAt, Direction: core.Descending}.Encode()
	}
	if !slices.Equal(paged, ids) {
		t.Fatalf("paged = %v, want newest first %v", paged, ids)
	}

	at := clk.Now()
	update(func(tx store.Tx) error {
		if changed, err := tx.SetInboxRead(ctx, f.actor.ID, ids[0], &at); err != nil || !changed {
			t.Errorf("first read: changed = %v, err = %v", changed, err)
		}
		if changed, err := tx.SetInboxRead(ctx, f.actor.ID, ids[0], &at); err != nil || changed {
			t.Errorf("second read: changed = %v, err = %v", changed, err)
		}
		if changed, err := tx.SetInboxRead(ctx, other.ID, ids[1], &at); err != nil || changed {
			t.Errorf("another recipient marked the item: changed = %v, err = %v", changed, err)
		}
		if _, err := tx.GetInboxItem(ctx, other.ID, ids[1]); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("another recipient read the item: err = %v", err)
		}
		n, err := tx.CountUnreadInbox(ctx, f.actor.ID)
		if err != nil || n != 2 {
			t.Errorf("unread = %d, err = %v, want 2", n, err)
		}
		if n, err := tx.ReadAllInbox(ctx, f.actor.ID, at); err != nil || n != 2 {
			t.Errorf("read all marked %d, err = %v, want 2", n, err)
		}
		if n, err := tx.CountUnreadInbox(ctx, other.ID); err != nil || n != 1 {
			t.Errorf("other's unread = %d, err = %v, want 1", n, err)
		}
		if err := tx.SetInboxMutes(ctx, f.actor.ID, []core.InboxKind{core.InboxComment, core.InboxStatus}); err != nil {
			return err
		}
		if err := tx.SetInboxMutes(ctx, f.actor.ID, []core.InboxKind{core.InboxStatus}); err != nil {
			return err
		}
		mutes, err := tx.InboxMutesOf(ctx, []string{f.actor.ID, other.ID})
		if err != nil || !slices.Equal(mutes[f.actor.ID], []core.InboxKind{core.InboxStatus}) || len(mutes[other.ID]) != 0 {
			t.Errorf("mutes = %v, err = %v", mutes, err)
		}
		if _, err := tx.StarTask(ctx, other.ID, task.ID); err != nil {
			return err
		}
		starrers, err := tx.TaskStarrers(ctx, task.ID)
		if err != nil || !slices.Equal(starrers, []string{other.ID}) {
			t.Errorf("starrers = %v, err = %v", starrers, err)
		}
		return tx.DeleteTask(ctx, task.ID, true)
	})
	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		left, err := tx.ListInbox(ctx, f.actor.ID, core.InboxFilter{Page: core.Page{Sort: core.SortCreatedAt, Direction: core.Descending}})
		if err != nil {
			return err
		}
		if len(left) != 1 || left[0].TaskID != kept.ID || !left[0].Read() || left[0].FromStatus != "todo" {
			t.Errorf("after purging the task the inbox holds %+v, want only the kept item, read", left)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
