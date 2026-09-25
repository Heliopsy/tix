// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// flowSep separates the states of a submitted route.
const flowSep = ">"

// maxFlowSteps bounds a submitted route. Every route a screen offers is a
// shortest path through the workflow, so nothing legitimate is near this; the
// bound is there so a crafted field cannot ask for an unbounded run of writes.
const maxFlowSteps = 16

// flowRoute is one state a task can reach and the way it gets there.
type flowRoute struct {
	From string
	To   core.State
	Via  []core.State
}

// MultiHop reports whether the route passes through another state on the way.
func (r flowRoute) MultiHop() bool { return len(r.Via) > 0 }

// Hops is how many transitions applying the route performs.
func (r flowRoute) Hops() int { return len(r.Via) + 1 }

// Value is the route as the form submits it, in the order it is applied.
func (r flowRoute) Value() string {
	keys := make([]string, 0, r.Hops())
	for _, s := range r.Via {
		keys = append(keys, s.Key)
	}
	return strings.Join(append(keys, r.To.Key), flowSep)
}

// Path spells the whole journey out, starting where the task is now, so a
// reader consents to the states it passes through rather than discovering
// them in the trail afterwards.
func (r flowRoute) Path() string {
	labels := make([]string, 0, r.Hops()+1)
	labels = append(labels, r.From)
	for _, s := range r.Via {
		labels = append(labels, s.Label)
	}
	return strings.Join(append(labels, r.To.Label), " → ")
}

// flowRoutes lists every state reachable from the given one, each with the
// shortest route to it.
//
// Breadth first, so a neighbour is always offered as one hop and never as a
// detour, and the visited set is what stops a workflow's cycles (todo, doing,
// todo) walking forever. The one-hop entries are the same set targetsFrom
// returns, minus any duplicate edge.
func flowRoutes(def core.WorkflowDefinition, from string) []flowRoute {
	edges := make(map[string][]string, len(def.States))
	for _, t := range def.Transitions {
		edges[t.From] = append(edges[t.From], t.To)
	}
	type node struct {
		key string
		via []core.State
	}
	visited := map[string]bool{from: true}
	queue := []node{{key: from}}
	var out []flowRoute
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range edges[cur.key] {
			if visited[next] {
				continue
			}
			state, ok := def.State(next)
			if !ok {
				continue
			}
			visited[next] = true
			out = append(out, flowRoute{From: from, To: state, Via: cur.via})
			via := append(append(make([]core.State, 0, len(cur.via)+1), cur.via...), state)
			queue = append(queue, node{key: next, via: via})
		}
	}
	return out
}

// Flow lists the moves a row offers, adjacent states and the ones reachable
// through them.
//
// It hangs off the view rather than the template func map because a template
// may only call functions that map names, and render.go builds it as a
// literal this file cannot extend; a method on the value the screen is already
// executed against needs no registration at all.
func (tasksView) Flow(def core.WorkflowDefinition, from string) []flowRoute {
	return flowRoutes(def, from)
}

// parseFlowRoute reads a submitted route into the states to apply in order.
//
// A route may not name the same state twice: every offered route is a shortest
// path, so a repeat is either a crafted field or a loop, and either way it
// would write audit entries nobody asked for.
func parseFlowRoute(raw string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	for _, part := range strings.Split(raw, flowSep) {
		step := strings.TrimSpace(part)
		if step == "" {
			continue
		}
		if seen[step] {
			return nil, core.Invalid("route %q passes through %q twice", raw, step)
		}
		seen[step] = true
		out = append(out, step)
	}
	if len(out) == 0 {
		return nil, core.Invalid("route names no state")
	}
	if len(out) > maxFlowSteps {
		return nil, core.Invalid("route is longer than %d states", maxFlowSteps)
	}
	return out, nil
}
