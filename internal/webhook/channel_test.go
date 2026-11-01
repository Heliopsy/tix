// SPDX-License-Identifier: AGPL-3.0-or-later

package webhook

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

func sampleItem() core.InboxItem {
	return core.InboxItem{ID: "i1", Kind: core.InboxMention, TaskID: "t1", TaskRef: "infra-42",
		TaskTitle: "Cut over", ActorHandle: "ada", Excerpt: "@bob ping @everyone"}
}

func TestEveryFormatSaysWhoDidWhatAndLinksTheTask(t *testing.T) {
	m := NewChannelMessage(sampleItem(), "https://tix.example.com/")
	if m.Link != "https://tix.example.com/tasks/infra-42" || m.What != "ada mentioned you on infra-42 Cut over" {
		t.Fatalf("message = %+v", m)
	}
	cases := []struct {
		format core.ChannelFormat
		want   []string
		ctype  string
	}{
		{core.ChannelSlack, []string{`ada mentioned you on \u003chttps://tix.example.com/tasks/infra-42|infra-42 Cut over\u003e`}, contentTypeJSON},
		{core.ChannelDiscord, []string{`[infra-42 Cut over](https://tix.example.com/tasks/infra-42)`, `"allowed_mentions":{"parse":[]}`}, contentTypeJSON},
		{core.ChannelGeneric, []string{`"type":"inbox.item"`, `"url":"https://tix.example.com/tasks/infra-42"`, `"kind":"mention"`}, contentTypeJSON},
		{core.ChannelNtfy, []string{"ada mentioned you on infra-42 Cut over\n@bob ping @everyone"}, "text/plain; charset=utf-8"},
	}
	for _, tc := range cases {
		req, err := ShapeChannelRequest(tc.format, m)
		if err != nil {
			t.Fatalf("%s: %v", tc.format, err)
		}
		if req.ContentType != tc.ctype {
			t.Errorf("%s: content type %q", tc.format, req.ContentType)
		}
		for _, want := range tc.want {
			if !strings.Contains(string(req.Body), want) {
				t.Errorf("%s: body %s lacks %s", tc.format, req.Body, want)
			}
		}
		if tc.format != core.ChannelNtfy && !json.Valid(req.Body) {
			t.Errorf("%s: body is not json", tc.format)
		}
	}
	ntfy, _ := ShapeChannelRequest(core.ChannelNtfy, m)
	if ntfy.Headers["Click"] != m.Link || ntfy.Headers["Title"] != "tix: infra-42" {
		t.Fatalf("ntfy headers = %v", ntfy.Headers)
	}
	bare := NewChannelMessage(sampleItem(), "")
	if bare.Link != "" {
		t.Fatalf("a tenant without a domain got a link %q", bare.Link)
	}
	if slack, _ := ShapeChannelRequest(core.ChannelSlack, bare); strings.Contains(string(slack.Body), "|") {
		t.Fatalf("a message without a link drew one: %s", slack.Body)
	}
}

func TestAChannelDeliveryWhoseChannelIsOffFailsWithoutAnAttempt(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	rec := newReceiver(t)
	var d core.ChannelDelivery
	if err := f.store.Update(ctx, f.scope, func(tx store.Tx) error {
		a := core.Actor{Kind: core.ActorUser, Handle: "bob"}
		if err := tx.CreateActor(ctx, &a); err != nil {
			return err
		}
		c := core.InboxChannel{ActorID: a.ID, Name: "off", URL: rec.URL, Secret: "s",
			Format: core.ChannelGeneric, Kinds: core.InboxKinds()}
		if err := tx.CreateInboxChannel(ctx, &c); err != nil {
			return err
		}
		d = core.ChannelDelivery{ChannelID: c.ID}
		return tx.EnqueueChannelDelivery(ctx, &d)
	}); err != nil {
		t.Fatal(err)
	}
	disp := NewDispatcher(f.store, f.scope, f.clk)
	defer disp.Stop()
	if res, err := disp.Drain(ctx, DefaultBound()); err != nil || res.Attempted != 0 {
		t.Fatalf("drain = %+v, %v; a channel turned off was attempted", res, err)
	}
	var got *core.ChannelDelivery
	_ = f.store.View(ctx, f.scope, func(tx store.Tx) error {
		got, _ = tx.GetChannelDelivery(ctx, d.ID)
		return nil
	})
	if got.Status != core.DeliveryFailed || got.LastError != "channel is turned off" || rec.count() != 0 {
		t.Fatalf("delivery = %+v, receiver saw %d", got, rec.count())
	}
}
