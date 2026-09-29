// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"strings"
)

// RouteSep separates the states of a route wherever one is written down: a
// browser form field, a command line argument, a JSON string.
const RouteSep = ">"

// MaxRouteSteps bounds a route. Every route a surface offers is a shortest
// path through a workflow, so nothing legitimate is near this; the bound is
// there so a crafted field cannot ask for an unbounded run of writes.
const MaxRouteSteps = 16

// maxRoutesPerState caps how many distinct shortest routes to one state are
// enumerated. A workflow is small, but the number of equal-length paths
// through a graph is not bounded by its size, and a menu with forty ways to
// reach one state is not a menu.
const maxRoutesPerState = 4

// Route is one state a task can reach and the way it gets there.
//
// Via holds the states passed through on the way, counting neither the state
// the task starts in nor To, so a direct move has no Via at all.
type Route struct {
	// From is the key of the state the task is in now.
	From string
	// To is the state the route ends in.
	To State
	// Via are the states the route passes through, in order.
	Via []State
}

// MultiHop reports whether the route passes through another state on the way.
func (r Route) MultiHop() bool { return len(r.Via) > 0 }

// Hops is how many transitions applying the route performs.
func (r Route) Hops() int { return len(r.Via) + 1 }

// Keys are the state keys to move through, in the order they are applied.
func (r Route) Keys() []string {
	out := make([]string, 0, r.Hops())
	for _, s := range r.Via {
		out = append(out, s.Key)
	}
	return append(out, r.To.Key)
}

// Labels are the human names of the same states, in the same order.
func (r Route) Labels() []string {
	out := make([]string, 0, r.Hops())
	for _, s := range r.Via {
		out = append(out, StateLabel(s))
	}
	return append(out, StateLabel(r.To))
}

// Value is the route as a field, an argument or a JSON string spells it.
func (r Route) Value() string { return strings.Join(r.Keys(), RouteSep) }

// StateLabel is what to call a state, falling back to its key so a workflow
// that labels nothing still reads.
func StateLabel(s State) string {
	if s.Label != "" {
		return s.Label
	}
	return s.Key
}

// Routes lists every state reachable from the given one, with the shortest
// route to it. A state that two equally short routes reach appears once per
// route, so a caller offering a menu offers the journey and never has to guess
// which one a reader meant.
//
// Breadth first, so a neighbour is always offered as one hop and never as a
// detour, and a state is only ever reached at its own shortest distance, which
// is what stops a workflow's cycles (todo, doing, todo) walking forever.
func Routes(def WorkflowDefinition, from string) []Route {
	edges := make(map[string][]string, len(def.States))
	for _, t := range def.Transitions {
		edges[t.From] = append(edges[t.From], t.To)
	}
	type node struct {
		key string
		via []State
	}
	dist := map[string]int{from: 0}
	seen := map[string]bool{}
	frontier := []node{{key: from}}
	var out []Route
	for depth := 1; depth <= MaxRouteSteps && len(frontier) > 0; depth++ {
		counts := map[string]int{}
		var next []node
		for _, cur := range frontier {
			for _, to := range edges[cur.key] {
				if d, ok := dist[to]; ok && d < depth {
					continue
				}
				state, ok := def.State(to)
				if !ok {
					continue
				}
				route := Route{From: from, To: state, Via: cur.via}
				if seen[route.Value()] || counts[to] >= maxRoutesPerState {
					continue
				}
				seen[route.Value()] = true
				counts[to]++
				dist[to] = depth
				out = append(out, route)
				via := append(append(make([]State, 0, len(cur.via)+1), cur.via...), state)
				next = append(next, node{key: to, via: via})
			}
		}
		frontier = next
	}
	return out
}

// FindRoute is the shortest route from one state to another.
//
// It refuses rather than choosing when two routes are equally short. Picking
// one would walk the task through a state nobody named, and every state a task
// passes through is written to its audit trail and announced to subscribers,
// so the wrong guess is not a display detail but a false history.
func FindRoute(def WorkflowDefinition, from, to string) (Route, error) {
	if from == to {
		return Route{}, Invalid("task is already in %q", to)
	}
	if !def.HasState(to) {
		return Route{}, Invalid("status %q is not a state of this project's workflow", to)
	}
	var found []Route
	for _, r := range Routes(def, from) {
		if r.To.Key == to {
			found = append(found, r)
		}
	}
	switch len(found) {
	case 0:
		return Route{}, Precondition("no route leads from %q to %q", from, to)
	case 1:
		return found[0], nil
	}
	return Route{}, Invalid("two routes of %d steps lead from %q to %q, %s and %s; name the route you want instead of the state",
		found[0].Hops(), from, to, found[0].Value(), found[1].Value())
}

// ParseRoute reads a written route into the states to apply, in order.
//
// A route may not name the same state twice: every route a surface offers is a
// shortest path, so a repeat is either a crafted field or a loop, and either
// way it would write audit entries nobody asked for.
func ParseRoute(raw string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	for _, part := range strings.Split(raw, RouteSep) {
		step := strings.TrimSpace(part)
		if step == "" {
			continue
		}
		if seen[step] {
			return nil, Invalid("route %q passes through %q twice", raw, step)
		}
		seen[step] = true
		out = append(out, step)
	}
	if len(out) == 0 {
		return nil, Invalid("route names no state")
	}
	if len(out) > MaxRouteSteps {
		return nil, Invalid("route is longer than %d states", MaxRouteSteps)
	}
	return out, nil
}
