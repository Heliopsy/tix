// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// tokenContext mints a token acting as the fixture's agent, bob, and returns
// a context authenticated with it through the real verifier.
func (f *claimFixture) tokenContext(t *testing.T, name string, requireClaim bool) context.Context {
	t.Helper()
	issued, err := f.local.CreateToken(f.ctx, core.CreateTokenInput{Name: name, ActorID: f.other.Handle,
		Scopes: []core.Scope{core.ScopeAll}, RequireClaim: requireClaim})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	if issued.RequireClaim != requireClaim {
		t.Fatalf("issued token require_claim = %v, want %v", issued.RequireClaim, requireClaim)
	}
	actor, err := verify(t, f.local, f.clock, f.scope, issued.Token)
	if err != nil {
		t.Fatalf("verifying the token: %v", err)
	}
	return core.WithActor(context.Background(), actor)
}

// seedSecondProject adds a project on the fixture's workflow, for moves.
func (f *claimFixture) seedSecondProject(t *testing.T) *core.Project {
	t.Helper()
	ctx := context.Background()
	project := core.Project{Key: "ops", Name: "Ops", WorkflowID: f.workflow.ID}
	if err := f.local.store.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.CreateProject(ctx, &project)
	}); err != nil {
		t.Fatalf("seeding a second project: %v", err)
	}
	return &project
}

// gatedChange is one task change the claim requirement judges.
type gatedChange struct {
	name string
	run  func(ctx context.Context, f *claimFixture, task *core.Task) error
}

func gatedChanges() []gatedChange {
	priority := core.PriorityHigh
	title := "renamed"
	return []gatedChange{
		{"transition", func(ctx context.Context, f *claimFixture, task *core.Task) error {
			_, err := f.local.TransitionTask(ctx, ref(task), core.TransitionInput{To: "doing"})
			return err
		}},
		{"transition dry run", func(ctx context.Context, f *claimFixture, task *core.Task) error {
			_, err := f.local.TransitionTask(ctx, ref(task), core.TransitionInput{To: "doing", DryRun: true})
			return err
		}},
		{"route", func(ctx context.Context, f *claimFixture, task *core.Task) error {
			_, err := f.local.TransitionRoute(ctx, ref(task), core.RouteInput{To: "done"})
			return err
		}},
		{"route dry run", func(ctx context.Context, f *claimFixture, task *core.Task) error {
			_, err := f.local.TransitionRoute(ctx, ref(task), core.RouteInput{To: "done", DryRun: true})
			return err
		}},
		{"edit fields", func(ctx context.Context, f *claimFixture, task *core.Task) error {
			_, err := f.local.UpdateTask(ctx, ref(task), core.UpdateTaskInput{Priority: &priority, Title: &title})
			return err
		}},
		{"checklist tick", func(ctx context.Context, f *claimFixture, task *core.Task) error {
			_, err := f.local.CheckTaskItem(ctx, ref(task), core.CheckItemInput{Item: 1, Done: true})
			return err
		}},
		{"add tag", func(ctx context.Context, f *claimFixture, task *core.Task) error {
			return f.local.AddTag(ctx, ref(task), "urgent")
		}},
		{"remove tag", func(ctx context.Context, f *claimFixture, task *core.Task) error {
			return f.local.RemoveTag(ctx, ref(task), "seeded")
		}},
		{"move to another project", func(ctx context.Context, f *claimFixture, task *core.Task) error {
			_, err := f.local.MoveTask(ctx, ref(task), core.MoveTaskInput{ProjectRef: "ops"})
			return err
		}},
		{"delete", func(ctx context.Context, f *claimFixture, task *core.Task) error {
			return f.local.DeleteTask(ctx, ref(task), core.DeleteTaskInput{})
		}},
	}
}

// seedGated seeds a task every gated change can act on: a checklist item to
// tick and a tag to remove.
func (f *claimFixture) seedGated(t *testing.T) *core.Task {
	t.Helper()
	task := f.seedTask(t, "gated", "todo", core.PriorityNormal)
	body := "- [ ] first"
	if _, err := f.local.UpdateTask(f.ctx, ref(task), core.UpdateTaskInput{Body: &body}); err != nil {
		t.Fatalf("seeding the checklist: %v", err)
	}
	if err := f.local.AddTag(f.ctx, ref(task), "seeded"); err != nil {
		t.Fatalf("seeding the tag: %v", err)
	}
	return f.reload(t, task)
}

func TestAClaimRequiringTokenIsRefusedEveryChangeToAnUnclaimedTask(t *testing.T) {
	for _, c := range gatedChanges() {
		t.Run(c.name, func(t *testing.T) {
			f := newClaimFixture(t, 0)
			f.seedSecondProject(t)
			task := f.seedGated(t)
			ctx := f.tokenContext(t, "agent", true)

			err := c.run(ctx, f, task)
			if !core.IsKind(err, core.KindForbidden) {
				t.Fatalf("err = %v, want forbidden", err)
			}
			want := "this token requires a claim on " + task.Ref + " before changing it; run: tix claim task " +
				task.Ref + " --start"
			if !strings.Contains(err.Error(), want) {
				t.Errorf("message = %q, want it to contain %q", err.Error(), want)
			}
			after := f.reload(t, task)
			if after.Version != task.Version || after.Status != task.Status || after.ProjectID != task.ProjectID ||
				after.Deleted() {
				t.Errorf("the refused change was written: %+v", after)
			}
		})
	}
}

func TestPeopleAndPlainTokensChangeUnclaimedTasksAsBefore(t *testing.T) {
	for _, who := range []string{"person", "plain token"} {
		for _, c := range gatedChanges() {
			t.Run(who+"/"+c.name, func(t *testing.T) {
				f := newClaimFixture(t, 0)
				f.seedSecondProject(t)
				task := f.seedGated(t)
				ctx := f.ctx
				if who == "plain token" {
					ctx = f.tokenContext(t, "script", false)
				}
				if err := c.run(ctx, f, task); err != nil {
					t.Fatalf("err = %v, want the change applied", err)
				}
			})
		}
	}
}

func TestAClaimRequiringTokenChangesATaskItsActorHolds(t *testing.T) {
	for _, c := range gatedChanges() {
		if strings.Contains(c.name, "transition") || strings.Contains(c.name, "route") {
			continue
		}
		t.Run(c.name, func(t *testing.T) {
			f := newClaimFixture(t, 0)
			f.seedSecondProject(t)
			task := f.seedGated(t)
			ctx := f.tokenContext(t, "agent", true)
			if _, err := f.local.ClaimTask(ctx, ref(task), core.ClaimInput{}); err != nil {
				t.Fatalf("ClaimTask: %v", err)
			}
			if err := c.run(ctx, f, task); err != nil {
				t.Fatalf("err = %v, want the change applied under the claim", err)
			}
		})
	}
}

func TestAClaimRequiringTokenFinishesItsClaimAndTheLeaseIsReleased(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "work", "todo", core.PriorityNormal)
	ctx := f.tokenContext(t, "agent", true)

	claim, err := f.local.ClaimTask(ctx, ref(task), core.ClaimInput{Start: true})
	if err != nil {
		t.Fatalf("claim with start: %v", err)
	}
	if claim.Task.Status != "doing" || claim.Task.ClaimedByActorID != f.other.ID {
		t.Fatalf("claim --start left %q held by %q", claim.Task.Status, claim.Task.ClaimedByActorID)
	}
	done, err := f.local.TransitionTask(ctx, ref(task), core.TransitionInput{To: "done", LeaseToken: claim.LeaseToken})
	if err != nil {
		t.Fatalf("finishing under the claim: %v", err)
	}
	if done.Status != "done" || done.ClaimedAtTime(f.clock.Now()) {
		t.Errorf("finished task = %q, claimed %v; want done and released", done.Status, done.ClaimedAtTime(f.clock.Now()))
	}
	if n := len(f.events(t, core.EventTaskReleased)); n != 1 {
		t.Errorf("release events = %d, want 1", n)
	}
}

func TestAClaimRequiringTokenClaimsNextAndStartsInOneStep(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "queued", "todo", core.PriorityNormal)
	ctx := f.tokenContext(t, "agent", true)
	claim, err := f.local.ClaimNext(ctx, core.ClaimNextInput{Start: true})
	if err != nil {
		t.Fatalf("ClaimNext with start: %v", err)
	}
	if claim.Task.ID != task.ID || claim.Task.Status != "doing" {
		t.Errorf("claim = %s in %q, want %s in doing", claim.Task.Ref, claim.Task.Status, task.Ref)
	}
}

func TestAnotherActorsClaimOrAnExpiredOneDoesNotCount(t *testing.T) {
	priority := core.PriorityHigh
	t.Run("another actor's claim", func(t *testing.T) {
		f := newClaimFixture(t, 0)
		task := f.seedTask(t, "held", "todo", core.PriorityNormal)
		mustClaim(t, f, task)
		ctx := f.tokenContext(t, "agent", true)
		_, err := f.local.UpdateTask(ctx, ref(task), core.UpdateTaskInput{Priority: &priority})
		if !core.IsKind(err, core.KindForbidden) {
			t.Fatalf("err = %v, want forbidden", err)
		}
	})
	t.Run("expired claim", func(t *testing.T) {
		f := newClaimFixture(t, 0)
		task := f.seedTask(t, "lapsed", "todo", core.PriorityNormal)
		ctx := f.tokenContext(t, "agent", true)
		claim, err := f.local.ClaimTask(ctx, ref(task), core.ClaimInput{})
		if err != nil {
			t.Fatalf("ClaimTask: %v", err)
		}
		f.clock.Advance(claim.LeaseExpiresAt.Sub(f.clock.Now()) + time.Second)
		_, err = f.local.UpdateTask(ctx, ref(task), core.UpdateTaskInput{Priority: &priority})
		if !core.IsKind(err, core.KindForbidden) {
			t.Fatalf("err = %v, want forbidden once the claim lapsed", err)
		}
	})
}

func TestAClaimRequiringTokenStillCreatesCommentsAndReleases(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "open", "todo", core.PriorityNormal)
	ctx := f.tokenContext(t, "agent", true)

	if _, err := f.local.CreateTask(ctx, core.CreateTaskInput{ProjectRef: f.project.Key, Title: "new"}); err != nil {
		t.Errorf("CreateTask: %v", err)
	}
	comment, err := f.local.AddComment(ctx, ref(task), "looking at it")
	if err != nil {
		t.Fatalf("AddComment: %v", err)
	}
	if _, err := f.local.AddReaction(ctx, comment.ID, "👍"); err != nil {
		t.Errorf("AddReaction: %v", err)
	}
	if _, err := f.local.GetTask(ctx, ref(task)); err != nil {
		t.Errorf("GetTask: %v", err)
	}
	claim, err := f.local.ClaimTask(ctx, ref(task), core.ClaimInput{})
	if err != nil {
		t.Fatalf("ClaimTask: %v", err)
	}
	if err := f.local.ReleaseLease(ctx, ref(task), claim.LeaseToken, core.ReleaseInput{}); err != nil {
		t.Errorf("ReleaseLease: %v", err)
	}
}

func TestABulkActionJudgesTheClaimPerTask(t *testing.T) {
	f := newClaimFixture(t, 0)
	held := f.seedTask(t, "held", "todo", core.PriorityNormal)
	free := f.seedTask(t, "free", "todo", core.PriorityNormal)
	ctx := f.tokenContext(t, "agent", true)
	if _, err := f.local.ClaimTask(ctx, ref(held), core.ClaimInput{}); err != nil {
		t.Fatalf("ClaimTask: %v", err)
	}
	res, err := f.local.BulkTasks(ctx, core.BulkTaskInput{Action: core.BulkPriority, Priority: core.PriorityHigh,
		Refs: []string{held.Ref, free.Ref}})
	if err != nil {
		t.Fatalf("BulkTasks: %v", err)
	}
	if res.Applied != 1 || res.Refused != 1 {
		t.Fatalf("applied %d refused %d, want 1 and 1: %+v", res.Applied, res.Refused, res.Outcomes)
	}
	for _, o := range res.Outcomes {
		switch o.Ref {
		case held.Ref:
			if !o.Applied {
				t.Errorf("the held task was refused: %+v", o)
			}
		case free.Ref:
			if o.Applied || o.Code != string(core.KindForbidden) || !strings.Contains(o.Error, "tix claim task "+free.Ref) {
				t.Errorf("the unclaimed task's outcome = %+v, want a forbidden refusal naming the claim", o)
			}
		}
	}
	if got := f.reload(t, free); got.Priority != core.PriorityNormal {
		t.Errorf("the refused task's priority changed to %d", got.Priority)
	}
}

func TestACascadingDeleteNeedsAClaimOnEveryTask(t *testing.T) {
	f := newClaimFixture(t, 0)
	parent := f.seedTask(t, "parent", "todo", core.PriorityNormal)
	child := f.seedTask(t, "child", "todo", core.PriorityNormal)
	parentRef := parent.ID
	if _, err := f.local.UpdateTask(f.ctx, ref(child), core.UpdateTaskInput{ParentRef: &parentRef}); err != nil {
		t.Fatalf("parenting: %v", err)
	}
	ctx := f.tokenContext(t, "agent", true)
	if _, err := f.local.ClaimTask(ctx, ref(parent), core.ClaimInput{}); err != nil {
		t.Fatalf("ClaimTask: %v", err)
	}
	err := f.local.DeleteTask(ctx, ref(parent), core.DeleteTaskInput{Cascade: true})
	if !core.IsKind(err, core.KindForbidden) || !strings.Contains(err.Error(), child.Ref) {
		t.Fatalf("err = %v, want forbidden naming the unclaimed child", err)
	}
	if f.reload(t, parent).Deleted() || f.reload(t, child).Deleted() {
		t.Error("a refused cascade deleted something")
	}
}

func TestRotatingAClaimRequiringTokenKeepsTheRequirement(t *testing.T) {
	for _, want := range []bool{true, false} {
		f := newClaimFixture(t, 0)
		if _, err := f.local.CreateToken(f.ctx, core.CreateTokenInput{Name: "agent",
			Scopes: []core.Scope{core.ScopeTaskRead}, RequireClaim: want}); err != nil {
			t.Fatalf("CreateToken: %v", err)
		}
		issued, err := f.local.RotateToken(f.ctx, "agent", core.RotateTokenInput{})
		if err != nil {
			t.Fatalf("RotateToken: %v", err)
		}
		if issued.RequireClaim != want {
			t.Errorf("rotated require_claim = %v, want %v", issued.RequireClaim, want)
		}
		listed, err := f.local.ListTokens(f.ctx, "")
		if err != nil {
			t.Fatalf("ListTokens: %v", err)
		}
		for _, tok := range listed {
			if tok.RequireClaim != want {
				t.Errorf("listed %s require_claim = %v, want %v", tok.ID, tok.RequireClaim, want)
			}
		}
	}
}
