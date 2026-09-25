// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// cyclic is a workflow with a loop in it, so the walk has to prove it stops.
func cyclic() core.WorkflowDefinition {
	return core.WorkflowDefinition{
		Initial: "todo",
		States: []core.State{
			{Key: "todo", Label: "To do"},
			{Key: "doing", Label: "Doing"},
			{Key: "blocked", Label: "Blocked"},
			{Key: "done", Label: "Done"},
			{Key: "orphan", Label: "Orphan"},
		},
		Transitions: []core.Transition{
			{From: "todo", To: "doing"},
			{From: "doing", To: "todo"},
			{From: "doing", To: "blocked"},
			{From: "blocked", To: "doing"},
			{From: "doing", To: "done"},
			{From: "done", To: "todo"},
			{From: "todo", To: "ghost"},
		},
	}
}

func TestFlowRoutesWalkTheWholeWorkflowWithoutLooping(t *testing.T) {
	t.Parallel()
	def := cyclic()
	tests := []struct {
		name  string
		from  string
		want  []string
		paths []string
	}{
		{
			name:  "reaches what only a route reaches",
			from:  "todo",
			want:  []string{"doing", "blocked", "done"},
			paths: []string{"todo → Doing", "todo → Doing → Blocked", "todo → Doing → Done"},
		},
		{
			name:  "never offers the state it is already in",
			from:  "doing",
			want:  []string{"todo", "blocked", "done"},
			paths: []string{"doing → To do", "doing → Blocked", "doing → Done"},
		},
		{
			name:  "a terminal state still reaches the rest",
			from:  "done",
			want:  []string{"todo", "doing", "blocked"},
			paths: []string{"done → To do", "done → To do → Doing", "done → To do → Doing → Blocked"},
		},
		{
			name: "a state nothing leaves offers nothing",
			from: "orphan",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			routes := flowRoutes(def, tc.from)
			var keys, paths []string
			for _, r := range routes {
				keys = append(keys, r.To.Key)
				paths = append(paths, r.Path())
			}
			if strings.Join(keys, ",") != strings.Join(tc.want, ",") {
				t.Errorf("reachable from %q = %v, want %v", tc.from, keys, tc.want)
			}
			if strings.Join(paths, "|") != strings.Join(tc.paths, "|") {
				t.Errorf("routes from %q = %v, want %v", tc.from, paths, tc.paths)
			}
			for _, r := range routes {
				if got, want := r.Hops(), len(r.Via)+1; got != want {
					t.Errorf("hops to %q = %d, want %d", r.To.Key, got, want)
				}
				if r.MultiHop() != (len(r.Via) > 0) {
					t.Errorf("multi-hop to %q disagrees with its own route", r.To.Key)
				}
			}
		})
	}
}

// TestFlowRoutesOfferNoStateTheWorkflowDoesNotDeclare pins that an edge naming
// a state the definition never declares is dropped rather than offered.
func TestFlowRoutesOfferNoStateTheWorkflowDoesNotDeclare(t *testing.T) {
	t.Parallel()
	for _, r := range flowRoutes(cyclic(), "todo") {
		if r.To.Key == "ghost" {
			t.Fatal("a transition to an undeclared state was offered as a move")
		}
	}
}

func TestParseFlowRoute(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		raw  string
		want []string
		bad  bool
	}{
		{name: "one state", raw: "doing", want: []string{"doing"}},
		{name: "a route", raw: "doing>done", want: []string{"doing", "done"}},
		{name: "blanks are dropped", raw: " doing > > done ", want: []string{"doing", "done"}},
		{name: "a loop is refused", raw: "doing>todo>doing", bad: true},
		{name: "nothing is refused", raw: ">>", bad: true},
		{name: "an unbounded run is refused",
			raw: strings.Join([]string{"a", "b", "c", "d", "e", "f", "g", "h",
				"i", "j", "k", "l", "m", "n", "o", "p", "q"}, ">"), bad: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseFlowRoute(tc.raw)
			if tc.bad {
				if err == nil {
					t.Fatalf("parsing %q succeeded, want a refusal", tc.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("parsing %q: %v", tc.raw, err)
			}
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("parsing %q = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}
