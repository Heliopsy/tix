// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/heliopsy/tix/internal/connect"
	"github.com/heliopsy/tix/internal/core"
)

// routeOutcome is one task's journey, reported per reference the way every
// other bulk command reports one.
type routeOutcome struct {
	Ref     string   `json:"ref" yaml:"ref"`
	Status  string   `json:"status" yaml:"status"`
	From    string   `json:"from,omitempty" yaml:"from,omitempty"`
	Route   []string `json:"route,omitempty" yaml:"route,omitempty"`
	Applied []string `json:"applied,omitempty" yaml:"applied,omitempty"`
	Stopped string   `json:"stopped,omitempty" yaml:"stopped,omitempty"`
	Hops    int      `json:"hops" yaml:"hops"`
	DryRun  bool     `json:"dry_run,omitempty" yaml:"dry_run,omitempty"`
	Code    string   `json:"code,omitempty" yaml:"code,omitempty"`
	Error   string   `json:"error,omitempty" yaml:"error,omitempty"`
}

// moveByRoute finds the route to a status, prints it, and applies it one hop
// at a time. A dry run stops after printing, which is the whole difference
// between the two: the route a dry run shows is the route the real run takes,
// because both are resolved by the same call.
func (g *globals) moveByRoute(cmd *cobra.Command, conn *connect.Conn, ctx context.Context,
	refs []core.TaskRef, status string, in core.RouteInput, dryRun bool,
) error {
	results := make([]routeOutcome, 0, len(refs))
	var failure error
	for _, ref := range refs {
		out := routeOutcome{Ref: ref.String(), Status: statusOK, DryRun: dryRun}
		route, from, err := g.routeFor(ctx, conn, ref, status)
		if err != nil {
			out.Status, out.Code, out.Error = statusFailed, string(core.KindOf(err)), err.Error()
			results = append(results, out)
			if failure == nil {
				failure = err
			}
			continue
		}
		out.From, out.Route, out.Hops = from, route, len(route)
		printRoute(cmd, ref.String(), from, route, dryRun)
		if dryRun {
			out.Status = statusPlanned
			results = append(results, out)
			continue
		}
		hop := in
		hop.Route = route
		result, err := conn.Service.TransitionRoute(ctx, ref, hop)
		switch {
		case err != nil:
			out.Status, out.Code, out.Error = statusFailed, string(core.KindOf(err)), err.Error()
			if failure == nil {
				failure = err
			}
		default:
			out.Applied, out.Hops, out.Stopped = result.Applied, result.Hops(), result.Stopped
			if result.Partial() {
				out.Status, out.Error = statusFailed, result.Reason
				if failure == nil {
					failure = core.Precondition("%s", result.Sentence())
				}
			}
		}
		results = append(results, out)
	}
	if err := g.render(cmd, results); err != nil {
		return err
	}
	return failure
}

// routeFor resolves the route a task takes to reach a status. A status written
// with separators is the route itself, already named by the caller, which is
// what the refusal of an ambiguous destination asks them to do.
func (g *globals) routeFor(ctx context.Context, conn *connect.Conn, ref core.TaskRef, status string,
) ([]string, string, error) {
	task, err := conn.Service.GetTask(ctx, ref)
	if err != nil {
		return nil, "", err
	}
	if strings.Contains(status, core.RouteSep) {
		steps, err := core.ParseRoute(status)
		return steps, task.Status, err
	}
	def, err := workflowOf(ctx, conn, task)
	if err != nil {
		return nil, task.Status, err
	}
	route, err := core.FindRoute(def, task.Status, status)
	if err != nil {
		return nil, task.Status, err
	}
	return route.Keys(), task.Status, nil
}

// workflowOf reads the state machine a task's project runs under.
func workflowOf(ctx context.Context, conn *connect.Conn, task *core.Task) (core.WorkflowDefinition, error) {
	project, err := conn.Service.GetProject(ctx, projectKeyOf(task.Ref))
	if err != nil {
		return core.WorkflowDefinition{}, err
	}
	workflows, err := conn.Service.ListWorkflows(ctx)
	if err != nil {
		return core.WorkflowDefinition{}, err
	}
	for _, wf := range workflows {
		if wf.ID == project.WorkflowID {
			return wf.Definition, nil
		}
	}
	return core.WorkflowDefinition{}, core.NotFound("workflow for project %q", project.Key)
}

// projectKeyOf is the project half of a task reference.
func projectKeyOf(ref string) string {
	key, _, _ := strings.Cut(ref, "-")
	return key
}

// printRoute names the journey before anything is written, so a route through
// three states is read and not discovered in the trail afterwards.
func printRoute(cmd *cobra.Command, ref, from string, route []string, dryRun bool) {
	line := ref + ": " + strings.Join(append([]string{from}, route...), " -> ") +
		" (" + plural(len(route), "step") + ")"
	if dryRun {
		line += " [dry run, nothing written]"
	}
	cmd.PrintErrln(line)
}
