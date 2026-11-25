// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import "strings"

// RouteInput moves a task along a route, one ordinary transition per hop.
//
// A caller either names the whole route or names the destination and lets the
// workflow find it. Naming both is refused rather than reconciled, because the
// two can disagree and no reading of the pair is obviously right.
type RouteInput struct {
	// Route lists the states to pass through in order, the current one left
	// out, each once; set this or To.
	Route []string `json:"route,omitempty" yaml:"route,omitempty"`
	// To is the destination state, reached by the route the workflow finds;
	// set this or Route.
	To string `json:"to,omitempty" yaml:"to,omitempty"`
	// Comment is attached to every hop's transition.
	Comment string `json:"comment,omitempty" yaml:"comment,omitempty"`
	// LeaseToken proves the caller's claim when the task is leased.
	LeaseToken string `json:"lease_token,omitempty" yaml:"lease_token,omitempty"`
	// CustomFields are custom field values set on the last hop, keyed by
	// field definition key.
	CustomFields map[string]any `json:"custom_fields,omitempty" yaml:"custom_fields,omitempty"`
	// Version is the task version the move was based on, checked on the first
	// hop; a stale one is a conflict and omitted skips the check.
	Version int `json:"version,omitempty" yaml:"version,omitempty"`
	// DryRun walks the route in one transaction and rolls it back: each hop is
	// judged against the state the hops before it would leave, and nothing is
	// written.
	DryRun bool `json:"dry_run,omitempty" yaml:"dry_run,omitempty"`
}

// Validate checks the input.
func (in RouteInput) Validate() error {
	switch {
	case len(in.Route) == 0 && in.To == "":
		return Invalid("a route or a target status is required")
	case len(in.Route) > 0 && in.To != "":
		return Invalid("name a route or a target status, not both")
	case len(in.Route) > MaxRouteSteps:
		return Invalid("route is longer than %d states", MaxRouteSteps)
	}
	seen := map[string]bool{}
	for _, step := range in.Route {
		if strings.TrimSpace(step) == "" {
			return Invalid("route names an empty state")
		}
		if seen[step] {
			return Invalid("route passes through %q twice", step)
		}
		seen[step] = true
	}
	return nil
}

// RouteResult reports what a route actually did, which is not always what it
// asked for. A route that stops halfway leaves the task somewhere, and a
// caller told only that it failed would have to go and look.
type RouteResult struct {
	// Task is the task as it stands after the last hop that landed.
	Task *Task `json:"task,omitempty" yaml:"task,omitempty"`
	// From is the status the task started in.
	From string `json:"from" yaml:"from"`
	// Route is the full list of states the move set out to pass through.
	Route []string `json:"route" yaml:"route"`
	// Applied lists the states the task actually entered, in order.
	Applied []string `json:"applied" yaml:"applied"`
	// Stopped is the state a hop was refused for, empty when the route completed.
	Stopped string `json:"stopped,omitempty" yaml:"stopped,omitempty"`
	// Reason is why that hop was refused.
	Reason string `json:"reason,omitempty" yaml:"reason,omitempty"`
	// DryRun is true when the route was judged and nothing was written.
	DryRun bool `json:"dry_run,omitempty" yaml:"dry_run,omitempty"`
}

// Partial reports whether the route stopped before the end.
func (r RouteResult) Partial() bool { return r.Stopped != "" }

// Hops is how many transitions the route actually performed, which is how many
// audit entries and how many events it wrote.
func (r RouteResult) Hops() int { return len(r.Applied) }

// Status is where the task ended up: the last hop that landed, or where it
// started when none did.
func (r RouteResult) Status() string {
	if len(r.Applied) == 0 {
		return r.From
	}
	return r.Applied[len(r.Applied)-1]
}

// Sentence says where the task went and, when it stopped early, where it
// stopped and why. It is what a surface with one line to spend reports, so a
// browser flash and a shell's last line say the same thing about the same
// move.
func (r RouteResult) Sentence() string {
	var b strings.Builder
	b.WriteString("task moved ")
	switch len(r.Applied) {
	case 0:
		b.WriteString("nowhere")
	case 1:
		b.WriteString("to " + r.Applied[0])
	default:
		b.WriteString(strings.Join(append([]string{r.From}, r.Applied...), " → "))
	}
	if !r.Partial() {
		if len(r.Applied) > 1 {
			b.WriteString(", one step at a time")
		}
		return b.String()
	}
	b.WriteString(", then stopped: could not move to " + r.Stopped)
	if r.Reason != "" {
		b.WriteString(" (" + r.Reason + ")")
	}
	return b.String()
}
