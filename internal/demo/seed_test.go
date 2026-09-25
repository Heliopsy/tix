// SPDX-License-Identifier: AGPL-3.0-or-later

package demo

import (
	"context"
	"fmt"
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

// signIn exchanges one of the seeded credentials for a session on the
// unauthenticated, tenant-pinned context a browser sign-in arrives on. It is
// the login the web interface performs, not a lookup of the row behind it: a
// users row with no usable password reads the same either way, which is how a
// demo nobody could open passed for a seeded one.
func (tg *target) signIn(email, password string) (*core.Session, error) {
	ctx := core.WithTenant(context.Background(), core.TenantScope{TenantID: tg.admin.TenantID})
	return tg.svc.Login(ctx, email, password)
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
	page, err := tg.svc.ListTasks(tg.ctx, core.TaskFilter{
		IncludeDeleted: true, Page: core.Page{Limit: core.MaxPageLimit},
	})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(page.Tasks) != 0 {
		t.Fatalf("reset left %d tasks behind", len(page.Tasks))
	}
	projects, _, err := tg.svc.ListProjects(tg.ctx, core.ProjectFilter{
		IncludeArchived: true, Page: core.Page{Limit: core.MaxPageLimit},
	})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != 0 {
		t.Fatalf("reset left %d projects behind", len(projects))
	}

	second := tg.seed(t, DefaultDays)
	if second.Tasks != first.Tasks || second.Projects != first.Projects {
		t.Errorf("second seed wrote %d tasks in %d projects, want %d in %d",
			second.Tasks, second.Projects, first.Tasks, first.Projects)
	}
}

// TestResetKeepsTheAccountsItCannotRecreate is why Reset stops at the work.
// A deleted user leaves the actor row that owns its handle, and no call
// deletes an actor, so a reset that removed the accounts left handles nothing
// could ever be credentialed under again.
func TestResetKeepsTheAccountsItCannotRecreate(t *testing.T) {
	tg := newTarget(t)
	tg.seed(t, DefaultDays)

	if err := Reset(tg.ctx, tg.svc); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	users, _, err := tg.svc.ListUsers(tg.ctx, core.Page{Limit: core.MaxPageLimit})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != len(people) {
		t.Fatalf("reset left %d accounts, want the %d the fixture wrote", len(users), len(people))
	}
}

// TestSeedLeavesCredentialsThatSignIn is the point of the whole fixture: the
// demonstration it writes can be opened. It signs in rather than reading the
// users table, because a users row with no password is exactly what the table
// showed while the browser answered every page with a redirect to the login.
func TestSeedLeavesCredentialsThatSignIn(t *testing.T) {
	tg := newTarget(t)
	summary := tg.seed(t, DefaultDays)

	if len(summary.Credentials) < 2 {
		t.Fatalf("seed reported %d sign-ins, want an admin and one other role",
			len(summary.Credentials))
	}
	roles := map[core.Role]bool{}
	for _, c := range summary.Credentials {
		roles[c.Role] = true
		session, err := tg.signIn(c.Email, c.Password)
		if err != nil {
			t.Fatalf("signing in as %s: %v", c.Email, err)
		}
		if session.Token == "" {
			t.Fatalf("sign-in as %s issued no session token", c.Email)
		}
		if _, err := tg.signIn(c.Email, c.Password+"-wrong"); err == nil {
			t.Fatalf("sign-in as %s accepted a password that is not the seeded one", c.Email)
		}
	}
	if !roles[core.RoleAdmin] {
		t.Error("no seeded account is an administrator, so the settings screens cannot be shown")
	}
	if len(roles) < 2 {
		t.Error("every seeded account holds the same role, so the demo shows one view of the product")
	}
}

// TestResetThenSeedLeavesAWorkingLogin covers the path that produced a demo
// nobody could open: a reseed over an existing demo.
func TestResetThenSeedLeavesAWorkingLogin(t *testing.T) {
	tg := newTarget(t)
	tg.seed(t, DefaultDays)
	if err := Reset(tg.ctx, tg.svc); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	summary := tg.seed(t, DefaultDays)

	if len(summary.Credentials) == 0 {
		t.Fatal("the reseed reported no sign-in at all")
	}
	for _, c := range summary.Credentials {
		if _, err := tg.signIn(c.Email, c.Password); err != nil {
			t.Fatalf("signing in as %s after a reseed: %v", c.Email, err)
		}
	}
}

// TestSeedPutsItsOwnPasswordOnAnAdoptedAccount checks that the credentials the
// summary reports are the ones the database holds, rather than whatever the
// previous seed left on the same handles.
func TestSeedPutsItsOwnPasswordOnAnAdoptedAccount(t *testing.T) {
	const second = "another-demo-password"
	tg := newTarget(t)
	first := tg.seed(t, DefaultDays)
	if err := Reset(tg.ctx, tg.svc); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	summary, err := Seed(tg.ctx, tg.svc, tg.clk, tg.admin,
		Options{Days: DefaultDays, Now: seedEnd, Password: second})
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}

	for _, c := range summary.Credentials {
		if c.Password != second {
			t.Errorf("summary reports %q for %s, want the password the seed was given", c.Password, c.Email)
		}
		if _, err := tg.signIn(c.Email, second); err != nil {
			t.Fatalf("signing in as %s with the new password: %v", c.Email, err)
		}
	}
	if _, err := tg.signIn(first.Credentials[0].Email, DefaultPassword); err == nil {
		t.Error("the password the first seed set still signs in after it was replaced")
	}
}

// TestSeedRefusesAFixtureWithNoWayIn guards the invariant the whole change
// exists for: a seed that leaves no account anybody can sign in as is a
// failure, not a demonstration.
func TestSeedRefusesAFixtureWithNoWayIn(t *testing.T) {
	restore := people
	stripped := append([]personSeed(nil), people...)
	for i := range stripped {
		stripped[i].signIn = false
	}
	people = stripped
	t.Cleanup(func() { people = restore })

	tg := newTarget(t)
	_, err := Seed(tg.ctx, tg.svc, tg.clk, tg.admin, Options{Days: DefaultDays, Now: seedEnd})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("Seed = %v, want an invalid-argument error", err)
	}
}

// TestSeedGivesAgentsNoPassword keeps the fleet authenticating the way the
// product says it does: an agent carries a token, and a password on one would
// demonstrate a route that does not exist.
func TestSeedGivesAgentsNoPassword(t *testing.T) {
	tg := newTarget(t)
	summary := tg.seed(t, DefaultDays)

	credentialed := map[string]bool{}
	for _, c := range summary.Credentials {
		credentialed[c.Handle] = true
	}
	for _, p := range people {
		if !p.agent {
			continue
		}
		if credentialed[p.handle] {
			t.Errorf("agent %q is reported as a sign-in", p.handle)
		}
		if _, err := tg.signIn(p.email, DefaultPassword); err == nil {
			t.Errorf("agent %q signs in with the demo password", p.handle)
		}
	}
}

// TestSeedRejectsAPasswordTheServiceWouldRefuse keeps the failure at the
// options rather than halfway through a replay. The service rejects a short
// password too, so the assertion that matters is where it stops: the projects
// are written before the first account, and a seed that got that far has
// already changed the database it was told to refuse.
func TestSeedRejectsAPasswordTheServiceWouldRefuse(t *testing.T) {
	tg := newTarget(t)
	_, err := Seed(tg.ctx, tg.svc, tg.clk, tg.admin,
		Options{Days: DefaultDays, Now: seedEnd, Password: "short"})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("Seed = %v, want an invalid-argument error", err)
	}
	listed, _, err := tg.svc.ListProjects(tg.ctx, core.ProjectFilter{
		IncludeArchived: true, Page: core.Page{Limit: core.MaxPageLimit},
	})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	seeded := map[string]bool{}
	for _, p := range projects {
		seeded[p.key] = true
	}
	for _, p := range listed {
		if seeded[p.Key] {
			t.Fatalf("project %q was written before the password was refused", p.Key)
		}
	}
}

// TestSeedRefusesAHandleHeldByAnAccountlessActor covers a database left by the
// reset this change removed: the handle is taken by an actor no account can be
// created under, and seeding on regardless would write a history whose authors
// nobody can sign in as.
func TestSeedRefusesAHandleHeldByAnAccountlessActor(t *testing.T) {
	tg := newTarget(t)
	tg.seed(t, DefaultDays)
	users, _, err := tg.svc.ListUsers(tg.ctx, core.Page{Limit: core.MaxPageLimit})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	for _, u := range users {
		if err := tg.svc.DeleteUser(tg.ctx, u.ID); err != nil {
			t.Fatalf("DeleteUser: %v", err)
		}
	}
	if err := Reset(tg.ctx, tg.svc); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	_, err = Seed(tg.ctx, tg.svc, tg.clk, tg.admin, Options{Days: DefaultDays, Now: seedEnd})
	if !core.IsKind(err, core.KindConflict) {
		t.Fatalf("Seed = %v, want a conflict naming the handle it cannot credential", err)
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

// TestSeedLeavesAClaimThatRecentlyExpired covers the state the demo could not
// show before: an agent took a task, stopped answering, and the sweeper left
// the evidence behind. Everything is judged against seedEnd, the instant the
// seeder was told the window ends at, so the assertion does not depend on how
// long the test itself takes to run.
func TestSeedLeavesAClaimThatRecentlyExpired(t *testing.T) {
	tg := newTarget(t)
	summary := tg.seed(t, DefaultDays)

	page, err := tg.svc.ListTasks(tg.ctx, core.TaskFilter{Page: core.Page{Limit: core.MaxPageLimit}})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	projects, _, err := tg.svc.ListProjects(tg.ctx, core.ProjectFilter{Page: core.Page{Limit: core.MaxPageLimit}})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	keys := map[string]string{}
	for _, p := range projects {
		keys[p.ID] = p.Key
	}

	var expired, repeated int
	var worst core.Task
	for _, task := range page.Tasks {
		if !task.ClaimExpiredRecently(seedEnd) {
			continue
		}
		expired++
		if task.ClaimCount > worst.ClaimCount {
			worst = task
		}
		if task.ClaimCount > 1 {
			repeated++
		}
		if task.LeaseExpiredByActorID == "" {
			t.Errorf("task %s reports an expiry with no holder, so the detail page names nobody", task.Ref)
		}
		if keys[task.ProjectID] != "agents" {
			t.Errorf("task %s is in project %q, want the dropped claims in the agent fleet",
				task.Ref, keys[task.ProjectID])
		}
	}

	if expired != summary.Abandoned {
		t.Errorf("%d tasks read as recently expired, want the %d the seed reported", expired, summary.Abandoned)
	}
	if expired < 1 {
		t.Fatal("no task reads as a recently expired claim, so the badge cannot be seen in the demo")
	}
	if repeated < 1 {
		t.Fatal("no expired claim was taken more than once, so the claim count beside the badge never renders")
	}
	if expired > summary.Tasks/10 {
		t.Errorf("%d of %d tasks have a dropped claim, which reads as a broken fleet rather than a backlog",
			expired, summary.Tasks)
	}
	if worst.ClaimedAtTime(seedEnd) {
		t.Errorf("task %s still holds a live lease, so it renders as held rather than expired", worst.Ref)
	}
	if worst.LeaseExpiredAt.After(seedEnd) {
		t.Errorf("task %s expired at %s, after the end of the window", worst.Ref, worst.LeaseExpiredAt)
	}

	entries, _, err := tg.svc.ListAudit(tg.ctx, core.AuditFilter{
		SubjectType: "task", SubjectID: worst.ID, Page: core.Page{Limit: core.MaxPageLimit},
	})
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	actions := map[string]int{}
	for _, e := range entries {
		actions[e.Action]++
	}
	if actions["task.claim"] != worst.ClaimCount || actions["task.lease_expire"] != worst.ClaimCount {
		t.Errorf("task %s counts %d claims but its history holds %d claims and %d expiries; "+
			"the row and the trail disagree",
			worst.Ref, worst.ClaimCount, actions["task.claim"], actions["task.lease_expire"])
	}
}

// TestSeedDropsTheStarterListsItDidNotWrite keeps the Projects screen, the
// project filter and the tenant tree to the lists the fixture works out of.
func TestSeedDropsTheStarterListsItDidNotWrite(t *testing.T) {
	tg := newTarget(t)
	summary := tg.seed(t, DefaultDays)

	listed, _, err := tg.svc.ListProjects(tg.ctx, core.ProjectFilter{
		IncludeArchived: true, Page: core.Page{Limit: core.MaxPageLimit},
	})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(listed) != summary.Projects {
		t.Errorf("tenant holds %d projects, want the %d the fixture wrote", len(listed), summary.Projects)
	}
	seeded := map[string]bool{}
	for _, p := range projects {
		seeded[p.key] = true
	}
	for _, p := range listed {
		if !seeded[p.Key] {
			t.Errorf("project %q is not part of the fixture, so the demo shows an empty list", p.Key)
		}
	}
}

// TestSeedRefusesAnAbandonedEntryItCannotReplay keeps the guard on the one
// field of the abandoned table that has no sensible reading: a task dropped
// fewer than once.
func TestSeedRefusesAnAbandonedEntryItCannotReplay(t *testing.T) {
	for _, tc := range []struct {
		name  string
		entry abandonedSeed
	}{
		{"never claimed", abandonedSeed{key: "agents-audit", holder: "scout", claims: 0, ago: time.Hour}},
		{"unknown holder", abandonedSeed{key: "agents-audit", holder: "nobody", claims: 1, ago: time.Hour}},
		{"unknown task", abandonedSeed{key: "nothing", holder: "scout", claims: 1, ago: time.Hour}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			restore := abandoned
			abandoned = []abandonedSeed{tc.entry}
			t.Cleanup(func() { abandoned = restore })

			tg := newTarget(t)
			_, err := Seed(tg.ctx, tg.svc, tg.clk, tg.admin, Options{Days: DefaultDays, Now: seedEnd})
			if !core.IsKind(err, core.KindInvalid) {
				t.Fatalf("Seed = %v, want an invalid-argument error", err)
			}
		})
	}
}

// statsWindowDays are the windows the statistics screen offers. The chart is
// only as honest as the narrowest of them, so every property of the spread is
// asserted at each one rather than over the whole seeded window.
var statsWindowDays = []int{7, 14, 30, 90}

// TestSeedCompletionsAreUneven guards the property the chart needs and the
// seed once failed: not that completions exist, but that their per-day counts
// differ. One completion on every day draws a panel of identical full-length
// bars, because every value is also the maximum the template scales against.
func TestSeedCompletionsAreUneven(t *testing.T) {
	tg := newTarget(t)
	tg.seed(t, DefaultDays)

	for _, days := range statsWindowDays {
		t.Run(fmt.Sprintf("%d days", days), func(t *testing.T) {
			stats := tg.stats(t, seedEnd.Add(-time.Duration(days)*dayDur))
			counts := map[int]int{}
			tallest, total := 0, 0
			for _, d := range stats.PerDay {
				counts[d.Completed]++
				total += d.Completed
				if d.Completed > tallest {
					tallest = d.Completed
				}
			}
			if len(stats.PerDay) < 3 {
				t.Fatalf("%d completions land on %d days, which is too few bars to read as a chart",
					total, len(stats.PerDay))
			}
			if len(counts) < 2 {
				t.Errorf("all %d days carry %d completion(s), so every bar is drawn at full length",
					len(stats.PerDay), tallest)
			}
			if tallest < 2 {
				t.Errorf("the busiest day of the window carries %d completion, so nothing stands out",
					tallest)
			}
		})
	}
}

// TestFixtureKeepsABurstOnOneDay guards what makes the bursts survive the
// clock. A fixture day is offset from the instant the seed ran, so it spans
// two UTC dates; the chart counts UTC dates. Completions sharing a fixture day
// therefore share an hour, or the hour somebody happened to run the seed at
// decides whether a burst of three renders as three or as two and one.
func TestFixtureKeepsABurstOnOneDay(t *testing.T) {
	hours := map[int]int{}
	for _, seed := range tasks {
		if seed.end != outcomeDone {
			continue
		}
		if hour, seen := hours[seed.done]; seen && hour != seed.doneHour {
			t.Errorf("day %d completes at both %02d:00 and %02d:00, so the burst splits across "+
				"two UTC dates whenever the seed runs late enough in the day",
				seed.done, hour, seed.doneHour)
			continue
		}
		hours[seed.done] = seed.doneHour
	}
}

// TestSeedPutsADroppedClaimOnTheFirstPage covers the screenshot the demo could
// not take: the expired-claim badge and the pager on the same page. Both
// dropped claims used to sit on low-priority work, so the default urgency
// ordering left them past row fifty, on the last page, where there is no Next
// to show beside them.
func TestSeedPutsADroppedClaimOnTheFirstPage(t *testing.T) {
	tg := newTarget(t)
	summary := tg.seed(t, DefaultDays)

	page, err := tg.svc.ListTasks(tg.ctx, core.TaskFilter{Page: core.Page{Limit: core.DefaultPageLimit}})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(page.Tasks) != core.DefaultPageLimit {
		t.Fatalf("the first page holds %d of the %d seeded tasks, want a full page of %d",
			len(page.Tasks), summary.Tasks, core.DefaultPageLimit)
	}
	if page.NextCursor == "" {
		t.Error("the first page is the whole listing, so no pager renders beside the badge")
	}

	at := -1
	for i, task := range page.Tasks {
		if task.ClaimExpiredRecently(seedEnd) {
			at = i + 1
			break
		}
	}
	if at < 0 {
		t.Fatalf("none of the first %d rows of the default ordering carries an expired claim, "+
			"so the headline listing shows the pager or the badge but never both",
			core.DefaultPageLimit)
	}
	t.Logf("first dropped claim sits at row %d of %d", at, core.DefaultPageLimit)
}
