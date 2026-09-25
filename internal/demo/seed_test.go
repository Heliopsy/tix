// SPDX-License-Identifier: AGPL-3.0-or-later

package demo

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/sqlite"
)

// seedEnd is the instant the seeded history ends at under test, so every
// assertion about the window is made against a fixed point.
var seedEnd = time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

// target is a service over a fresh database, with the clock the seeder drives.
type target struct {
	svc   *service.Local
	store store.Store
	clk   *clock.Fake
	admin *core.Actor
	ctx   context.Context
}

func newTarget(t *testing.T) *target {
	t.Helper()
	clk := clock.NewFake(seedEnd)
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "tix.db"), clk)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	svc := service.New(st, service.WithClock(clk))
	tenant, err := svc.EnsureDefaults(ctx)
	if err != nil {
		t.Fatalf("EnsureDefaults: %v", err)
	}
	admin := core.SystemActor(tenant.ID)
	return &target{svc: svc, store: st, clk: clk, admin: admin, ctx: core.WithActor(ctx, admin)}
}

func (tg *target) seed(t *testing.T, days int) *Summary {
	t.Helper()
	out, err := Seed(tg.ctx, tg.svc, tg.clk, tg.admin, Options{Days: days, Now: seedEnd})
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	return out
}

// stats reads the figures over the whole seeded window.
func (tg *target) stats(t *testing.T, since time.Time) *core.Stats {
	t.Helper()
	out, err := tg.svc.Stats(tg.ctx, core.StatsInput{Since: since, TopActors: core.MaxStatsTopActors})
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	return out
}

// TestFixtureIsWellFormed checks the tables the seeder replays, so a typo in
// them fails here rather than halfway through a seed.
func TestFixtureIsWellFormed(t *testing.T) {
	known := map[string]bool{}
	for _, p := range people {
		known[p.handle] = true
	}
	lists := map[string]bool{}
	for _, p := range projects {
		lists[p.key] = true
	}
	created := map[string]int{}

	for _, seed := range tasks {
		if _, dup := created[seed.key]; dup {
			t.Errorf("task key %q appears twice", seed.key)
		}
		created[seed.key] = seed.created
	}

	for _, tc := range tasks {
		t.Run(tc.key, func(t *testing.T) {
			if tc.title == "" || tc.body == "" {
				t.Error("every seeded task needs a title and a body")
			}
			if !lists[tc.project] {
				t.Errorf("project %q is not seeded", tc.project)
			}
			if !known[tc.assignee] || !known[tc.creator] {
				t.Errorf("assignee %q or creator %q is not seeded", tc.assignee, tc.creator)
			}
			if !tc.priority.Valid() {
				t.Errorf("priority %d is out of range", tc.priority)
			}
			if tc.created < 0 || tc.created >= fixtureDays {
				t.Errorf("created day %d is outside the fixture window", tc.created)
			}
			if tc.hour < 6 || tc.hour > 16 {
				t.Errorf("hour %d leaves no room for the moves that follow it", tc.hour)
			}
			if tc.parent != "" && created[tc.parent] > tc.created {
				t.Errorf("parent %q is created after its child", tc.parent)
			}
			for _, dep := range tc.dependsOn {
				if day, ok := created[dep]; !ok || day > tc.created {
					t.Errorf("dependency %q is not created before it is depended on", dep)
				}
			}
			for _, c := range tc.comments {
				if c.day < tc.created || c.day >= fixtureDays {
					t.Errorf("comment on day %d does not sit inside the task's life", c.day)
				}
				if !known[c.by] {
					t.Errorf("comment author %q is not seeded", c.by)
				}
			}
			steps, err := flowOf(tc)
			if err != nil {
				t.Fatalf("flowOf: %v", err)
			}
			last := tc.created
			for _, st := range steps {
				if st.day < last {
					t.Errorf("move to %q goes backwards in time", st.to)
				}
				if !known[st.by] {
					t.Errorf("move to %q is made by unseeded actor %q", st.to, st.by)
				}
				last = st.day
			}
			if tc.end == outcomeDone && last >= fixtureDays {
				t.Errorf("completion on day %d falls outside the window", last)
			}
		})
	}
}

// TestFixtureSpreadsTheWorkOut guards the properties the statistics screens
// need: completions on many days, by many actors, after many different waits.
func TestFixtureSpreadsTheWorkOut(t *testing.T) {
	days, actors, done := map[int]bool{}, map[string]bool{}, 0
	sameDay, longWait := 0, 0
	for _, seed := range tasks {
		if seed.end != outcomeDone {
			continue
		}
		done++
		days[seed.done] = true
		actors[seed.doneBy] = true
		switch wait := seed.done - seed.created; {
		case wait == 0:
			sameDay++
		case wait >= 21:
			longWait++
		}
	}
	for _, tc := range []struct {
		name string
		got  int
		want int
	}{
		{"tasks", len(tasks), 60},
		{"completions", done, 20},
		{"completion days", len(days), 20},
		{"completing actors", len(actors), 4},
		{"same day completions", sameDay, 3},
		{"completions after weeks", longWait, 3},
	} {
		if tc.got < tc.want {
			t.Errorf("%s = %d, want at least %d", tc.name, tc.got, tc.want)
		}
	}
}

// TestSeedPopulatesTheStatistics is the reason the seeder replays history
// through the service: the leaderboard and the lead times are derived from the
// audit entry written at the instant the task was completed.
func TestSeedPopulatesTheStatistics(t *testing.T) {
	tg := newTarget(t)
	summary := tg.seed(t, DefaultDays)
	stats := tg.stats(t, summary.Since)

	if stats.Completed != summary.Completed {
		t.Errorf("completed = %d, want the %d the seed reported", stats.Completed, summary.Completed)
	}
	if len(stats.PerDay) < 20 {
		t.Errorf("completions land on %d days, want a spread of at least 20", len(stats.PerDay))
	}
	if len(stats.TopActors) < 4 {
		t.Errorf("leaderboard names %d actors, want at least 4", len(stats.TopActors))
	}
	for _, a := range stats.TopActors {
		if a.Handle == "" {
			t.Errorf("actor %q has no handle, so the leaderboard shows an identifier", a.ActorID)
		}
	}
	if stats.MedianLeadTime.D() <= 0 {
		t.Errorf("median lead time = %s, want a positive duration", stats.MedianLeadTime)
	}
	if stats.SlowestLeadTime.D() <= stats.MedianLeadTime.D() {
		t.Errorf("slowest lead time %s does not exceed the median %s; the waits do not vary",
			stats.SlowestLeadTime, stats.MedianLeadTime)
	}
	if len(stats.Oldest) == 0 {
		t.Error("nothing is ageing, so the whole backlog is finished")
	}
}

// TestSeedWritesABacklogAPersonRecognises checks the attributes the boards and
// the list views render, which an empty seed would leave blank.
func TestSeedWritesABacklogAPersonRecognises(t *testing.T) {
	tg := newTarget(t)
	summary := tg.seed(t, DefaultDays)

	page, err := tg.svc.ListTasks(tg.ctx, core.TaskFilter{Page: core.Page{Limit: core.MaxPageLimit}})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(page.Tasks) != summary.Tasks {
		t.Fatalf("listed %d tasks, want the %d seeded", len(page.Tasks), summary.Tasks)
	}

	counts := map[string]int{}
	var tagged, assigned, dated, bodied, custom, overdue, upcoming int
	for _, task := range page.Tasks {
		counts[task.Status]++
		if len(task.Tags) > 0 {
			tagged++
		}
		if task.AssigneeActorID != "" {
			assigned++
		}
		if task.Body != "" {
			bodied++
		}
		if len(task.CustomFields) > 0 {
			custom++
		}
		if task.DueAt != nil {
			dated++
			if task.DueAt.Before(seedEnd) {
				overdue++
			} else {
				upcoming++
			}
		}
	}

	for _, tc := range []struct {
		name string
		got  int
		want int
	}{
		{"tasks with tags", tagged, summary.Tasks},
		{"tasks with an assignee", assigned, summary.Tasks},
		{"tasks with a body", bodied, summary.Tasks},
		{"tasks with a due date", dated, 40},
		{"past due tasks", overdue, 5},
		{"upcoming due tasks", upcoming, 5},
		{"tasks with custom fields", custom, 10},
		{"tasks still to do", counts["todo"], 5},
		{"tasks in progress", counts["doing"], 5},
		{"blocked tasks", counts["blocked"], 3},
		{"finished tasks", counts["done"], 20},
	} {
		if tc.got < tc.want {
			t.Errorf("%s = %d, want at least %d", tc.name, tc.got, tc.want)
		}
	}
	if summary.Comments < 25 || summary.Dependencies < 3 {
		t.Errorf("summary = %d comments and %d dependencies, want a populated detail view",
			summary.Comments, summary.Dependencies)
	}
}

// TestSeedFitsTheRequestedWindow checks that a shorter or longer window scales
// the fixture onto it rather than spilling out of either end.
func TestSeedFitsTheRequestedWindow(t *testing.T) {
	for _, tc := range []struct {
		name string
		days int
	}{
		{"a week", MinDays},
		{"a month", 30},
		{"the default", DefaultDays},
		{"half a year", 180},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tg := newTarget(t)
			summary := tg.seed(t, tc.days)
			want := seedEnd.Add(-time.Duration(tc.days) * dayDur)
			if !summary.Since.Equal(want) {
				t.Errorf("window starts at %s, want %s", summary.Since, want)
			}

			page, err := tg.svc.ListTasks(tg.ctx, core.TaskFilter{Page: core.Page{Limit: core.MaxPageLimit}})
			if err != nil {
				t.Fatalf("ListTasks: %v", err)
			}
			for _, task := range page.Tasks {
				if task.CreatedAt.Before(summary.Since) || task.CreatedAt.After(seedEnd) {
					t.Fatalf("task %s was created at %s, outside the window", task.Ref, task.CreatedAt)
				}
			}
			if stats := tg.stats(t, summary.Since); len(stats.PerDay) < 2 {
				t.Errorf("completions land on %d days, want more than one bar", len(stats.PerDay))
			}
		})
	}
}

// TestSeedRejectsAWindowNothingFitsIn keeps the guard on the options.
func TestSeedRejectsAWindowNothingFitsIn(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts Options
	}{
		{"too short", Options{Days: MinDays - 1, Now: seedEnd}},
		{"too long", Options{Days: MaxDays + 1, Now: seedEnd}},
		{"negative", Options{Days: -30, Now: seedEnd}},
		{"no end", Options{Days: DefaultDays}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tg := newTarget(t)
			_, err := Seed(tg.ctx, tg.svc, tg.clk, tg.admin, tc.opts)
			if !core.IsKind(err, core.KindInvalid) {
				t.Fatalf("Seed = %v, want an invalid-argument error", err)
			}
		})
	}
}

// TestHasDataSeesWorkButNotTheStarterLists is what stops a seed from landing
// on somebody's real tracker, and from refusing on a fresh installation.
func TestHasDataSeesWorkButNotTheStarterLists(t *testing.T) {
	tg := newTarget(t)
	populated, err := HasData(tg.ctx, tg.svc)
	if err != nil {
		t.Fatalf("HasData: %v", err)
	}
	if populated {
		t.Fatal("a fresh installation reports data, so a seed would always refuse")
	}

	tg.seed(t, DefaultDays)
	if populated, err = HasData(tg.ctx, tg.svc); err != nil {
		t.Fatalf("HasData: %v", err)
	}
	if !populated {
		t.Fatal("a seeded database reports no data, so a second seed would stack on it")
	}
}

// TestResetClearsTheTargetForAnotherSeed covers the --reset path end to end.
func TestResetClearsTheTargetForAnotherSeed(t *testing.T) {
	tg := newTarget(t)
	first := tg.seed(t, DefaultDays)

	if err := Reset(tg.ctx, tg.svc); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	populated, err := HasData(tg.ctx, tg.svc)
	if err != nil {
		t.Fatalf("HasData: %v", err)
	}
	if populated {
		t.Fatal("reset left work behind")
	}

	second := tg.seed(t, DefaultDays)
	if second.Tasks != first.Tasks || second.Projects != first.Projects {
		t.Errorf("second seed wrote %d tasks in %d projects, want %d in %d",
			second.Tasks, second.Projects, first.Tasks, first.Projects)
	}
}

// TestSeedNeedsAnActorToWriteAs guards the one argument with no sensible
// default: history written by nobody names nobody in the audit trail.
func TestSeedNeedsAnActorToWriteAs(t *testing.T) {
	tg := newTarget(t)
	_, err := Seed(tg.ctx, tg.svc, tg.clk, nil, Options{Days: DefaultDays, Now: seedEnd})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("Seed = %v, want an invalid-argument error", err)
	}
}

// TestUnreachableTargetIsReported checks that a store that cannot answer fails
// the command rather than reporting a seed that never happened.
func TestUnreachableTargetIsReported(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(*target) error
	}{
		{"has data", func(tg *target) error {
			_, err := HasData(tg.ctx, tg.svc)
			return err
		}},
		{"reset", func(tg *target) error { return Reset(tg.ctx, tg.svc) }},
		{"seed", func(tg *target) error {
			_, err := Seed(tg.ctx, tg.svc, tg.clk, tg.admin, Options{Days: DefaultDays, Now: seedEnd})
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tg := newTarget(t)
			if err := tg.store.Close(); err != nil {
				t.Fatalf("closing store: %v", err)
			}
			if err := tc.call(tg); err == nil {
				t.Fatal("a closed store reported success")
			}
		})
	}
}

// TestSeederRefusesAFixtureItCannotResolve covers the guards that turn a typo
// in the tables into an error naming it, rather than a nil dereference.
func TestSeederRefusesAFixtureItCannotResolve(t *testing.T) {
	s := &seeder{actors: map[string]*core.Actor{}, refs: map[string]string{}}
	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"unknown actor", func() error {
			_, err := s.as(context.Background(), "nobody")
			return err
		}},
		{"unknown task", func() error {
			_, err := s.ref("nothing")
			return err
		}},
		{"unknown outcome", func() error {
			_, err := flowOf(taskSeed{key: "x", end: outcome("sideways")})
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); !core.IsKind(err, core.KindInvalid) {
				t.Fatalf("got %v, want an invalid-argument error", err)
			}
		})
	}
}
