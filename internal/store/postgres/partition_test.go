// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

func TestEventsLandInTheirMonthlyPartition(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		e := core.Event{Type: core.EventTaskCreated, SubjectType: "task", SubjectID: "t1"}
		return tx.AppendEvent(ctx, &e)
	}); err != nil {
		t.Fatalf("appending an event: %v", err)
	}

	want := partitionName("events", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	var n int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+want).Scan(&n); err != nil {
		t.Fatalf("reading partition %q: %v", want, err)
	}
	if n != 1 {
		t.Fatalf("partition %q holds %d events, want 1", want, n)
	}
}

func TestPartitionsAreCreatedAheadAndDroppedForRetention(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		e := core.Event{Type: core.EventTaskCreated, SubjectType: "task", SubjectID: "t1"}
		return tx.AppendEvent(ctx, &e)
	}); err != nil {
		t.Fatalf("appending an event: %v", err)
	}

	if err := s.EnsurePartitions(ctx, clk.Now().AddDate(3, 0, 0)); err != nil {
		t.Fatalf("ensuring partitions: %v", err)
	}
	ahead := partitionName("audit_entries", time.Date(2029, 1, 1, 0, 0, 0, 0, time.UTC))
	var exists bool
	if err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_class WHERE relname = $1)`, ahead).Scan(&exists); err != nil {
		t.Fatalf("looking for %q: %v", ahead, err)
	}
	if !exists {
		t.Fatalf("partition %q was not created ahead of time", ahead)
	}

	dropped, err := s.DropPartitionsBefore(ctx, clk.Now().AddDate(0, -1, 0))
	if err != nil {
		t.Fatalf("dropping partitions: %v", err)
	}
	if dropped == 0 {
		t.Fatal("retention dropped no partitions")
	}

	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		events, err := tx.ReadEvents(ctx, 0, 10)
		if err != nil {
			return err
		}
		if len(events) != 1 {
			t.Fatalf("events after dropping older partitions = %d, want 1", len(events))
		}
		return nil
	}); err != nil {
		t.Fatalf("reading events: %v", err)
	}
}

// TestPartitionNamesMatchTheirBounds pins the partition name to the range the
// same statement declares. Retention drops a partition by parsing its name, so
// a name that disagrees with its bounds by even one month deletes live data;
// both sides moving together is exactly what a test built on partitionName
// alone cannot see.
func TestPartitionNamesMatchTheirBounds(t *testing.T) {
	tests := []struct {
		name  string
		table string
		month time.Time
		want  string
		from  string
		to    string
	}{
		{"january", "events", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			"events_p202601", "2026-01-01T00:00:00Z", "2026-02-01T00:00:00Z"},
		{"december rolls the year", "audit_entries", time.Date(2029, 12, 1, 0, 0, 0, 0, time.UTC),
			"audit_entries_p202912", "2029-12-01T00:00:00Z", "2030-01-01T00:00:00Z"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := partitionName(tc.table, tc.month); got != tc.want {
				t.Errorf("partitionName = %q, want %q", got, tc.want)
			}

			stmt := partitionStatement(tc.table, tc.month)
			for _, want := range []string{tc.want, "FROM ('" + tc.from + "')", "TO ('" + tc.to + "')"} {
				if !strings.Contains(stmt, want) {
					t.Errorf("partitionStatement = %q, want it to contain %q", stmt, want)
				}
			}

			end, ok := partitionEnd(tc.table, tc.want)
			if !ok {
				t.Fatalf("partitionEnd(%q, %q) did not parse", tc.table, tc.want)
			}
			if got := end.UTC().Format(time.RFC3339); got != tc.to {
				t.Errorf("retention reads %q as ending %s, but it holds rows up to %s",
					tc.want, got, tc.to)
			}
		})
	}
}
