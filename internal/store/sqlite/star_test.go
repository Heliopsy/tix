// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

func starAs(t *testing.T, f fixture, actorID, taskID string, star bool) bool {
	t.Helper()
	var changed bool
	err := f.store.Update(context.Background(), f.scope, func(tx store.Tx) error {
		var err error
		if star {
			changed, err = tx.StarTask(context.Background(), actorID, taskID)
		} else {
			changed, err = tx.UnstarTask(context.Background(), actorID, taskID)
		}
		return err
	})
	if err != nil {
		t.Fatalf("star %v of %s by %s: %v", star, taskID, actorID, err)
	}
	return changed
}

func starWalk(t *testing.T, f fixture, reader string, dir core.SortDirection) ([]int64, []bool) {
	t.Helper()
	ctx := context.Background()
	var seqs []int64
	var stars []bool
	cursor := ""
	for range 20 {
		var batch []core.Task
		if err := f.store.View(ctx, f.scope, func(tx store.Tx) error {
			var err error
			batch, err = tx.ListTasks(ctx, core.TaskFilter{StarredBy: reader, Page: core.Page{
				Limit: 2, Cursor: cursor, Sort: core.SortSeq, Direction: dir,
			}})
			return err
		}); err != nil {
			t.Fatalf("listing: %v", err)
		}
		if len(batch) == 0 {
			return seqs, stars
		}
		for _, task := range batch {
			seqs = append(seqs, task.Seq)
			stars = append(stars, task.Starred)
		}
		last := batch[len(batch)-1]
		cursor = core.Cursor{SortValue: strconv.FormatInt(last.Seq, 10), Starred: last.Starred,
			ID: last.ID, Sort: core.SortSeq, Direction: dir}.Encode()
	}
	t.Fatal("the walk did not end")
	return nil, nil
}

func TestStarredTasksLeadAndPageAcrossTheBoundary(t *testing.T) {
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	other := core.Actor{Kind: core.ActorUser, Handle: "other"}
	if err := s.Update(context.Background(), f.scope, func(tx store.Tx) error {
		return tx.CreateActor(context.Background(), &other)
	}); err != nil {
		t.Fatal(err)
	}
	var ids []string
	for i := range 9 {
		ids = append(ids, f.newTask(t, fmt.Sprintf("t%d", i), core.PriorityNormal).ID)
	}
	for _, i := range []int{2, 6, 8} {
		if !starAs(t, f, f.actor.ID, ids[i], true) {
			t.Fatalf("first star of t%d reported no change", i)
		}
	}
	if starAs(t, f, f.actor.ID, ids[2], true) {
		t.Fatal("a second star reported a change")
	}
	starAs(t, f, other.ID, ids[0], true)

	cases := []struct {
		name   string
		reader string
		dir    core.SortDirection
		want   []int64
		stars  int
	}{
		{"ascending", f.actor.ID, core.Ascending, []int64{3, 7, 9, 1, 2, 4, 5, 6, 8}, 3},
		{"descending", f.actor.ID, core.Descending, []int64{9, 7, 3, 8, 6, 5, 4, 2, 1}, 3},
		{"another reader", other.ID, core.Ascending, []int64{1, 2, 3, 4, 5, 6, 7, 8, 9}, 1},
		{"no reader", "", core.Ascending, []int64{1, 2, 3, 4, 5, 6, 7, 8, 9}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seqs, stars := starWalk(t, f, tc.reader, tc.dir)
			if !slices.Equal(seqs, tc.want) {
				t.Fatalf("walked %v, want %v", seqs, tc.want)
			}
			for i, starred := range stars {
				if starred != (i < tc.stars) {
					t.Fatalf("row %d (seq %d) starred = %v", i, seqs[i], starred)
				}
			}
		})
	}

	if !starAs(t, f, f.actor.ID, ids[2], false) || starAs(t, f, f.actor.ID, ids[2], false) {
		t.Fatal("unstar did not report exactly one change")
	}
}

func TestStarsStayInsideTheirTenant(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	a := seed(t, s, clk, "acme")
	b := seed(t, s, clk, "beta")
	victim := a.newTask(t, "a's task", core.PriorityNormal)
	b.newTask(t, "b's task", core.PriorityNormal)

	if err := s.Update(ctx, b.scope, func(tx store.Tx) error {
		_, err := tx.StarTask(ctx, a.actor.ID, victim.ID)
		return err
	}); err != nil {
		t.Fatalf("writing a cross-tenant star row: %v", err)
	}

	if err := s.View(ctx, a.scope, func(tx store.Tx) error {
		got, err := tx.ListTasks(ctx, core.TaskFilter{StarredBy: a.actor.ID, Starred: core.Yes})
		if err != nil {
			return err
		}
		if len(got) != 0 {
			t.Errorf("a star row written under another tenant reached this one: %v", got)
		}
		stars, err := tx.StarredTasks(ctx, a.actor.ID, []string{victim.ID})
		if err != nil {
			return err
		}
		if stars[victim.ID] {
			t.Error("StarredTasks read another tenant's row")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
