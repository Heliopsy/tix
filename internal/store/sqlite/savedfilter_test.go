// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

func TestSavedFiltersListOwnAndSharedAndStayInTheirTenant(t *testing.T) {
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
	read := func(f fixture, actorID string) map[string]core.SavedFilter {
		t.Helper()
		out := map[string]core.SavedFilter{}
		if err := s.View(ctx, f.scope, func(tx store.Tx) error {
			list, err := tx.SavedFilters(ctx, actorID)
			for _, sf := range list {
				out[sf.Name] = sf
			}
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return out
	}

	other := core.Actor{Kind: core.ActorAgent, Handle: "lena"}
	mine := core.SavedFilter{OwnerID: a.actor.ID, Name: "Ops", Query: "tag:ops", View: "board", Sort: "due_at", Position: 1}
	theirs := core.SavedFilter{Name: "Private"}
	shared := core.SavedFilter{Name: "Team", Query: "status:todo", Shared: true}
	write(a, func(tx store.Tx) error {
		if err := tx.CreateActor(ctx, &other); err != nil {
			return err
		}
		theirs.OwnerID, shared.OwnerID = other.ID, other.ID
		for _, f := range []*core.SavedFilter{&mine, &theirs, &shared} {
			if err := tx.CreateSavedFilter(ctx, f); err != nil {
				return err
			}
		}
		return nil
	})
	got := read(a, a.actor.ID)
	if len(got) != 2 {
		t.Fatalf("listed %v, want Ops and Team", got)
	}
	if g := got["Ops"]; g.Query != "tag:ops" || g.View != "board" || g.Sort != "due_at" || g.Position != 1 || g.ID == "" || g.CreatedAt.IsZero() {
		t.Errorf("Ops read back as %+v", g)
	}
	if !got["Team"].Shared || got["Team"].OwnerID != other.ID {
		t.Errorf("Team read back as %+v", got["Team"])
	}
	if _, ok := got["Private"]; ok {
		t.Error("another reader's private filter was listed")
	}

	mine.Name, mine.Shared, mine.Position = "Ops board", true, 3
	write(a, func(tx store.Tx) error { return tx.UpdateSavedFilter(ctx, mine) })
	if g := read(a, other.ID)["Ops board"]; !g.Shared || g.Position != 3 {
		t.Errorf("after update the other reader sees %+v", g)
	}

	write(b, func(tx store.Tx) error {
		if n, err := tx.DeleteSavedFilter(ctx, mine.ID); err != nil || n != 0 {
			t.Errorf("another tenant deleted %d rows: %v", n, err)
		}
		if err := tx.UpdateSavedFilter(ctx, mine); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("another tenant's update answered %v", err)
		}
		return nil
	})
	if got := read(b, a.actor.ID); len(got) != 0 {
		t.Errorf("another tenant lists %v", got)
	}

	write(a, func(tx store.Tx) error {
		n, err := tx.DeleteSavedFilter(ctx, mine.ID)
		if err != nil || n != 1 {
			t.Errorf("delete removed %d: %v", n, err)
		}
		return nil
	})
	if got := read(a, a.actor.ID); len(got) != 1 {
		t.Errorf("after delete: %v", got)
	}
}
