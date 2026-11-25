// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"slices"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func starredKeys(t *testing.T, l *Local, ctx context.Context) []string {
	t.Helper()
	projects, err := l.ListStarredProjects(ctx)
	if err != nil {
		t.Fatalf("listing starred projects: %v", err)
	}
	keys := make([]string, len(projects))
	for i, p := range projects {
		if !p.Starred {
			t.Errorf("%s is listed as starred without the mark", p.Key)
		}
		keys[i] = p.Key
	}
	return keys
}

func TestAProjectStarIsTheReadersAlone(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	other := taskContext(seedTaskActor(t, l, scope, "bob", core.RoleMember, core.ScopeAll))
	seedTaskProject(t, l, scope, "web")

	for range 2 {
		if err := l.StarProject(ctx, "web"); err != nil {
			t.Fatalf("starring: %v", err)
		}
	}
	if got := starredKeys(t, l, ctx); !slices.Equal(got, []string{"web"}) {
		t.Fatalf("the starrer lists %v, want [web]", got)
	}
	if got := starredKeys(t, l, other); len(got) != 0 {
		t.Fatalf("another reader lists %v", got)
	}
	mine, err := l.GetProject(ctx, "web")
	if err != nil || !mine.Starred {
		t.Fatalf("GetProject for the starrer: starred=%v err=%v", mine != nil && mine.Starred, err)
	}
	theirs, err := l.GetProject(other, "web")
	if err != nil || theirs.Starred {
		t.Fatalf("GetProject for another reader carries the star: %v", err)
	}
	listed, _, err := l.ListProjects(other, core.ProjectFilter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range listed {
		if p.Starred {
			t.Errorf("another reader's listing marks %s starred", p.Key)
		}
	}

	prefs, err := l.ListGroupPrefs(ctx, core.ViewLanes)
	if err != nil || len(prefs) != 1 || prefs[0].Group != "web" || !prefs[0].Starred {
		t.Fatalf("a project star is not the lane star: %+v %v", prefs, err)
	}

	for range 2 {
		if err := l.UnstarProject(ctx, "web"); err != nil {
			t.Fatalf("unstarring: %v", err)
		}
	}
	if got := starredKeys(t, l, ctx); len(got) != 0 {
		t.Fatalf("after unstar the starrer lists %v", got)
	}
	if prefs, _ := l.ListGroupPrefs(ctx, core.ViewLanes); len(prefs) != 0 {
		t.Fatalf("unstarring left a plain preference behind: %+v", prefs)
	}
	if err := l.StarProject(ctx, "missing"); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("starring an unknown project: %v, want not_found", err)
	}
}

func TestALaneStarIsAProjectStarInTheLanesOrder(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	for _, key := range []string{"web", "ops"} {
		seedTaskProject(t, l, scope, key)
	}
	yes := true
	for _, key := range []string{"infra", "web", "ops"} {
		if _, err := l.SetGroupPref(ctx, core.ViewLanes, key, core.GroupPrefInput{Starred: &yes}); err != nil {
			t.Fatalf("starring lane %s: %v", key, err)
		}
	}
	if _, err := l.OrderGroups(ctx, core.ViewLanes, core.GroupOrderInput{Groups: []string{"ops", "web"}}); err != nil {
		t.Fatal(err)
	}
	if got := starredKeys(t, l, ctx); !slices.Equal(got, []string{"ops", "web", "infra"}) {
		t.Fatalf("listed %v, want the lanes' order [ops web infra]", got)
	}
	if err := l.UnstarProject(ctx, "web"); err != nil {
		t.Fatal(err)
	}
	prefs, err := l.ListGroupPrefs(ctx, core.ViewLanes)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range prefs {
		if p.Group == "web" && (p.Starred || p.Position != 2) {
			t.Fatalf("unstarring lost the lane's position or kept the star: %+v", p)
		}
	}
}
