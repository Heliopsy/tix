// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// trashFixture holds five tasks, three of them soft deleted, created a second
// apart so the created_at ordering is total.
func trashFixture(t *testing.T) (*Store, fixture, []string, []string) {
	t.Helper()
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	var live, gone []string
	for i := range 5 {
		task := f.newTask(t, fmt.Sprintf("task-%d", i), core.PriorityNormal)
		clk.Advance(time.Second)
		if i%2 == 1 {
			live = append(live, task.ID)
			continue
		}
		if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
			return tx.DeleteTask(ctx, task.ID, false)
		}); err != nil {
			t.Fatalf("deleting %s: %v", task.ID, err)
		}
		gone = append(gone, task.ID)
	}
	return s, f, live, gone
}

func listTrash(t *testing.T, s *Store, f fixture, filter core.TaskFilter) []core.Task {
	t.Helper()
	ctx := context.Background()
	var out []core.Task
	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		var err error
		out, err = tx.ListTasks(ctx, filter)
		return err
	}); err != nil {
		t.Fatalf("listing %+v: %v", filter, err)
	}
	return out
}

func listIDs(t *testing.T, s *Store, f fixture, filter core.TaskFilter) []string {
	t.Helper()
	var out []string
	for _, task := range listTrash(t, s, f, filter) {
		out = append(out, task.ID)
	}
	return out
}

func TestTrashedSelectsByDeletion(t *testing.T) {
	s, f, live, gone := trashFixture(t)
	byAge := core.Page{Sort: core.SortCreatedAt, Direction: core.Ascending}
	tests := []struct {
		name   string
		filter core.TaskFilter
		want   []string
	}{
		{"the default lists live tasks", core.TaskFilter{}, live},
		{"trashed lists only deleted tasks", core.TaskFilter{Trashed: core.Yes}, gone},
		{"trashed wins over include deleted", core.TaskFilter{Trashed: core.Yes, IncludeDeleted: true}, gone},
		{"untrashed lists live tasks", core.TaskFilter{Trashed: core.No}, live},
		{"untrashed narrows include deleted back to live", core.TaskFilter{Trashed: core.No, IncludeDeleted: true}, live},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.filter.Page = byAge
			if got := listIDs(t, s, f, tt.filter); !slices.Equal(got, tt.want) {
				t.Fatalf("listed %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTheTrashPagesByKeyset(t *testing.T) {
	s, f, _, gone := trashFixture(t)
	var seen []string
	cursor := ""
	for range len(gone) + 2 {
		batch := listTrash(t, s, f, core.TaskFilter{Trashed: core.Yes, Page: core.Page{
			Limit: 1, Cursor: cursor, Sort: core.SortCreatedAt, Direction: core.Ascending,
		}})
		if len(batch) == 0 {
			break
		}
		for _, task := range batch {
			seen = append(seen, task.ID)
		}
		cursor = taskCursor(batch[len(batch)-1], core.Ascending)
	}
	if !slices.Equal(seen, gone) {
		t.Fatalf("paged through %v, want %v", seen, gone)
	}
}
