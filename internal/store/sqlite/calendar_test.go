// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

func TestCalendarFeedsAreOneLivePerActorAndStayInTheirTenant(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	a := seed(t, s, clk, "acme")
	b := seed(t, s, clk, "beta")
	created := clk.Now().Truncate(time.Second)

	update := func(f fixture, fn func(tx store.Tx) error) error {
		t.Helper()
		return s.Update(ctx, f.scope, fn)
	}
	view := func(f fixture, fn func(tx store.Tx) error) {
		t.Helper()
		if err := s.View(ctx, f.scope, fn); err != nil {
			t.Fatal(err)
		}
	}

	feed := core.CalendarFeed{ID: "01FEED0000000000000000000A", ActorID: a.actor.ID, Filter: "tag:ops", CreatedAt: created}
	if err := update(a, func(tx store.Tx) error { return tx.CreateCalendarFeed(ctx, &feed, "hash-a") }); err != nil {
		t.Fatalf("creating: %v", err)
	}
	second := core.CalendarFeed{ID: "01FEED0000000000000000000B", ActorID: a.actor.ID, CreatedAt: created}
	if err := update(a, func(tx store.Tx) error { return tx.CreateCalendarFeed(ctx, &second, "hash-b") }); err == nil {
		t.Fatal("a second live feed for one actor was stored")
	}

	view(a, func(tx store.Tx) error {
		live, err := tx.LiveCalendarFeed(ctx, a.actor.ID)
		if err != nil {
			return err
		}
		byHash, err := tx.CalendarFeedByHash(ctx, "hash-a")
		if err != nil {
			return err
		}
		if live.ID != feed.ID || byHash.ID != feed.ID || live.Filter != "tag:ops" ||
			live.TenantID != a.scope.TenantID || !live.CreatedAt.Equal(created) || live.RevokedAt != nil {
			t.Fatalf("read back %+v and %+v", live, byHash)
		}
		return nil
	})
	view(b, func(tx store.Tx) error {
		if _, err := tx.CalendarFeedByHash(ctx, "hash-a"); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("another tenant found the feed: %v", err)
		}
		return nil
	})
	if err := update(b, func(tx store.Tx) error { return tx.RevokeCalendarFeed(ctx, feed.ID, created) }); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("another tenant revoked the feed: %v", err)
	}

	if err := update(a, func(tx store.Tx) error { return tx.SetCalendarFeedFilter(ctx, feed.ID, "") }); err != nil {
		t.Fatal(err)
	}
	if err := update(a, func(tx store.Tx) error { return tx.RevokeCalendarFeed(ctx, feed.ID, created) }); err != nil {
		t.Fatal(err)
	}
	view(a, func(tx store.Tx) error {
		if _, err := tx.LiveCalendarFeed(ctx, a.actor.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("a revoked feed is still live: %v", err)
		}
		if _, err := tx.CalendarFeedByHash(ctx, "hash-a"); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("a revoked feed is still found by hash: %v", err)
		}
		return nil
	})
	if err := update(a, func(tx store.Tx) error { return tx.SetCalendarFeedFilter(ctx, feed.ID, "x") }); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("a revoked feed took a filter: %v", err)
	}
	if err := update(a, func(tx store.Tx) error { return tx.CreateCalendarFeed(ctx, &second, "hash-b") }); err != nil {
		t.Fatalf("a new feed after revoking: %v", err)
	}
}
