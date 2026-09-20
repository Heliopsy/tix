package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
)

func TestEveryBindingIsDocumented(t *testing.T) {
	k := DefaultKeyMap()
	for _, v := range []viewKind{viewProjects, viewBoard, viewDetail, viewHelp} {
		for _, e := range append(k.ViewHelp(v), k.GlobalHelp()...) {
			if e.Keys == "" || e.Desc == "" {
				t.Fatalf("view %v advertises an undocumented binding %+v", v, e)
			}
		}
		if len(k.ShortHelp(v)) == 0 {
			t.Fatalf("view %v advertises no bindings at all", v)
		}
	}
}

func TestViewBindingsDoNotCollide(t *testing.T) {
	k := DefaultKeyMap()
	bindings := map[viewKind][]key.Binding{
		viewProjects: {k.Up, k.Down, k.Top, k.Bottom, k.Enter},
		viewBoard: {
			k.Up, k.Down, k.Left, k.Right, k.Top, k.Bottom, k.Enter, k.Filter,
			k.ClearFltr, k.Claim, k.Release, k.Transition, k.Projects, k.Back,
		},
		viewDetail: {k.Up, k.Down, k.Claim, k.Release, k.Transition, k.Back},
	}
	global := []key.Binding{k.Help, k.Refresh, k.Quit, k.Interrupt}
	for v, list := range bindings {
		seen := map[string]bool{}
		for _, b := range append(list, global...) {
			for _, keyName := range b.Keys() {
				if seen[keyName] {
					t.Fatalf("view %v binds %q twice", v, keyName)
				}
				seen[keyName] = true
			}
		}
	}
}

func TestTaskFilterAlwaysScopesToAProject(t *testing.T) {
	m := New(Config{})
	if got := m.taskFilter("infra"); len(got.ProjectKeys) != 1 || got.ProjectKeys[0] != "infra" {
		t.Fatalf("taskFilter = %+v", got.ProjectKeys)
	}
	m = m.applyFilterText("project:web status:todo")
	got := m.taskFilter("infra")
	if len(got.ProjectKeys) != 1 || got.ProjectKeys[0] != "web" {
		t.Fatalf("an explicit project term was overridden: %+v", got.ProjectKeys)
	}
	if got.Page.Cursor != "" || got.Page.Limit != 500 {
		t.Fatalf("page = %+v", got.Page)
	}
}
