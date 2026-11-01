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
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/webhook"
)

func newChannelFixture(t *testing.T, opts ...Option) inboxFixture {
	t.Helper()
	l, _, scope, me := newLocalWith(t, opts...)
	seedTaskProject(t, l, scope, "infra")
	bob := seedTaskActor(t, l, scope, "bob", core.RoleMember, core.ScopeAll)
	carol := seedTaskActor(t, l, scope, "carol", core.RoleMember, core.ScopeAll)
	return inboxFixture{l: l, scope: scope, ada: me, bob: bob, carol: carol,
		adaC: taskContext(me), bobC: taskContext(bob), carolC: taskContext(carol)}
}

func mustChannel(t *testing.T, l *Local, ctx context.Context, in core.InboxChannelInput) *core.IssuedInboxChannel {
	t.Helper()
	c, err := l.CreateInboxChannel(ctx, in)
	if err != nil {
		t.Fatalf("creating channel %q: %v", in.Name, err)
	}
	return c
}

// queued lists every channel delivery in the store, oldest first.
func queued(t *testing.T, l *Local, scope core.TenantScope, channelID string) []core.ChannelDelivery {
	t.Helper()
	ctx := context.Background()
	var out []core.ChannelDelivery
	if err := l.store.View(ctx, scope, func(tx store.Tx) error {
		var err error
		out, err = tx.ListChannelDeliveries(ctx, core.ChannelDeliveryFilter{ChannelID: channelID,
			Page: core.Page{Limit: 100, Sort: core.SortCreatedAt, Direction: core.Ascending}})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestChannelsAreOwnedValidatedAndShowTheirSecretOnce(t *testing.T) {
	f := newChannelFixture(t, WithHooks(HookServer))
	issued := mustChannel(t, f.l, f.bobC, core.InboxChannelInput{Name: "phone", URL: "https://ntfy.sh/bob-tix", Format: "NTFY"})
	if issued.SigningSecret == "" || issued.Format != core.ChannelNtfy || !issued.Active ||
		len(issued.Kinds) != len(core.InboxKinds()) {
		t.Fatalf("created = %+v", issued)
	}
	raw, _ := json.Marshal(issued.InboxChannel)
	if strings.Contains(string(raw), issued.SigningSecret) {
		t.Fatal("a channel serialises its secret outside the one-time answer")
	}
	listed, err := f.l.ListInboxChannels(f.bobC)
	if err != nil || len(listed) != 1 {
		t.Fatalf("listed = %v, %v", listed, err)
	}
	if mine, _ := f.l.ListInboxChannels(f.carolC); len(mine) != 0 {
		t.Fatalf("carol sees bob's channels: %v", mine)
	}

	cases := []struct {
		name string
		in   core.InboxChannelInput
		kind core.Kind
	}{
		{"no name", core.InboxChannelInput{URL: "https://example.com/h"}, core.KindInvalid},
		{"unknown format", core.InboxChannelInput{Name: "x", URL: "https://example.com/h", Format: "pager"}, core.KindInvalid},
		{"no kinds", core.InboxChannelInput{Name: "x", URL: "https://example.com/h", Kinds: []string{}}, core.KindInvalid},
		{"unknown kind", core.InboxChannelInput{Name: "x", URL: "https://example.com/h", Kinds: []string{"all"}}, core.KindInvalid},
		{"a private address", core.InboxChannelInput{Name: "x", URL: "http://169.254.169.254/latest"}, core.KindInvalid},
		{"the same name", core.InboxChannelInput{Name: "Phone", URL: "https://example.com/h"}, core.KindConflict},
	}
	for _, tc := range cases {
		if _, err := f.l.CreateInboxChannel(f.bobC, tc.in); !core.IsKind(err, tc.kind) {
			t.Errorf("%s: err = %v, want %s", tc.name, err, tc.kind)
		}
	}

	off := false
	if _, err := f.l.UpdateInboxChannel(f.carolC, issued.ID, core.InboxChannelUpdate{Active: &off}); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("carol changing bob's channel: err = %v, want not found", err)
	}
	events, audits := countRows(t, f.l, f.scope)
	kinds := []string{"mention"}
	updated, err := f.l.UpdateInboxChannel(f.bobC, issued.ID, core.InboxChannelUpdate{Kinds: &kinds, Active: &off})
	if err != nil || updated.Active || len(updated.Kinds) != 1 {
		t.Fatalf("updated = %+v, %v", updated, err)
	}
	if _, err := f.l.UpdateInboxChannel(f.bobC, issued.ID, core.InboxChannelUpdate{Kinds: &kinds}); err != nil {
		t.Fatal(err)
	}
	if e, a := countRows(t, f.l, f.scope); e != events+1 || a != audits+1 {
		t.Fatalf("an update and a no-op wrote %d events and %d audits, want one each", e-events, a-audits)
	}
	if err := f.l.DeleteInboxChannel(f.carolC, issued.ID); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("carol deleting bob's channel: err = %v", err)
	}
	if err := f.l.DeleteInboxChannel(f.bobC, issued.ID); err != nil {
		t.Fatal(err)
	}
	if left, _ := f.l.ListInboxChannels(f.bobC); len(left) != 0 {
		t.Fatalf("deleted channel still listed: %v", left)
	}
}

func TestTheAuditLogNeverKeepsAChannelsURL(t *testing.T) {
	f := newChannelFixture(t, WithHooks(HookServer))
	mustChannel(t, f.l, f.bobC, core.InboxChannelInput{Name: "slack",
		URL: "https://hooks.slack.com/services/T000/B000/sekret", Format: "slack"})
	entries, _, err := f.l.ListAudit(f.adaC, core.AuditFilter{Page: core.Page{Limit: 10}})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(string(e.After), "sekret") {
			t.Fatalf("the audit entry %s keeps the url: %s", e.Action, e.After)
		}
	}
}

func TestInboxItemsQueueForTheRecipientsOwnMatchingChannels(t *testing.T) {
	f := newChannelFixture(t, WithHooks(HookServer))
	mentions := mustChannel(t, f.l, f.bobC, core.InboxChannelInput{Name: "mentions", URL: "https://example.com/m",
		Kinds: []string{"mention"}})
	everything := mustChannel(t, f.l, f.bobC, core.InboxChannelInput{Name: "all", URL: "https://example.com/a"})
	off := false
	sleeping := mustChannel(t, f.l, f.bobC, core.InboxChannelInput{Name: "off", URL: "https://example.com/o", Active: &off})
	carols := mustChannel(t, f.l, f.carolC, core.InboxChannelInput{Name: "carol", URL: "https://example.com/c"})

	task := mustCreateTask(t, f.l, f.adaC, core.CreateTaskInput{ProjectRef: "infra", Title: "t", AssigneeActorID: "bob"})
	if got := len(queued(t, f.l, f.scope, mentions.ID)); got != 0 {
		t.Fatalf("an assignment reached a mentions-only channel: %d", got)
	}
	if got := len(queued(t, f.l, f.scope, everything.ID)); got != 1 {
		t.Fatalf("the assignment queued %d deliveries on the all channel, want 1", got)
	}
	if _, err := f.l.AddComment(f.adaC, core.TaskRef{ID: task.ID}, "@bob look"); err != nil {
		t.Fatal(err)
	}
	if got := queued(t, f.l, f.scope, mentions.ID); len(got) != 1 || got[0].ItemID == "" {
		t.Fatalf("the mention queued %v on the mentions channel", got)
	}
	if n := len(queued(t, f.l, f.scope, sleeping.ID)); n != 0 {
		t.Fatalf("a channel turned off was queued %d deliveries", n)
	}
	if n := len(queued(t, f.l, f.scope, carols.ID)); n != 0 {
		t.Fatalf("somebody else's item reached carol's channel %d times", n)
	}

	if _, err := f.l.AddComment(f.adaC, core.TaskRef{ID: task.ID}, "@carol and @bob"); err != nil {
		t.Fatal(err)
	}
	if got := queued(t, f.l, f.scope, carols.ID); len(got) != 1 {
		t.Fatalf("a comment mentioning two people queued %d deliveries on carol's channel, want her own one", len(got))
	}
	if n := len(queued(t, f.l, f.scope, everything.ID)); n != 3 {
		t.Fatalf("bob's all channel holds %d deliveries, want 3", n)
	}

	if _, err := f.l.PutInboxPrefs(f.bobC, core.InboxPrefsInput{Muted: []string{"comment", "mention"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.l.AddComment(f.adaC, core.TaskRef{ID: task.ID}, "@bob again"); err != nil {
		t.Fatal(err)
	}
	if n := len(queued(t, f.l, f.scope, everything.ID)); n != 3 {
		t.Fatalf("muted kinds were delivered: the all channel holds %d deliveries, want 3", n)
	}

	if _, err := f.l.AddComment(f.bobC, core.TaskRef{ID: task.ID}, "@bob note to self"); err != nil {
		t.Fatal(err)
	}
	if n := len(queued(t, f.l, f.scope, everything.ID)); n != 3 {
		t.Fatalf("bob's own comment reached his channel: %d deliveries", n)
	}
}

func TestNothingIsQueuedWhenThisProcessDeliversNothing(t *testing.T) {
	f := newChannelFixture(t, WithHooks(HookOff))
	c := mustChannel(t, f.l, f.bobC, core.InboxChannelInput{Name: "all", URL: "https://example.com/a"})
	mustCreateTask(t, f.l, f.adaC, core.CreateTaskInput{ProjectRef: "infra", Title: "t", AssigneeActorID: "bob"})
	if n := len(queued(t, f.l, f.scope, c.ID)); n != 0 {
		t.Fatalf("hook mode off queued %d deliveries", n)
	}
}

// channelReceiver records what a channel's endpoint was sent.
type channelReceiver struct {
	*httptest.Server
	mu     sync.Mutex
	bodies []string
	heads  []http.Header
	status int
}

func newChannelReceiver(t *testing.T) *channelReceiver {
	t.Helper()
	r := &channelReceiver{status: http.StatusOK}
	r.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		r.mu.Lock()
		r.bodies, r.heads = append(r.bodies, string(body)), append(r.heads, req.Header.Clone())
		status := r.status
		r.mu.Unlock()
		w.WriteHeader(status)
	}))
	t.Cleanup(r.Close)
	return r
}

func (r *channelReceiver) last() (string, http.Header) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.bodies) == 0 {
		return "", nil
	}
	return r.bodies[len(r.bodies)-1], r.heads[len(r.heads)-1]
}

func TestAMentionIsPostedToTheChannelInItsFormatAndSigned(t *testing.T) {
	f := newChannelFixture(t, WithPrivateWebhookTargets(true), WithInsecureWebhooks(true))
	rec := newChannelReceiver(t)
	c := mustChannel(t, f.l, f.bobC, core.InboxChannelInput{Name: "slack", URL: rec.URL, Format: "slack"})
	task := mustCreateTask(t, f.l, f.adaC, core.CreateTaskInput{ProjectRef: "infra", Title: "Cut over"})
	if _, err := f.l.AddComment(f.adaC, core.TaskRef{ID: task.ID}, "@bob <look> & see"); err != nil {
		t.Fatal(err)
	}
	body, head := rec.last()
	var msg struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(body), &msg); err != nil {
		t.Fatalf("the channel was sent %q, not slack json: %v", body, err)
	}
	if !strings.Contains(msg.Text, "alice mentioned you on "+task.Ref+" Cut over") ||
		!strings.Contains(msg.Text, "&lt;look&gt; &amp; see") {
		t.Fatalf("slack text = %q", msg.Text)
	}
	if !webhook.Verify(c.SigningSecret, head.Get(webhook.HeaderTimestamp), []byte(body), head.Get(webhook.HeaderSignature)) ||
		head.Get(webhook.HeaderEvent) != webhook.ChannelEventItem {
		t.Fatalf("the delivery is not signed with the channel's secret: %v", head)
	}
	if got := queued(t, f.l, f.scope, c.ID); len(got) != 1 || got[0].Status != core.DeliveryDelivered {
		t.Fatalf("the delivery is recorded as %+v", got)
	}
}

func TestSendTestAndRedeliver(t *testing.T) {
	f := newChannelFixture(t, WithPrivateWebhookTargets(true), WithInsecureWebhooks(true))
	rec := newChannelReceiver(t)
	c := mustChannel(t, f.l, f.bobC, core.InboxChannelInput{Name: "hook", URL: rec.URL})
	d, err := f.l.TestInboxChannel(f.bobC, c.ID)
	if err != nil || d.Status != core.DeliveryDelivered || !d.Test() {
		t.Fatalf("test delivery = %+v, %v", d, err)
	}
	if body, head := rec.last(); !strings.Contains(body, `"type":"inbox.test"`) || head.Get(webhook.HeaderEvent) != webhook.ChannelEventTest {
		t.Fatalf("the test sent %q", body)
	}
	if _, err := f.l.TestInboxChannel(f.carolC, c.ID); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("carol testing bob's channel: err = %v", err)
	}

	rec.mu.Lock()
	rec.status = http.StatusInternalServerError
	rec.mu.Unlock()
	failing, err := f.l.TestInboxChannel(f.bobC, c.ID)
	if err != nil || failing.Status != core.DeliveryPending || failing.LastStatusCode != 500 {
		t.Fatalf("a refused test = %+v, %v", failing, err)
	}
	rec.mu.Lock()
	rec.status = http.StatusOK
	rec.mu.Unlock()
	if err := f.l.RedeliverChannelDelivery(f.carolC, failing.ID); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("carol redelivering bob's delivery: err = %v", err)
	}
	if err := f.l.RedeliverChannelDelivery(f.bobC, failing.ID); err != nil {
		t.Fatal(err)
	}
	page, _, err := f.l.ListChannelDeliveries(f.bobC, core.ChannelDeliveryFilter{ChannelID: c.ID})
	if err != nil || len(page) != 2 || page[0].ID != failing.ID || page[0].Status != core.DeliveryDelivered {
		t.Fatalf("deliveries after redelivery = %+v, %v", page, err)
	}
	if _, _, err := f.l.ListChannelDeliveries(f.carolC, core.ChannelDeliveryFilter{ChannelID: c.ID}); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("carol listing bob's deliveries: err = %v", err)
	}

	off := false
	if _, err := f.l.UpdateInboxChannel(f.bobC, c.ID, core.InboxChannelUpdate{Active: &off}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.l.TestInboxChannel(f.bobC, c.ID); !core.IsKind(err, core.KindPrecondition) {
		t.Fatalf("testing a channel turned off: err = %v, want precondition", err)
	}
}
