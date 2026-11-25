// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/lease"
)

func TestClaimWithStartMovesTheTaskInTheSameWrite(t *testing.T) {
	tests := []struct {
		name, status, named, want, from string
	}{
		{"first in-progress state", "todo", "", "doing", "todo"},
		{"a named state", "todo", "stuck", "stuck", "todo"},
		{"already in progress stays put", "doing", "", "doing", ""},
		{"already in the named state stays put", "doing", "doing", "doing", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newClaimFixture(t, 0)
			task := f.seedTask(t, "start me", tt.status, core.PriorityNormal)
			claim, err := f.local.ClaimTask(f.ctx, ref(task), core.ClaimInput{Start: true, StartStatus: tt.named})
			if err != nil {
				t.Fatalf("ClaimTask: %v", err)
			}
			if claim.Task.Status != tt.want || claim.StartedFrom != tt.from {
				t.Errorf("status %q from %q, want %q from %q", claim.Task.Status, claim.StartedFrom, tt.want, tt.from)
			}
			after := f.reload(t, task)
			if after.Status != tt.want || !after.ClaimedAtTime(f.clock.Now()) {
				t.Errorf("stored task = %q claimed %v", after.Status, after.ClaimedAtTime(f.clock.Now()))
			}
			moved := len(f.events(t, core.EventTaskTransitioned))
			if (tt.from != "") != (moved == 1) {
				t.Errorf("transition events = %d", moved)
			}
			if _, err := f.local.RenewLease(f.ctx, ref(task), claim.LeaseToken, 0); err != nil {
				t.Errorf("the claim's token stopped working after the start: %v", err)
			}
		})
	}
}

func TestClaimWithoutStartLeavesTheStatus(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "plain", "todo", core.PriorityNormal)
	claim := mustClaim(t, f, task)
	if claim.Task.Status != "todo" || claim.StartedFrom != "" {
		t.Errorf("a plain claim moved the task: %q from %q", claim.Task.Status, claim.StartedFrom)
	}
}

func TestClaimNextWithStart(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "queued", "todo", core.PriorityNormal)
	claim, err := f.local.ClaimNext(f.ctx, core.ClaimNextInput{Statuses: []string{"todo"}, Start: true})
	if err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}
	if claim.Task.ID != task.ID || claim.Task.Status != "doing" || claim.StartedFrom != "todo" {
		t.Errorf("claim = %s in %q from %q", claim.Task.Ref, claim.Task.Status, claim.StartedFrom)
	}
	actions := auditActions(t, f.local, f.scope)
	if !slices.Contains(actions, "task.claim") || !slices.Contains(actions, "task.transition") {
		t.Errorf("audit = %v, want a claim and a transition", actions)
	}
}

func TestAClaimThatCannotStartClaimsNothing(t *testing.T) {
	tests := []struct {
		name, status, named string
		kind                core.Kind
	}{
		{"unknown state", "todo", "bogus", core.KindInvalid},
		{"no edge to the named state", "todo", "done", core.KindPrecondition},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newClaimFixture(t, 0)
			task := f.seedTask(t, "stuck", tt.status, core.PriorityNormal)
			_, err := f.local.ClaimTask(f.ctx, ref(task), core.ClaimInput{StartStatus: tt.named, Start: true})
			if !core.IsKind(err, tt.kind) {
				t.Fatalf("ClaimTask = %v, want %s", err, tt.kind)
			}
			after := f.reload(t, task)
			if after.ClaimedByActorID != "" || after.Status != tt.status {
				t.Errorf("a refused start left the task %q held by %q", after.Status, after.ClaimedByActorID)
			}
			_, err = f.local.ClaimNext(f.ctx, core.ClaimNextInput{StartStatus: tt.named, Start: true})
			if !core.IsKind(err, tt.kind) {
				t.Errorf("ClaimNext = %v, want %s", err, tt.kind)
			}
			if f.reload(t, task).ClaimedByActorID != "" {
				t.Error("a refused queue start left the task held")
			}
		})
	}
}

func TestStartTargetWithNoInProgressEdge(t *testing.T) {
	def := core.WorkflowDefinition{
		Initial: "new",
		States: []core.State{
			{Key: "new"}, {Key: "triage", Category: core.CategoryTodo},
			{Key: "doing"}, {Key: "done", Terminal: true},
		},
		Transitions: []core.Transition{{From: "new", To: "triage"}, {From: "triage", To: "doing"}},
	}
	_, err := startTarget(def, &core.Task{Ref: "x-1", Status: "new"}, "")
	if !core.IsKind(err, core.KindPrecondition) || !strings.Contains(err.Error(), "triage") {
		t.Errorf("startTarget = %v, want a precondition naming the allowed target", err)
	}
	if to, err := startTarget(def, &core.Task{Status: "triage"}, ""); err != nil || to != "doing" {
		t.Errorf("startTarget from triage = %q, %v", to, err)
	}
}

func TestUnknownStatusListsTheWorkflowStates(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "where", "todo", core.PriorityNormal)
	_, err := f.local.TransitionTask(f.ctx, ref(task), core.TransitionInput{To: "in-progress"})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("TransitionTask = %v, want invalid", err)
	}
	if !strings.Contains(err.Error(), "todo, doing, stuck, review, done, audit") {
		t.Errorf("message does not list the states: %v", err)
	}
	if _, err := f.local.ClaimTask(f.ctx, ref(task), core.ClaimInput{}); err != nil {
		t.Fatal(err)
	}
}

func TestFinishingUnderTheLeaseReleasesIt(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "finish", "doing", core.PriorityNormal)
	claim := mustClaim(t, f, task)

	moved, err := f.local.TransitionTask(f.ctx, ref(task), core.TransitionInput{To: "done", LeaseToken: claim.LeaseToken})
	if err != nil {
		t.Fatalf("TransitionTask: %v", err)
	}
	if moved.ClaimedByActorID != "" || f.reload(t, task).ClaimedAtTime(f.clock.Now()) {
		t.Error("the task is still claimed after moving to done under its token")
	}
	if got := f.events(t, core.EventTaskReleased); len(got) != 1 {
		t.Fatalf("released events = %d, want 1", len(got))
	}
	if !slices.Contains(auditActions(t, f.local, f.scope), "task.release") {
		t.Error("the audit trail does not show the release")
	}

	if err := f.local.ReleaseLease(f.ctx, ref(task), claim.LeaseToken, core.ReleaseInput{}); err != nil {
		t.Errorf("a release after the finishing move = %v, want success", err)
	}
	if got := f.events(t, core.EventTaskReleased); len(got) != 1 {
		t.Errorf("the no-op release wrote: released events = %d", len(got))
	}
	err = f.local.ReleaseLease(f.ctx, ref(task), claim.LeaseToken,
		core.ReleaseInput{Result: map[string]any{"ok": true}})
	if !core.IsKind(err, core.KindLeaseExpired) {
		t.Errorf("a release with a result after the finishing move = %v, want lease expired", err)
	}
	if len(f.artifacts(t, task)) != 0 {
		t.Error("a stale token wrote a result to finished work")
	}
}

func TestANonTerminalMoveKeepsTheLease(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "keep", "doing", core.PriorityNormal)
	claim := mustClaim(t, f, task)
	if _, err := f.local.TransitionTask(f.ctx, ref(task), core.TransitionInput{To: "todo", LeaseToken: claim.LeaseToken}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.local.RenewLease(f.ctx, ref(task), claim.LeaseToken, 0); err != nil {
		t.Errorf("the lease did not survive a non-terminal move: %v", err)
	}
}

func TestFinishingAnotherHoldersTaskIsStillRefused(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "theirs", "doing", core.PriorityNormal)
	if _, err := f.local.ClaimTask(f.asOther(), ref(task), core.ClaimInput{}); err != nil {
		t.Fatal(err)
	}
	_, err := f.local.TransitionTask(f.ctx, ref(task), core.TransitionInput{To: "done"})
	if !core.IsKind(err, core.KindLeaseExpired) {
		t.Fatalf("TransitionTask = %v, want lease expired", err)
	}
	if strings.Contains(err.Error(), "by you") {
		t.Errorf("somebody else's claim was described as the caller's: %v", err)
	}
	after := f.reload(t, task)
	if after.Status != "doing" || after.ClaimedByActorID != f.other.ID {
		t.Errorf("task = %q held by %q", after.Status, after.ClaimedByActorID)
	}
}

func TestTheHoldersTokenlessMoveNamesTheFlag(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "mine", "todo", core.PriorityNormal)
	mustClaim(t, f, task)
	_, err := f.local.TransitionTask(f.ctx, ref(task), core.TransitionInput{To: "doing"})
	if !core.IsKind(err, core.KindLeaseExpired) {
		t.Fatalf("TransitionTask = %v, want lease expired", err)
	}
	for _, want := range []string{"by you", "--lease-token", "lease_token"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message %q lacks %q", err, want)
		}
	}
}

func TestTransitionDryRunJudgesWhatTheMoveJudges(t *testing.T) {
	tests := []struct {
		name, to string
		claimed  bool
		kind     core.Kind
	}{
		{"accepted", "doing", false, ""},
		{"unknown state", "bogus", false, core.KindInvalid},
		{"no edge", "done", false, core.KindPrecondition},
		{"leased without a token", "doing", true, core.KindLeaseExpired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newClaimFixture(t, 0)
			task := f.seedTask(t, "dry", "todo", core.PriorityNormal)
			if tt.claimed {
				if _, err := f.local.ClaimTask(f.asOther(), ref(task), core.ClaimInput{}); err != nil {
					t.Fatal(err)
				}
			}
			before := f.reload(t, task)
			_, beforeAudits := countRows(t, f.local, f.scope)
			dry, dryErr := f.local.TransitionTask(f.ctx, ref(task), core.TransitionInput{To: tt.to, DryRun: true})
			after := f.reload(t, task)
			if after.Status != before.Status || after.Version != before.Version {
				t.Errorf("a dry run changed the task: %q v%d", after.Status, after.Version)
			}
			if _, audits := countRows(t, f.local, f.scope); audits != beforeAudits {
				t.Errorf("a dry run wrote %d audit entries", audits-beforeAudits)
			}
			_, realErr := f.local.TransitionTask(f.ctx, ref(task), core.TransitionInput{To: tt.to})
			if tt.kind == "" {
				if dryErr != nil || realErr != nil {
					t.Fatalf("dry %v, real %v, want both to succeed", dryErr, realErr)
				}
				if dry.Status != tt.to {
					t.Errorf("dry run answered status %q, want %q", dry.Status, tt.to)
				}
				return
			}
			if !core.IsKind(dryErr, tt.kind) || dryErr.Error() != realErr.Error() {
				t.Errorf("dry %v, real %v, want the same %s", dryErr, realErr, tt.kind)
			}
		})
	}
}

func TestRouteDryRunStopsWhereTheRouteWould(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "route", "todo", core.PriorityNormal)
	got, err := f.local.TransitionRoute(f.ctx, ref(task), core.RouteInput{Route: []string{"doing", "bogus"}, DryRun: true})
	if err != nil {
		t.Fatalf("TransitionRoute: %v", err)
	}
	if !got.DryRun || !slices.Equal(got.Applied, []string{"doing"}) || got.Stopped != "bogus" {
		t.Errorf("result = %+v", got)
	}
	if after := f.reload(t, task); after.Status != "todo" {
		t.Errorf("a dry route moved the task to %q", after.Status)
	}
	full, err := f.local.TransitionRoute(f.ctx, ref(task), core.RouteInput{Route: []string{"doing", "done"}, DryRun: true})
	if err != nil || full.Partial() || full.Task.Status != "done" {
		t.Errorf("full dry route = %+v, %v", full, err)
	}
	if _, err := f.local.TransitionRoute(f.ctx, ref(task), core.RouteInput{Route: []string{"bogus"}, DryRun: true}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("a first hop to an unknown state = %v, want invalid", err)
	}
}

func TestReleaseAfterAnExpiredFinishIsStillRefused(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "lapsed", "doing", core.PriorityNormal)
	claim := mustClaim(t, f, task)
	f.clock.Advance(lease.DefaultTTL + time.Minute)
	err := f.local.ReleaseLease(f.ctx, ref(task), claim.LeaseToken, core.ReleaseInput{})
	if !core.IsKind(err, core.KindLeaseExpired) {
		t.Errorf("release of a lapsed lease on unfinished work = %v, want lease expired", err)
	}
}
