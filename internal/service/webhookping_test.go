// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/webhook"
)

// signedPost is one delivery as a receiver saw it.
type signedPost struct {
	header http.Header
	body   []byte
}

// signedReceiver keeps every delivery whole, so a test can check the
// signature the way a real receiver would.
type signedReceiver struct {
	server *httptest.Server
	mu     sync.Mutex
	posts  []signedPost
}

func newSignedReceiver(t *testing.T) *signedReceiver {
	t.Helper()
	r := &signedReceiver{}
	r.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		r.mu.Lock()
		r.posts = append(r.posts, signedPost{header: req.Header.Clone(), body: body})
		r.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(r.server.Close)
	return r
}

func (r *signedReceiver) received() []signedPost {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]signedPost(nil), r.posts...)
}

// deliveriesOf returns the queued deliveries of one event.
func deliveriesOf(t *testing.T, l *Local, scope core.TenantScope, seq int64) []core.WebhookDelivery {
	t.Helper()
	var out []core.WebhookDelivery
	for _, d := range allDeliveries(t, l, scope) {
		if d.EventSeq == seq {
			out = append(out, d)
		}
	}
	return out
}

// A ping reaches the endpoint it names, signed with that endpoint's secret,
// even though the endpoint's filter would not take a webhook.ping event, and
// reaches no other endpoint, not even one subscribed to everything.
func TestPingWebhookDeliversASignedPingToThatEndpointOnly(t *testing.T) {
	l, ctx, scope := hookFixture(t, HookInline)
	target, bystander := newSignedReceiver(t), newSignedReceiver(t)
	pinged := putEndpoint(t, l, ctx, core.WebhookInput{
		URL: target.server.URL, EventTypes: []string{"task.*"}, Active: true,
	})
	putEndpoint(t, l, ctx, core.WebhookInput{URL: bystander.server.URL, Active: true})
	before := len(bystander.received())

	d, err := l.PingWebhook(ctx, pinged.ID)
	if err != nil {
		t.Fatalf("PingWebhook: %v", err)
	}
	if d.Status != core.DeliveryDelivered || d.EndpointID != pinged.ID || d.Attempts != 1 {
		t.Fatalf("ping answered %+v, want one delivered attempt to %s", d, pinged.ID)
	}

	posts := target.received()
	if len(posts) != 1 {
		t.Fatalf("the pinged endpoint received %d posts, want 1", len(posts))
	}
	got := posts[0]
	if ev := got.header.Get(webhook.HeaderEvent); ev != "webhook.ping" {
		t.Errorf("the ping arrived as event %q, want webhook.ping", ev)
	}
	if !webhook.Verify(pinged.SigningSecret, got.header.Get(webhook.HeaderTimestamp), got.body,
		got.header.Get(webhook.HeaderSignature)) {
		t.Error("the ping's signature does not verify with the endpoint's secret")
	}
	var event core.Event
	if err := json.Unmarshal(got.body, &event); err != nil || event.Seq != d.EventSeq || event.SubjectID != pinged.ID {
		t.Errorf("the ping body is %s (%v), want event %d about endpoint %s", got.body, err, d.EventSeq, pinged.ID)
	}

	if n := len(bystander.received()) - before; n != 0 {
		t.Errorf("an endpoint subscribed to every event received %d posts of somebody else's ping", n)
	}
	if queued := deliveriesOf(t, l, scope, d.EventSeq); len(queued) != 1 {
		t.Errorf("the ping event was queued %d times, want once", len(queued))
	}

	listed, _, err := l.ListDeliveries(ctx, core.DeliveryFilter{EndpointID: pinged.ID})
	if err != nil {
		t.Fatalf("ListDeliveries: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != d.ID {
		t.Errorf("the delivery log lists %+v for the endpoint, want the ping %s", listed, d.ID)
	}

	history := dumpHistory(t, l, scope)
	if !strings.Contains(history, `"action":"webhook.ping"`) {
		t.Error("the ping wrote no audit entry")
	}
	if strings.Contains(history, pinged.SigningSecret) {
		t.Error("the signing secret reached the audit log or the event stream")
	}

	if err := l.RedeliverWebhook(ctx, d.ID); err != nil {
		t.Fatalf("RedeliverWebhook of a ping: %v", err)
	}
	if again := target.received(); len(again) != 2 || again[1].header.Get(webhook.HeaderEvent) != "webhook.ping" {
		t.Errorf("redelivering the ping sent %d posts, want a second webhook.ping", len(again))
	}
}

// In the server mode the ping is queued for the server and answered pending.
func TestPingWebhookQueuesForTheServer(t *testing.T) {
	l, ctx, scope := hookFixture(t, HookServer)
	e := putEndpoint(t, l, ctx, core.WebhookInput{URL: "https://hooks.example.com/tix", Active: true})
	d, err := l.PingWebhook(ctx, e.ID)
	if err != nil {
		t.Fatalf("PingWebhook: %v", err)
	}
	if d.Status != core.DeliveryPending || d.Attempts != 0 {
		t.Errorf("a ping in the server mode answered %+v, want pending and unattempted", d)
	}
	if queued := deliveriesOf(t, l, scope, d.EventSeq); len(queued) != 1 || queued[0].EndpointID != e.ID {
		t.Errorf("the ping queued %+v, want one delivery to %s", queued, e.ID)
	}
}

func TestPingWebhookRefusals(t *testing.T) {
	cases := []struct {
		name     string
		mode     HookMode
		inactive bool
		id       func(e *core.IssuedWebhook) string
		ctx      func(ctx context.Context) context.Context
		want     core.Kind
	}{
		{name: "an inactive endpoint", mode: HookServer, inactive: true, want: core.KindPrecondition},
		{name: "delivery turned off", mode: HookOff, want: core.KindPrecondition},
		{name: "an unknown endpoint", mode: HookServer, want: core.KindNotFound,
			id: func(*core.IssuedWebhook) string { return "nosuchendpoint" }},
		{name: "no identifier", mode: HookServer, want: core.KindInvalid,
			id: func(*core.IssuedWebhook) string { return " " }},
		{name: "a caller without the webhook scope", mode: HookServer, want: core.KindForbidden,
			ctx: func(ctx context.Context) context.Context {
				actor, _ := core.RequireActor(ctx)
				return core.WithActor(context.Background(), &core.Actor{
					ID: "v1", TenantID: actor.TenantID, Kind: core.ActorUser, Role: core.RoleViewer})
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l, ctx, scope := hookFixture(t, tc.mode)
			e := putEndpoint(t, l, ctx, core.WebhookInput{
				URL: "https://hooks.example.com/tix", Active: !tc.inactive,
			})
			before := len(allDeliveries(t, l, scope))
			id, call := e.ID, ctx
			if tc.id != nil {
				id = tc.id(e)
			}
			if tc.ctx != nil {
				call = tc.ctx(ctx)
			}
			if _, err := l.PingWebhook(call, id); !core.IsKind(err, tc.want) {
				t.Fatalf("PingWebhook = %v, want %s", err, tc.want)
			}
			if after := len(allDeliveries(t, l, scope)); after != before {
				t.Errorf("a refused ping queued %d deliveries", after-before)
			}
		})
	}
}
