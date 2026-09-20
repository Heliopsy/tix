package webhook

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/thereisnotime/tix/internal/clock"
	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/store"
	"github.com/thereisnotime/tix/internal/store/sqlite"
)

type fixture struct {
	store *sqlite.Store
	clk   *clock.Fake
	scope core.TenantScope
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	ctx := context.Background()
	clk := clock.NewFakeAt()
	s, err := sqlite.Open(filepath.Join(t.TempDir(), "tix.db"), clk)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	tenant := core.Tenant{Key: "acme", Name: "Acme"}
	if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
		return u.CreateTenant(ctx, &tenant)
	}); err != nil {
		t.Fatalf("creating tenant: %v", err)
	}
	return fixture{store: s, clk: clk, scope: core.TenantScope{TenantID: tenant.ID}}
}

func (f fixture) addEndpoint(t *testing.T, url, secret string, active bool, types ...string) core.WebhookEndpoint {
	t.Helper()
	e := core.WebhookEndpoint{URL: url, Secret: secret, Active: active, EventTypes: types}
	if err := f.store.Update(context.Background(), f.scope, func(tx store.Tx) error {
		return tx.PutWebhook(context.Background(), &e)
	}); err != nil {
		t.Fatalf("registering endpoint: %v", err)
	}
	return e
}

func (f fixture) setActive(t *testing.T, e core.WebhookEndpoint, active bool) {
	t.Helper()
	e.Active = active
	if err := f.store.Update(context.Background(), f.scope, func(tx store.Tx) error {
		return tx.PutWebhook(context.Background(), &e)
	}); err != nil {
		t.Fatalf("updating endpoint: %v", err)
	}
}

// emit appends an event and queues it for every matching endpoint, which is
// what a mutation does inside its own transaction.
func (f fixture) emit(t *testing.T, typ core.EventType) (core.Event, int) {
	t.Helper()
	ctx := context.Background()
	e := core.Event{Type: typ, SubjectType: "task", SubjectID: "task-1", Payload: map[string]any{"ref": "ALPHA-1"}}
	queued := 0
	if err := f.store.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.AppendEvent(ctx, &e); err != nil {
			return err
		}
		n, err := Enqueue(ctx, tx, e)
		queued = n
		return err
	}); err != nil {
		t.Fatalf("emitting event: %v", err)
	}
	return e, queued
}

func (f fixture) deliveries(t *testing.T) []core.WebhookDelivery {
	t.Helper()
	ctx := context.Background()
	var out []core.WebhookDelivery
	if err := f.store.View(ctx, f.scope, func(tx store.Tx) error {
		var err error
		out, err = tx.ListDeliveries(ctx, core.DeliveryFilter{})
		return err
	}); err != nil {
		t.Fatalf("listing deliveries: %v", err)
	}
	return out
}

func (f fixture) delivery(t *testing.T, id string) core.WebhookDelivery {
	t.Helper()
	ctx := context.Background()
	var out core.WebhookDelivery
	if err := f.store.View(ctx, f.scope, func(tx store.Tx) error {
		d, err := tx.GetDelivery(ctx, id)
		if err != nil {
			return err
		}
		out = *d
		return nil
	}); err != nil {
		t.Fatalf("reading delivery %q: %v", id, err)
	}
	return out
}

func (f fixture) only(t *testing.T) core.WebhookDelivery {
	t.Helper()
	rows := f.deliveries(t)
	if len(rows) != 1 {
		t.Fatalf("deliveries = %d, want 1", len(rows))
	}
	return rows[0]
}

func TestMatchesType(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		typ     string
		want    bool
	}{
		{"exact", "task.created", "task.created", true},
		{"exact mismatch", "task.created", "task.updated", false},
		{"family", "task.*", "task.transitioned", true},
		{"family mismatch", "task.*", "project.created", false},
		{"everything", "*", "project.created", true},
		{"padded", " task.* ", "task.created", true},
		{"prefix without star", "task", "task.created", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := MatchesType(c.pattern, c.typ); got != c.want {
				t.Fatalf("MatchesType(%q, %q) = %v, want %v", c.pattern, c.typ, got, c.want)
			}
		})
	}
}

func TestMatches(t *testing.T) {
	cases := []struct {
		name     string
		endpoint core.WebhookEndpoint
		typ      core.EventType
		want     bool
	}{
		{"inactive", core.WebhookEndpoint{Active: false, EventTypes: []string{"*"}}, core.EventTaskCreated, false},
		{"no filter", core.WebhookEndpoint{Active: true}, core.EventTaskCreated, true},
		{"family", core.WebhookEndpoint{Active: true, EventTypes: []string{"task.*"}}, core.EventTaskClaimed, true},
		{"other family", core.WebhookEndpoint{Active: true, EventTypes: []string{"task.*"}}, core.EventProjectCreated, false},
		{"second pattern", core.WebhookEndpoint{Active: true, EventTypes: []string{"comment.*", "project.*"}}, core.EventProjectCreated, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Matches(c.endpoint, c.typ); got != c.want {
				t.Fatalf("Matches = %v, want %v", got, c.want)
			}
		})
	}
}

func TestValidateEndpoint(t *testing.T) {
	valid := core.WebhookEndpoint{URL: "https://example.test/hook", Secret: "shh", EventTypes: []string{"task.*"}}
	cases := []struct {
		name     string
		endpoint core.WebhookEndpoint
		wantErr  bool
	}{
		{"valid", valid, false},
		{"plain http", core.WebhookEndpoint{URL: "http://example.test/h", Secret: "s"}, false},
		{"relative", core.WebhookEndpoint{URL: "/hook", Secret: "s"}, true},
		{"no host", core.WebhookEndpoint{URL: "https://", Secret: "s"}, true},
		{"wrong scheme", core.WebhookEndpoint{URL: "ftp://example.test/h", Secret: "s"}, true},
		{"unparsable", core.WebhookEndpoint{URL: "://\x7f", Secret: "s"}, true},
		{"no secret", core.WebhookEndpoint{URL: "https://example.test/h", Secret: "  "}, true},
		{"empty filter", core.WebhookEndpoint{URL: "https://example.test/h", Secret: "s", EventTypes: []string{" "}}, true},
		{"inner star", core.WebhookEndpoint{URL: "https://example.test/h", Secret: "s", EventTypes: []string{"task.*.created"}}, true},
		{"two stars", core.WebhookEndpoint{URL: "https://example.test/h", Secret: "s", EventTypes: []string{"*.*"}}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateEndpoint(c.endpoint)
			if (err != nil) != c.wantErr {
				t.Fatalf("ValidateEndpoint = %v, wantErr %v", err, c.wantErr)
			}
			if err != nil && !core.IsKind(err, core.KindInvalid) {
				t.Fatalf("error kind = %v, want invalid", core.KindOf(err))
			}
		})
	}
}

func TestParseMode(t *testing.T) {
	cases := []struct {
		in      string
		want    Mode
		wantErr bool
	}{
		{"inline", ModeInline, false},
		{"SERVER", ModeServer, false},
		{" off ", ModeOff, false},
		{"", DefaultMode, false},
		{"always", "", true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := ParseMode(c.in)
			if (err != nil) != c.wantErr {
				t.Fatalf("ParseMode(%q) error = %v, wantErr %v", c.in, err, c.wantErr)
			}
			if got != c.want {
				t.Fatalf("ParseMode(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
	if !ModeInline.DrainsInline() || ModeServer.DrainsInline() || ModeOff.DrainsInline() {
		t.Fatal("only inline mode drains in the writing process")
	}
	if len(Modes()) != 3 {
		t.Fatalf("modes = %v", Modes())
	}
}

func TestRedactRemovesSecret(t *testing.T) {
	in := []core.WebhookEndpoint{{ID: "a", Secret: "top-secret"}, {ID: "b", Secret: "other"}}
	for _, e := range RedactAll(in) {
		if e.Secret != "" {
			t.Fatalf("endpoint %q still carries its secret", e.ID)
		}
	}
	if in[0].Secret != "top-secret" {
		t.Fatal("redaction must not mutate the caller's endpoint")
	}
}

func TestEnqueueFiltersEndpoints(t *testing.T) {
	f := newFixture(t)
	all := f.addEndpoint(t, "https://a.test/hook", "s1", true, "*")
	tasks := f.addEndpoint(t, "https://b.test/hook", "s2", true, "task.*")
	comments := f.addEndpoint(t, "https://c.test/hook", "s3", true, "comment.added")
	f.addEndpoint(t, "https://d.test/hook", "s4", false, "*")

	_, queued := f.emit(t, core.EventTaskCreated)
	if queued != 2 {
		t.Fatalf("queued = %d, want 2", queued)
	}
	got := map[string]bool{}
	for _, d := range f.deliveries(t) {
		got[d.EndpointID] = true
	}
	if !got[all.ID] || !got[tasks.ID] || got[comments.ID] {
		t.Fatalf("queued endpoints = %v", got)
	}
	for _, d := range f.deliveries(t) {
		if d.Status != core.DeliveryPending {
			t.Fatalf("delivery %q status = %q, want pending", d.ID, d.Status)
		}
	}
}

func TestEnqueueRejectsUnpersistedEvent(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	err := f.store.Update(ctx, f.scope, func(tx store.Tx) error {
		_, err := Enqueue(ctx, tx, core.Event{Type: core.EventTaskCreated})
		return err
	})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("Enqueue with no sequence = %v, want invalid", err)
	}
}
