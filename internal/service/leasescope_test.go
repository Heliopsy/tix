// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

func leaseOf(d time.Duration) *core.Duration {
	v := core.Duration(d)
	return &v
}

func (f *claimFixture) setProjectLease(t *testing.T, def, max *core.Duration) error {
	t.Helper()
	_, err := f.local.UpdateProject(f.ctx, f.project.Key, core.UpdateProjectInput{LeaseDefault: def, LeaseMax: max})
	return err
}

func (f *claimFixture) setTaskLease(t *testing.T, task *core.Task, d time.Duration) error {
	t.Helper()
	_, err := f.local.UpdateTask(f.ctx, core.TaskRef{ID: task.ID}, core.UpdateTaskInput{Lease: leaseOf(d)})
	return err
}

// TestLeasePrecedenceAtEveryLevel walks the order one level at a time: each
// case sets every level below the one it names, so the named level only wins
// if it really outranks all of them.
func TestLeasePrecedenceAtEveryLevel(t *testing.T) {
	cases := []struct {
		name     string
		workflow time.Duration
		tenant   time.Duration
		project  time.Duration
		task     time.Duration
		claim    time.Duration
		want     time.Duration
		source   core.LeaseSource
	}{
		{name: "builtin", want: 30 * time.Minute, source: core.LeaseFromBuiltin},
		{name: "tenant", tenant: 7 * time.Minute, want: 7 * time.Minute, source: core.LeaseFromTenant},
		{name: "workflow", tenant: 7 * time.Minute, workflow: time.Hour, want: time.Hour, source: core.LeaseFromWorkflow},
		{name: "project", tenant: 7 * time.Minute, workflow: time.Hour, project: 2 * time.Hour,
			want: 2 * time.Hour, source: core.LeaseFromProject},
		{name: "task", tenant: 7 * time.Minute, workflow: time.Hour, project: 2 * time.Hour, task: 4 * time.Hour,
			want: 4 * time.Hour, source: core.LeaseFromTask},
		{name: "claim", tenant: 7 * time.Minute, workflow: time.Hour, project: 2 * time.Hour, task: 4 * time.Hour,
			claim: 3 * time.Minute, want: 3 * time.Minute, source: core.LeaseFromClaim},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newClaimFixture(t, core.Duration(tc.workflow))
			if tc.tenant != 0 {
				f.setTenantLease(t, core.Duration(tc.tenant), 0)
			}
			if tc.project != 0 {
				if err := f.setProjectLease(t, leaseOf(tc.project), nil); err != nil {
					t.Fatalf("setting the project lease: %v", err)
				}
			}
			task := f.seedTask(t, "a", "todo", core.PriorityNormal)
			if tc.task != 0 {
				if err := f.setTaskLease(t, task, tc.task); err != nil {
					t.Fatalf("setting the task lease: %v", err)
				}
			}

			if tc.claim == 0 {
				grant, err := f.local.TaskLease(f.ctx, core.TaskRef{ID: task.ID})
				if err != nil {
					t.Fatalf("reading the lease: %v", err)
				}
				if grant.TTL.D() != tc.want || grant.Source != tc.source {
					t.Fatalf("TaskLease = %s from %s, want %s from %s", grant.TTL, grant.Source, tc.want, tc.source)
				}
			}

			claim, err := f.local.ClaimTask(f.ctx, core.TaskRef{ID: task.ID}, core.ClaimInput{TTL: core.Duration(tc.claim)})
			if err != nil {
				t.Fatalf("claiming: %v", err)
			}
			if got := claim.LeaseExpiresAt.Sub(f.clock.Now()); got != tc.want {
				t.Fatalf("claim held for %s, want %s", got, tc.want)
			}
			if claim.Lease == nil || claim.Lease.Source != tc.source {
				t.Fatalf("claim lease = %+v, want source %s", claim.Lease, tc.source)
			}

			renewed, err := f.local.RenewLease(f.ctx, core.TaskRef{ID: task.ID}, claim.LeaseToken, core.Duration(tc.claim))
			if err != nil {
				t.Fatalf("renewing: %v", err)
			}
			if got := renewed.LeaseExpiresAt.Sub(f.clock.Now()); got != tc.want {
				t.Fatalf("renewal held for %s, want %s", got, tc.want)
			}
		})
	}
}

// TestClaimNextSettlesTheTaskAndProjectLease covers the queue path, which
// only learns the task after the compare-and-swap.
func TestClaimNextSettlesTheTaskAndProjectLease(t *testing.T) {
	f := newClaimFixture(t, 0)
	if err := f.setProjectLease(t, leaseOf(2*time.Hour), nil); err != nil {
		t.Fatal(err)
	}
	if got := f.claimHeldFor(t, "project", 0); got != 2*time.Hour {
		t.Fatalf("queue claim held for %s, want the project's 2h", got)
	}

	task := f.seedTask(t, "task", "todo", core.PriorityHigh)
	if err := f.setTaskLease(t, task, 5*time.Hour); err != nil {
		t.Fatal(err)
	}
	claim, err := f.local.ClaimNext(f.ctx, core.ClaimNextInput{})
	if err != nil {
		t.Fatal(err)
	}
	if got := claim.LeaseExpiresAt.Sub(f.clock.Now()); got != 5*time.Hour || claim.Lease.Source != core.LeaseFromTask {
		t.Fatalf("queue claim = %s from %s, want 5h from the task", got, claim.Lease.Source)
	}
}

// TestTheTenantMaximumClampsTheTaskAndProject proves the tenant ceiling still
// governs the new levels when it is lowered after they were set.
func TestTheTenantMaximumClampsTheTaskAndProject(t *testing.T) {
	f := newClaimFixture(t, 0)
	if err := f.setProjectLease(t, leaseOf(3*time.Hour), nil); err != nil {
		t.Fatal(err)
	}
	task := f.seedTask(t, "a", "todo", core.PriorityNormal)
	if err := f.setTaskLease(t, task, 5*time.Hour); err != nil {
		t.Fatal(err)
	}
	f.setTenantLease(t, 0, core.Duration(time.Hour))

	grant, err := f.local.TaskLease(f.ctx, core.TaskRef{ID: task.ID})
	if err != nil {
		t.Fatal(err)
	}
	if grant.TTL.D() != time.Hour || !grant.Clamped || grant.MaxSource != core.LeaseFromTenant {
		t.Fatalf("lease = %+v, want 1h clamped by the tenant", grant)
	}
	if _, err := f.local.ClaimTask(f.ctx, core.TaskRef{ID: task.ID}, core.ClaimInput{TTL: core.Duration(2 * time.Hour)}); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("claim asking above the ceiling = %v, want invalid", err)
	}
}

// TestAProjectMaximumOnlyLowersTheCeiling covers the project's own maximum:
// it clamps below the tenant's, and one above the tenant's is refused.
func TestAProjectMaximumOnlyLowersTheCeiling(t *testing.T) {
	f := newClaimFixture(t, core.Duration(4*time.Hour))
	f.setTenantLease(t, 0, core.Duration(6*time.Hour))

	if err := f.setProjectLease(t, nil, leaseOf(8*time.Hour)); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("project maximum above the tenant's = %v, want invalid", err)
	}
	if err := f.setProjectLease(t, nil, leaseOf(90*time.Minute)); err != nil {
		t.Fatalf("lowering the ceiling: %v", err)
	}
	task := f.seedTask(t, "a", "todo", core.PriorityNormal)
	grant, err := f.local.TaskLease(f.ctx, core.TaskRef{ID: task.ID})
	if err != nil {
		t.Fatal(err)
	}
	if grant.TTL.D() != 90*time.Minute || grant.MaxSource != core.LeaseFromProject || !grant.Clamped {
		t.Fatalf("lease = %+v, want the workflow's 4h clamped to the project's 90m", grant)
	}
	if err := f.setTaskLease(t, task, 2*time.Hour); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("task lease above the project maximum = %v, want invalid", err)
	}
	if err := f.setProjectLease(t, leaseOf(2*time.Hour), nil); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("project default above its maximum = %v, want invalid", err)
	}
	if err := f.setProjectLease(t, nil, leaseOf(0)); err != nil {
		t.Fatalf("inheriting the maximum again: %v", err)
	}
}

// TestLeaseSettingsAreAuditedWithTheirChange checks both writes land in the
// audit trail carrying the new figure, written by the update's own record.
func TestLeaseSettingsAreAuditedWithTheirChange(t *testing.T) {
	f := newClaimFixture(t, 0)
	if err := f.setProjectLease(t, leaseOf(2*time.Hour), nil); err != nil {
		t.Fatal(err)
	}
	task := f.seedTask(t, "a", "todo", core.PriorityNormal)
	if err := f.setTaskLease(t, task, 4*time.Hour); err != nil {
		t.Fatal(err)
	}
	entries, _, err := f.local.ListAudit(f.ctx, core.AuditFilter{})
	if err != nil {
		t.Fatal(err)
	}
	var project, taskEntry bool
	for _, e := range entries {
		after := string(e.After)
		switch e.Action {
		case auditProjectUpdate:
			project = project || strings.Contains(after, `"lease_default":"2h0m0s"`)
		case "task.update":
			taskEntry = taskEntry || strings.Contains(after, `"lease":"4h0m0s"`)
		}
	}
	if !project || !taskEntry {
		t.Fatalf("audit carried the project lease %v and the task lease %v, want both", project, taskEntry)
	}
}

// TestACopyCarriesTheTaskLease is the decision that the lease belongs to the
// work and travels with it.
func TestACopyCarriesTheTaskLease(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "a", "todo", core.PriorityNormal)
	if err := f.setTaskLease(t, task, 4*time.Hour); err != nil {
		t.Fatal(err)
	}
	copied, err := f.local.CopyTask(f.ctx, core.TaskRef{ID: task.ID}, core.CopyTaskInput{})
	if err != nil {
		t.Fatal(err)
	}
	if copied.Lease.D() != 4*time.Hour {
		t.Fatalf("copy lease = %s, want 4h", copied.Lease)
	}
}

func TestCreateTaskTakesALeaseHeldToTheMaximum(t *testing.T) {
	f := newClaimFixture(t, 0)
	task, err := f.local.CreateTask(f.ctx, core.CreateTaskInput{ProjectRef: f.project.Key, Title: "build",
		Lease: core.Duration(2 * time.Hour)})
	if err != nil {
		t.Fatalf("creating with a lease: %v", err)
	}
	claim, err := f.local.ClaimTask(f.ctx, core.TaskRef{ID: task.ID}, core.ClaimInput{})
	if err != nil {
		t.Fatal(err)
	}
	if got := claim.LeaseExpiresAt.Sub(f.clock.Now()); got != 2*time.Hour || claim.Lease.Source != core.LeaseFromTask {
		t.Fatalf("claim = %s from %s, want 2h from the task", got, claim.Lease.Source)
	}

	if err := f.setProjectLease(t, nil, leaseOf(time.Hour)); err != nil {
		t.Fatal(err)
	}
	_, err = f.local.CreateTask(f.ctx, core.CreateTaskInput{ProjectRef: f.project.Key, Title: "too long",
		Lease: core.Duration(2 * time.Hour)})
	if core.KindOf(err) != core.KindInvalid || core.FieldOf(err) != "lease" {
		t.Fatalf("a lease above the project maximum = %v, want invalid on lease", err)
	}
	page, err := f.local.ListTasks(f.ctx, core.TaskFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Tasks) != 1 {
		t.Fatalf("after the refused create the tenant holds %d tasks, want only the first", len(page.Tasks))
	}
}
