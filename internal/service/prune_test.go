package service

import (
	"context"
	"testing"
	"time"

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/store"
)

func seedEvents(t *testing.T, l *Local, scope core.TenantScope, n int, at time.Time) []core.Event {
	t.Helper()
	ctx := context.Background()
	out := make([]core.Event, 0, n)
	if err := l.store.Update(ctx, scope, func(tx store.Tx) error {
		for i := 0; i < n; i++ {
			e := core.Event{
				Type:        core.EventTaskCreated,
				SubjectType: "task",
				SubjectID:   "t",
				OccurredAt:  at,
			}
			if err := tx.AppendEvent(ctx, &e); err != nil {
				return err
			}
			out = append(out, e)
		}
		return nil
	}); err != nil {
		t.Fatalf("seeding events: %v", err)
	}
	return out
}

func seedAudit(t *testing.T, l *Local, scope core.TenantScope, n int, at time.Time) {
	t.Helper()
	ctx := context.Background()
	if err := l.store.Update(ctx, scope, func(tx store.Tx) error {
		for i := 0; i < n; i++ {
			e := core.AuditEntry{
				Action:      "task.update",
				SubjectType: "task",
				SubjectID:   "t",
				Source:      core.SourceCLI,
				OccurredAt:  at,
			}
			if err := tx.AppendAudit(ctx, &e); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("seeding audit entries: %v", err)
	}
}

func seedDeliveries(t *testing.T, l *Local, scope core.TenantScope, n int, at time.Time) {
	t.Helper()
	ctx := context.Background()
	if err := l.store.Update(ctx, scope, func(tx store.Tx) error {
		ep := core.WebhookEndpoint{URL: "https://example.test/hook", Active: true, CreatedAt: at}
		if err := tx.PutWebhook(ctx, &ep); err != nil {
			return err
		}
		for i := 0; i < n; i++ {
			d := core.WebhookDelivery{
				EndpointID:    ep.ID,
				EventSeq:      int64(i + 1),
				Status:        core.DeliveryDelivered,
				NextAttemptAt: at,
				CreatedAt:     at,
			}
			if err := tx.EnqueueDelivery(ctx, &d); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("seeding deliveries: %v", err)
	}
}

func auditCount(t *testing.T, l *Local, scope core.TenantScope) int {
	t.Helper()
	_, n := countRows(t, l, scope)
	return n
}

// Events are a transport buffer and audit entries are the compliance record, so
// the default policy must let a prune clear old events while the audit entries
// written at the same instant survive.
func TestPruneRemovesExpiredEventsAndKeepsAudit(t *testing.T) {
	l, clk, scope, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	old := clk.Now()
	seedEvents(t, l, scope, 3, old)
	seedAudit(t, l, scope, 2, old)
	clk.Advance(60 * 24 * time.Hour)

	res, err := l.Prune(ctx, core.PruneInput{})
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if res.Events != 3 {
		t.Errorf("pruned events = %d, want 3", res.Events)
	}
	if res.AuditEntries != 0 {
		t.Errorf("pruned audit entries = %d, want 0 inside the longer window", res.AuditEntries)
	}
	events, audits := countRows(t, l, scope)
	if events != 0 {
		t.Errorf("%d events survived their window", events)
	}
	if audits != 3 {
		t.Errorf("audit rows = %d, want the 2 seeded plus the prune entry", audits)
	}
}

func TestPruneEachClassUsesItsOwnWindow(t *testing.T) {
	l, clk, scope, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	old := clk.Now()
	seedEvents(t, l, scope, 2, old)
	seedAudit(t, l, scope, 2, old)
	seedDeliveries(t, l, scope, 2, old)

	if _, err := l.PutRetention(ctx, core.RetentionPolicy{
		Events:            core.Duration(time.Hour),
		AuditEntries:      core.Duration(365 * 24 * time.Hour),
		WebhookDeliveries: core.Duration(time.Hour),
	}); err != nil {
		t.Fatalf("put retention: %v", err)
	}
	clk.Advance(48 * time.Hour)

	res, err := l.Prune(ctx, core.PruneInput{})
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if res.Events != 2 || res.WebhookDeliveries != 2 {
		t.Errorf("prune removed %d events and %d deliveries, want 2 and 2", res.Events, res.WebhookDeliveries)
	}
	if res.AuditEntries != 0 {
		t.Errorf("pruned %d audit entries under a one-year window", res.AuditEntries)
	}
}

func TestPruneDryRunRemovesNothingAndWritesNoAuditEntry(t *testing.T) {
	l, clk, scope, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	old := clk.Now()
	seedEvents(t, l, scope, 4, old)
	clk.Advance(60 * 24 * time.Hour)
	before := auditCount(t, l, scope)

	dry, err := l.Prune(ctx, core.PruneInput{DryRun: true})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if !dry.DryRun {
		t.Error("dry run result is not marked as one")
	}
	if dry.Events != 4 {
		t.Errorf("dry run predicted %d events, want 4", dry.Events)
	}
	events, audits := countRows(t, l, scope)
	if events != 4 {
		t.Errorf("a dry run removed %d events", 4-events)
	}
	if audits != before {
		t.Errorf("a dry run wrote %d audit entries", audits-before)
	}

	actual, err := l.Prune(ctx, core.PruneInput{})
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if actual.Events != dry.Events {
		t.Errorf("real run removed %d events, dry run predicted %d", actual.Events, dry.Events)
	}
}

func TestPruneWritesOneAuditEntry(t *testing.T) {
	l, clk, scope, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	seedEvents(t, l, scope, 2, clk.Now())
	clk.Advance(60 * 24 * time.Hour)
	before := auditCount(t, l, scope)

	if _, err := l.Prune(ctx, core.PruneInput{}); err != nil {
		t.Fatalf("prune: %v", err)
	}
	if got := auditCount(t, l, scope); got != before+1 {
		t.Fatalf("prune wrote %d audit entries, want exactly 1", got-before)
	}

	entries, _, err := l.ListAudit(ctx, core.AuditFilter{Actions: []string{"retention.prune"}})
	if err != nil {
		t.Fatalf("listing audit: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("found %d pruning audit entries, want 1", len(entries))
	}
	if entries[0].ActorID != actor.ID {
		t.Errorf("prune audit actor = %q, want %q", entries[0].ActorID, actor.ID)
	}
	if len(entries[0].After) == 0 {
		t.Error("prune audit entry records no counts")
	}
}

// A run that removes nothing must not write an audit entry, or an idle
// scheduled pruner would grow the compliance record forever.
func TestPruneWithNothingExpiredWritesNoAuditEntry(t *testing.T) {
	l, clk, scope, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	seedEvents(t, l, scope, 2, clk.Now())
	before := auditCount(t, l, scope)

	res, err := l.Prune(ctx, core.PruneInput{})
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if res.Events != 0 {
		t.Errorf("prune removed %d events inside their window", res.Events)
	}
	if got := auditCount(t, l, scope); got != before {
		t.Errorf("an empty prune wrote %d audit entries", got-before)
	}
}

func TestPruneLimitBoundsARun(t *testing.T) {
	l, clk, scope, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	seedEvents(t, l, scope, 5, clk.Now())
	clk.Advance(60 * 24 * time.Hour)

	res, err := l.Prune(ctx, core.PruneInput{Limit: 2})
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if res.Events != 2 {
		t.Fatalf("a run limited to 2 removed %d events", res.Events)
	}
	if events, _ := countRows(t, l, scope); events != 3 {
		t.Errorf("%d events remain, want 3", events)
	}

	if _, err := l.Prune(ctx, core.PruneInput{Limit: 2}); err != nil {
		t.Fatalf("second prune: %v", err)
	}
	if events, _ := countRows(t, l, scope); events != 1 {
		t.Errorf("%d events remain after a second bounded run, want 1", events)
	}
}

func TestPruneRejectsNegativeLimit(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	if _, err := l.Prune(ctx, core.PruneInput{Limit: -1}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("prune with a negative limit = %v, want invalid", err)
	}
}

// The retention spec forbids removing an event at or below a live subscriber's
// resume cursor, however far past its window the event is.
func TestPruneRetainsEventsALiveSubscriberStillNeeds(t *testing.T) {
	l, clk, scope, actor := newLocal(t)
	base := core.WithActor(context.Background(), actor)

	seeded := seedEvents(t, l, scope, 5, clk.Now())
	clk.Advance(60 * 24 * time.Hour)

	subCtx, cancel := context.WithCancel(base)
	stream, err := l.Subscribe(subCtx, core.EventFilter{SinceSeq: seeded[1].Seq})
	if err != nil {
		cancel()
		t.Fatalf("subscribe: %v", err)
	}

	res, err := l.Prune(base, core.PruneInput{})
	if err != nil {
		cancel()
		t.Fatalf("prune: %v", err)
	}
	if res.Events != 1 {
		t.Errorf("pruned %d events, want only the one below the subscriber cursor", res.Events)
	}
	if res.RetainedForSubscribers != 4 {
		t.Errorf("retained for subscribers = %d, want 4", res.RetainedForSubscribers)
	}
	if events, _ := countRows(t, l, scope); events != 4 {
		t.Errorf("%d events remain, want the 4 the subscriber depends on", events)
	}

	cancel()
	for range stream {
	}

	after, err := l.Prune(base, core.PruneInput{})
	if err != nil {
		t.Fatalf("prune after the subscriber left: %v", err)
	}
	if after.Events != 4 {
		t.Errorf("pruned %d events once no subscriber depended on them, want 4", after.Events)
	}
	if after.RetainedForSubscribers != 0 {
		t.Errorf("retained %d events with no live subscriber", after.RetainedForSubscribers)
	}
}

func TestPruneDryRunReportsSubscriberRetention(t *testing.T) {
	l, clk, scope, actor := newLocal(t)
	base := core.WithActor(context.Background(), actor)

	seeded := seedEvents(t, l, scope, 4, clk.Now())
	clk.Advance(60 * 24 * time.Hour)

	subCtx, cancel := context.WithCancel(base)
	defer cancel()
	if _, err := l.Subscribe(subCtx, core.EventFilter{SinceSeq: seeded[0].Seq}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	res, err := l.Prune(base, core.PruneInput{DryRun: true})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if res.Events != 0 {
		t.Errorf("dry run predicted %d removable events, want 0", res.Events)
	}
	if res.RetainedForSubscribers != 4 {
		t.Errorf("dry run reported %d retained events, want 4", res.RetainedForSubscribers)
	}
	if events, _ := countRows(t, l, scope); events != 4 {
		t.Errorf("a dry run removed %d events", 4-events)
	}
}

func TestPruneRequiresRetentionScope(t *testing.T) {
	l, _, scope, _ := newLocal(t)
	reader := &core.Actor{ID: "r1", TenantID: scope.TenantID, Kind: core.ActorAgent,
		Scopes: []core.Scope{core.ScopeTaskRead, core.ScopeAuditRead}}
	ctx := core.WithActor(context.Background(), reader)

	if _, err := l.Prune(ctx, core.PruneInput{}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("prune without the retention scope = %v, want forbidden", err)
	}
}

// A dry run must predict every class, not just events, and still leave the
// stored data exactly as it found it.
func TestPruneDryRunCountsEveryClass(t *testing.T) {
	l, clk, scope, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	old := clk.Now()
	seedEvents(t, l, scope, 2, old)
	seedAudit(t, l, scope, 3, old)
	seedDeliveries(t, l, scope, 4, old)
	if _, err := l.PutRetention(ctx, core.RetentionPolicy{
		Events:            core.Duration(time.Hour),
		AuditEntries:      core.Duration(time.Hour),
		WebhookDeliveries: core.Duration(time.Hour),
	}); err != nil {
		t.Fatalf("put retention: %v", err)
	}
	clk.Advance(48 * time.Hour)

	dry, err := l.Prune(ctx, core.PruneInput{DryRun: true})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if dry.Events != 2 || dry.AuditEntries != 4 || dry.WebhookDeliveries != 4 {
		t.Fatalf("dry run = %+v, want 2 events, 4 audit entries and 4 deliveries", dry)
	}

	actual, err := l.Prune(ctx, core.PruneInput{})
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if actual.Events != dry.Events || actual.AuditEntries != dry.AuditEntries ||
		actual.WebhookDeliveries != dry.WebhookDeliveries {
		t.Errorf("real run %+v did not match the dry run %+v", actual, dry)
	}
}

// The configured windows must reach the pruner, and a window a tenant stored
// explicitly must still beat them.
func TestPruneAppliesConfiguredRetentionDefaults(t *testing.T) {
	const hour = core.Duration(time.Hour)
	year := core.Duration(365 * 24 * time.Hour)

	tests := []struct {
		name          string
		configured    core.RetentionPolicy
		stored        *core.RetentionPolicy
		advance       time.Duration
		wantEvents    int64
		wantAudit     int64
		wantDelivered int64
	}{
		{
			name:          "configuration prunes what the shipped default would keep",
			configured:    core.RetentionPolicy{Events: hour, AuditEntries: hour, WebhookDeliveries: hour},
			advance:       48 * time.Hour,
			wantEvents:    2,
			wantAudit:     2,
			wantDelivered: 2,
		},
		{
			name:       "shipped default still governs a class configuration leaves alone",
			configured: core.RetentionPolicy{AuditEntries: hour},
			advance:    48 * time.Hour,
			wantEvents: 0,
			wantAudit:  2,
		},
		{
			name:       "a stored policy that moved off the shipped default beats configuration",
			configured: core.RetentionPolicy{Events: hour, AuditEntries: hour},
			stored:     &core.RetentionPolicy{Events: year, AuditEntries: 2 * year},
			advance:    48 * time.Hour,
			wantEvents: 0,
			wantAudit:  0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l, clk, scope, actor := newLocalWith(t, WithRetentionDefaults(tc.configured))
			ctx := core.WithActor(context.Background(), actor)

			old := clk.Now()
			seedEvents(t, l, scope, 2, old)
			seedAudit(t, l, scope, 2, old)
			seedDeliveries(t, l, scope, 2, old)
			if tc.stored != nil {
				if _, err := l.PutRetention(ctx, *tc.stored); err != nil {
					t.Fatalf("put retention: %v", err)
				}
			}
			clk.Advance(tc.advance)

			res, err := l.Prune(ctx, core.PruneInput{})
			if err != nil {
				t.Fatalf("prune: %v", err)
			}
			if res.Events != tc.wantEvents {
				t.Errorf("pruned events = %d, want %d", res.Events, tc.wantEvents)
			}
			if res.AuditEntries != tc.wantAudit {
				t.Errorf("pruned audit entries = %d, want %d", res.AuditEntries, tc.wantAudit)
			}
			if res.WebhookDeliveries != tc.wantDelivered {
				t.Errorf("pruned deliveries = %d, want %d", res.WebhookDeliveries, tc.wantDelivered)
			}
		})
	}
}
