package postgres

import (
	"context"
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
