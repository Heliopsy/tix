// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// laneCard is one task as a lane draws it: its state for the badge, what its
// tick does, and the menus its status and priority open.
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
	// Tick holds the routes the tick offers. With Choose unset it is one
	// route the box applies; with Choose set it is a menu of them.
	Tick   []flowRoute
	Choose bool
	// Locked says why the tick offers no route, shown in the panel the
	// tick opens instead.
	Locked lockReason
	// Moves are the routes the status menu offers this reader.
	Moves []flowRoute
	// Prioritise is set for a reader who may change the priority.
	Prioritise bool
	// Write is set for a reader who may edit the task: rename, assign, tag.
	Write bool
	// Delete is set for a reader who may delete the task.
	Delete bool
	// Commented are the moves that need a comment, which the more actions
	// menu offers as links to the task, where there is a field for one.
	Commented []flowRoute
}

// lockReason is why a tick offers nothing: a catalog key and the one value
// some of them carry, so the page can say it in the reader's language.
type lockReason struct {
	Key string
	Arg string
}

// lockKeys are every reason a tick can be locked, for the catalog tests.
var lockKeys = []string{
	"you cannot move tasks",
	"its workflow could not be read",
	"no route leads to done from here",
	"no route leads back to an open state",
	"finishing it needs a comment",
	"reopening it needs a comment",
	"finishing it needs the %s scope",
	"reopening it needs the %s scope",
}

// Tickable reports whether the tick is a box that applies one route.
func (c laneCard) Tickable() bool { return !c.Choose && len(c.Tick) == 1 }

// Complete is the route the box applies, valid only when Tickable.
func (c laneCard) Complete() flowRoute {
	if len(c.Tick) == 0 {
		return flowRoute{}
	}
	return c.Tick[0]
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
	r := c.Complete()
	if !r.MultiHop() {
		return core.StateLabel(r.To)
	}
	return r.Path()
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
	// Head is the lane's heading as the reader arranged it.
	Head groupHead
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
	// Write is set for a reader who may edit tasks, so the page carries the
	// handles the assignee fields suggest.
	Write bool
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
	out := taskLanes{Write: moves.HasScope != nil && moves.HasScope(core.ScopeTaskWrite)}
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
	write := has != nil && has(core.ScopeTaskWrite)
	c := laneCard{Task: t, Prioritise: write, Write: write, Delete: has != nil && has(core.ScopeTaskDelete)}
	if !haveDef {
		c.Locked = lockReason{Key: "its workflow could not be read"}
		return c
	}
	if st, ok := def.State(t.Status); ok {
		c.State, c.Known = st, true
		c.State.Category = laneCategory(def, st)
		c.Finished = st.Terminal
	}
	c.Tick, c.Choose, c.Locked = tickRoutes(def, t.Status, has, c.Finished)
	c.Moves = statusMoves(def, t.Status, has)
	c.Commented = commentedMoves(def, t.Status, has)
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

// tickRoutes is what a tick offers, completing an unfinished task or
// reopening a finished one, or why it offers nothing.
//
// Completing considers the shortest route into each terminal done state,
// because each is a different outcome. Reopening considers the routes to open
// states with the fewest steps, because any open state means not finished and
// the nearest is the one meant. One candidate is a box. More than one is a
// menu of the usable ones and never a box, even when only one is usable,
// because guessing writes a history nobody chose. A step needing a comment
// has nowhere to type one, and a step needing a scope the reader lacks would
// only be refused after the click, so both make a candidate unusable.
func tickRoutes(def core.WorkflowDefinition, from string, has func(core.Scope) bool, reopen bool) ([]flowRoute, bool, lockReason) {
	if has == nil || !has(core.ScopeTaskTransition) {
		return nil, false, lockReason{Key: "you cannot move tasks"}
	}
	found := tickCandidates(def, from, reopen)
	if len(found) == 0 {
		if reopen {
			return nil, false, lockReason{Key: "no route leads back to an open state"}
		}
		return nil, false, lockReason{Key: "no route leads to done from here"}
	}
	var usable []flowRoute
	var why lockReason
	for _, r := range found {
		reason := routeBlocked(def, r, has, reopen)
		if reason.Key == "" {
			usable = append(usable, r)
		} else if why.Key == "" {
			why = reason
		}
	}
	if len(usable) == 0 {
		return nil, false, why
	}
	return usable, len(found) > 1, lockReason{}
}

// tickCandidates are the routes a tick chooses between.
func tickCandidates(def core.WorkflowDefinition, from string, reopen bool) []flowRoute {
	var found []flowRoute
	fewest := 0
	for _, r := range flowRoutes(def, from) {
		switch {
		case reopen && !r.To.Terminal:
			if fewest == 0 || r.Hops() < fewest {
				fewest, found = r.Hops(), found[:0]
			}
			if r.Hops() == fewest {
				found = append(found, r)
			}
		case !reopen && r.To.Terminal && laneCategory(def, r.To) == core.CategoryDone:
			found = append(found, r)
		}
	}
	return found
}

// routeBlocked is why this reader cannot apply a route from a lane, or the
// zero reason when they can.
func routeBlocked(def core.WorkflowDefinition, r flowRoute, has func(core.Scope) bool, reopen bool) lockReason {
	verb := "finishing"
	if reopen {
		verb = "reopening"
	}
	at := r.From
	for _, next := range r.Keys() {
		edge, _ := def.CanTransition(at, next)
		if edge.RequiresComment {
			return lockReason{Key: verb + " it needs a comment"}
		}
		if edge.RequiresScope != "" && !has(edge.RequiresScope) {
			return lockReason{Key: verb + " it needs the %s scope", Arg: string(edge.RequiresScope)}
		}
		at = next
	}
	return lockReason{}
}

// statusMoves are the routes the status menu offers: every state the
// workflow reaches from here, less the ones this reader could not apply from
// a lane.
func statusMoves(def core.WorkflowDefinition, from string, has func(core.Scope) bool) []flowRoute {
	if has == nil || !has(core.ScopeTaskTransition) {
		return nil
	}
	var out []flowRoute
	for _, r := range flowRoutes(def, from) {
		if routeBlocked(def, r, has, false).Key == "" {
			out = append(out, r)
		}
	}
	return out
}

// commentedMoves are the routes this reader could take but for a comment,
// which a menu has nowhere to type.
func commentedMoves(def core.WorkflowDefinition, from string, has func(core.Scope) bool) []flowRoute {
	if has == nil || !has(core.ScopeTaskTransition) {
		return nil
	}
	var out []flowRoute
	for _, r := range flowRoutes(def, from) {
		if routeBlocked(def, r, has, false).Key == "finishing it needs a comment" {
			out = append(out, r)
		}
	}
	return out
}
