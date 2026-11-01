// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

func TestInboxChannelsQueueClaimMarkAndCascade(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	update := func(fn func(tx store.Tx) error) {
		t.Helper()
		if err := s.Update(ctx, f.scope, fn); err != nil {
			t.Fatal(err)
		}
	}
	c := core.InboxChannel{ActorID: f.actor.ID, Name: "phone", URL: "https://ntfy.invalid/x", Secret: "s",
		Format: core.ChannelNtfy, Kinds: []core.InboxKind{core.InboxMention}, Active: true}
	var first, second core.ChannelDelivery
	update(func(tx store.Tx) error {
		if err := tx.CreateInboxChannel(ctx, &c); err != nil {
			return err
		}
		c.Name, c.Active = "pager", false
		if err := tx.UpdateInboxChannel(ctx, &c); err != nil {
			return err
		}
		first = core.ChannelDelivery{ChannelID: c.ID, ItemID: "item-1"}
		if err := tx.EnqueueChannelDelivery(ctx, &first); err != nil {
			return err
		}
		clk.Advance(time.Second)
		second = core.ChannelDelivery{ChannelID: c.ID}
		return tx.EnqueueChannelDelivery(ctx, &second)
	})
	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		got, err := tx.ListInboxChannels(ctx, []string{f.actor.ID})
		if err != nil || len(got) != 1 || got[0].Name != "pager" || got[0].Active || got[0].Kinds[0] != core.InboxMention {
			t.Errorf("channels = %+v, %v", got, err)
		}
		if other, _ := tx.ListInboxChannels(ctx, []string{"nobody"}); len(other) != 0 {
			t.Errorf("another actor's listing holds %+v", other)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	now := clk.Now()
	update(func(tx store.Tx) error {
		claimed, err := tx.ClaimChannelDeliveries(ctx, "me", now, now.Add(time.Minute), 10)
		if err != nil || len(claimed) != 2 || claimed[0].ID != first.ID {
			t.Errorf("claimed = %+v, %v", claimed, err)
		}
		again, err := tx.ClaimChannelDeliveries(ctx, "other", now, now.Add(time.Minute), 10)
		if err != nil || len(again) != 0 {
			t.Errorf("a second claimer took %+v", again)
		}
		if err := tx.MarkChannelDelivered(ctx, first.ID, 200, now); err != nil {
			return err
		}
		return tx.MarkChannelFailed(ctx, second.ID, 500, "endpoint responded 500", now.Add(time.Hour), true)
	})
	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		page, err := tx.ListChannelDeliveries(ctx, core.ChannelDeliveryFilter{ChannelID: c.ID,
			Page: core.Page{Limit: 10, Sort: core.SortCreatedAt, Direction: core.Descending}})
		if err != nil || len(page) != 2 || page[0].ID != second.ID || !page[0].Test() ||
			page[0].Status != core.DeliveryFailed || page[1].Status != core.DeliveryDelivered || page[1].Attempts != 1 {
			t.Errorf("deliveries = %+v, %v", page, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	update(func(tx store.Tx) error { return tx.DeleteInboxChannel(ctx, c.ID) })
	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		if _, err := tx.GetChannelDelivery(ctx, first.ID); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("a removed channel's delivery survived: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
