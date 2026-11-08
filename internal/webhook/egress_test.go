// SPDX-License-Identifier: AGPL-3.0-or-later

package webhook

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/egress"
	"github.com/heliopsy/tix/internal/store"
)

// loopbackGuard is the operator's escape hatch opened for loopback only, so
// the suite can deliver to an httptest server.
var loopbackGuard = Guard{Policy: egress.Policy{Allow: []netip.Prefix{
	netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("::1/128")}}}

// hitCounter is a loopback receiver that counts what reached it.
func hitCounter(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

// addChannel stores an active channel with one queued test delivery.
func (f fixture) addChannel(t *testing.T, target string) string {
	t.Helper()
	ctx := context.Background()
	var id string
	if err := f.store.Update(ctx, f.scope, func(tx store.Tx) error {
		a := core.Actor{Kind: core.ActorUser, Handle: "carol"}
		if err := tx.CreateActor(ctx, &a); err != nil {
			return err
		}
		c := core.InboxChannel{ActorID: a.ID, Name: "phone", URL: target, Secret: "s",
			Format: core.ChannelGeneric, Kinds: core.InboxKinds(), Active: true}
		if err := tx.CreateInboxChannel(ctx, &c); err != nil {
			return err
		}
		d := core.ChannelDelivery{ChannelID: c.ID}
		if err := tx.EnqueueChannelDelivery(ctx, &d); err != nil {
			return err
		}
		id = d.ID
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return id
}

// channelDelivery reads one channel delivery back.
func (f fixture) channelDelivery(t *testing.T, id string) *core.ChannelDelivery {
	t.Helper()
	var got *core.ChannelDelivery
	if err := f.store.View(context.Background(), f.scope, func(tx store.Tx) error {
		var err error
		got, err = tx.GetChannelDelivery(context.Background(), id)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return got
}

// deliverTo queues one delivery of the kind named to target and drains it
// under the guard given.
func deliverTo(t *testing.T, kind, target string, g Guard) (fixture, string) {
	t.Helper()
	f := newFixture(t)
	id := ""
	switch kind {
	case "webhook":
		f.addEndpoint(t, target, "top-secret", true, "*")
		f.emit(t, core.EventTaskCreated)
	case "channel":
		id = f.addChannel(t, target)
	}
	d := NewDispatcher(f.store, f.scope, f.clk, WithGuard(g))
	defer d.Stop()
	if _, err := d.Drain(context.Background(), DefaultBound()); err != nil {
		t.Fatalf("draining: %v", err)
	}
	return f, id
}

// lastError is the recorded failure of the one delivery of the kind named.
func lastError(t *testing.T, f fixture, kind, id string) string {
	t.Helper()
	if kind == "channel" {
		return f.channelDelivery(t, id).LastError
	}
	return f.only(t).LastError
}

func TestDeliveryToALoopbackReceiverNeedsTheEscapeHatch(t *testing.T) {
	for _, kind := range []string{"webhook", "channel"} {
		t.Run(kind+" strict", func(t *testing.T) {
			srv, hits := hitCounter(t)
			f, id := deliverTo(t, kind, srv.URL, Guard{})
			if hits.Load() != 0 {
				t.Fatal("a strict guard delivered to a loopback receiver")
			}
			if got := lastError(t, f, kind, id); got != "endpoint address is not permitted" {
				t.Fatalf("last error = %q, want the generic refusal", got)
			}
		})
		t.Run(kind+" by name", func(t *testing.T) {
			srv, hits := hitCounter(t)
			byName := strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)
			deliverTo(t, kind, byName, Guard{})
			if hits.Load() != 0 {
				t.Fatal("a strict guard delivered to a name that resolves to loopback")
			}
		})
		t.Run(kind+" allowed", func(t *testing.T) {
			srv, hits := hitCounter(t)
			deliverTo(t, kind, srv.URL, loopbackGuard)
			if hits.Load() != 1 {
				t.Fatalf("the receiver saw %d deliveries with loopback allowed, want 1", hits.Load())
			}
		})
		t.Run(kind+" allow private", func(t *testing.T) {
			srv, hits := hitCounter(t)
			deliverTo(t, kind, srv.URL, NewGuard(true))
			if hits.Load() != 1 {
				t.Fatalf("the receiver saw %d deliveries with private targets allowed, want 1", hits.Load())
			}
		})
	}
}

// A receiver the operator allowed answers with a redirect into the network;
// the delivery stops at the redirect and the inner address is never reached.
func TestDeliveryRedirectIntoTheNetworkIsNotFollowed(t *testing.T) {
	for _, kind := range []string{"webhook", "channel"} {
		t.Run(kind, func(t *testing.T) {
			inner, hits := hitCounter(t)
			outer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, inner.URL+"/latest/meta-data/", http.StatusFound)
			}))
			t.Cleanup(outer.Close)
			byName := strings.Replace(outer.URL, "127.0.0.1", "localhost", 1)
			g := Guard{Policy: egress.Policy{AllowHosts: []string{"localhost"}}}
			f, id := deliverTo(t, kind, byName, g)
			if hits.Load() != 0 {
				t.Fatal("the redirect target was reached")
			}
			if got := lastError(t, f, kind, id); strings.Contains(got, "127.0.0.1") || got == "" {
				t.Fatalf("last error = %q, want a failure that names no address", got)
			}
		})
	}
}

// Registration settles a literal address or localhost; a name that resolves to
// loopback only at delivery time is the dialer's to refuse. This dials the
// delivery client's own transport, as such a delivery would.
func TestDeliveryTransportRefusesALoopbackAddressAtDialTime(t *testing.T) {
	srv, _ := hitCounter(t)
	addr := strings.TrimPrefix(srv.URL, "http://")
	for name, tc := range map[string]struct {
		g  Guard
		ok bool
	}{
		"strict":       {Guard{}, false},
		"escape hatch": {loopbackGuard, true},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			d := NewDispatcher(f.store, f.scope, f.clk, WithGuard(tc.g))
			defer d.Stop()
			conn, err := d.client.Transport.(*http.Transport).DialContext(context.Background(), "tcp", addr)
			if conn != nil {
				_ = conn.Close()
			}
			if tc.ok != (err == nil) {
				t.Fatalf("dial = %v, want ok=%v", err, tc.ok)
			}
			if err != nil && (!errors.Is(err, ErrBlockedTarget) || strings.Contains(err.Error(), "127.0.0.1")) {
				t.Fatalf("dial = %v, want the bare refusal", err)
			}
		})
	}
}
