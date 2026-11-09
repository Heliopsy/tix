// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"fmt"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

func TestCountTasksStopsAtItsLimitAndSeekTaskFindsTheNthRow(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	titles := []string{}
	for i := 1; i <= 7; i++ {
		priority := core.PriorityNormal
		if i%2 == 0 {
			priority = core.PriorityHigh
		}
		titles = append(titles, f.newTask(t, fmt.Sprintf("task %d", i), priority).Title)
	}
	high := core.TaskFilter{Priorities: []core.Priority{core.PriorityHigh}}
	bySeq := core.TaskFilter{Page: core.Page{Sort: core.SortSeq}}
	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		for _, tc := range []struct {
			name   string
			filter core.TaskFilter
			limit  int
			want   int
		}{
			{"every task", core.TaskFilter{}, 100, 7},
			{"a filter", high, 100, 3},
			{"stopped at the limit", core.TaskFilter{}, 4, 4},
		} {
			n, err := tx.CountTasks(ctx, tc.filter, tc.limit)
			if err != nil {
				return err
			}
			if n != tc.want {
				t.Errorf("%s: counted %d, want %d", tc.name, n, tc.want)
			}
		}
		for n, want := range map[int]string{1: titles[0], 5: titles[4], 7: titles[6]} {
			got, err := tx.SeekTask(ctx, bySeq, n)
			if err != nil {
				return err
			}
			if got == nil || got.Title != want {
				t.Errorf("row %d of the walk = %v, want %q", n, got, want)
			}
		}
		past, err := tx.SeekTask(ctx, bySeq, 8)
		if err != nil {
			return err
		}
		if past != nil {
			t.Errorf("row 8 of seven = %q, want none", past.Title)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
