// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/lease"
	"github.com/heliopsy/tix/internal/store"
)

// snapshotField reads one field out of an audit entry's stored snapshot.
func snapshotField(t *testing.T, raw []byte, field string) string {
	t.Helper()
	if len(raw) == 0 {
		return ""
	}
	var into map[string]any
	if err := json.Unmarshal(raw, &into); err != nil {
		t.Fatalf("decoding the audit snapshot: %v", err)
	}
	s, _ := into[field].(string)
	return s
}

// forceClaim seeds a live lease through the store, so a terminal task can be
// left holding one. ClaimTask refuses a finished task, which is correct and is
// exactly why the stuck-lease-on-a-finished-task case needs seeding.
func (f *claimFixture) forceClaim(t *testing.T, task *core.Task, holder string, ttl time.Duration) error {
	t.Helper()
	token, err := lease.NewToken()
	if err != nil {
		return err
	}
	return f.local.store.Update(context.Background(), f.scope, func(tx store.Tx) error {
		ok, err := tx.ClaimTask(context.Background(), store.ClaimRow{
			TaskID:     task.ID,
			ActorID:    holder,
			Now:        f.clock.Now(),
			Until:      f.clock.Now().Add(ttl),
			LeaseToken: token,
		})
		if err != nil {
			return err
		}
		if !ok {
			t.Fatalf("seeding a lease on %q matched no row", task.Ref)
		}
		return nil
	})
}

// memberActor is an actor holding everything a worker holds and nothing more:
// it can claim, so it can be the holder, and it cannot reclaim, which is the
// separation task:reclaim exists to draw.
func (f *claimFixture) memberActor(t *testing.T) *core.Actor {
	t.Helper()
	a := core.Actor{Kind: core.ActorUser, Handle: "carol", Role: core.RoleMember}
	if err := f.local.store.Update(context.Background(), f.scope, func(tx store.Tx) error {
		return tx.CreateActor(context.Background(), &a)
	}); err != nil {
		t.Fatalf("seeding the member: %v", err)
	}
	a.TenantID = f.scope.TenantID
	return &a
}

// reclaimEntries returns the audit entries a reclaim or an expiry left, in the
// order they were written.
func (f *claimFixture) reclaimEntries(t *testing.T, actions ...string) []core.AuditEntry {
	t.Helper()
	var out []core.AuditEntry
	if err := f.local.store.View(context.Background(), f.scope, func(tx store.Tx) error {
		entries, err := tx.ListAudit(context.Background(), core.AuditFilter{
			Actions: actions, Page: core.Page{Limit: 200},
		})
		out = entries
		return err
	}); err != nil {
		t.Fatalf("listing audit entries: %v", err)
	}
	return out
}

// A worker that can claim must not be able to end another worker's claim, and
// the refusal has to name the scope it wanted: "forbidden" alone leaves an
// operator guessing which of twenty scopes to grant.
func TestForceReclaimIsRefusedWithoutTheReclaimScope(t *testing.T) {
	f := newClaimFixture(t, core.Duration(time.Hour))
	task := f.seedTask(t, "stuck", "doing", core.PriorityNormal)
	if _, err := f.local.ClaimTask(f.ctx, ref(task), core.ClaimInput{}); err != nil {
		t.Fatalf("seeding the claim: %v", err)
	}

	member := core.WithActor(context.Background(), f.memberActor(t))
	_, err := f.local.ForceReclaim(member, ref(task), core.ForceReclaimInput{})
	if !core.IsKind(err, core.KindForbidden) {
		t.Fatalf("ForceReclaim as a member = %v, want a forbidden error", err)
	}
	var detailed *core.Error
	if !errors.As(err, &detailed) {
		t.Fatalf("the refusal is not a core error: %v", err)
	}
	if got := detailed.Details["scope"]; got != string(core.ScopeTaskReclaim) {
		t.Errorf("the refusal names scope %v, want %q", got, core.ScopeTaskReclaim)
	}

	if after := f.reload(t, task); after.ClaimedByActorID != f.actor.ID {
		t.Errorf("the refused reclaim moved the lease: holder is %q, want %q",
			after.ClaimedByActorID, f.actor.ID)
	}
}

// The forced-away holder meets the same wall an expired holder meets. There is
// no second way for a token to die: requireLeaseToken sees an unclaimed task
// and a token offered anyway, and reports lease expiry.
func TestForceReclaimKillsThePreviousHoldersToken(t *testing.T) {
	f := newClaimFixture(t, core.Duration(time.Hour))
	task := f.seedTask(t, "stuck", "doing", core.PriorityNormal)
	claim, err := f.local.ClaimTask(f.ctx, ref(task), core.ClaimInput{})
	if err != nil {
		t.Fatalf("seeding the claim: %v", err)
	}

	if _, err := f.local.ForceReclaim(f.asOther(), ref(task), core.ForceReclaimInput{}); err != nil {
		t.Fatalf("ForceReclaim: %v", err)
	}

	for _, c := range []struct {
		name string
		call func() error
	}{
		{"renew", func() error {
			_, err := f.local.RenewLease(f.ctx, ref(task), claim.LeaseToken, core.Duration(time.Hour))
			return err
		}},
		{"release", func() error {
			return f.local.ReleaseLease(f.ctx, ref(task), claim.LeaseToken, core.ReleaseInput{})
		}},
		{"transition", func() error {
			_, err := f.local.TransitionTask(f.ctx, ref(task), core.TransitionInput{
				To: "done", LeaseToken: claim.LeaseToken,
			})
			return err
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if err := c.call(); !core.IsKind(err, core.KindLeaseExpired) {
				t.Fatalf("%s with the reclaimed token = %v, want a lease-expired error", c.name, err)
			}
		})
	}

	if after := f.reload(t, task); after.Status != "doing" {
		t.Errorf("a refused write moved the task to %q, want %q", after.Status, "doing")
	}
}

// Reclaim only, never reopen. The sweeper reverts a task out of a state
// carrying revert_on_lease_expiry and moves nothing terminal; a reclaim moves
// neither, because reopening is a transition an administrator makes on purpose.
func TestForceReclaimLeavesTheStatusWhereItIs(t *testing.T) {
	for _, c := range []struct {
		name   string
		status string
	}{
		{"a terminal task stays terminal", "done"},
		{"a state that reverts on expiry does not revert on a reclaim", "doing"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newClaimFixture(t, core.Duration(time.Hour))
			task := f.seedTask(t, "stuck", c.status, core.PriorityNormal)
			if err := f.forceClaim(t, task, f.actor.ID, time.Hour); err != nil {
				t.Fatalf("seeding the claim: %v", err)
			}

			reclaimed, err := f.local.ForceReclaim(f.asOther(), ref(task), core.ForceReclaimInput{})
			if err != nil {
				t.Fatalf("ForceReclaim: %v", err)
			}
			if reclaimed.Status != c.status {
				t.Errorf("the reclaim returned status %q, want %q", reclaimed.Status, c.status)
			}
			after := f.reload(t, task)
			if after.Status != c.status {
				t.Errorf("the reclaim moved the task to %q, want %q", after.Status, c.status)
			}
			if after.ClaimedByActorID != "" || after.LeaseExpiresAt != nil {
				t.Errorf("the lease survived the reclaim: holder %q, expiry %v",
					after.ClaimedByActorID, after.LeaseExpiresAt)
			}
			for _, e := range f.reclaimEntries(t, "task.transition") {
				t.Errorf("the reclaim wrote a transition entry: %+v", e.After)
			}
		})
	}
}

// "An admin took this" and "this expired" are different facts about different
// failures. An operator reading the trail has to be able to tell them apart,
// and the row a sweep leaves behind must not be forged by an override.
func TestForceReclaimIsLegibleAsAForceRatherThanAnExpiry(t *testing.T) {
	f := newClaimFixture(t, core.Duration(time.Hour))
	task := f.seedTask(t, "stuck", "doing", core.PriorityNormal)
	if _, err := f.local.ClaimTask(f.ctx, ref(task), core.ClaimInput{}); err != nil {
		t.Fatalf("seeding the claim: %v", err)
	}

	if _, err := f.local.ForceReclaim(f.asOther(), ref(task),
		core.ForceReclaimInput{Reason: "the agent host died"}); err != nil {
		t.Fatalf("ForceReclaim: %v", err)
	}

	if expired := f.reclaimEntries(t, "task.lease_expire"); len(expired) != 0 {
		t.Fatalf("the reclaim wrote %d lease-expiry entries, want none", len(expired))
	}
	entries := f.reclaimEntries(t, "task.reclaim")
	if len(entries) != 1 {
		t.Fatalf("the reclaim wrote %d task.reclaim entries, want exactly one", len(entries))
	}
	entry := entries[0]
	if entry.ActorID != f.other.ID {
		t.Errorf("the entry attributes the reclaim to %q, want %q", entry.ActorID, f.other.ID)
	}
	// Who it was taken from, and that the task did not move, both read off the
	// entry's own snapshots: the row has been rewritten and no longer says.
	if got := snapshotField(t, entry.Before, "claimed_by_actor_id"); got != f.actor.ID {
		t.Errorf("the entry's before names holder %q, want %q", got, f.actor.ID)
	}
	if got := snapshotField(t, entry.After, "claimed_by_actor_id"); got != "" {
		t.Errorf("the entry's after still names holder %q, want none", got)
	}
	for _, snap := range []struct {
		name string
		raw  []byte
	}{{"before", entry.Before}, {"after", entry.After}} {
		if got := snapshotField(t, snap.raw, "status"); got != "doing" {
			t.Errorf("the entry's %s says status %q, want %q", snap.name, got, "doing")
		}
	}

	// The row itself must not claim an expiry. Those two columns are how a
	// repeatedly dying agent is found, and an operator's decision written into
	// them would read as exactly the failure it is not.
	after := f.reload(t, task)
	if after.LeaseExpiredAt != nil || after.LeaseExpiredByActorID != "" {
		t.Errorf("the reclaim left expiry evidence: at %v, by %q",
			after.LeaseExpiredAt, after.LeaseExpiredByActorID)
	}

	events := f.events(t, core.EventTaskReclaimed)
	if len(events) != 1 {
		t.Fatalf("the reclaim published %d task.reclaimed events, want exactly one", len(events))
	}
	for key, want := range map[string]any{
		"forced":                 true,
		"previous_holder":        f.actor.ID,
		"previous_holder_handle": f.actor.Handle,
		"reason":                 "the agent host died",
		"status":                 "doing",
	} {
		if got := events[0].Payload[key]; got != want {
			t.Errorf("the event's %s is %v, want %v", key, got, want)
		}
	}
	if lapses := f.events(t, core.EventTaskLeaseExpired); len(lapses) != 0 {
		t.Errorf("the reclaim published %d lease-expired events, want none", len(lapses))
	}
}

// The compare-and-swap re-asserts the holder the caller read, so a reclaim
// aimed at one holder can never land on another's brand-new lease.
func TestForceReclaimRefusesWhenTheHolderChangedUnderneath(t *testing.T) {
	f := newClaimFixture(t, core.Duration(time.Hour))
	task := f.seedTask(t, "stuck", "doing", core.PriorityNormal)
	if err := f.forceClaim(t, task, f.other.ID, time.Hour); err != nil {
		t.Fatalf("seeding the claim: %v", err)
	}

	// The store call directly, naming a holder the row does not carry: this is
	// the state a second administrator's transaction sees after the first one
	// committed, without needing the two to be interleaved by hand.
	var reclaimed bool
	if err := f.local.store.Update(context.Background(), f.scope, func(tx store.Tx) error {
		ok, err := tx.ForceReclaim(context.Background(), store.ForceReclaimRow{
			TaskID: task.ID, HolderID: f.actor.ID, At: f.clock.Now(),
		})
		reclaimed = ok
		return err
	}); err != nil {
		t.Fatalf("ForceReclaim against the store: %v", err)
	}
	if reclaimed {
		t.Fatal("the reclaim reported success against a holder the row does not carry")
	}
	if after := f.reload(t, task); after.ClaimedByActorID != f.other.ID {
		t.Errorf("the refused reclaim moved the lease: holder is %q, want %q",
			after.ClaimedByActorID, f.other.ID)
	}
}

// A task nobody holds has no lease to force, and saying so beats reporting a
// successful reclaim of nothing.
func TestForceReclaimRefusesATaskWithNoLiveLease(t *testing.T) {
	f := newClaimFixture(t, core.Duration(time.Minute))
	task := f.seedTask(t, "free", "todo", core.PriorityNormal)

	if _, err := f.local.ForceReclaim(f.asOther(), ref(task), core.ForceReclaimInput{}); !core.IsKind(err, core.KindConflict) {
		t.Fatalf("ForceReclaim on an unclaimed task = %v, want a conflict", err)
	}

	if _, err := f.local.ClaimTask(f.ctx, ref(task), core.ClaimInput{}); err != nil {
		t.Fatalf("seeding the claim: %v", err)
	}
	f.clock.Advance(2 * time.Minute)
	if _, err := f.local.ForceReclaim(f.asOther(), ref(task), core.ForceReclaimInput{}); !core.IsKind(err, core.KindConflict) {
		t.Fatalf("ForceReclaim on a lapsed lease = %v, want a conflict", err)
	}
}

// A renewal moves the expiry and leaves the holder alone, so an agent renewing
// on a timer cannot starve an administrator out of the override.
func TestForceReclaimWinsOverAConcurrentRenewal(t *testing.T) {
	f := newClaimFixture(t, core.Duration(time.Hour))
	task := f.seedTask(t, "busy", "doing", core.PriorityNormal)
	claim, err := f.local.ClaimTask(f.ctx, ref(task), core.ClaimInput{})
	if err != nil {
		t.Fatalf("seeding the claim: %v", err)
	}
	if _, err := f.local.RenewLease(f.ctx, ref(task), claim.LeaseToken, core.Duration(4*time.Hour)); err != nil {
		t.Fatalf("renewing: %v", err)
	}

	if _, err := f.local.ForceReclaim(f.asOther(), ref(task), core.ForceReclaimInput{}); err != nil {
		t.Fatalf("ForceReclaim after a renewal: %v", err)
	}
	if after := f.reload(t, task); after.ClaimedByActorID != "" {
		t.Errorf("the renewed lease survived the reclaim: holder is %q", after.ClaimedByActorID)
	}
}
