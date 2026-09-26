// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
)

func TestEveryBindingIsDocumented(t *testing.T) {
	k := DefaultKeyMap()
	for _, v := range []viewKind{viewProjects, viewBoard, viewDetail, viewHelp} {
		for _, e := range append(k.ViewHelp(v), k.GlobalHelp(allViews)...) {
			if e.Keys == "" || e.Desc == "" {
				t.Fatalf("view %v advertises an undocumented binding %+v", v, e)
			}
		}
		if len(k.ShortHelp(v, ActionContext{HasProject: true, HasTask: true, CanTransition: true})) == 0 {
			t.Fatalf("view %v advertises no bindings at all", v)
		}
	}
}

func TestViewBindingsDoNotCollide(t *testing.T) {
	k := DefaultKeyMap()
	bindings := map[viewKind][]key.Binding{
		viewProjects: {k.Up, k.Down, k.Top, k.Bottom, k.Enter, k.NewProject},
		viewBoard: {
			k.Up, k.Down, k.Left, k.Right, k.Top, k.Bottom, k.Enter, k.Filter,
			k.ClearFltr, k.Claim, k.Release, k.Transition, k.New, k.EditTitle,
			k.EditBody, k.Priority, k.Assign, k.Comment, k.Tag, k.Untag,
			k.Depend, k.ClaimNext, k.Renew, k.Back,
		},
		viewDetail: {
			k.Up, k.Down, k.Claim, k.Release, k.Transition, k.New, k.EditTitle,
			k.EditBody, k.Priority, k.Assign, k.Comment, k.Tag, k.Untag,
			k.Depend, k.Renew, k.Back,
		},
	}
	global := []key.Binding{k.Help, k.Refresh, k.Projects, k.Quit, k.Interrupt}
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
	m := New(Config{Access: fullAccess()})
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

// TestTheFooterOnlyPromisesKeysThatWillWork holds the rule that a key in the
// footer is a promise: a task that cannot accept an action must not have that
// action advertised against it.
func TestTheFooterOnlyPromisesKeysThatWillWork(t *testing.T) {
	k := DefaultKeyMap()
	claim, release, renew, transition := k.Claim.Help().Key, k.Release.Help().Key,
		k.Renew.Help().Key, k.Transition.Help().Key

	tests := []struct {
		name    string
		ctx     ActionContext
		present []string
		absent  []string
	}{
		{
			name:    "unclaimed and claimable",
			ctx:     ActionContext{HasProject: true, HasTask: true, CanTransition: true},
			present: []string{claim, transition},
			absent:  []string{release, renew},
		},
		{
			name:    "held by this session",
			ctx:     ActionContext{HasProject: true, HasTask: true, HeldHere: true, CanTransition: true},
			present: []string{release, renew},
			absent:  []string{claim},
		},
		{
			name:    "held by another worker",
			ctx:     ActionContext{HasProject: true, HasTask: true, HeldElsewhere: true, CanTransition: true},
			present: []string{transition},
			absent:  []string{claim, release, renew},
		},
		{
			name:    "a terminal state offers no transition",
			ctx:     ActionContext{HasProject: true, HasTask: true},
			present: []string{claim},
			absent:  []string{transition},
		},
		{
			name:    "no task selected",
			ctx:     ActionContext{HasProject: true},
			present: nil,
			absent:  []string{claim, release, renew, transition},
		},
	}
	for _, tc := range tests {
		for _, v := range []viewKind{viewBoard, viewDetail} {
			t.Run(tc.name, func(t *testing.T) {
				var keys []string
				for _, e := range k.ShortHelp(v, tc.ctx) {
					keys = append(keys, e.Keys)
				}
				joined := " " + strings.Join(keys, " ") + " "
				for _, want := range tc.present {
					if !strings.Contains(joined, " "+want+" ") {
						t.Errorf("view %v: %q is missing from %v", v, want, keys)
					}
				}
				for _, unwanted := range tc.absent {
					if strings.Contains(joined, " "+unwanted+" ") {
						t.Errorf("view %v: %q is offered but would be refused: %v", v, unwanted, keys)
					}
				}
			})
		}
	}
}

func TestTheHelpViewStillDocumentsEveryBindingRegardlessOfContext(t *testing.T) {
	k := DefaultKeyMap()
	for _, v := range []viewKind{viewBoard, viewDetail} {
		var keys []string
		for _, e := range k.ViewHelp(v) {
			keys = append(keys, e.Keys)
		}
		joined := strings.Join(keys, " ")
		for _, want := range []string{k.Claim.Help().Key, k.Release.Help().Key, k.Renew.Help().Key} {
			if !strings.Contains(joined, want) {
				t.Errorf("view %v: help does not document %q", v, want)
			}
		}
	}
}
