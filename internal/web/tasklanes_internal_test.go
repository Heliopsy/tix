// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"slices"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
)

// holds is a reader holding exactly the named scopes.
func holds(scopes ...core.Scope) func(core.Scope) bool {
	return func(s core.Scope) bool { return slices.Contains(scopes, s) }
}

// A tick box is one unambiguous move into done that this reader can make
// without typing anything, and nothing else.
func TestCompletionOffersOnlyOneUnambiguousRoute(t *testing.T) {
	t.Parallel()
	member := holds(core.ScopeTaskTransition)
	edit := func(mutate func(d *core.WorkflowDefinition)) core.WorkflowDefinition {
		d := service.BuiltinWorkflow()
		mutate(&d)
		return d
	}
	needs := func(field string) core.WorkflowDefinition {
		return edit(func(d *core.WorkflowDefinition) {
			for i, tr := range d.Transitions {
				if tr.From == "doing" && tr.To == "done" {
					if field == "comment" {
						d.Transitions[i].RequiresComment = true
					} else {
						d.Transitions[i].RequiresScope = core.ScopeTaskReclaim
					}
				}
			}
		})
	}
	tests := []struct {
		name   string
		def    core.WorkflowDefinition
		from   string
		has    func(core.Scope) bool
		route  string
		locked bool
	}{
		{"one hop", service.BuiltinWorkflow(), "doing", member, "done", false},
		{"through another state", service.BuiltinWorkflow(), "todo", member, "doing>done", false},
		{"a reader who cannot transition", service.BuiltinWorkflow(), "doing", holds(core.ScopeTaskRead), "", true},
		{"no reader at all", service.BuiltinWorkflow(), "doing", nil, "", true},
		{"a step needing a comment", needs("comment"), "doing", member, "", true},
		{"a step needing a scope the reader lacks", needs("scope"), "doing", member, "", true},
		{"a step needing a scope the reader holds", needs("scope"), "doing",
			holds(core.ScopeTaskTransition, core.ScopeTaskReclaim), "done", false},
		{"two done states", edit(func(d *core.WorkflowDefinition) {
			d.States = append(d.States, core.State{Key: "shipped", Category: core.CategoryDone, Terminal: true})
			d.Transitions = append(d.Transitions, core.Transition{From: "doing", To: "shipped"})
		}), "doing", member, "", true},
		{"two equally short ways into done", edit(func(d *core.WorkflowDefinition) {
			d.States = append(d.States, core.State{Key: "review", Category: core.CategoryInProgress})
			d.Transitions = append(d.Transitions,
				core.Transition{From: "todo", To: "review"}, core.Transition{From: "review", To: "done"})
		}), "todo", member, "", true},
		{"a terminal state with no category counts as done", edit(func(d *core.WorkflowDefinition) {
			d.States[3].Category = ""
		}), "doing", member, "done", false},
		{"cancelled is not done", edit(func(d *core.WorkflowDefinition) {
			d.Transitions = slices.DeleteFunc(d.Transitions, func(tr core.Transition) bool { return tr.To == "done" })
		}), "doing", member, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			route, locked := completion(tt.def, tt.from, tt.has)
			if (locked != "") != tt.locked {
				t.Fatalf("locked = %q, want locked %v", locked, tt.locked)
			}
			if !tt.locked && route.Value() != tt.route {
				t.Errorf("route = %q, want %q", route.Value(), tt.route)
			}
		})
	}
}

// A state that declares no category is coloured the way statistics count it.
func TestLaneCategoryDerivesAMissingCategory(t *testing.T) {
	t.Parallel()
	def := core.WorkflowDefinition{Initial: "new"}
	tests := []struct {
		state core.State
		want  core.StateCategory
	}{
		{core.State{Key: "x", Category: core.CategoryWaiting}, core.CategoryWaiting},
		{core.State{Key: "x", Terminal: true}, core.CategoryDone},
		{core.State{Key: "new"}, core.CategoryTodo},
		{core.State{Key: "x"}, core.CategoryInProgress},
	}
	for _, tt := range tests {
		if got := laneCategory(def, tt.state); got != tt.want {
			t.Errorf("laneCategory(%+v) = %q, want %q", tt.state, got, tt.want)
		}
	}
}

// Lanes follow the order projects are given in, keep the page's order inside
// a lane, draw an empty lane only for a project the filter names, and report
// a task whose project is not listed rather than dropping it.
func TestBuildLanesGroupsByProject(t *testing.T) {
	t.Parallel()
	projects := []core.Project{{ID: "p2", Key: "zeta"}, {ID: "p1", Key: "alpha"}, {ID: "p3", Key: "empty"}}
	def := service.BuiltinWorkflow()
	moves := laneMoves{
		Workflows: map[string]core.WorkflowDefinition{"p1": def, "p2": def, "p3": def},
		HasScope:  holds(core.ScopeTaskTransition),
	}
	tasks := []core.Task{
		{Ref: "A-2", ProjectID: "p1", Status: "todo"},
		{Ref: "Z-1", ProjectID: "p2", Status: "done"},
		{Ref: "A-1", ProjectID: "p1", Status: "doing"},
		{Ref: "X-1", ProjectID: "gone", Status: "todo"},
	}
	refs := func(l lane) []string {
		var out []string
		for _, c := range l.Cards {
			out = append(out, c.Task.Ref)
		}
		return out
	}

	got := buildLanes(projects, core.TaskFilter{}, tasks, moves)
	if len(got.Lanes) != 2 || got.Lanes[0].Project.Key != "zeta" || got.Lanes[1].Project.Key != "alpha" {
		t.Fatalf("lanes = %+v, want zeta then alpha and no empty lane", got.Lanes)
	}
	if r := refs(got.Lanes[1]); !slices.Equal(r, []string{"A-2", "A-1"}) {
		t.Errorf("alpha holds %v, want the page order A-2, A-1", r)
	}
	if got.Lanes[0].Open != 0 || !got.Lanes[0].AllFinished() || !got.Lanes[0].Cards[0].Finished {
		t.Errorf("a lane of one done task is not all finished: %+v", got.Lanes[0])
	}
	if len(got.Stray) != 1 || got.Stray[0].Ref != "X-1" {
		t.Errorf("stray = %v, want X-1 reported", got.Stray)
	}

	named := buildLanes(projects, core.TaskFilter{ProjectKeys: []string{"empty", "alpha"}}, tasks[:1], moves)
	var keys []string
	for _, l := range named.Lanes {
		keys = append(keys, l.Project.Key)
	}
	if !slices.Equal(keys, []string{"alpha", "empty"}) {
		t.Errorf("named lanes = %v, want alpha and the empty named lane", keys)
	}

	away := buildLanes(projects, core.TaskFilter{Exclude: core.TaskExclude{ProjectKeys: []string{"zeta"}}}, tasks, moves)
	for _, l := range away.Lanes {
		if l.Project.Key == "zeta" {
			t.Error("an excluded project has a lane")
		}
	}
}
