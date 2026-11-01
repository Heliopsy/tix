// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

func TestGroupPrefsSaveReplaceForgetAndStayInTheirTenant(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	a := seed(t, s, clk, "acme")
	b := seed(t, s, clk, "beta")

	write := func(f fixture, fn func(tx store.Tx) error) {
		t.Helper()
		if err := s.Update(ctx, f.scope, fn); err != nil {
			t.Fatal(err)
		}
	}
	read := func(f fixture, actorID, view string) []core.GroupPref {
		t.Helper()
		var out []core.GroupPref
		if err := s.View(ctx, f.scope, func(tx store.Tx) error {
			var err error
			out, err = tx.GroupPrefs(ctx, actorID, view)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return out
	}

	write(a, func(tx store.Tx) error {
		if err := tx.SaveGroupPref(ctx, a.actor.ID, core.GroupPref{View: "board", Group: "done", Hidden: true}); err != nil {
			return err
		}
		if err := tx.SaveGroupPref(ctx, a.actor.ID, core.GroupPref{View: "board", Group: "todo", Starred: true, Position: 2}); err != nil {
			return err
		}
		return tx.SaveGroupPref(ctx, a.actor.ID, core.GroupPref{View: "board", Group: "done", Collapsed: true, Position: 1})
	})
	got := read(a, a.actor.ID, "board")
	want := []core.GroupPref{
		{View: "board", Group: "done", Collapsed: true, Position: 1},
		{View: "board", Group: "todo", Starred: true, Position: 2},
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("prefs = %+v, want %+v", got, want)
	}
	if other := read(a, a.actor.ID, "lanes"); len(other) != 0 {
		t.Errorf("another view holds %+v", other)
	}

	write(b, func(tx store.Tx) error {
		if n, err := tx.DeleteGroupPrefs(ctx, a.actor.ID, "board", ""); err != nil || n != 0 {
			t.Errorf("another tenant deleted %d rows: %v", n, err)
		}
		return tx.SaveGroupPref(ctx, a.actor.ID, core.GroupPref{View: "board", Group: "doing", Hidden: true})
	})
	if got := read(a, a.actor.ID, "board"); len(got) != 2 {
		t.Errorf("a row written under another tenant reached this one: %+v", got)
	}

	write(a, func(tx store.Tx) error {
		n, err := tx.DeleteGroupPrefs(ctx, a.actor.ID, "board", "done")
		if err != nil || n != 1 {
			t.Errorf("deleting one group removed %d: %v", n, err)
		}
		n, err = tx.DeleteGroupPrefs(ctx, a.actor.ID, "board", "")
		if err != nil || n != 1 {
			t.Errorf("deleting the view removed %d: %v", n, err)
		}
		return nil
	})
	if got := read(a, a.actor.ID, "board"); len(got) != 0 {
		t.Errorf("after deletes: %+v", got)
	}
}
