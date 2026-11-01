// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// calendarFixture is a tenant with one project, an administrator and a member
// who will own the feed.
type calendarFixture struct {
	l       *Local
	scope   core.TenantScope
	admin   context.Context
	member  context.Context
	actor   *core.Actor
	project *core.Project
}

func newCalendarFixture(t *testing.T) calendarFixture {
	t.Helper()
	l, adminCtx, scope, admin, project := newTaskFixture(t)
	user := seedUser(t, l, admin, "bob@example.com")
	member := &core.Actor{ID: user.ID, TenantID: scope.TenantID, Kind: core.ActorUser, Handle: "bob", Role: core.RoleMember}
	return calendarFixture{l: l, scope: scope, admin: adminCtx, member: taskContext(member), actor: member, project: project}
}

// feedCtx is what the HTTP router hands a feed read: a tenant and no actor.
func (f calendarFixture) feedCtx() context.Context {
	return core.WithTenant(context.Background(), f.scope)
}

func (f calendarFixture) task(t *testing.T, title string, due *time.Time, assignee string) *core.Task {
	t.Helper()
	return mustCreateTask(t, f.l, f.admin, core.CreateTaskInput{
		Title: title, ProjectRef: f.project.Key, DueAt: due, AssigneeActorID: assignee,
	})
}

func (f calendarFixture) finish(t *testing.T, task *core.Task) {
	t.Helper()
	for _, to := range []string{"doing", "done"} {
		if _, err := f.l.TransitionTask(f.admin, core.TaskRef{ID: task.ID}, core.TransitionInput{To: to}); err != nil {
			t.Fatalf("moving %s to %s: %v", task.Ref, to, err)
		}
	}
}

func dueOn(y int, m time.Month, d int) *time.Time {
	v := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return &v
}

func entryRefs(e *core.CalendarEntries) []string {
	out := make([]string, 0, len(e.Entries))
	for _, x := range e.Entries {
		out = append(out, x.Task.Ref)
	}
	return out
}

func TestDefaultFeedListsTheOwnersOpenDatedTasks(t *testing.T) {
	f := newCalendarFixture(t)
	mine := f.task(t, "mine", dueOn(2026, 11, 5), f.actor.ID)
	later := f.task(t, "later", dueOn(2026, 12, 1), f.actor.ID)
	finished := f.task(t, "finished", dueOn(2026, 11, 6), f.actor.ID)
	f.finish(t, finished)
	f.task(t, "undated", nil, f.actor.ID)
	f.task(t, "somebody else's", dueOn(2026, 11, 7), "")

	issued, err := f.l.CreateCalendarFeed(f.member, core.CalendarFeedInput{})
	if err != nil {
		t.Fatalf("creating the feed: %v", err)
	}
	if !strings.HasPrefix(issued.Token, auth.CalendarTokenPrefix) {
		t.Fatalf("issued token %q lacks the feed prefix", issued.Token)
	}
	got, err := f.l.ReadCalendarFeed(f.feedCtx(), issued.Token)
	if err != nil {
		t.Fatalf("reading the feed: %v", err)
	}
	if want := []string{later.Ref, mine.Ref}; !slices.Equal(entryRefs(got), want) {
		t.Fatalf("feed lists %v, want %v (latest due first)", entryRefs(got), want)
	}
	if got.OwnerHandle != "bob" || got.Entries[1].State != "Todo" || got.Entries[1].Category != core.CategoryTodo {
		t.Fatalf("feed answered owner %q and entry %+v", got.OwnerHandle, got.Entries[1])
	}
}

func TestFeedFilterSelectsFinishedWorkToo(t *testing.T) {
	f := newCalendarFixture(t)
	other := seedTaskProject(t, f.l, f.scope, "web")
	finished := f.task(t, "finished", dueOn(2026, 11, 6), "")
	f.finish(t, finished)
	elsewhere := mustCreateTask(t, f.l, f.admin, core.CreateTaskInput{Title: "web", ProjectRef: other.Key, DueAt: dueOn(2026, 11, 8)})

	issued, err := f.l.CreateCalendarFeed(f.member, core.CalendarFeedInput{Filter: "  project:infra  "})
	if err != nil {
		t.Fatalf("creating the feed: %v", err)
	}
	if issued.Filter != "project:infra" {
		t.Fatalf("filter stored as %q, want it trimmed", issued.Filter)
	}
	got, err := f.l.ReadCalendarFeed(f.feedCtx(), issued.Token)
	if err != nil {
		t.Fatal(err)
	}
	if refs := entryRefs(got); !slices.Equal(refs, []string{finished.Ref}) || slices.Contains(refs, elsewhere.Ref) {
		t.Fatalf("feed lists %v, want only %s", refs, finished.Ref)
	}
	if got.Entries[0].Category != core.CategoryDone {
		t.Fatalf("finished task read as category %q", got.Entries[0].Category)
	}

	if _, err := f.l.SetCalendarFeedFilter(f.member, core.CalendarFeedInput{Filter: "project:web"}); err != nil {
		t.Fatalf("changing the filter: %v", err)
	}
	got, err = f.l.ReadCalendarFeed(f.feedCtx(), issued.Token)
	if err != nil {
		t.Fatalf("the address stopped answering after a filter change: %v", err)
	}
	if refs := entryRefs(got); !slices.Equal(refs, []string{elsewhere.Ref}) {
		t.Fatalf("after the filter change the feed lists %v", refs)
	}
}

func TestFeedFilterIsCheckedWhenStored(t *testing.T) {
	f := newCalendarFixture(t)
	cases := map[string]core.Kind{
		"project:nosuch":            core.KindNotFound,
		"due:sometime":              core.KindInvalid,
		strings.Repeat("a", 1001):   core.KindInvalid,
		"status:nosuchstate tag:ok": core.KindNotFound,
	}
	for filter, kind := range cases {
		if _, err := f.l.CreateCalendarFeed(f.member, core.CalendarFeedInput{Filter: filter}); !core.IsKind(err, kind) {
			t.Errorf("filter %.20q: err = %v, want %s", filter, err, kind)
		}
	}
	if _, err := f.l.GetCalendarFeed(f.member); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("a refused filter left a feed behind: %v", err)
	}
}

func TestFeedLifecycle(t *testing.T) {
	f := newCalendarFixture(t)
	f.task(t, "mine", dueOn(2026, 11, 5), f.actor.ID)
	first, err := f.l.CreateCalendarFeed(f.member, core.CalendarFeedInput{Filter: "tag:ops"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.l.CreateCalendarFeed(f.member, core.CalendarFeedInput{}); !core.IsKind(err, core.KindConflict) {
		t.Fatalf("a second feed: err = %v, want conflict", err)
	}
	if _, err := f.l.ReadCalendarFeed(f.feedCtx(), first.Token); err != nil {
		t.Fatalf("the first feed stopped after a refused second: %v", err)
	}
	shown, err := f.l.GetCalendarFeed(f.member)
	if err != nil || shown.ID != first.ID || shown.Filter != "tag:ops" {
		t.Fatalf("GetCalendarFeed = %+v, %v", shown, err)
	}

	rotated, err := f.l.RotateCalendarFeed(f.member)
	if err != nil {
		t.Fatalf("rotating: %v", err)
	}
	if rotated.Token == first.Token || rotated.Filter != "tag:ops" {
		t.Fatalf("rotation answered %+v", rotated)
	}
	if _, err := f.l.ReadCalendarFeed(f.feedCtx(), first.Token); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("the rotated-away address: err = %v, want not found", err)
	}
	if _, err := f.l.ReadCalendarFeed(f.feedCtx(), rotated.Token); err != nil {
		t.Fatalf("the rotated address: %v", err)
	}

	if err := f.l.RevokeCalendarFeed(f.member); err != nil {
		t.Fatalf("revoking: %v", err)
	}
	if _, err := f.l.ReadCalendarFeed(f.feedCtx(), rotated.Token); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("a revoked address: err = %v, want not found", err)
	}
	if err := f.l.RevokeCalendarFeed(f.member); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("revoking nothing: err = %v, want not found", err)
	}
	if _, err := f.l.RotateCalendarFeed(f.member); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("rotating nothing: err = %v, want not found", err)
	}
	if _, err := f.l.CreateCalendarFeed(f.member, core.CalendarFeedInput{}); err != nil {
		t.Fatalf("a new feed after revoking: %v", err)
	}
}

func TestFeedStopsWhenItsOwnerCannotRead(t *testing.T) {
	cases := map[string]func(t *testing.T, f calendarFixture){
		"removed": func(t *testing.T, f calendarFixture) {
			if err := f.l.RemoveMember(f.admin, f.actor.ID); err != nil {
				t.Fatal(err)
			}
		},
		"disabled": func(t *testing.T, f calendarFixture) {
			yes := true
			if _, err := f.l.UpdateUser(f.admin, f.actor.ID, core.UpdateUserInput{Disabled: &yes}); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, stop := range cases {
		t.Run(name, func(t *testing.T) {
			f := newCalendarFixture(t)
			issued, err := f.l.CreateCalendarFeed(f.member, core.CalendarFeedInput{})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.l.ReadCalendarFeed(f.feedCtx(), issued.Token); err != nil {
				t.Fatalf("before: %v", err)
			}
			stop(t, f)
			if _, err := f.l.ReadCalendarFeed(f.feedCtx(), issued.Token); !core.IsKind(err, core.KindNotFound) {
				t.Fatalf("after: err = %v, want not found", err)
			}
		})
	}
}

func TestFeedReadRefusesWhatIsNotALiveFeed(t *testing.T) {
	f := newCalendarFixture(t)
	minted, _, err := auth.MintCalendarToken()
	if err != nil {
		t.Fatal(err)
	}
	for name, token := range map[string]string{
		"never issued": minted,
		"malformed":    "tix_cal_nope",
		"an api token": auth.TokenPrefix + strings.TrimPrefix(minted, auth.CalendarTokenPrefix),
	} {
		if _, err := f.l.ReadCalendarFeed(f.feedCtx(), token); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("%s: err = %v, want not found", name, err)
		}
	}
	if _, err := f.l.ReadCalendarFeed(context.Background(), minted); err == nil {
		t.Fatal("a read with no tenant answered")
	}
}

func TestFeedOfAnotherTenantIsNotFound(t *testing.T) {
	f := newCalendarFixture(t)
	issued, err := f.l.CreateCalendarFeed(f.member, core.CalendarFeedInput{})
	if err != nil {
		t.Fatal(err)
	}
	other := otherTenantActor(t, f.l)
	ctx := core.WithTenant(context.Background(), core.TenantScope{TenantID: other.TenantID})
	if _, err := f.l.ReadCalendarFeed(ctx, issued.Token); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("a feed read under another tenant's host: err = %v, want not found", err)
	}
}

func TestFeedNeedsAMember(t *testing.T) {
	f := newCalendarFixture(t)
	if _, err := f.l.CreateCalendarFeed(f.admin, core.CalendarFeedInput{}); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("an actor with no membership: err = %v, want invalid", err)
	}
	pinned := *f.actor
	pinned.ProjectID = f.project.ID
	if _, err := f.l.CreateCalendarFeed(taskContext(&pinned), core.CalendarFeedInput{}); !core.IsKind(err, core.KindForbidden) {
		t.Fatalf("a project-pinned token: err = %v, want forbidden", err)
	}
}

func TestFeedReaderHoldsNoMoreThanReading(t *testing.T) {
	f := newCalendarFixture(t)
	if _, err := f.l.UpdateUser(f.admin, f.actor.ID, core.UpdateUserInput{Role: ptrRole(core.RoleAdmin)}); err != nil {
		t.Fatal(err)
	}
	var reader *core.Actor
	if err := f.l.store.View(context.Background(), f.scope, func(tx store.Tx) error {
		var err error
		reader, err = feedReader(context.Background(), tx, f.scope.TenantID, f.actor.ID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if reader.Role != "" || !slices.Equal(reader.Scopes, calendarReadScopes) {
		t.Fatalf("an administrator's feed reads with role %q and scopes %v, want only %v",
			reader.Role, reader.Scopes, calendarReadScopes)
	}
	for _, s := range []core.Scope{core.ScopeTaskWrite, core.ScopeTokenAdmin, core.ScopeExport} {
		if reader.HasScope(s) {
			t.Errorf("the feed reader holds %s", s)
		}
	}
}

func ptrRole(r core.Role) *core.Role { return &r }

func TestFeedTrailNeverCarriesTheSecret(t *testing.T) {
	f := newCalendarFixture(t)
	issued, err := f.l.CreateCalendarFeed(f.member, core.CalendarFeedInput{})
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := f.l.RotateCalendarFeed(f.member)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.l.SetCalendarFeedFilter(f.member, core.CalendarFeedInput{Filter: "tag:x"}); err != nil {
		t.Fatal(err)
	}
	if err := f.l.RevokeCalendarFeed(f.member); err != nil {
		t.Fatal(err)
	}
	secrets := []string{issued.Token, rotated.Token, auth.HashToken(issued.Token), auth.HashToken(rotated.Token)}
	var audits []core.AuditEntry
	var events []core.Event
	if err := f.l.store.View(context.Background(), f.scope, func(tx store.Tx) error {
		var err error
		if audits, err = tx.ListAudit(context.Background(), core.AuditFilter{Page: core.Page{Limit: 100}}); err != nil {
			return err
		}
		events, err = tx.ReadEvents(context.Background(), 0, 100)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	actions := map[string]bool{}
	for _, a := range audits {
		actions[a.Action] = true
	}
	for _, want := range []string{auditCalendarCreate, auditCalendarRotate, auditCalendarFilter, auditCalendarRevoke} {
		if !actions[want] {
			t.Errorf("no %s audit entry was written", want)
		}
	}
	raw, err := json.Marshal(struct {
		A []core.AuditEntry
		E []core.Event
	}{audits, events})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range secrets {
		if strings.Contains(string(raw), s) {
			t.Fatalf("the trail carries a feed secret or its hash")
		}
	}
}

// A state name may finish work in one workflow and be a waypoint in another,
// so the default feed decides by each task's own workflow rather than by name.
func TestDefaultFeedJudgesFinishedByTheTasksOwnWorkflow(t *testing.T) {
	f := newCalendarFixture(t)
	def := func(terminal bool) core.WorkflowDefinition {
		return core.WorkflowDefinition{
			Initial: "todo",
			States: []core.State{
				{Key: "todo", Label: "Todo"},
				{Key: "shipped", Label: "Shipped", Terminal: terminal},
			},
			Transitions: []core.Transition{{From: "todo", To: "shipped"}},
		}
	}
	finishes := seedTaskProjectWith(t, f.l, f.scope, "fin", def(true))
	passes := seedTaskProjectWith(t, f.l, f.scope, "way", def(false))
	ship := func(p *core.Project) *core.Task {
		task := mustCreateTask(t, f.l, f.admin, core.CreateTaskInput{
			Title: p.Key, ProjectRef: p.Key, DueAt: dueOn(2026, 11, 9), AssigneeActorID: f.actor.ID,
		})
		if _, err := f.l.TransitionTask(f.admin, core.TaskRef{ID: task.ID}, core.TransitionInput{To: "shipped"}); err != nil {
			t.Fatalf("shipping %s: %v", task.Ref, err)
		}
		return task
	}
	ship(finishes)
	open := ship(passes)

	issued, err := f.l.CreateCalendarFeed(f.member, core.CalendarFeedInput{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.l.ReadCalendarFeed(f.feedCtx(), issued.Token)
	if err != nil {
		t.Fatal(err)
	}
	if refs := entryRefs(got); !slices.Equal(refs, []string{open.Ref}) {
		t.Fatalf("feed lists %v, want only the task whose shipped is a waypoint, %s", refs, open.Ref)
	}
}
