// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

func TestClientObjectsAreUniqueInTheirTenant(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	a := seed(t, s, clk, "acme")
	b := seed(t, s, clk, "beta")
	first, second := a.newTask(t, "first", core.PriorityNormal), a.newTask(t, "second", core.PriorityNormal)
	other := b.newTask(t, "other", core.PriorityNormal)
	put := func(f fixture, o store.DAVObjectName) error {
		o.CreatedAt = clk.Now()
		return s.Update(ctx, f.scope, func(tx store.Tx) error { return tx.CreateDAVObject(ctx, &o) })
	}
	if err := put(a, store.DAVObjectName{TaskID: first.ID, UID: "ABC", Name: "ABC"}); err != nil {
		t.Fatal(err)
	}
	for name, o := range map[string]store.DAVObjectName{
		"the same UID":  {TaskID: second.ID, UID: "ABC", Name: "other"},
		"the same name": {TaskID: second.ID, UID: "other", Name: "ABC"},
	} {
		if err := put(a, o); !core.IsKind(err, core.KindConflict) {
			t.Errorf("%s in one tenant: %v", name, err)
		}
	}
	if err := put(b, store.DAVObjectName{TaskID: other.ID, UID: "ABC", Name: "ABC"}); err != nil {
		t.Errorf("another tenant could not use the same UID: %v", err)
	}
	if err := s.View(ctx, a.scope, func(tx store.Tx) error {
		byName, err := tx.GetDAVObjectByName(ctx, "ABC")
		if err != nil || byName.TaskID != first.ID {
			t.Errorf("by name %+v %v", byName, err)
		}
		byUID, err := tx.GetDAVObjectByUID(ctx, "ABC")
		if err != nil || byUID.TaskID != first.ID {
			t.Errorf("by UID %+v %v", byUID, err)
		}
		if _, err := tx.GetDAVObjectByUID(ctx, "nope"); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("a missing UID: %v", err)
		}
		list, err := tx.ListDAVObjects(ctx, []string{first.ID, second.ID, other.ID})
		if err != nil || len(list) != 1 || list[0].TaskID != first.ID {
			t.Errorf("listed %+v %v", list, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestKeptPropertiesArePerPersonAndTask(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	a := seed(t, s, clk, "acme")
	task := a.newTask(t, "first", core.PriorityNormal)
	other := core.Actor{Kind: core.ActorUser, Handle: "raj"}
	write := func(fn func(tx store.Tx) error) {
		t.Helper()
		if err := s.Update(ctx, a.scope, fn); err != nil {
			t.Fatal(err)
		}
	}
	write(func(tx store.Tx) error { return tx.CreateActor(ctx, &other) })
	write(func(tx store.Tx) error { return tx.PutDAVProps(ctx, a.actor.ID, task.ID, "X-A:1", clk.Now()) })
	write(func(tx store.Tx) error { return tx.PutDAVProps(ctx, a.actor.ID, task.ID, "X-A:2", clk.Now()) })
	read := func(actorID string) map[string]string {
		t.Helper()
		var out map[string]string
		if err := s.View(ctx, a.scope, func(tx store.Tx) error {
			var err error
			out, err = tx.GetDAVProps(ctx, actorID, []string{task.ID})
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return out
	}
	if got := read(a.actor.ID); got[task.ID] != "X-A:2" || len(got) != 1 {
		t.Errorf("kept %q", got)
	}
	if got := read(other.ID); len(got) != 0 {
		t.Errorf("another person reads %q", got)
	}
	write(func(tx store.Tx) error { return tx.PutDAVProps(ctx, a.actor.ID, task.ID, "", clk.Now()) })
	if got := read(a.actor.ID); len(got) != 0 {
		t.Errorf("an empty write kept %q", got)
	}
	write(func(tx store.Tx) error { return tx.PutDAVProps(ctx, other.ID, task.ID, "X-B:1", clk.Now()) })
	write(func(tx store.Tx) error { return tx.DeleteTask(ctx, task.ID, true) })
	if got := read(other.ID); len(got) != 0 {
		t.Errorf("a removed task left %q", got)
	}
}

func TestInvalidatingSyncStatesSpendsTheShortCut(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	a := seed(t, s, clk, "acme")
	davToken(t, s, a, "01DAVTOKEN0000000000000001", "dav-worker-iphone", a.actor.ID)
	state := store.DAVSyncState{TokenID: "01DAVTOKEN0000000000000001", ActorID: a.actor.ID, Collection: "assigned",
		SetHash: "h", EventSeq: 7, EventMark: 3, ValidUntil: clk.Now(), SeenAt: clk.Now()}
	if err := s.Update(ctx, a.scope, func(tx store.Tx) error {
		if err := tx.PutDAVSyncState(ctx, &state); err != nil {
			return err
		}
		return tx.InvalidateDAVSyncStates(ctx, a.actor.ID)
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.View(ctx, a.scope, func(tx store.Tx) error {
		got, err := tx.GetDAVSyncState(ctx, state.TokenID, "assigned", "h")
		if err == nil && (got.EventSeq != -1 || got.EventMark != -1) {
			t.Errorf("after invalidating: seq %d mark %d", got.EventSeq, got.EventMark)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
}
