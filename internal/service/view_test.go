// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

func yes() *bool { v := true; return &v }
func no() *bool  { v := false; return &v }

func mustPrefs(t *testing.T, l *Local, ctx context.Context, view string) map[string]core.GroupPref {
	t.Helper()
	prefs, err := l.ListGroupPrefs(ctx, view)
	if err != nil {
		t.Fatalf("listing %s: %v", view, err)
	}
	out := map[string]core.GroupPref{}
	for _, p := range prefs {
		if p.View != view && p.View != "project:infra" {
			t.Errorf("listed view = %q, want %q", p.View, view)
		}
		out[p.Group] = p
	}
	return out
}

func storedPrefs(t *testing.T, l *Local, scope core.TenantScope, actorID, view string) []core.GroupPref {
	t.Helper()
	var out []core.GroupPref
	if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		var err error
		out, err = tx.GroupPrefs(context.Background(), actorID, view)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestGroupPrefsArePersonalAndNamedByKey(t *testing.T) {
	l, ctx, scope, me, project := newTaskFixture(t)
	other := taskContext(seedTaskActor(t, l, scope, "bob", core.RoleMember, core.ScopeAll))

	if _, err := l.SetGroupPref(ctx, core.ViewBoard, "done", core.GroupPrefInput{Hidden: yes()}); err != nil {
		t.Fatalf("hiding done: %v", err)
	}
	got, err := l.SetGroupPref(ctx, core.ViewLanes, "INFRA", core.GroupPrefInput{Starred: yes(), Collapsed: yes()})
	if err != nil {
		t.Fatalf("starring the infra lane: %v", err)
	}
	if got.Group != "infra" || !got.Starred || !got.Collapsed || got.View != core.ViewLanes {
		t.Fatalf("set answered %+v", got)
	}
	if _, err := l.SetGroupPref(ctx, "project:infra", "doing", core.GroupPrefInput{Collapsed: yes()}); err != nil {
		t.Fatalf("collapsing a project board column: %v", err)
	}

	if p := mustPrefs(t, l, ctx, core.ViewBoard); !p["done"].Hidden || len(p) != 1 {
		t.Errorf("board = %+v", p)
	}
	if p := mustPrefs(t, l, ctx, core.ViewLanes); !p["infra"].Starred {
		t.Errorf("lanes = %+v", p)
	}
	if p := mustPrefs(t, l, ctx, "project:infra"); !p["doing"].Collapsed {
		t.Errorf("project board = %+v", p)
	}
	if p := mustPrefs(t, l, other, core.ViewBoard); len(p) != 0 {
		t.Errorf("another reader sees %+v", p)
	}

	lanes := storedPrefs(t, l, scope, me.ID, core.ViewLanes)
	if len(lanes) != 1 || lanes[0].Group != project.ID {
		t.Errorf("a lane is stored as %+v, want the project id %s", lanes, project.ID)
	}
	if rows := storedPrefs(t, l, scope, me.ID, core.ProjectView(project.ID)); len(rows) != 1 {
		t.Errorf("a project board is stored under its id: %+v", rows)
	}
}

func TestGroupPrefWritesOnceAndForgetsAPlainGroup(t *testing.T) {
	l, ctx, scope, me, _ := newTaskFixture(t)
	steps := []struct {
		name  string
		in    core.GroupPrefInput
		write bool
		rows  int
	}{
		{"hide", core.GroupPrefInput{Hidden: yes()}, true, 1},
		{"hide again", core.GroupPrefInput{Hidden: yes()}, false, 1},
		{"show", core.GroupPrefInput{Hidden: no()}, true, 0},
		{"show again", core.GroupPrefInput{Hidden: no()}, false, 0},
	}
	for _, step := range steps {
		events, audits := countRows(t, l, scope)
		if _, err := l.SetGroupPref(ctx, core.ViewBoard, "done", step.in); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		afterEvents, afterAudits := countRows(t, l, scope)
		wrote := afterEvents == events+1 && afterAudits == audits+1
		silent := afterEvents == events && afterAudits == audits
		if step.write && !wrote || !step.write && !silent {
			t.Fatalf("%s: events %d -> %d, audits %d -> %d, want a write = %v",
				step.name, events, afterEvents, audits, afterAudits, step.write)
		}
		if n := len(storedPrefs(t, l, scope, me.ID, core.ViewBoard)); n != step.rows {
			t.Fatalf("%s: %d rows, want %d", step.name, n, step.rows)
		}
		if !step.write {
			continue
		}
		if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
			evs, err := tx.ReadEvents(context.Background(), 0, 1000)
			if err != nil {
				return err
			}
			last := evs[len(evs)-1]
			if last.Type != core.EventViewArranged || last.Payload["view"] != core.ViewBoard {
				t.Errorf("%s: event = %s %v", step.name, last.Type, last.Payload)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestGroupPrefRefusals(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	pinned := taskContext(&core.Actor{ID: "pinned", TenantID: scope.TenantID, Kind: core.ActorAgent,
		ProjectID: "elsewhere", Scopes: []core.Scope{core.ScopeAll}})
	none := taskContext(&core.Actor{ID: "nobody", TenantID: scope.TenantID, Kind: core.ActorAgent,
		Scopes: []core.Scope{core.ScopeProjectRead}})
	viewer := taskContext(seedTaskActor(t, l, scope, "vee", core.RoleViewer, core.ScopeTaskRead))
	cases := []struct {
		name        string
		ctx         context.Context
		view, group string
		in          core.GroupPrefInput
		kind        core.Kind
	}{
		{"unknown view", ctx, "calendar", "todo", core.GroupPrefInput{Hidden: yes()}, core.KindInvalid},
		{"empty project view", ctx, "project:", "todo", core.GroupPrefInput{Hidden: yes()}, core.KindInvalid},
		{"unknown project view", ctx, "project:nope", "todo", core.GroupPrefInput{Hidden: yes()}, core.KindNotFound},
		{"a state the workflow lacks", ctx, "project:infra", "shipped", core.GroupPrefInput{Hidden: yes()}, core.KindInvalid},
		{"a state no workflow has", ctx, core.ViewBoard, "shipped", core.GroupPrefInput{Hidden: yes()}, core.KindInvalid},
		{"an unknown lane", ctx, core.ViewLanes, "nope", core.GroupPrefInput{Hidden: yes()}, core.KindNotFound},
		{"nothing to change", ctx, core.ViewBoard, "done", core.GroupPrefInput{}, core.KindInvalid},
		{"no group", ctx, core.ViewBoard, " ", core.GroupPrefInput{Hidden: yes()}, core.KindInvalid},
		{"a pinned token", pinned, core.ViewBoard, "done", core.GroupPrefInput{Hidden: yes()}, core.KindForbidden},
		{"no task:read", none, core.ViewBoard, "done", core.GroupPrefInput{Hidden: yes()}, core.KindForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := l.SetGroupPref(tc.ctx, tc.view, tc.group, tc.in)
			if !core.IsKind(err, tc.kind) {
				t.Fatalf("err = %v, want %s", err, tc.kind)
			}
		})
	}
	if _, err := l.SetGroupPref(viewer, core.ViewBoard, "done", core.GroupPrefInput{Hidden: yes()}); err != nil {
		t.Fatalf("a reader could not arrange: %v", err)
	}
}

func TestOrderGroupsWritesPositionsAndResetForgets(t *testing.T) {
	l, ctx, scope, me, _ := newTaskFixture(t)
	seedTaskProject(t, l, scope, "ops")
	if _, err := l.SetGroupPref(ctx, core.ViewLanes, "ops", core.GroupPrefInput{Starred: yes()}); err != nil {
		t.Fatal(err)
	}
	got, err := l.OrderGroups(ctx, core.ViewLanes, core.GroupOrderInput{Groups: []string{"ops", "infra"}})
	if err != nil {
		t.Fatalf("ordering: %v", err)
	}
	if len(got) != 2 || got[0].Group != "ops" || got[0].Position != 1 || !got[0].Starred || got[1].Position != 2 {
		t.Fatalf("order answered %+v", got)
	}
	events, _ := countRows(t, l, scope)
	if _, err := l.OrderGroups(ctx, core.ViewLanes, core.GroupOrderInput{Groups: []string{"ops", "infra"}}); err != nil {
		t.Fatal(err)
	}
	if after, _ := countRows(t, l, scope); after != events {
		t.Errorf("an order that changed nothing wrote %d events", after-events)
	}
	for _, bad := range [][]string{nil, {"ops", "OPS"}, {"ops", "nope"}} {
		if _, err := l.OrderGroups(ctx, core.ViewLanes, core.GroupOrderInput{Groups: bad}); err == nil {
			t.Errorf("order %v was accepted", bad)
		}
	}

	if err := l.ResetGroupPrefs(ctx, core.ViewLanes, "infra"); err != nil {
		t.Fatalf("resetting one lane: %v", err)
	}
	if p := mustPrefs(t, l, ctx, core.ViewLanes); len(p) != 1 || !p["ops"].Starred {
		t.Fatalf("after resetting infra: %+v", p)
	}
	if err := l.ResetGroupPrefs(ctx, core.ViewLanes, ""); err != nil {
		t.Fatalf("resetting the view: %v", err)
	}
	if n := len(storedPrefs(t, l, scope, me.ID, core.ViewLanes)); n != 0 {
		t.Fatalf("%d rows after a reset", n)
	}
	events, _ = countRows(t, l, scope)
	if err := l.ResetGroupPrefs(ctx, core.ViewLanes, ""); err != nil {
		t.Fatal(err)
	}
	if after, _ := countRows(t, l, scope); after != events {
		t.Errorf("a reset of nothing wrote %d events", after-events)
	}
}
