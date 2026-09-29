// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"strings"
	"testing"
)

// forkedWorkflow reaches "done" from "todo" by two routes of the same length,
// which is the case FindRoute must refuse rather than decide.
func forkedWorkflow() WorkflowDefinition {
	return WorkflowDefinition{
		Initial: "todo",
		States: []State{
			{Key: "todo", Label: "To do"},
			{Key: "review", Label: "In review"},
			{Key: "qa", Label: "In QA"},
			{Key: "done", Label: "Done", Terminal: true},
		},
		Transitions: []Transition{
			{From: "todo", To: "review"},
			{From: "todo", To: "qa"},
			{From: "review", To: "done"},
			{From: "qa", To: "done"},
		},
	}
}

func routeValues(routes []Route) []string {
	out := make([]string, 0, len(routes))
	for _, r := range routes {
		out = append(out, r.Value())
	}
	return out
}

func TestRoutesOfferEveryReachableStateShortestFirst(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		def  WorkflowDefinition
		from string
		want []string
	}{
		{"direct and through", testWorkflow(), "todo",
			[]string{"doing", "cancelled", "doing" + RouteSep + "blocked", "doing" + RouteSep + "done"}},
		{"a cycle does not walk forever", testWorkflow(), "blocked",
			[]string{"doing", "doing" + RouteSep + "done"}},
		{"a terminal state reaches nothing", testWorkflow(), "done", nil},
		{"both forks are offered", forkedWorkflow(), "todo",
			[]string{"review", "qa", "review" + RouteSep + "done", "qa" + RouteSep + "done"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := routeValues(Routes(tt.def, tt.from))
			if len(got) != len(tt.want) {
				t.Fatalf("Routes(%q) = %v, want %v", tt.from, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("Routes(%q) = %v, want %v", tt.from, got, tt.want)
				}
			}
		})
	}
}

func TestARouteNamesItsHopsAndItsLabels(t *testing.T) {
	t.Parallel()
	def := testWorkflow()
	var direct, through Route
	for _, r := range Routes(def, "todo") {
		switch r.To.Key {
		case "doing":
			direct = r
		case "done":
			through = r
		}
	}
	if direct.MultiHop() || direct.Hops() != 1 {
		t.Errorf("direct route = %+v, want one hop through nothing", direct)
	}
	if !through.MultiHop() || through.Hops() != 2 {
		t.Errorf("route to done = %+v, want two hops", through)
	}
	if got := strings.Join(through.Keys(), ","); got != "doing,done" {
		t.Errorf("Keys = %q, want doing,done", got)
	}
	if got := strings.Join(through.Labels(), ","); got != "Doing,Done" {
		t.Errorf("Labels = %q, want Doing,Done", got)
	}
	if got := StateLabel(State{Key: "raw"}); got != "raw" {
		t.Errorf("StateLabel of an unlabelled state = %q, want its key", got)
	}
}

func TestFindRouteRefusesRatherThanChoosing(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		def      WorkflowDefinition
		from, to string
		want     string
		kind     Kind
	}{
		{"direct", testWorkflow(), "todo", "doing", "doing", ""},
		{"through one state", testWorkflow(), "todo", "done", "doing" + RouteSep + "done", ""},
		{"already there", testWorkflow(), "todo", "todo", "", KindInvalid},
		{"not a state", testWorkflow(), "todo", "nowhere", "", KindInvalid},
		{"unreachable", testWorkflow(), "done", "doing", "", KindPrecondition},
		{"two equal routes", forkedWorkflow(), "todo", "done", "", KindInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := FindRoute(tt.def, tt.from, tt.to)
			if tt.kind != "" {
				if !IsKind(err, tt.kind) {
					t.Fatalf("FindRoute(%q, %q) error = %v, want %s", tt.from, tt.to, err, tt.kind)
				}
				return
			}
			if err != nil {
				t.Fatalf("FindRoute(%q, %q) = %v", tt.from, tt.to, err)
			}
			if got.Value() != tt.want {
				t.Errorf("FindRoute(%q, %q) = %q, want %q", tt.from, tt.to, got.Value(), tt.want)
			}
		})
	}
}

// TestTheAmbiguousRefusalNamesBothRoutes is the half of the refusal that makes
// it actionable: a caller told only that it is ambiguous cannot name the route
// they wanted, and naming one of the two would be the choice this refuses.
func TestTheAmbiguousRefusalNamesBothRoutes(t *testing.T) {
	t.Parallel()
	_, err := FindRoute(forkedWorkflow(), "todo", "done")
	if err == nil {
		t.Fatal("FindRoute over a fork returned no error")
	}
	msg := err.Error()
	for _, want := range []string{"review" + RouteSep + "done", "qa" + RouteSep + "done", "name the route you want"} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal %q does not name %q", msg, want)
		}
	}
}

func TestParseRouteReadsWhatARouteWritesDown(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		raw  string
		want []string
		bad  bool
	}{
		{"one state", "done", []string{"done"}, false},
		{"several", "doing>review>done", []string{"doing", "review", "done"}, false},
		{"spaces are trimmed", " doing > done ", []string{"doing", "done"}, false},
		{"empty", "", nil, true},
		{"separators only", ">>", nil, true},
		{"a state twice", "doing>done>doing", nil, true},
		{"longer than the bound", strings.Join(manyStates(MaxRouteSteps+1), RouteSep), nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseRoute(tt.raw)
			if tt.bad {
				if !IsKind(err, KindInvalid) {
					t.Fatalf("ParseRoute(%q) error = %v, want invalid", tt.raw, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRoute(%q) = %v", tt.raw, err)
			}
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("ParseRoute(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func manyStates(n int) []string {
	out := make([]string, 0, n)
	for i := range n {
		out = append(out, string(rune('a'+i))+"state")
	}
	return out
}

// TestRoutesStayBoundedOnAWideGraph holds the two caps. A graph where every
// state reaches every other has a factorial number of paths, and an
// enumeration without a cap would neither terminate usefully nor produce a
// menu anybody could read.
func TestRoutesStayBoundedOnAWideGraph(t *testing.T) {
	t.Parallel()
	def := WorkflowDefinition{Initial: "s0"}
	const n = 8
	for i := range n {
		def.States = append(def.States, State{Key: stateKey(i)})
	}
	for i := range n {
		for j := range n {
			if i != j {
				def.Transitions = append(def.Transitions, Transition{From: stateKey(i), To: stateKey(j)})
			}
		}
	}
	routes := Routes(def, "s0")
	if len(routes) != n-1 {
		t.Fatalf("a fully connected graph offered %d routes, want %d one-hop ones", len(routes), n-1)
	}
	for _, r := range routes {
		if r.MultiHop() {
			t.Errorf("%s is offered as a detour where a direct edge exists", r.Value())
		}
	}
}

func stateKey(i int) string { return "s" + string(rune('0'+i)) }
