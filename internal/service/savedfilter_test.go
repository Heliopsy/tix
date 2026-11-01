// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func str(s string) *string { return &s }

func filterNames(t *testing.T, l *Local, ctx context.Context) []string {
	t.Helper()
	list, err := l.ListSavedFilters(ctx)
	if err != nil {
		t.Fatalf("listing saved filters: %v", err)
	}
	out := make([]string, 0, len(list))
	for _, f := range list {
		out = append(out, f.Name)
	}
	return out
}

func TestSavedFiltersArePersonalUnlessShared(t *testing.T) {
	l, admin, scope, _, _ := newTaskFixture(t)
	bob := seedTaskActor(t, l, scope, "bob", core.RoleMember, core.ScopeTaskRead, core.ScopeTaskWrite)
	cat := seedTaskActor(t, l, scope, "cat", core.RoleMember, core.ScopeTaskRead, core.ScopeTaskWrite)
	bobCtx, catCtx := taskContext(bob), taskContext(cat)

	ops, err := l.CreateSavedFilter(bobCtx, core.SavedFilterInput{Name: " Ops ", Query: "tag:ops", View: "Board", Sort: "due_at"})
	if err != nil {
		t.Fatalf("saving: %v", err)
	}
	if ops.Name != "Ops" || ops.View != core.FilterViewBoard || ops.Position != 1 || !ops.Mine || !ops.Editable || ops.OwnerID != bob.ID {
		t.Fatalf("save answered %+v", ops)
	}
	if ev := lastEvent(t, l, scope); ev.Type != core.EventFilterSaved || ev.SubjectType != "filter" || ev.Payload["name"] != "Ops" {
		t.Errorf("save emitted %s %s %v", ev.Type, ev.SubjectType, ev.Payload)
	}
	if _, err := l.CreateSavedFilter(bobCtx, core.SavedFilterInput{Name: "team", Query: "status:todo", Shared: true}); err != nil {
		t.Fatal(err)
	}
	if got := filterNames(t, l, catCtx); len(got) != 1 || got[0] != "team" {
		t.Fatalf("another reader lists %v, want only the shared one", got)
	}
	if _, err := l.GetSavedFilter(catCtx, "Ops"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("another reader read a private filter: %v", err)
	}
	shared, err := l.GetSavedFilter(catCtx, "TEAM")
	if err != nil || shared.Mine || shared.Editable {
		t.Fatalf("the shared filter reads as %+v, %v", shared, err)
	}
	if _, err := l.UpdateSavedFilter(catCtx, "team", core.SavedFilterUpdate{Name: str("mine now")}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("a member renamed somebody's shared filter: %v", err)
	}
	if err := l.DeleteSavedFilter(catCtx, "team"); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("a member deleted somebody's shared filter: %v", err)
	}
	asAdmin, err := l.GetSavedFilter(admin, "team")
	if err != nil || !asAdmin.Editable {
		t.Fatalf("an administrator reads the shared filter as %+v, %v", asAdmin, err)
	}
	if _, err := l.UpdateSavedFilter(admin, "team", core.SavedFilterUpdate{Name: str("Team")}); err != nil {
		t.Fatalf("an administrator could not rename a shared filter: %v", err)
	}
	if _, err := l.CreateSavedFilter(catCtx, core.SavedFilterInput{Name: "Team"}); err != nil {
		t.Fatalf("a name is unique per owner, not per tenant: %v", err)
	}
	if got, err := l.GetSavedFilter(catCtx, "team"); err != nil || !got.Mine {
		t.Errorf("a reader's own name wins over a shared one: %+v, %v", got, err)
	}
	if err := l.DeleteSavedFilter(admin, "Team"); err != nil {
		t.Fatalf("an administrator could not delete a shared filter: %v", err)
	}
	if ev := lastEvent(t, l, scope); ev.Type != core.EventFilterDeleted {
		t.Errorf("delete emitted %s", ev.Type)
	}
	if got := filterNames(t, l, bobCtx); len(got) != 1 || got[0] != "Ops" {
		t.Errorf("after the delete bob lists %v", got)
	}
}

func TestSavedFilterRefusals(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	if _, err := l.CreateSavedFilter(ctx, core.SavedFilterInput{Name: "Ops", Query: "tag:ops"}); err != nil {
		t.Fatal(err)
	}
	none := taskContext(&core.Actor{ID: "nobody", TenantID: scope.TenantID, Kind: core.ActorAgent,
		Scopes: []core.Scope{core.ScopeProjectRead}})
	long := make([]byte, core.MaxSavedFilterName+1)
	for i := range long {
		long[i] = 'x'
	}
	cases := []struct {
		name string
		ctx  context.Context
		in   core.SavedFilterInput
		kind core.Kind
	}{
		{"no name", ctx, core.SavedFilterInput{Name: "  "}, core.KindInvalid},
		{"a long name", ctx, core.SavedFilterInput{Name: string(long)}, core.KindInvalid},
		{"an unknown view", ctx, core.SavedFilterInput{Name: "x", View: "calendar"}, core.KindInvalid},
		{"an unknown sort", ctx, core.SavedFilterInput{Name: "x", Sort: "colour"}, core.KindInvalid},
		{"an unreadable filter", ctx, core.SavedFilterInput{Name: "x", Query: "nosuchkey:1"}, core.KindInvalid},
		{"a taken name", ctx, core.SavedFilterInput{Name: "OPS"}, core.KindConflict},
		{"no task:read", none, core.SavedFilterInput{Name: "x"}, core.KindForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := l.CreateSavedFilter(tc.ctx, tc.in); !core.IsKind(err, tc.kind) {
				t.Fatalf("err = %v, want %s", err, tc.kind)
			}
		})
	}
	if _, err := l.UpdateSavedFilter(ctx, "Ops", core.SavedFilterUpdate{}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("an empty update answered %v", err)
	}
	if _, err := l.UpdateSavedFilter(ctx, "Ops", core.SavedFilterUpdate{Query: str("status:")}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("an unreadable expression was saved: %v", err)
	}
}

func TestSavedFilterUpdateWritesOnceAndOrderMovesOnlyYourOwn(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	for _, name := range []string{"a", "b", "c"} {
		if _, err := l.CreateSavedFilter(ctx, core.SavedFilterInput{Name: name}); err != nil {
			t.Fatal(err)
		}
	}
	events, audits := countRows(t, l, scope)
	got, err := l.UpdateSavedFilter(ctx, "b", core.SavedFilterUpdate{Name: str("b")})
	if err != nil || got.Name != "b" {
		t.Fatalf("a no-op update answered %+v, %v", got, err)
	}
	if e, a := countRows(t, l, scope); e != events || a != audits {
		t.Errorf("an update that changed nothing wrote %d events and %d audits", e-events, a-audits)
	}
	got, err = l.UpdateSavedFilter(ctx, "b", core.SavedFilterUpdate{Query: str("is:starred"), Shared: yes(), View: str("lanes")})
	if err != nil || got.Query != "is:starred" || !got.Shared || got.View != core.FilterViewLanes {
		t.Fatalf("update answered %+v, %v", got, err)
	}
	if e, a := countRows(t, l, scope); e != events+1 || a != audits+1 {
		t.Errorf("an update wrote %d events and %d audits, want one of each", e-events, a-audits)
	}
	if _, err := l.UpdateSavedFilter(ctx, "b", core.SavedFilterUpdate{Name: str("A")}); !core.IsKind(err, core.KindConflict) {
		t.Errorf("a rename onto a taken name answered %v", err)
	}

	ordered, err := l.OrderSavedFilters(ctx, core.SavedFilterOrderInput{Filters: []string{"c", "a"}})
	if err != nil {
		t.Fatalf("ordering: %v", err)
	}
	if len(ordered) != 3 || ordered[0].Name != "c" || ordered[1].Name != "a" || ordered[2].Name != "b" || ordered[2].Position != 3 {
		t.Fatalf("order answered %+v", ordered)
	}
	if names := filterNames(t, l, ctx); names[0] != "c" || names[1] != "a" || names[2] != "b" {
		t.Errorf("listed in %v after ordering", names)
	}
	events, _ = countRows(t, l, scope)
	if _, err := l.OrderSavedFilters(ctx, core.SavedFilterOrderInput{Filters: []string{"c", "a", "b"}}); err != nil {
		t.Fatal(err)
	}
	if e, _ := countRows(t, l, scope); e != events {
		t.Errorf("an order that changed nothing wrote %d events", e-events)
	}
	bob := taskContext(seedTaskActor(t, l, scope, "bob", core.RoleMember, core.ScopeTaskRead))
	for _, bad := range [][]string{nil, {"a", "A"}, {"nope"}} {
		if _, err := l.OrderSavedFilters(ctx, core.SavedFilterOrderInput{Filters: bad}); err == nil {
			t.Errorf("order %v was accepted", bad)
		}
	}
	if _, err := l.OrderSavedFilters(bob, core.SavedFilterOrderInput{Filters: []string{"b"}}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("another reader ordered a shared filter they do not own: %v", err)
	}
}

func TestAssigneeMeIsTheCaller(t *testing.T) {
	l, ctx, scope, me, _ := newTaskFixture(t)
	bob := seedTaskActor(t, l, scope, "bob", core.RoleMember, core.ScopeAll)
	mine := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "mine", AssigneeActorID: me.ID})
	mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "his", AssigneeActorID: bob.ID})
	f := core.TaskFilter{AssigneeIDs: []string{"Me"}}
	page, err := l.ListTasks(ctx, f)
	if err != nil {
		t.Fatalf("listing assignee:me: %v", err)
	}
	if len(page.Tasks) != 1 || page.Tasks[0].ID != mine.ID {
		t.Fatalf("assignee:me listed %d tasks", len(page.Tasks))
	}
	if f.AssigneeIDs[0] != "Me" {
		t.Errorf("the caller's filter was rewritten to %v", f.AssigneeIDs)
	}
	page, err = l.ListTasks(taskContext(bob), core.TaskFilter{Exclude: core.TaskExclude{AssigneeIDs: []string{"me"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Tasks) != 1 || page.Tasks[0].ID != mine.ID {
		t.Errorf("-assignee:me for bob listed %d tasks", len(page.Tasks))
	}
}
