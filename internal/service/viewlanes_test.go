// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestLaneGroupingsAreArrangedUnderTheirOwnViews(t *testing.T) {
	l, ctx, scope, me, _ := newTaskFixture(t)
	lena := seedTaskActor(t, l, scope, "lena", core.RoleMember, core.ScopeAll)
	mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "x", Tags: []string{"Ops"}})
	assignee := core.LanesView(core.LanesByAssignee)

	cases := []struct {
		name, view, group, shown, stored string
	}{
		{"a person by handle", assignee, "lena", "lena", lena.ID},
		{"the unassigned lane", assignee, core.GroupNone, core.GroupNone, core.GroupNone},
		{"a priority", core.LanesView(core.LanesByPriority), "High", "high", "high"},
		{"a tag as the tenant spells it", core.LanesView(core.LanesByTag), "ops", "Ops", "Ops"},
		{"the no tag lane", core.LanesView(core.LanesByTag), core.GroupNone, core.GroupNone, core.GroupNone},
		{"a deadline bucket", core.LanesView(core.LanesByDue), "Overdue", "overdue", "overdue"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := l.SetGroupPref(ctx, tc.view, tc.group, core.GroupPrefInput{Starred: yes()})
			if err != nil {
				t.Fatalf("starring %s on %s: %v", tc.group, tc.view, err)
			}
			if got.View != tc.view || got.Group != tc.shown {
				t.Errorf("set answered %s/%s, want %s/%s", got.View, got.Group, tc.view, tc.shown)
			}
			if !mustPrefs(t, l, ctx, tc.view)[tc.shown].Starred {
				t.Errorf("%s is not listed starred on %s", tc.shown, tc.view)
			}
			found := false
			for _, p := range storedPrefs(t, l, scope, me.ID, tc.view) {
				found = found || p.Group == tc.stored
			}
			if !found {
				t.Errorf("%s is not stored as %q", tc.group, tc.stored)
			}
		})
	}
	if _, err := l.SetGroupPref(ctx, assignee, lena.ID, core.GroupPrefInput{Collapsed: yes()}); err != nil {
		t.Fatal(err)
	}
	if p := mustPrefs(t, l, ctx, assignee)["lena"]; !p.Starred || !p.Collapsed {
		t.Errorf("naming lena by id changed another preference: %+v", p)
	}
	if p := mustPrefs(t, l, ctx, core.ViewLanes); len(p) != 0 {
		t.Errorf("the project lanes picked up %+v", p)
	}
}

func TestLaneGroupingRefusals(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	cases := []struct {
		name, view, group string
		kind              core.Kind
	}{
		{"an unknown grouping", "lanes:colour", "red", core.KindInvalid},
		{"project spelled as a prefix", "lanes:project", "infra", core.KindInvalid},
		{"an unknown priority", core.LanesView(core.LanesByPriority), "urgent", core.KindInvalid},
		{"an unknown person", core.LanesView(core.LanesByAssignee), "nobody", core.KindNotFound},
		{"an unknown tag", core.LanesView(core.LanesByTag), "nope", core.KindInvalid},
		{"an unknown bucket", core.LanesView(core.LanesByDue), "soon", core.KindInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := l.SetGroupPref(ctx, tc.view, tc.group, core.GroupPrefInput{Hidden: yes()}); !core.IsKind(err, tc.kind) {
				t.Fatalf("err = %v, want %s", err, tc.kind)
			}
		})
	}
}
