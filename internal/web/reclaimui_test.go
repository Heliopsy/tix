// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// reclaimAction is the form action the reclaim control posts to, for a page
// that carries a screenful of other forms.
func reclaimAction(ref string) string { return "/tasks/" + ref + "/reclaim" }

// claimed puts a live lease on a task, dated against the wall clock rather
// than the fixture's fake one. The badge that decides whether the control is
// drawn compares the expiry to time.Now(), so a lease written at the fake
// clock's instant reads as long lapsed and the screen would be right to hide
// the control.
func (f *fixture) claimed(t *testing.T, ref string) {
	t.Helper()
	task, err := f.svc.GetTask(f.ctx(), core.TaskRef{ProjectKey: "infra", Seq: seqOf(t, ref)})
	if err != nil {
		t.Fatalf("reading %s: %v", ref, err)
	}
	now := time.Now().UTC()
	if err := f.store.Update(context.Background(), core.TenantScope{TenantID: f.tenantA.ID},
		func(tx store.Tx) error {
			ok, err := tx.ClaimTask(context.Background(), store.ClaimRow{
				TaskID: task.ID, ActorID: f.actorA.ID, Now: now,
				Until: now.Add(time.Hour), LeaseToken: "held-by-somebody",
			})
			if err != nil {
				return err
			}
			if !ok {
				t.Fatalf("seeding a lease on %s matched no row", ref)
			}
			return nil
		}); err != nil {
		t.Fatalf("seeding a lease on %s: %v", ref, err)
	}
}

// The control is an administrative override over a live lease. It has nothing
// to act on when the task is free, so it is not drawn then, and a reader
// without the scope never sees it at all.
func TestReclaimControlAppearsOnlyOverALiveLeaseAndOnlyForAReaderWhoMayForceOne(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	admin := f.as("alice")
	ref := admin.createTask("infra", "stuck behind a lease")

	if page := admin.page("/tasks/" + ref); strings.Contains(page, reclaimAction(ref)) {
		t.Error("the reclaim control is offered over a task nobody holds")
	}

	f.claimed(t, ref)

	page := admin.page("/tasks/" + ref)
	if !strings.Contains(page, reclaimAction(ref)) {
		t.Fatal("the reclaim control is missing beside a live lease")
	}
	if form := formAt(t, page, reclaimAction(ref)); !strings.Contains(form, `name="reason"`) {
		t.Errorf("the reclaim form asks for no reason: %s", form)
	}

	// A viewer holds task:read and not task:reclaim. The affordance must be
	// absent, and the route must refuse them anyway: a hidden control is not a
	// check, and a form can be posted without ever being rendered.
	viewer := f.as("viewer")
	if page := viewer.page("/tasks/" + ref); strings.Contains(page, reclaimAction(ref)) {
		t.Error("a reader without task:reclaim is offered the control")
	}
	resp := viewer.post(reclaimAction(ref), url.Values{"reason": {"trying it on"}})
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusSeeOther {
		t.Error("a reader without task:reclaim was allowed to post the reclaim")
	}
}

// The screen's reclaim ends the lease and leaves the task where it is, which is
// the whole of what the operation does.
func TestReclaimFromTheTaskScreenFreesTheLeaseAndKeepsTheStatus(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	admin := f.as("alice")
	ref := admin.createTask("infra", "held by a dead worker")
	f.claimed(t, ref)

	before, err := f.svc.GetTask(f.ctx(), core.TaskRef{ProjectKey: "infra", Seq: seqOf(t, ref)})
	if err != nil {
		t.Fatalf("reading the claimed task: %v", err)
	}

	resp := admin.post(reclaimAction(ref), url.Values{"reason": {"the agent host died"}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)

	after, err := f.svc.GetTask(f.ctx(), core.TaskRef{ProjectKey: "infra", Seq: seqOf(t, ref)})
	if err != nil {
		t.Fatalf("reading the reclaimed task: %v", err)
	}
	if after.ClaimedByActorID != "" || after.LeaseExpiresAt != nil {
		t.Errorf("the lease survived: holder %q, expiry %v", after.ClaimedByActorID, after.LeaseExpiresAt)
	}
	if after.Status != before.Status {
		t.Errorf("the reclaim moved the task from %q to %q", before.Status, after.Status)
	}
	if after.LeaseExpiredAt != nil {
		t.Errorf("the reclaim recorded an expiry at %v", after.LeaseExpiredAt)
	}
	if page := admin.page("/tasks/" + ref); strings.Contains(page, reclaimAction(ref)) {
		t.Error("the control is still offered over a task that is free again")
	}
}
