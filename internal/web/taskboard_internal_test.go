// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// twoStateFlow is the smallest machine a board can be drawn from.
func twoStateFlow() core.WorkflowDefinition {
	return core.WorkflowDefinition{
		Initial: "todo",
		States: []core.State{
			{Key: "todo", Label: "To do", Category: core.CategoryTodo},
			{Key: "done", Label: "Done", Category: core.CategoryDone, Terminal: true},
		},
		Transitions: []core.Transition{{From: "todo", To: "done"}},
	}
}

// Two definitions that describe the same machine have the same shape whatever
// they are called and whichever row they are stored in, and a definition that
// differs in anything a board draws or a move depends on does not.
func TestWorkflowShapeComparesTheMachineAndNotTheName(t *testing.T) {
	t.Parallel()
	base := twoStateFlow()

	same := func(mutate func(d *core.WorkflowDefinition)) core.WorkflowDefinition {
		out := twoStateFlow()
		mutate(&out)
		return out
	}

	agree := map[string]core.WorkflowDefinition{
		"a separately stored copy": twoStateFlow(),
		"a different initial state": same(func(d *core.WorkflowDefinition) {
			d.Initial = "done"
		}),
		"a different default lease": same(func(d *core.WorkflowDefinition) {
			d.DefaultLease = core.Duration(42)
		}),
		"different lease reversion": same(func(d *core.WorkflowDefinition) {
			d.States[0].RevertOnLeaseExpiry, d.States[0].RevertTo = true, "done"
		}),
	}
	for name, def := range agree {
		if workflowShape(def) != workflowShape(base) {
			t.Errorf("%s reads as a different workflow, so a drawable board is refused", name)
		}
	}

	differ := map[string]core.WorkflowDefinition{
		"an extra state": same(func(d *core.WorkflowDefinition) {
			d.States = append(d.States, core.State{Key: "parked"})
		}),
		"a renamed state": same(func(d *core.WorkflowDefinition) {
			d.States[0].Key = "open"
		}),
		"a relabelled state": same(func(d *core.WorkflowDefinition) {
			d.States[0].Label = "Backlog"
		}),
		"a reordered state": same(func(d *core.WorkflowDefinition) {
			d.States[0], d.States[1] = d.States[1], d.States[0]
		}),
		"a different category": same(func(d *core.WorkflowDefinition) {
			d.States[0].Category = core.CategoryWaiting
		}),
		"a different terminal flag": same(func(d *core.WorkflowDefinition) {
			d.States[0].Terminal = true
		}),
		"an extra transition": same(func(d *core.WorkflowDefinition) {
			d.Transitions = append(d.Transitions, core.Transition{From: "done", To: "todo"})
		}),
		"a transition requiring a scope": same(func(d *core.WorkflowDefinition) {
			d.Transitions[0].RequiresScope = core.ScopeAll
		}),
		"a transition requiring a comment": same(func(d *core.WorkflowDefinition) {
			d.Transitions[0].RequiresComment = true
		}),
	}
	for name, def := range differ {
		if workflowShape(def) == workflowShape(base) {
			t.Errorf("%s reads as the same workflow, so a board would merge two machines", name)
		}
	}
}

// A card's moves come from its own project's workflow, not from the one the
// columns were drawn from. The two cannot disagree on a board that agreed,
// which is exactly why this is asserted here, where they can be made to.
func TestACardsRoutesComeFromItsOwnProjectsWorkflow(t *testing.T) {
	t.Parallel()
	columns := twoStateFlow()
	columns.Transitions = append(columns.Transitions, core.Transition{From: "todo", To: "parked"})
	columns.States = append(columns.States, core.State{Key: "parked", Label: "Parked"})

	group := boardGroup{Known: true, Workflow: core.Workflow{Key: "wide", Definition: columns},
		Projects: []core.Project{{ID: "p1", Key: "one"}}}
	tasks := []core.Task{{ID: "t1", ProjectID: "p1", Status: "todo"}}
	moves := map[string]core.WorkflowDefinition{"p1": twoStateFlow()}

	board := buildBoard([]boardGroup{group}, tasks, moves)
	if !board.Agreed {
		t.Fatal("one known group did not draw a board")
	}
	var offered []string
	for _, col := range board.Columns {
		for _, c := range col.Tasks {
			for _, r := range c.Routes {
				offered = append(offered, r.Value())
			}
		}
	}
	if strings.Contains(strings.Join(offered, ","), "parked") {
		t.Errorf("a card was offered %v, which only the column workflow allows", offered)
	}
	if strings.Join(offered, ",") != "done" {
		t.Errorf("a card offers %v, its own workflow allows [done]", offered)
	}
}

// A task the drawn columns cannot hold is reported rather than dropped.
func TestATaskNoColumnCanHoldIsReportedAsStray(t *testing.T) {
	t.Parallel()
	group := boardGroup{Known: true, Workflow: core.Workflow{Definition: twoStateFlow()},
		Projects: []core.Project{{ID: "p1", Key: "one"}}}
	tasks := []core.Task{{ID: "t1", ProjectID: "p1", Status: "todo"},
		{ID: "t2", ProjectID: "p1", Status: "gone"}}

	board := buildBoard([]boardGroup{group}, tasks, map[string]core.WorkflowDefinition{"p1": twoStateFlow()})
	if len(board.Stray) != 1 || board.Stray[0].ID != "t2" {
		t.Errorf("the unplaced task is not reported: %v", board.Stray)
	}
}

// More than one workflow among the selected projects is a refusal, and the
// groups it names are in a stable order with their projects named.
func TestDisagreeingWorkflowsAreGroupedDeterministically(t *testing.T) {
	t.Parallel()
	wide := twoStateFlow()
	wide.States = append(wide.States, core.State{Key: "parked"})
	workflows := map[string]core.Workflow{
		"w1": {ID: "w1", Key: "narrow", Name: "Narrow", Definition: twoStateFlow()},
		"w2": {ID: "w2", Key: "copy", Name: "Narrow copy", Definition: twoStateFlow()},
		"w3": {ID: "w3", Key: "wide", Name: "Wide", Definition: wide},
	}
	projects := []core.Project{
		{ID: "p3", Key: "web", WorkflowID: "w3"},
		{ID: "p2", Key: "ops", WorkflowID: "w2"},
		{ID: "p1", Key: "infra", WorkflowID: "w1"},
		{ID: "p4", Key: "lost", WorkflowID: "gone"},
	}

	groups := groupByWorkflow(projects, workflows)
	if len(groups) != 3 {
		t.Fatalf("got %d groups, want one per distinct machine plus the unresolved one", len(groups))
	}
	if got := strings.Join(groups[0].Keys(), ","); got != "infra,ops" {
		t.Errorf("the agreeing projects grouped as %q, want infra,ops", got)
	}
	if got := groups[0].Name(); got != "Narrow" {
		t.Errorf("the merged group is named %q; two names for one machine must resolve the same way "+
			"whatever order the projects arrive in", got)
	}
	if got := strings.Join(groups[2].Keys(), ","); got != "lost" {
		t.Errorf("the last group is %q; a project whose workflow cannot be resolved must stay named", got)
	}
	if !strings.Contains(groups[2].Name(), "no workflow") {
		t.Errorf("an unresolved workflow is named %q", groups[2].Name())
	}
	if board := buildBoard(groups, nil, nil); board.Agreed {
		t.Error("a board was drawn over three distinct machines")
	}
}

// The candidate set is read off the assembled filter, so an explicit project
// term, a negated one and the visibility exclusion are one rule.
func TestCandidateProjectsFollowTheAssembledFilter(t *testing.T) {
	t.Parallel()
	projects := []core.Project{{Key: "infra"}, {Key: "ops"}, {Key: "web"}}
	keys := func(in []core.Project) string {
		out := make([]string, 0, len(in))
		for _, p := range in {
			out = append(out, p.Key)
		}
		return strings.Join(out, ",")
	}

	if got := keys(candidateProjects(projects, core.TaskFilter{})); got != "infra,ops,web" {
		t.Errorf("an unfiltered listing selects %q", got)
	}
	named := core.TaskFilter{ProjectKeys: []string{"OPS"}}
	if got := keys(candidateProjects(projects, named)); got != "ops" {
		t.Errorf("an explicit project term selects %q, want ops whatever its case", got)
	}
	excluded := core.TaskFilter{Exclude: core.TaskExclude{ProjectKeys: []string{"web"}}}
	if got := keys(candidateProjects(projects, excluded)); got != "infra,ops" {
		t.Errorf("an exclusion selects %q", got)
	}
}
