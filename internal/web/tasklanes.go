// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// laneCard is one task as a lane draws it: its state for the badge, and
// either the one route ticking it applies or the reason there is none.
type laneCard struct {
	Task core.Task
	// State is the task's state in its own project's workflow. Known is
	// false when the workflow could not be read or no longer has the state,
	// and the badge then shows the raw status.
	State core.State
	Known bool
	// Finished is set for a task in any terminal state. It renders ticked
	// and is what Show completed hides.
	Finished bool
	// Complete is the route the tick box submits, valid only when Tickable.
	Complete flowRoute
	Tickable bool
	// Locked says why an unfinished task has no tick box, for the link that
	// stands in its place.
	Locked string
}

// Category is the category the badge is coloured by.
func (c laneCard) Category() core.StateCategory {
	if !c.Known {
		return ""
	}
	return c.State.Category
}

// Label is what the badge says.
func (c laneCard) Label() string {
	if !c.Known {
		return c.Task.Status
	}
	return core.StateLabel(c.State)
}

// Journey names the states ticking passes through, for the box's accessible
// name: a route through another state is consented to before it is applied.
func (c laneCard) Journey() string {
	if !c.Complete.MultiHop() {
		return core.StateLabel(c.Complete.To)
	}
	return c.Complete.Path() + ", " + c.Complete.Steps()
}

// lane is one project's column of tasks.
type lane struct {
	Project core.Project
	Cards   []laneCard
	// Open counts the unfinished cards, for the heading.
	Open int
	// Named is set when a project: term asked for this project, which is
	// the only reason an empty lane is drawn.
	Named bool
}

// AllFinished reports whether every card is finished, so a lane emptied by
// Show completed can still say why it is empty.
func (l lane) AllFinished() bool { return len(l.Cards) > 0 && l.Open == 0 }

// taskLanes is the lanes view of one page.
type taskLanes struct {
	Lanes []lane
	// Stray are tasks whose project the screen cannot name, listed rather
	// than dropped so the lanes are never quietly short.
	Stray []core.Task
}

// laneMoves is what deciding a card needs beyond the task: each project's
// workflow and whether the reader holds a scope.
type laneMoves struct {
	Workflows map[string]core.WorkflowDefinition
	HasScope  func(core.Scope) bool
}

// buildLanes groups one page of tasks into a lane per project.
//
// Lanes follow the order projects are given in, which is the order the
// screen lists them in everywhere else. A project gets a lane when a task on
// the page is in it, or when the filter names it: an empty named lane is the
// reader's own request answered, while an empty lane per project of a large
// tenant would be columns of nothing, mostly because their tasks are on
// another page.
func buildLanes(projects []core.Project, filter core.TaskFilter, tasks []core.Task, moves laneMoves) taskLanes {
	named := map[string]bool{}
	for _, key := range filter.ProjectKeys {
		named[strings.ToLower(key)] = true
	}
	candidates := candidateProjects(projects, filter)
	byProject := make(map[string][]core.Task, len(candidates))
	known := make(map[string]bool, len(candidates))
	for _, p := range candidates {
		known[p.ID] = true
	}
	var out taskLanes
	for _, t := range tasks {
		if !known[t.ProjectID] {
			out.Stray = append(out.Stray, t)
			continue
		}
		byProject[t.ProjectID] = append(byProject[t.ProjectID], t)
	}
	for _, p := range candidates {
		l := lane{Project: p, Named: named[strings.ToLower(p.Key)]}
		if len(byProject[p.ID]) == 0 && !l.Named {
			continue
		}
		def, ok := moves.Workflows[p.ID]
		for _, t := range byProject[p.ID] {
			c := newLaneCard(t, def, ok, moves.HasScope)
			if !c.Finished {
				l.Open++
			}
			l.Cards = append(l.Cards, c)
		}
		out.Lanes = append(out.Lanes, l)
	}
	return out
}

// newLaneCard decides how one task is drawn in its lane.
func newLaneCard(t core.Task, def core.WorkflowDefinition, haveDef bool, has func(core.Scope) bool) laneCard {
	c := laneCard{Task: t}
	if !haveDef {
		c.Locked = "its workflow could not be read"
		return c
	}
	if st, ok := def.State(t.Status); ok {
		c.State, c.Known = st, true
		c.State.Category = laneCategory(def, st)
		c.Finished = st.Terminal
	}
	if c.Finished {
		return c
	}
	c.Complete, c.Locked = completion(def, t.Status, has)
	c.Tickable = c.Locked == ""
	return c
}

// laneCategory is the category a state is coloured by. A state that declares
// none takes the one statistics derives for it, so a workflow written before
// categories existed still colours its badges sensibly.
func laneCategory(def core.WorkflowDefinition, st core.State) core.StateCategory {
	switch {
	case st.Category != "":
		return st.Category
	case st.Terminal:
		return core.CategoryDone
	case st.Key == def.Initial:
		return core.CategoryTodo
	default:
		return core.CategoryInProgress
	}
}

// completion is the one route a tick box may apply, or why there is none.
//
// Ticking means finished, and that is only a tick box if it is one move: the
// single shortest route into a terminal done state. Two such routes, whether
// to two done states or two ways into one, is a choice the reader has to
// make on the task's own screen, because guessing writes a history nobody
// chose. A step needing a comment has nowhere to type one, and a step needing
// a scope the reader lacks would only be refused after the click.
func completion(def core.WorkflowDefinition, from string, has func(core.Scope) bool) (flowRoute, string) {
	if has == nil || !has(core.ScopeTaskTransition) {
		return flowRoute{}, "you cannot move tasks"
	}
	var found []flowRoute
	for _, r := range flowRoutes(def, from) {
		if r.To.Terminal && laneCategory(def, r.To) == core.CategoryDone {
			found = append(found, r)
		}
	}
	if len(found) == 0 {
		return flowRoute{}, "no route leads to done from here"
	}
	if len(found) > 1 {
		return flowRoute{}, "there is more than one way to finish it"
	}
	at := from
	for _, next := range found[0].Keys() {
		edge, _ := def.CanTransition(at, next)
		if edge.RequiresComment {
			return flowRoute{}, "finishing it needs a comment"
		}
		if edge.RequiresScope != "" && !has(edge.RequiresScope) {
			return flowRoute{}, "finishing it needs the " + string(edge.RequiresScope) + " scope"
		}
		at = next
	}
	return found[0], ""
}
