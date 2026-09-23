// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/lease"
	"github.com/heliopsy/tix/internal/store"
)

// claimFixture is a seeded queue: one workflow, one project and the actors
// racing over it. It is seeded through the store so it does not wait on the
// task and project service methods landing.
type claimFixture struct {
	local    *Local
	clock    *clock.Fake
	scope    core.TenantScope
	ctx      context.Context
	actor    *core.Actor
	other    *core.Actor
	project  *core.Project
	workflow *core.Workflow
}

func claimWorkflowDefinition(defaultLease core.Duration) core.WorkflowDefinition {
	return core.WorkflowDefinition{
		Initial: "todo",
		States: []core.State{
			{Key: "todo"},
			{Key: "doing", RevertOnLeaseExpiry: true, RevertTo: "todo"},
			{Key: "stuck", RevertOnLeaseExpiry: true},
			{Key: "review"},
			{Key: "done", Terminal: true},
			{Key: "audit", Terminal: true},
		},
		Transitions: []core.Transition{
			{From: "todo", To: "doing"},
			{From: "doing", To: "todo"},
			{From: "doing", To: "done"},
			{From: "doing", To: "review", RequiresComment: true},
			{From: "todo", To: "stuck"},
			{From: "doing", To: "audit", RequiresScope: core.ScopeTenantAdmin},
		},
		DefaultLease: defaultLease,
	}
}

func newClaimFixture(t *testing.T, defaultLease core.Duration) *claimFixture {
	t.Helper()
	l, clk, scope, actor := newLocal(t)
	ctx := context.Background()

	f := &claimFixture{local: l, clock: clk, scope: scope, actor: actor}
	f.ctx = core.WithActor(ctx, actor)

	wf := core.Workflow{Key: "agents", Name: "Agents", Definition: claimWorkflowDefinition(defaultLease)}
	other := core.Actor{Kind: core.ActorAgent, Handle: "bob", Scopes: []core.Scope{core.ScopeAll}}
	project := core.Project{Key: "infra", Name: "Infra"}

	if err := l.store.Update(ctx, scope, func(tx store.Tx) error {
		if err := tx.PutWorkflow(ctx, &wf); err != nil {
			return err
		}
		if err := tx.CreateActor(ctx, &other); err != nil {
			return err
		}
		project.WorkflowID = wf.ID
		return tx.CreateProject(ctx, &project)
	}); err != nil {
		t.Fatalf("seeding the queue: %v", err)
	}
	other.TenantID = scope.TenantID

	f.workflow, f.other, f.project = &wf, &other, &project
	return f
}

func (f *claimFixture) seedTask(t *testing.T, title, status string, priority core.Priority) *core.Task {
	t.Helper()
	ctx := context.Background()
	task := core.Task{
		ProjectID:      f.project.ID,
		Title:          title,
		Status:         status,
		Priority:       priority,
		CreatorActorID: f.actor.ID,
	}
	if err := f.local.store.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.CreateTask(ctx, &task)
	}); err != nil {
		t.Fatalf("seeding task %q: %v", title, err)
	}
	return &task
}

func (f *claimFixture) dependOn(t *testing.T, task, dependsOn *core.Task) {
	t.Helper()
	ctx := context.Background()
	if err := f.local.store.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.AddDependency(ctx, &core.Dependency{TaskID: task.ID, DependsOn: dependsOn.ID})
	}); err != nil {
		t.Fatalf("seeding dependency: %v", err)
	}
}

func (f *claimFixture) reload(t *testing.T, task *core.Task) *core.Task {
	t.Helper()
	ctx := context.Background()
	var out *core.Task
	if err := f.local.store.View(ctx, f.scope, func(tx store.Tx) error {
		loaded, err := tx.GetTask(ctx, core.TaskRef{ID: task.ID})
		out = loaded
		return err
	}); err != nil {
		t.Fatalf("reloading task: %v", err)
	}
	return out
}

func (f *claimFixture) artifacts(t *testing.T, task *core.Task) []core.Artifact {
	t.Helper()
	ctx := context.Background()
	var out []core.Artifact
	if err := f.local.store.View(ctx, f.scope, func(tx store.Tx) error {
		loaded, err := tx.ListArtifacts(ctx, task.ID)
		out = loaded
		return err
	}); err != nil {
		t.Fatalf("listing artifacts: %v", err)
	}
	return out
}

func (f *claimFixture) events(t *testing.T, typ core.EventType) []core.Event {
	t.Helper()
	ctx := context.Background()
	var out []core.Event
	if err := f.local.store.View(ctx, f.scope, func(tx store.Tx) error {
		all, err := tx.ReadEvents(ctx, 0, 1000)
		if err != nil {
			return err
		}
		for _, e := range all {
			if e.Type == typ {
				out = append(out, e)
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("reading events: %v", err)
	}
	return out
}

func (f *claimFixture) asOther() context.Context {
	return core.WithActor(context.Background(), f.other)
}

func ref(task *core.Task) core.TaskRef { return core.TaskRef{ID: task.ID} }

func TestClaimSucceedsOnAnUnclaimedTask(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "build the thing", "todo", core.PriorityNormal)

	claimed, err := f.local.ClaimTask(f.ctx, ref(task), core.ClaimInput{})
	if err != nil {
		t.Fatalf("ClaimTask: %v", err)
	}
	if claimed.LeaseToken == "" {
		t.Error("a successful claim minted no lease token")
	}
	want := f.clock.Now().Add(lease.DefaultTTL)
	if !claimed.LeaseExpiresAt.Equal(want) {
		t.Errorf("lease expiry = %s, want the global default %s", claimed.LeaseExpiresAt, want)
	}
	if claimed.Task.ClaimedByActorID != f.actor.ID {
		t.Errorf("holder = %q, want %q", claimed.Task.ClaimedByActorID, f.actor.ID)
	}
	if !claimed.Task.ClaimedAtTime(f.clock.Now()) {
		t.Error("the claimed task does not read as claimed")
	}
	if got := f.events(t, core.EventTaskClaimed); len(got) != 1 {
		t.Fatalf("task.claimed events = %d, want 1", len(got))
	}
	if _, audits := countRows(t, f.local, f.scope); audits != 1 {
		t.Errorf("audit entries = %d, want 1", audits)
	}
}

// The event stream reaches webhook endpoints, so a claim must not carry the
// token that authorizes writing to the task.
func TestClaimEventDoesNotCarryTheToken(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "quiet", "todo", core.PriorityNormal)

	claimed, err := f.local.ClaimTask(f.ctx, ref(task), core.ClaimInput{})
	if err != nil {
		t.Fatalf("ClaimTask: %v", err)
	}
	for _, e := range f.events(t, core.EventTaskClaimed) {
		for k, v := range e.Payload {
			if s, ok := v.(string); ok && s == claimed.LeaseToken {
				t.Fatalf("event payload field %q leaked the lease token", k)
			}
		}
	}
}

func TestClaimConflictsWithALiveLease(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "contended", "todo", core.PriorityNormal)

	first, err := f.local.ClaimTask(f.ctx, ref(task), core.ClaimInput{})
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}

	f.clock.Advance(time.Minute)
	if _, err := f.local.ClaimTask(f.asOther(), ref(task), core.ClaimInput{}); !core.IsKind(err, core.KindConflict) {
		t.Fatalf("second claim = %v, want conflict", err)
	}

	after := f.reload(t, task)
	if after.ClaimedByActorID != f.actor.ID {
		t.Errorf("holder = %q, want the original holder %q", after.ClaimedByActorID, f.actor.ID)
	}
	if !after.LeaseExpiresAt.Equal(first.LeaseExpiresAt) {
		t.Errorf("lease expiry moved to %s, want %s", after.LeaseExpiresAt, first.LeaseExpiresAt)
	}
}

func TestReclaimByTheSameHolderIsAConflict(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "mine", "todo", core.PriorityNormal)

	if _, err := f.local.ClaimTask(f.ctx, ref(task), core.ClaimInput{}); err != nil {
		t.Fatalf("first claim: %v", err)
	}
	err := func() error {
		_, err := f.local.ClaimTask(f.ctx, ref(task), core.ClaimInput{})
		return err
	}()
	if !core.IsKind(err, core.KindConflict) {
		t.Fatalf("re-claim = %v, want conflict", err)
	}
	var domain *core.Error
	if !errors.As(err, &domain) || domain.Details["claimed_by"] != f.actor.ID {
		t.Errorf("conflict details = %+v, want the current holder", domain)
	}
}

func TestClaimSucceedsOnceTheLeaseExpires(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "abandoned", "todo", core.PriorityNormal)

	first, err := f.local.ClaimTask(f.ctx, ref(task), core.ClaimInput{TTL: core.Duration(time.Minute)})
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}

	f.clock.Advance(90 * time.Second)
	second, err := f.local.ClaimTask(f.asOther(), ref(task), core.ClaimInput{})
	if err != nil {
		t.Fatalf("claim after expiry: %v", err)
	}
	if second.LeaseToken == first.LeaseToken {
		t.Error("re-claiming reused the previous lease token")
	}
	if second.Task.ClaimedByActorID != f.other.ID {
		t.Errorf("holder = %q, want the new worker %q", second.Task.ClaimedByActorID, f.other.ID)
	}
	if !second.LeaseExpiresAt.After(f.clock.Now()) {
		t.Errorf("new lease expiry %s is not in the future", second.LeaseExpiresAt)
	}
}

// Nothing in the correctness of claiming may depend on the sweeper, because a
// command-line-only installation never runs one.
func TestExpiryIsAuthoritativeWithNoSweeper(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "lazy", "todo", core.PriorityNormal)

	if _, err := f.local.ClaimTask(f.ctx, ref(task), core.ClaimInput{TTL: core.Duration(time.Minute)}); err != nil {
		t.Fatalf("claim: %v", err)
	}
	f.clock.Advance(2 * time.Minute)

	if f.reload(t, task).ClaimedAtTime(f.clock.Now()) {
		t.Error("a task whose lease passed still reads as claimed")
	}
	next, err := f.local.ClaimNext(f.asOther(), core.ClaimNextInput{})
	if err != nil {
		t.Fatalf("claim next over an expired lease: %v", err)
	}
	if next.Task.ID != task.ID {
		t.Errorf("claim next returned %q, want the expired task", next.Task.Ref)
	}
}

func TestClaimTTLResolution(t *testing.T) {
	cases := []struct {
		name         string
		defaultLease core.Duration
		perClaim     core.Duration
		want         time.Duration
		wantKind     core.Kind
	}{
		{name: "global default", want: lease.DefaultTTL},
		{name: "workflow override", defaultLease: core.Duration(time.Hour), want: time.Hour},
		{
			name:         "per claim override",
			defaultLease: core.Duration(time.Hour),
			perClaim:     core.Duration(2 * time.Minute),
			want:         2 * time.Minute,
		},
		{name: "negative rejected", perClaim: core.Duration(-time.Minute), wantKind: core.KindInvalid},
		{
			name:     "excessive rejected",
			perClaim: core.Duration(lease.MaxTTL + time.Hour),
			wantKind: core.KindInvalid,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newClaimFixture(t, tc.defaultLease)
			task := f.seedTask(t, "ttl", "todo", core.PriorityNormal)

			claimed, err := f.local.ClaimTask(f.ctx, ref(task), core.ClaimInput{TTL: tc.perClaim})
			if tc.wantKind != "" {
				if !core.IsKind(err, tc.wantKind) {
					t.Fatalf("claim = %v, want %s", err, tc.wantKind)
				}
				if f.reload(t, task).ClaimedByActorID != "" {
					t.Error("a rejected claim took a lease anyway")
				}
				return
			}
			if err != nil {
				t.Fatalf("claim: %v", err)
			}
			if got := claimed.LeaseExpiresAt.Sub(f.clock.Now()); got != tc.want {
				t.Errorf("lease ran for %s, want %s", got, tc.want)
			}
		})
	}
}

func TestClaimOnBehalfOfAnotherActor(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "delegated", "todo", core.PriorityNormal)

	claimed, err := f.local.ClaimTask(f.ctx, ref(task), core.ClaimInput{ActorID: f.other.ID})
	if err != nil {
		t.Fatalf("claim on behalf of: %v", err)
	}
	if claimed.Task.ClaimedByActorID != f.other.ID {
		t.Errorf("holder = %q, want %q", claimed.Task.ClaimedByActorID, f.other.ID)
	}
	if _, err := f.local.ClaimTask(f.ctx, ref(f.seedTask(t, "ghost", "todo", core.PriorityNormal)),
		core.ClaimInput{ActorID: "nobodyatallhere"}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("claim for an unknown actor = %v, want not found", err)
	}
}

// --actor on claim next used to accept only an identifier, even though a
// handle is what a human types at a shell.
func TestClaimOnBehalfOfAnotherActorAcceptsHandleOrID(t *testing.T) {
	cases := []struct {
		name string
		ref  string
	}{
		{"by id", "id"},
		{"by handle", "bob"},
		{"by handle, mixed case", "BOB"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newClaimFixture(t, 0)
			onBehalf := tc.ref
			if onBehalf == "id" {
				onBehalf = f.other.ID
			}
			task := f.seedTask(t, "delegated-"+tc.name, "todo", core.PriorityNormal)
			claimed, err := f.local.ClaimTask(f.ctx, ref(task), core.ClaimInput{ActorID: onBehalf})
			if err != nil {
				t.Fatalf("claim on behalf of %q: %v", onBehalf, err)
			}
			if claimed.Task.ClaimedByActorID != f.other.ID {
				t.Errorf("holder = %q, want %q", claimed.Task.ClaimedByActorID, f.other.ID)
			}
		})
	}
}

func TestClaimRejectsAMissingTask(t *testing.T) {
	f := newClaimFixture(t, 0)
	if _, err := f.local.ClaimTask(f.ctx, core.TaskRef{ID: "missingtaskident"}, core.ClaimInput{}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("claim of an unknown task = %v, want not found", err)
	}
}

func TestClaimRequiresAnActor(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "anon", "todo", core.PriorityNormal)
	ctx := context.Background()

	if _, err := f.local.ClaimTask(ctx, ref(task), core.ClaimInput{}); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("claim with no actor = %v, want unauthenticated", err)
	}
	if _, err := f.local.ClaimNext(ctx, core.ClaimNextInput{}); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("claim next with no actor = %v, want unauthenticated", err)
	}
	if _, err := f.local.RenewLease(ctx, ref(task), "t", 0); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("renew with no actor = %v, want unauthenticated", err)
	}
	if err := f.local.ReleaseLease(ctx, ref(task), "t", core.ReleaseInput{}); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("release with no actor = %v, want unauthenticated", err)
	}
	if _, err := f.local.SweepLeases(ctx, 10); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("sweep with no actor = %v, want unauthenticated", err)
	}
}

func TestClaimNextPicksHighestPriorityAndSkipsBlocked(t *testing.T) {
	f := newClaimFixture(t, 0)
	blocker := f.seedTask(t, "blocker", "todo", core.PriorityNormal)
	blocked := f.seedTask(t, "blocked", "todo", core.PriorityHighest)
	ready := f.seedTask(t, "ready", "todo", core.PriorityHigh)
	f.dependOn(t, blocked, blocker)

	claimed, err := f.local.ClaimNext(f.ctx, core.ClaimNextInput{ProjectRefs: []string{f.project.Key}})
	if err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}
	if claimed.Task.ID != ready.ID {
		t.Fatalf("claim next returned %q, want the unblocked high-priority task", claimed.Task.Title)
	}

	if err := f.local.ReleaseLease(f.ctx, ref(blocker), mustClaim(t, f, blocker).LeaseToken,
		core.ReleaseInput{Status: "stuck"}); err != nil {
		t.Fatalf("closing out the blocker: %v", err)
	}
	if _, err := f.local.ClaimNext(f.asOther(), core.ClaimNextInput{}); err != nil {
		t.Fatalf("claim next with the blocker non-terminal: %v", err)
	}
	if got := f.reload(t, blocked); got.ClaimedByActorID != "" {
		t.Error("a task whose dependency is not terminal was handed out")
	}
}

func mustClaim(t *testing.T, f *claimFixture, task *core.Task) *core.Claim {
	t.Helper()
	claimed, err := f.local.ClaimTask(f.ctx, ref(task), core.ClaimInput{})
	if err != nil {
		t.Fatalf("claiming %q: %v", task.Title, err)
	}
	return claimed
}

func TestClaimNextUnblocksOnceDependenciesAreTerminal(t *testing.T) {
	f := newClaimFixture(t, 0)
	blocker := f.seedTask(t, "blocker", "doing", core.PriorityLowest)
	blocked := f.seedTask(t, "blocked", "todo", core.PriorityHighest)
	f.dependOn(t, blocked, blocker)

	claimed := mustClaim(t, f, blocker)
	if err := f.local.ReleaseLease(f.ctx, ref(blocker), claimed.LeaseToken, core.ReleaseInput{Status: "done"}); err != nil {
		t.Fatalf("finishing the blocker: %v", err)
	}

	next, err := f.local.ClaimNext(f.asOther(), core.ClaimNextInput{})
	if err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}
	if next.Task.ID != blocked.ID {
		t.Errorf("claim next returned %q, want the now-unblocked task", next.Task.Title)
	}
}

func TestClaimNextTieBreakIsDeterministic(t *testing.T) {
	f := newClaimFixture(t, 0)
	first := f.seedTask(t, "a", "todo", core.PriorityNormal)
	second := f.seedTask(t, "b", "todo", core.PriorityNormal)

	wantFirst, wantSecond := first, second
	if second.ID < first.ID {
		wantFirst, wantSecond = second, first
	}

	one, err := f.local.ClaimNext(f.ctx, core.ClaimNextInput{})
	if err != nil {
		t.Fatalf("first claim next: %v", err)
	}
	two, err := f.local.ClaimNext(f.asOther(), core.ClaimNextInput{})
	if err != nil {
		t.Fatalf("second claim next: %v", err)
	}
	if one.Task.ID != wantFirst.ID || two.Task.ID != wantSecond.ID {
		t.Errorf("claim order = %q then %q, want %q then %q",
			one.Task.Title, two.Task.Title, wantFirst.Title, wantSecond.Title)
	}
}

func TestClaimNextFiltersByStatusAndLabel(t *testing.T) {
	f := newClaimFixture(t, 0)
	f.seedTask(t, "wrong status", "todo", core.PriorityHighest)
	wanted := f.seedTask(t, "right status", "doing", core.PriorityLowest)

	claimed, err := f.local.ClaimNext(f.ctx, core.ClaimNextInput{Statuses: []string{"doing"}})
	if err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}
	if claimed.Task.ID != wanted.ID {
		t.Errorf("claim next returned %q, want the status-matched task", claimed.Task.Title)
	}
	if _, err := f.local.ClaimNext(f.asOther(), core.ClaimNextInput{Tags: []string{"nope"}}); !core.IsKind(err, core.KindNoTaskAvailable) {
		t.Errorf("claim next with an unmatched tag = %v, want no task available", err)
	}
}

func TestClaimNextReportsNothingAvailable(t *testing.T) {
	f := newClaimFixture(t, 0)

	_, err := f.local.ClaimNext(f.ctx, core.ClaimNextInput{})
	if !core.IsKind(err, core.KindNoTaskAvailable) {
		t.Fatalf("claim next on an empty queue = %v, want no task available", err)
	}
	if core.IsKind(err, core.KindInternal) || core.IsKind(err, core.KindNotFound) {
		t.Error("an empty queue is not distinguishable from a failure")
	}

	task := f.seedTask(t, "held", "todo", core.PriorityNormal)
	mustClaim(t, f, task)
	if _, err := f.local.ClaimNext(f.asOther(), core.ClaimNextInput{}); !core.IsKind(err, core.KindNoTaskAvailable) {
		t.Errorf("claim next over a live lease = %v, want no task available", err)
	}
}

func TestClaimNextRejectsAnUnknownProjectAndBadTTL(t *testing.T) {
	f := newClaimFixture(t, 0)
	f.seedTask(t, "any", "todo", core.PriorityNormal)

	if _, err := f.local.ClaimNext(f.ctx, core.ClaimNextInput{ProjectRefs: []string{"nope"}}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("claim next over an unknown project = %v, want not found", err)
	}
	if _, err := f.local.ClaimNext(f.ctx, core.ClaimNextInput{TTL: core.Duration(-time.Minute)}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("claim next with a negative ttl = %v, want invalid", err)
	}
}

func TestClaimNextAppliesTheWorkflowLease(t *testing.T) {
	f := newClaimFixture(t, core.Duration(30*time.Minute))
	f.seedTask(t, "queued", "todo", core.PriorityNormal)

	claimed, err := f.local.ClaimNext(f.ctx, core.ClaimNextInput{})
	if err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}
	if got := claimed.LeaseExpiresAt.Sub(f.clock.Now()); got != 30*time.Minute {
		t.Errorf("queue lease ran for %s, want the workflow default", got)
	}
	if got := f.reload(t, claimed.Task); !got.LeaseExpiresAt.Equal(claimed.LeaseExpiresAt) {
		t.Errorf("stored lease expiry %s disagrees with the reported %s", got.LeaseExpiresAt, claimed.LeaseExpiresAt)
	}

	other := f.seedTask(t, "explicit", "todo", core.PriorityNormal)
	explicit, err := f.local.ClaimTask(f.asOther(), ref(other), core.ClaimInput{TTL: core.Duration(time.Minute)})
	if err != nil {
		t.Fatalf("claim with an explicit ttl: %v", err)
	}
	if got := explicit.LeaseExpiresAt.Sub(f.clock.Now()); got != time.Minute {
		t.Errorf("explicit lease ran for %s, want one minute", got)
	}
}

func TestRenewExtendsTheLease(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "long job", "todo", core.PriorityNormal)
	claimed := mustClaim(t, f, task)

	f.clock.Advance(lease.RenewInterval(lease.DefaultTTL))
	renewed, err := f.local.RenewLease(f.ctx, ref(task), claimed.LeaseToken, core.Duration(time.Hour))
	if err != nil {
		t.Fatalf("RenewLease: %v", err)
	}
	want := f.clock.Now().Add(time.Hour)
	if !renewed.LeaseExpiresAt.Equal(want) {
		t.Errorf("renewed expiry = %s, want %s", renewed.LeaseExpiresAt, want)
	}
	if renewed.LeaseToken != claimed.LeaseToken {
		t.Error("renewal changed the lease token")
	}
	if got := f.reload(t, task); !got.LeaseExpiresAt.Equal(want) {
		t.Errorf("stored expiry = %s, want %s", got.LeaseExpiresAt, want)
	}
}

func TestRenewRejectsStaleAndMissingTokens(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "renewals", "todo", core.PriorityNormal)
	claimed := mustClaim(t, f, task)

	if _, err := f.local.RenewLease(f.ctx, ref(task), "", 0); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("renew with no token = %v, want invalid", err)
	}
	if _, err := f.local.RenewLease(f.ctx, ref(task), "not-the-token", 0); !core.IsKind(err, core.KindLeaseExpired) {
		t.Errorf("renew with a wrong token = %v, want lease expired", err)
	}

	f.clock.Advance(lease.DefaultTTL + time.Minute)
	if _, err := f.local.RenewLease(f.ctx, ref(task), claimed.LeaseToken, 0); !core.IsKind(err, core.KindLeaseExpired) {
		t.Errorf("renew after expiry = %v, want lease expired", err)
	}
	if got := f.reload(t, task); got.ClaimedAtTime(f.clock.Now()) {
		t.Error("a rejected renewal re-established the lease")
	}
}

func TestRenewOnAnUnclaimedTaskIsRejected(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "never claimed", "todo", core.PriorityNormal)

	if _, err := f.local.RenewLease(f.ctx, ref(task), "some-token", 0); !core.IsKind(err, core.KindLeaseExpired) {
		t.Errorf("renew of an unclaimed task = %v, want lease expired", err)
	}
	if got := f.reload(t, task); got.ClaimedByActorID != "" {
		t.Error("a rejected renewal claimed the task")
	}
}

func TestRenewRejectsABadTTL(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "renewals", "todo", core.PriorityNormal)
	claimed := mustClaim(t, f, task)

	if _, err := f.local.RenewLease(f.ctx, ref(task), claimed.LeaseToken, core.Duration(lease.MaxTTL*2)); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("renew with an excessive ttl = %v, want invalid", err)
	}
}

func TestReleaseWithoutAFinalStatus(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "plain release", "doing", core.PriorityNormal)
	claimed := mustClaim(t, f, task)

	if err := f.local.ReleaseLease(f.ctx, ref(task), claimed.LeaseToken, core.ReleaseInput{}); err != nil {
		t.Fatalf("ReleaseLease: %v", err)
	}

	after := f.reload(t, task)
	if after.ClaimedByActorID != "" || after.LeaseExpiresAt != nil {
		t.Errorf("released task is still claimed: %+v", after)
	}
	if after.Status != "doing" {
		t.Errorf("status = %q, want it unchanged", after.Status)
	}
	if _, err := f.local.RenewLease(f.ctx, ref(task), claimed.LeaseToken, 0); !core.IsKind(err, core.KindLeaseExpired) {
		t.Errorf("the released token still renews: %v", err)
	}
	if got := f.events(t, core.EventTaskReleased); len(got) != 1 {
		t.Errorf("task.released events = %d, want 1", len(got))
	}
}

func TestReleaseWithAFinalStatusAndResult(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "finish", "doing", core.PriorityNormal)
	claimed := mustClaim(t, f, task)

	err := f.local.ReleaseLease(f.ctx, ref(task), claimed.LeaseToken, core.ReleaseInput{
		Status: "done",
		Result: map[string]any{"exit_code": float64(0), "summary": "built"},
	})
	if err != nil {
		t.Fatalf("ReleaseLease: %v", err)
	}

	after := f.reload(t, task)
	if after.Status != "done" {
		t.Errorf("status = %q, want done", after.Status)
	}
	if after.CompletedAt == nil {
		t.Error("a terminal release recorded no completion time")
	}
	if after.ClaimedByActorID != "" {
		t.Error("a released task is still claimed")
	}

	got := f.artifacts(t, task)
	if len(got) != 1 || got[0].Kind != core.ArtifactResult {
		t.Fatalf("artifacts = %+v, want one result artifact", got)
	}
	if got[0].Payload["summary"] != "built" {
		t.Errorf("result payload = %+v, want the released result", got[0].Payload)
	}
	if len(f.events(t, core.EventTaskTransitioned)) != 1 {
		t.Error("a release with a final status emitted no transition event")
	}
}

func TestReleaseWithADisallowedStatus(t *testing.T) {
	cases := []struct {
		name   string
		status string
		input  core.ReleaseInput
	}{
		{name: "no such transition", status: "review", input: core.ReleaseInput{Status: "todo"}},
		{name: "unknown state", status: "doing", input: core.ReleaseInput{Status: "shipped"}},
		{name: "already there", status: "doing", input: core.ReleaseInput{Status: "doing"}},
		{name: "comment required", status: "doing", input: core.ReleaseInput{Status: "review"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newClaimFixture(t, 0)
			task := f.seedTask(t, "guarded", tc.status, core.PriorityNormal)
			claimed := mustClaim(t, f, task)

			if err := f.local.ReleaseLease(f.ctx, ref(task), claimed.LeaseToken, tc.input); !core.IsKind(err, core.KindInvalid) {
				t.Fatalf("release = %v, want invalid", err)
			}
			after := f.reload(t, task)
			if after.ClaimedByActorID != f.actor.ID {
				t.Error("a rejected release gave up the lease")
			}
			if after.Status != tc.status {
				t.Errorf("status = %q, want it unchanged", after.Status)
			}
		})
	}
}

func TestReleaseWithARequiredComment(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "reviewed", "doing", core.PriorityNormal)
	claimed := mustClaim(t, f, task)

	err := f.local.ReleaseLease(f.ctx, ref(task), claimed.LeaseToken,
		core.ReleaseInput{Status: "review", Comment: "needs a second pair of eyes"})
	if err != nil {
		t.Fatalf("ReleaseLease: %v", err)
	}
	if got := f.reload(t, task); got.Status != "review" {
		t.Errorf("status = %q, want review", got.Status)
	}

	ctx := context.Background()
	if err := f.local.store.View(ctx, f.scope, func(tx store.Tx) error {
		comments, err := tx.ListComments(ctx, task.ID)
		if err != nil {
			return err
		}
		if len(comments) != 1 || comments[0].Body != "needs a second pair of eyes" {
			t.Errorf("comments = %+v, want the release comment", comments)
		}
		return nil
	}); err != nil {
		t.Fatalf("listing comments: %v", err)
	}
}

// A worker whose lease was taken over must not be able to write anything, or a
// zombie process can overwrite the new holder's work.
func TestReleaseWithAStaleTokenWritesNothing(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "zombie", "doing", core.PriorityNormal)
	first := mustClaim(t, f, task)

	f.clock.Advance(lease.DefaultTTL + time.Minute)
	if _, err := f.local.ClaimTask(f.asOther(), ref(task), core.ClaimInput{}); err != nil {
		t.Fatalf("re-claim: %v", err)
	}

	err := f.local.ReleaseLease(f.ctx, ref(task), first.LeaseToken, core.ReleaseInput{
		Status: "done",
		Result: map[string]any{"exit_code": float64(1)},
	})
	if !core.IsKind(err, core.KindLeaseExpired) {
		t.Fatalf("stale release = %v, want lease expired", err)
	}
	if got := f.artifacts(t, task); len(got) != 0 {
		t.Errorf("a stale release wrote %d artifacts", len(got))
	}
	after := f.reload(t, task)
	if after.ClaimedByActorID != f.other.ID || after.Status != "doing" {
		t.Errorf("a stale release disturbed the new holder: %+v", after)
	}
}

func TestReleaseRequiresATokenAndAKnownTask(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "guards", "doing", core.PriorityNormal)
	mustClaim(t, f, task)

	if err := f.local.ReleaseLease(f.ctx, ref(task), "", core.ReleaseInput{}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("release with no token = %v, want invalid", err)
	}
	if err := f.local.ReleaseLease(f.ctx, core.TaskRef{ID: "missingtaskident"}, "tok", core.ReleaseInput{}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("release of an unknown task = %v, want not found", err)
	}
}

// The lease guard is what a transition gates on, so it is exercised directly
// rather than through the task service.
func TestLeaseTokenGuard(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "guarded", "doing", core.PriorityNormal)
	claimed := mustClaim(t, f, task)

	guard := func(token string) error {
		ctx := context.Background()
		var out error
		if err := f.local.store.Update(ctx, f.scope, func(tx store.Tx) error {
			loaded, err := tx.GetTask(ctx, ref(task))
			if err != nil {
				return err
			}
			out = requireLeaseToken(ctx, tx, loaded, token, f.clock.Now())
			return nil
		}); err != nil {
			t.Fatalf("guard transaction: %v", err)
		}
		return out
	}

	if err := guard(claimed.LeaseToken); err != nil {
		t.Errorf("the current token was rejected: %v", err)
	}
	if err := guard(""); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("a missing token on a claimed task = %v, want invalid", err)
	}
	if err := guard("someone-elses-token"); !core.IsKind(err, core.KindLeaseExpired) {
		t.Errorf("a wrong token = %v, want lease expired", err)
	}

	f.clock.Advance(lease.DefaultTTL + time.Minute)
	if err := guard(claimed.LeaseToken); !core.IsKind(err, core.KindLeaseExpired) {
		t.Errorf("a token whose lease passed = %v, want lease expired", err)
	}
	if err := guard(""); err != nil {
		t.Errorf("an expired lease still demands a token: %v", err)
	}
}

func TestSweepMaterializesExpiryAndReverts(t *testing.T) {
	f := newClaimFixture(t, 0)
	reverting := f.seedTask(t, "reverts", "doing", core.PriorityNormal)
	plain := f.seedTask(t, "stays", "review", core.PriorityNormal)
	live := f.seedTask(t, "live", "doing", core.PriorityNormal)

	mustClaim(t, f, reverting)
	mustClaim(t, f, plain)
	f.clock.Advance(lease.DefaultTTL + time.Minute)
	liveClaim := mustClaim(t, f, live)

	swept, err := f.local.SweepLeases(f.ctx, 100)
	if err != nil {
		t.Fatalf("SweepLeases: %v", err)
	}
	if swept != 2 {
		t.Fatalf("swept = %d, want the two expired leases", swept)
	}

	if got := f.reload(t, reverting); got.Status != "todo" || got.ClaimedByActorID != "" || got.LeaseExpiresAt != nil {
		t.Errorf("reverting task = %+v, want it cleared and back in todo", got)
	}
	if got := f.reload(t, plain); got.Status != "review" || got.ClaimedByActorID != "" {
		t.Errorf("plain task = %+v, want it cleared with its status intact", got)
	}
	if got := f.reload(t, live); !got.LeaseExpiresAt.Equal(liveClaim.LeaseExpiresAt) {
		t.Error("the sweeper disturbed a live lease")
	}
	if got := f.events(t, core.EventTaskLeaseExpired); len(got) != 2 {
		t.Fatalf("lease expired events = %d, want 2", len(got))
	}

	again, err := f.local.SweepLeases(f.ctx, 100)
	if err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	if again != 0 {
		t.Errorf("a repeated sweep swept %d tasks, want none", again)
	}
	if got := f.events(t, core.EventTaskLeaseExpired); len(got) != 2 {
		t.Errorf("a repeated sweep emitted %d lease expired events in total, want 2", len(got))
	}
}

func TestSweepRevertsToTheWorkflowInitialState(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "no revert target", "stuck", core.PriorityNormal)
	mustClaim(t, f, task)
	f.clock.Advance(lease.DefaultTTL + time.Minute)

	if _, err := f.local.SweepLeases(f.ctx, 0); err != nil {
		t.Fatalf("SweepLeases: %v", err)
	}
	if got := f.reload(t, task); got.Status != "todo" {
		t.Errorf("status = %q, want the workflow's initial state", got.Status)
	}
}

// The sweeper is a convenience. Every claim, renewal and release must behave
// the same way when one never runs.
func TestLeaseLifecycleWithoutASweeper(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "unattended", "doing", core.PriorityNormal)

	first := mustClaim(t, f, task)
	f.clock.Advance(lease.DefaultTTL + time.Second)

	if _, err := f.local.RenewLease(f.ctx, ref(task), first.LeaseToken, 0); !core.IsKind(err, core.KindLeaseExpired) {
		t.Errorf("renew after expiry = %v, want lease expired", err)
	}
	second, err := f.local.ClaimTask(f.asOther(), ref(task), core.ClaimInput{})
	if err != nil {
		t.Fatalf("re-claim without a sweeper: %v", err)
	}
	if err := f.local.ReleaseLease(f.ctx, ref(task), first.LeaseToken, core.ReleaseInput{}); !core.IsKind(err, core.KindLeaseExpired) {
		t.Errorf("stale release = %v, want lease expired", err)
	}
	if err := f.local.ReleaseLease(f.asOther(), ref(task), second.LeaseToken, core.ReleaseInput{Status: "done"}); err != nil {
		t.Fatalf("release by the new holder: %v", err)
	}
	if got := f.reload(t, task); got.Status != "done" || got.ClaimedByActorID != "" {
		t.Errorf("task = %+v, want it done and unclaimed", got)
	}
}

func TestSweeperTickerDrivesTheService(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "swept by ticker", "doing", core.PriorityNormal)
	mustClaim(t, f, task)
	f.clock.Advance(lease.DefaultTTL + time.Minute)

	sweeper := lease.NewSweeper(func(ctx context.Context, limit int) (int, error) {
		return f.local.SweepLeases(core.WithActor(ctx, f.actor), limit)
	}, lease.WithClock(f.clock), lease.WithInterval(time.Minute))

	n, err := sweeper.Once(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("sweeper pass = %d, %v, want one task swept", n, err)
	}
	if got := f.reload(t, task); got.ClaimedByActorID != "" || got.Status != "todo" {
		t.Errorf("task = %+v, want it swept and reverted", got)
	}
}

// projectActor is an agent whose token is bound to one project, which is how a
// worker is normally scoped down.
func (f *claimFixture) projectActor(projectID string) context.Context {
	return core.WithActor(context.Background(), &core.Actor{
		ID:        f.other.ID,
		TenantID:  f.scope.TenantID,
		Kind:      core.ActorAgent,
		Handle:    "scoped",
		Scopes:    []core.Scope{core.ScopeAll},
		ProjectID: projectID,
	})
}

func TestClaimIsRefusedOutsideTheActorsProject(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "elsewhere", "todo", core.PriorityNormal)
	ctx := f.projectActor("someotherproject")

	if _, err := f.local.ClaimTask(ctx, ref(task), core.ClaimInput{}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("claim outside the actor's project = %v, want forbidden", err)
	}
	if _, err := f.local.ClaimNext(ctx, core.ClaimNextInput{ProjectRefs: []string{f.project.Key}}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("claim next outside the actor's project = %v, want forbidden", err)
	}
	if _, err := f.local.ClaimNext(ctx, core.ClaimNextInput{}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("unfiltered claim next = %v, want it confined to the actor's own project", err)
	}
	if _, err := f.local.ClaimNext(f.projectActor(f.project.ID), core.ClaimNextInput{}); err != nil {
		t.Errorf("claim next inside the actor's project: %v", err)
	}
}

func TestClaimNextHonoursAnExplicitTTL(t *testing.T) {
	f := newClaimFixture(t, core.Duration(time.Hour))
	f.seedTask(t, "queued", "todo", core.PriorityNormal)

	claimed, err := f.local.ClaimNext(f.ctx, core.ClaimNextInput{TTL: core.Duration(90 * time.Second)})
	if err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}
	if got := claimed.LeaseExpiresAt.Sub(f.clock.Now()); got != 90*time.Second {
		t.Errorf("queue lease ran for %s, want the per-claim ttl", got)
	}
}

func TestClaimOfADeletedTaskIsNotFound(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "gone", "todo", core.PriorityNormal)

	ctx := context.Background()
	if err := f.local.store.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.DeleteTask(ctx, task.ID, false)
	}); err != nil {
		t.Fatalf("deleting the task: %v", err)
	}
	if _, err := f.local.ClaimTask(f.ctx, ref(task), core.ClaimInput{}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("claim of a deleted task = %v, want not found", err)
	}
}

func TestReleaseToAStatusRequiringAScope(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "audited", "doing", core.PriorityNormal)

	worker := &core.Actor{
		ID:       f.other.ID,
		TenantID: f.scope.TenantID,
		Kind:     core.ActorAgent,
		Handle:   "worker",
		Scopes: []core.Scope{
			core.ScopeTaskRead, core.ScopeTaskClaim, core.ScopeTaskTransition,
			core.ScopeArtifactWrite, core.ScopeCommentWrite,
		},
	}
	ctx := core.WithActor(context.Background(), worker)

	claimed, err := f.local.ClaimTask(ctx, ref(task), core.ClaimInput{})
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if err := f.local.ReleaseLease(ctx, ref(task), claimed.LeaseToken, core.ReleaseInput{Status: "audit"}); !core.IsKind(err, core.KindForbidden) {
		t.Fatalf("release into a scoped status = %v, want forbidden", err)
	}
	if got := f.reload(t, task); got.Status != "doing" || got.ClaimedByActorID != worker.ID {
		t.Errorf("a refused release changed the task: %+v", got)
	}
	if err := f.local.ReleaseLease(f.ctx, ref(task), claimed.LeaseToken, core.ReleaseInput{Status: "audit"}); err != nil {
		t.Fatalf("release by an actor holding every scope: %v", err)
	}
}

func TestClaimNextSkipsTasksAlreadyFinished(t *testing.T) {
	for _, status := range []string{"done", "audit"} {
		t.Run(status, func(t *testing.T) {
			f := newClaimFixture(t, 0)
			finished := f.seedTask(t, "finished", status, core.PriorityHighest)

			if _, err := f.local.ClaimNext(f.ctx, core.ClaimNextInput{}); !core.IsKind(err, core.KindNoTaskAvailable) {
				t.Fatalf("claim next over a finished queue = %v, want no task available", err)
			}
			if got := f.reload(t, finished); got.ClaimedByActorID != "" || got.ClaimCount != 0 {
				t.Fatalf("a task in terminal status %q was handed out: %+v", status, got)
			}

			open := f.seedTask(t, "open", "todo", core.PriorityLowest)
			claimed, err := f.local.ClaimNext(f.ctx, core.ClaimNextInput{})
			if err != nil {
				t.Fatalf("ClaimNext: %v", err)
			}
			if claimed.Task.ID != open.ID {
				t.Errorf("claim next returned %q, want the unfinished task", claimed.Task.Title)
			}
		})
	}
}

func TestClaimOfAFinishedTaskIsRefused(t *testing.T) {
	f := newClaimFixture(t, 0)
	finished := f.seedTask(t, "finished", "done", core.PriorityNormal)

	_, err := f.local.ClaimTask(f.ctx, ref(finished), core.ClaimInput{})
	if !core.IsKind(err, core.KindConflict) {
		t.Fatalf("claiming a finished task = %v, want conflict", err)
	}
	if got := f.reload(t, finished); got.ClaimedByActorID != "" || got.ClaimCount != 0 {
		t.Fatalf("a finished task was leased: %+v", got)
	}

	open := f.seedTask(t, "open", "todo", core.PriorityNormal)
	if _, err := f.local.ClaimTask(f.ctx, ref(open), core.ClaimInput{}); err != nil {
		t.Fatalf("claiming an unfinished task: %v", err)
	}
}
