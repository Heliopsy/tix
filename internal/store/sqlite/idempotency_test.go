// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

func TestIdempotencyKeysRoundTripStayInTheirTenantAndPrune(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	a := seed(t, s, clk, "acme")
	b := seed(t, s, clk, "beta")
	now := clk.Now().UTC().Truncate(time.Second)

	rec := store.IdempotencyRecord{
		ActorID: a.actor.ID, Key: "k-1", Fingerprint: "fp", State: store.IdempotencyReserved,
		CreatedAt: now, LeaseExpiresAt: now.Add(time.Minute), ExpiresAt: now.Add(24 * time.Hour),
	}
	if err := s.Update(ctx, a.scope, func(tx store.Tx) error { return tx.CreateIdempotencyKey(ctx, &rec) }); err != nil {
		t.Fatal(err)
	}
	dup := rec
	dup.ID = ""
	err := s.Update(ctx, a.scope, func(tx store.Tx) error { return tx.CreateIdempotencyKey(ctx, &dup) })
	if !core.IsKind(err, core.KindConflict) {
		t.Fatalf("a second row for the same actor and key: %v, want conflict", err)
	}
	if err := s.Update(ctx, a.scope, func(tx store.Tx) error {
		if err := tx.MarkIdempotencyExecuted(ctx, rec.ID); err != nil {
			return err
		}
		done := rec
		done.State, done.Status, done.ContentType, done.Body = store.IdempotencyDone, 201, "application/json", []byte(`{"id":1}`)
		return tx.FinishIdempotencyKey(ctx, &done)
	}); err != nil {
		t.Fatal(err)
	}

	var got *store.IdempotencyRecord
	if err := s.View(ctx, a.scope, func(tx store.Tx) error {
		got, err = tx.GetIdempotencyKey(ctx, a.actor.ID, "k-1")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if got.State != store.IdempotencyDone || got.Status != 201 || string(got.Body) != `{"id":1}` ||
		got.ContentType != "application/json" || got.Fingerprint != "fp" || !got.ExpiresAt.Equal(rec.ExpiresAt) ||
		!got.LeaseExpiresAt.Equal(rec.LeaseExpiresAt) {
		t.Fatalf("read back %+v", got)
	}

	if err := s.View(ctx, b.scope, func(tx store.Tx) error {
		_, err := tx.GetIdempotencyKey(ctx, a.actor.ID, "k-1")
		return err
	}); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("another tenant read the key: %v, want not found", err)
	}

	prune := func(at time.Time) (int64, int) {
		t.Helper()
		var n int64
		var expired int
		if err := s.Update(ctx, a.scope, func(tx store.Tx) error {
			var err error
			if expired, err = tx.CountExpiredIdempotencyKeys(ctx, at); err != nil {
				return err
			}
			n, err = tx.PruneIdempotencyKeys(ctx, at, 0)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return n, expired
	}
	if n, expired := prune(now.Add(time.Hour)); n != 0 || expired != 0 {
		t.Fatalf("inside the window: pruned %d, counted %d, want 0 and 0", n, expired)
	}
	if n, expired := prune(now.Add(25 * time.Hour)); n != 1 || expired != 1 {
		t.Fatalf("past the window: pruned %d, counted %d, want 1 and 1", n, expired)
	}
}

func TestMarkIdempotencyExecutedLeavesAFinishedRowAlone(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	a := seed(t, s, clk, "acme")
	now := clk.Now()
	rec := store.IdempotencyRecord{
		ActorID: a.actor.ID, Key: "k", Fingerprint: "fp", State: store.IdempotencyDone, Status: 200,
		CreatedAt: now, LeaseExpiresAt: now, ExpiresAt: now.Add(time.Hour),
	}
	var got *store.IdempotencyRecord
	if err := s.Update(ctx, a.scope, func(tx store.Tx) error {
		if err := tx.CreateIdempotencyKey(ctx, &rec); err != nil {
			return err
		}
		if err := tx.MarkIdempotencyExecuted(ctx, rec.ID); err != nil {
			return err
		}
		var err error
		got, err = tx.GetIdempotencyKey(ctx, a.actor.ID, "k")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if got.State != store.IdempotencyDone {
		t.Fatalf("state = %q, want done", got.State)
	}
}
