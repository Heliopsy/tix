// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// statsFixture is one tenant with a project whose workflow the statistics
// tests move tasks through. Everything is driven by the fake clock, so the
// window a read covers is decided by the test rather than by how long it ran.
type statsFixture struct {
	local   *Local
	clock   *clock.Fake
	scope   core.TenantScope
	actor   *core.Actor
	ctx     context.Context
	project *core.Project
}

func newStatsFixture(t *testing.T) *statsFixture {
	t.Helper()
	l, clk, scope, actor := newLocal(t)
	return &statsFixture{
		local: l, clock: clk, scope: scope, actor: actor,
		ctx:     taskContext(actor),
		project: seedTaskProject(t, l, scope, "infra"),
	}
}

// add creates a task at the clock's current instant.
func (f *statsFixture) add(t *testing.T, title string) *core.Task {
	t.Helper()
	task, err := f.local.CreateTask(f.ctx, core.CreateTaskInput{
		ProjectRef: f.project.Key, Title: title,
	})
	if err != nil {
		t.Fatalf("creating %q: %v", title, err)
	}
	return task
}

// finish moves a task to a terminal state at the clock's current instant.
func (f *statsFixture) finish(t *testing.T, task *core.Task) {
	t.Helper()
	f.finishAs(t, f.actor, task)
}

// finishAs is finish, attributing the move to another actor.
func (f *statsFixture) finishAs(t *testing.T, actor *core.Actor, task *core.Task) {
	t.Helper()
	ctx := taskContext(actor)
	ref := core.TaskRef{ID: task.ID}
	for _, to := range []string{"doing", "done"} {
		if _, err := f.local.TransitionTask(ctx, ref, core.TransitionInput{To: to}); err != nil {
			t.Fatalf("moving %q to %q: %v", task.Title, to, err)
		}
	}
}

func (f *statsFixture) stats(t *testing.T, in core.StatsInput) *core.Stats {
	t.Helper()
	got, err := f.local.Stats(f.ctx, in)
	if err != nil {
		t.Fatalf("Stats(%+v): %v", in, err)
	}
	return got
}

// seedWork lays down three tasks created together, two of which are finished on
// different days, and leaves the clock five days after the creations.
func (f *statsFixture) seedWork(t *testing.T) (a, b, c *core.Task) {
	t.Helper()
	a = f.add(t, "alpha")
	b = f.add(t, "beta")
	c = f.add(t, "gamma")

	f.clock.Advance(24 * time.Hour)
	f.finish(t, a)

	f.clock.Advance(72 * time.Hour)
	f.finish(t, b)

	f.clock.Advance(24 * time.Hour)
	return a, b, c
}

func categoryCount(t *testing.T, s *core.Stats, want core.StateCategory) int {
	t.Helper()
	for _, c := range s.ByCategory {
		if c.Category == want {
			return c.Count
		}
	}
	t.Fatalf("no count reported for category %q in %+v", want, s.ByCategory)
	return 0
}

// A terminal transition inside the window is counted once in the total and once
// on the day it happened.
func TestStatsCountsTerminalTransitionsInTheWindow(t *testing.T) {
	f := newStatsFixture(t)
	f.seedWork(t)

	got := f.stats(t, core.StatsInput{})

	if got.Completed != 2 {
		t.Errorf("completed = %d, want 2", got.Completed)
	}
	if got.Created != 3 {
		t.Errorf("created = %d, want 3", got.Created)
	}
	want := []core.StatsDay{{Date: "2026-01-02", Completed: 1}, {Date: "2026-01-05", Completed: 1}}
	if len(got.PerDay) != len(want) {
		t.Fatalf("per day = %+v, want %+v", got.PerDay, want)
	}
	for i, day := range want {
		if got.PerDay[i] != day {
			t.Errorf("per day[%d] = %+v, want %+v", i, got.PerDay[i], day)
		}
	}
	if got.MedianLeadTime != core.Duration(60*time.Hour) {
		t.Errorf("median lead time = %s, want 60h", got.MedianLeadTime)
	}
	if got.SlowestLeadTime != core.Duration(96*time.Hour) {
		t.Errorf("slowest lead time = %s, want 96h", got.SlowestLeadTime)
	}
	if got.Since.After(got.Until) {
		t.Errorf("window %s .. %s runs backwards", got.Since, got.Until)
	}
}

// Work that reached a terminal state before the window began is in neither the
// completed total nor the lead times.
func TestStatsExcludesWorkBeforeTheWindow(t *testing.T) {
	f := newStatsFixture(t)
	f.seedWork(t)

	got := f.stats(t, core.StatsInput{Window: core.Duration(48 * time.Hour)})

	if got.Completed != 1 {
		t.Errorf("completed = %d, want only the task finished inside the window", got.Completed)
	}
	if got.Created != 0 {
		t.Errorf("created = %d, want 0; every task was created before the window", got.Created)
	}
	if got.MedianLeadTime != core.Duration(96*time.Hour) {
		t.Errorf("median lead time = %s, want 96h; the earlier task must not weigh on it", got.MedianLeadTime)
	}
	if got.SlowestLeadTime != core.Duration(96*time.Hour) {
		t.Errorf("slowest lead time = %s, want 96h", got.SlowestLeadTime)
	}
	for _, day := range got.PerDay {
		if day.Date == "2026-01-02" {
			t.Errorf("per day still carries %+v, which is before the window", day)
		}
	}
}

// An explicit start is honoured as given, and contradicts nothing else.
func TestStatsHonoursAnExplicitStart(t *testing.T) {
	f := newStatsFixture(t)
	f.seedWork(t)

	since := time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)
	got := f.stats(t, core.StatsInput{Since: since})

	if !got.Since.Equal(since) {
		t.Errorf("since = %s, want %s", got.Since, since)
	}
	if got.Completed != 1 {
		t.Errorf("completed = %d, want 1", got.Completed)
	}
}

// A soft deleted task appears in no count at all.
func TestStatsExcludesSoftDeletedTasks(t *testing.T) {
	f := newStatsFixture(t)
	a, _, c := f.seedWork(t)

	before := f.stats(t, core.StatsInput{})
	if before.Completed != 2 || before.Created != 3 {
		t.Fatalf("fixture is wrong before deleting: completed %d, created %d",
			before.Completed, before.Created)
	}
	for _, task := range []*core.Task{a, c} {
		if err := f.local.DeleteTask(f.ctx, core.TaskRef{ID: task.ID}, core.DeleteTaskInput{}); err != nil {
			t.Fatalf("deleting %q: %v", task.Title, err)
		}
	}

	got := f.stats(t, core.StatsInput{})
	if got.Completed != 1 {
		t.Errorf("completed = %d, want 1; the deleted task is still counted", got.Completed)
	}
	if got.Created != 1 {
		t.Errorf("created = %d, want 1; a deleted task is still counted as created", got.Created)
	}
	if n := categoryCount(t, got, core.CategoryTodo); n != 0 {
		t.Errorf("todo = %d, want 0; the deleted open task is still counted", n)
	}
	if n := categoryCount(t, got, core.CategoryDone); n != 1 {
		t.Errorf("done = %d, want 1", n)
	}
	for _, o := range got.Oldest {
		if o.Ref == c.Ref {
			t.Errorf("the deleted task %q is still reported as ageing", o.Ref)
		}
	}
	total := 0
	for _, actor := range got.TopActors {
		total += actor.Moved
	}
	if total != 1 {
		t.Errorf("the leaderboard totals %d moves, want 1", total)
	}
}

// The count per state category is of where tasks are now, not of the window.
func TestStatsCountsCurrentStateCategories(t *testing.T) {
	f := newStatsFixture(t)
	_, _, c := f.seedWork(t)

	if _, err := f.local.TransitionTask(f.ctx, core.TaskRef{ID: c.ID},
		core.TransitionInput{To: "doing"}); err != nil {
		t.Fatalf("moving %q: %v", c.Title, err)
	}
	got := f.stats(t, core.StatsInput{})

	for _, tc := range []struct {
		category core.StateCategory
		want     int
	}{
		{core.CategoryTodo, 0},
		{core.CategoryInProgress, 1},
		{core.CategoryDone, 2},
	} {
		if n := categoryCount(t, got, tc.category); n != tc.want {
			t.Errorf("category %q = %d, want %d", tc.category, n, tc.want)
		}
	}
}

// The leaderboard counts terminal moves per actor and says what it counts.
func TestStatsLeaderboardCountsTerminalMovesAndNamesTheMeasure(t *testing.T) {
	f := newStatsFixture(t)
	a, b, c := f.seedWork(t)
	_ = a
	_ = b

	other := seedTaskActor(t, f.local, f.scope, "bob", core.RoleAdmin, core.ScopeAll)
	f.finishAs(t, other, c)

	got := f.stats(t, core.StatsInput{})

	if len(got.TopActors) != 2 {
		t.Fatalf("top actors = %+v, want two", got.TopActors)
	}
	if got.TopActors[0].ActorID != f.actor.ID || got.TopActors[0].Moved != 2 {
		t.Errorf("leader = %+v, want %q with 2", got.TopActors[0], f.actor.ID)
	}
	if got.TopActors[0].Handle != f.actor.Handle {
		t.Errorf("leader handle = %q, want %q", got.TopActors[0].Handle, f.actor.Handle)
	}
	if got.TopActors[1].ActorID != other.ID || got.TopActors[1].Moved != 1 {
		t.Errorf("runner up = %+v, want %q with 1", got.TopActors[1], other.ID)
	}
	if got.LeaderboardMeasure != core.StatsLeaderboardMeasure {
		t.Errorf("leaderboard measure = %q, want %q", got.LeaderboardMeasure, core.StatsLeaderboardMeasure)
	}
	if !strings.Contains(got.LeaderboardMeasure, "terminal") {
		t.Errorf("leaderboard measure %q does not say what it counts", got.LeaderboardMeasure)
	}
}

// The ageing list is the oldest tasks not yet terminal, with how long they have
// waited, and never a task that has been completed.
func TestStatsReportsTheOldestOpenTasks(t *testing.T) {
	f := newStatsFixture(t)
	_, _, c := f.seedWork(t)

	f.clock.Advance(24 * time.Hour)
	later := f.add(t, "delta")

	got := f.stats(t, core.StatsInput{})

	if len(got.Oldest) != 2 {
		t.Fatalf("oldest = %+v, want the two open tasks", got.Oldest)
	}
	if got.Oldest[0].Ref != c.Ref {
		t.Errorf("oldest[0] = %q, want the earliest open task %q", got.Oldest[0].Ref, c.Ref)
	}
	if got.Oldest[0].Age != core.Duration(144*time.Hour) {
		t.Errorf("age of %q = %s, want 144h", c.Ref, got.Oldest[0].Age)
	}
	if got.Oldest[1].Ref != later.Ref || got.Oldest[1].Age != 0 {
		t.Errorf("oldest[1] = %+v, want %q with no age yet", got.Oldest[1], later.Ref)
	}
	if got.Oldest[0].Status != "todo" {
		t.Errorf("status = %q, want todo", got.Oldest[0].Status)
	}
}

// A window in which nothing happened is an answer, not a failure.
func TestStatsEmptyWindowReportsZeroes(t *testing.T) {
	f := newStatsFixture(t)

	got := f.stats(t, core.StatsInput{Window: core.Duration(time.Hour)})

	if got.Completed != 0 || got.Created != 0 {
		t.Errorf("completed = %d and created = %d, want zero of each", got.Completed, got.Created)
	}
	if got.MedianLeadTime != 0 || got.SlowestLeadTime != 0 {
		t.Errorf("lead times = %s and %s, want zero", got.MedianLeadTime, got.SlowestLeadTime)
	}
	if len(got.PerDay) != 0 || len(got.TopActors) != 0 || len(got.Oldest) != 0 {
		t.Errorf("per day %+v, top actors %+v and oldest %+v; all three must be empty",
			got.PerDay, got.TopActors, got.Oldest)
	}
	for _, c := range got.ByCategory {
		if c.Count != 0 {
			t.Errorf("category %q = %d, want 0", c.Category, c.Count)
		}
	}
}

// Statistics for one tenant must contain nothing from another, in any field.
// This is the failure this project treats as unrecoverable, so the two tenants
// are seeded to look alike: same project key, same titles, same actor handle.
func TestStatsNeverCrossesATenant(t *testing.T) {
	f := newStatsFixture(t)
	other := f.otherTenant(t)

	f.seedWork(t)
	other.seedWork(t)
	// One extra open task on the other side, because the two tenants are
	// otherwise seeded to be indistinguishable: a task reference is unique per
	// project and both projects share a key, so only the counts can tell the
	// rows apart.
	other.add(t, "delta")

	got := f.stats(t, core.StatsInput{})
	foreign := other.stats(t, core.StatsInput{})

	if got.Completed != 2 || got.Created != 3 {
		t.Errorf("completed = %d and created = %d, want 2 and 3; the other tenant leaked in",
			got.Completed, got.Created)
	}
	if n := categoryCount(t, got, core.CategoryDone); n != 2 {
		t.Errorf("done = %d, want 2", n)
	}
	for _, day := range got.PerDay {
		if day.Completed != 1 {
			t.Errorf("day %+v counts more than this tenant finished", day)
		}
	}
	foreignActors := map[string]bool{}
	for _, a := range foreign.TopActors {
		foreignActors[a.ActorID] = true
	}
	for _, a := range got.TopActors {
		if foreignActors[a.ActorID] {
			t.Errorf("actor %q belongs to the other tenant", a.ActorID)
		}
		if a.ActorID != f.actor.ID {
			t.Errorf("leaderboard names %q, which is not an actor of this tenant", a.ActorID)
		}
	}
	if len(got.Oldest) != 1 {
		t.Errorf("oldest = %+v, want only this tenant's one open task", got.Oldest)
	}
	if len(foreign.Oldest) != 2 || foreign.Completed != 2 || foreign.Created != 4 {
		t.Errorf("the other tenant reads back wrong: oldest %+v, completed %d, created %d",
			foreign.Oldest, foreign.Completed, foreign.Created)
	}
}

// otherTenant builds a second tenant on the same store, deliberately
// indistinguishable from the first by every value a reader could confuse.
func (f *statsFixture) otherTenant(t *testing.T) *statsFixture {
	t.Helper()
	ctx := context.Background()
	tenant := core.Tenant{Key: "other", Name: "Other"}
	if err := f.local.store.Unscoped(ctx, func(u store.UnscopedTx) error {
		return u.CreateTenant(ctx, &tenant)
	}); err != nil {
		t.Fatalf("creating the second tenant: %v", err)
	}
	scope := core.TenantScope{TenantID: tenant.ID}
	actor := seedTaskActor(t, f.local, scope, f.actor.Handle, core.RoleAdmin, core.ScopeAll)
	return &statsFixture{
		local: f.local, clock: f.clock, scope: scope, actor: actor,
		ctx:     taskContext(actor),
		project: seedTaskProject(t, f.local, scope, f.project.Key),
	}
}

// One project's figures cover that project alone.
func TestStatsNarrowsToOneProject(t *testing.T) {
	f := newStatsFixture(t)
	f.seedWork(t)

	quiet := seedTaskProject(t, f.local, f.scope, "quiet")
	got := f.stats(t, core.StatsInput{ProjectRef: quiet.Key})

	if got.ProjectKey != quiet.Key || got.ProjectID != quiet.ID {
		t.Errorf("project = %q/%q, want %q/%q", got.ProjectID, got.ProjectKey, quiet.ID, quiet.Key)
	}
	if got.Completed != 0 || got.Created != 0 || len(got.Oldest) != 0 {
		t.Errorf("the empty project reports %+v", got)
	}

	busy := f.stats(t, core.StatsInput{ProjectRef: f.project.Key})
	if busy.Completed != 2 || busy.Created != 3 {
		t.Errorf("the busy project reports completed %d and created %d, want 2 and 3",
			busy.Completed, busy.Created)
	}
}

func TestStatsRejections(t *testing.T) {
	f := newStatsFixture(t)

	tests := []struct {
		name string
		in   core.StatsInput
		kind core.Kind
	}{
		{"negative window", core.StatsInput{Window: core.Duration(-time.Hour)}, core.KindInvalid},
		{"window past the bound", core.StatsInput{Window: core.MaxStatsWindow + 1}, core.KindInvalid},
		{"too many actors", core.StatsInput{TopActors: core.MaxStatsTopActors + 1}, core.KindInvalid},
		{"too many ageing rows", core.StatsInput{Oldest: core.MaxStatsOldest + 1}, core.KindInvalid},
		{"start in the future", core.StatsInput{Since: f.clock.Now().Add(time.Hour)}, core.KindInvalid},
		{"unknown project", core.StatsInput{ProjectRef: "nosuchproject"}, core.KindNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := f.local.Stats(f.ctx, tc.in); !core.IsKind(err, tc.kind) {
				t.Errorf("Stats = %v, want %s", err, tc.kind)
			}
		})
	}
}

func TestStatsRequiresAuthentication(t *testing.T) {
	f := newStatsFixture(t)
	if _, err := f.local.Stats(context.Background(), core.StatsInput{}); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("Stats without an actor = %v, want unauthenticated", err)
	}
}
