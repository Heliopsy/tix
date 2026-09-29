// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"strconv"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// flowRoute is a route as a screen shows it. The graph it comes from lives in
// internal/core, where every surface asks the same question; what is left here
// is how a browser renders the answer.
type flowRoute struct{ core.Route }

// Path spells the whole journey out, starting where the task is now, so a
// reader consents to the states it passes through rather than discovering them
// in the trail afterwards.
func (r flowRoute) Path() string {
	return strings.Join(append([]string{r.From}, r.Labels()...), " → ")
}

// Class is the modifier the button carries, so a route through another state
// looks like one before it is pressed.
func (r flowRoute) Class() string {
	if r.MultiHop() {
		return "flowgo is-multi"
	}
	return "flowgo"
}

// Steps names the hop count the way the button says it.
func (r flowRoute) Steps() string {
	if r.Hops() == 1 {
		return "1 step"
	}
	return strconv.Itoa(r.Hops()) + " steps"
}

// Option is how a <select> names the route: the destination, and for a route
// through another state the journey and its length, because a reader picking
// from a list still has to consent to the states in between.
func (r flowRoute) Option() string {
	if !r.MultiHop() {
		return core.StateLabel(r.To)
	}
	return core.StateLabel(r.To) + " (via " + strings.Join(r.Labels()[:len(r.Via)], ", ") +
		", " + r.Steps() + ")"
}

// flowRoutes lists every state reachable from the given one, each with the
// route to it, dressed for a screen.
func flowRoutes(def core.WorkflowDefinition, from string) []flowRoute {
	routes := core.Routes(def, from)
	out := make([]flowRoute, 0, len(routes))
	for _, r := range routes {
		out = append(out, flowRoute{r})
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
