// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/heliopsy/tix/internal/core"
)

// bulkActionNames spells each bulk action the way the command line takes it.
func bulkActionNames() []string {
	out := make([]string, len(core.BulkActions))
	for i, a := range core.BulkActions {
		out[i] = strings.ReplaceAll(string(a), "_", "-")
	}
	return out
}

// taskBulkCmd builds `task bulk`, one request applying an action to many tasks.
func taskBulkCmd(g *globals) *cobra.Command {
	var (
		in       core.BulkTaskInput
		priority string
	)
	cmd := &cobra.Command{
		Use:   "bulk ACTION REF...",
		Short: "Apply one action to many tasks in one request",
		Long: "Apply one action to many tasks in a single request. ACTION is one of " +
			strings.Join(bulkActionNames(), ", ") + ". The server changes each task on its own, with " +
			"its own audit entry and event, so a task that is refused leaves the others changed. One " +
			"outcome per task is printed and the exit status is non-zero when any task was refused.\n\n" +
			"move takes --to and walks the shortest route of each task's workflow, priority takes " +
			"--priority, tag-add and tag-remove take --tag, assign takes --assignee (empty clears), " +
			"copy takes --project and links each copy to its original, delete is a soft delete and " +
			"takes --cascade. Pass - as a reference to read references from standard input; at most " +
			"500 tasks per request.\n\n" +
			"Exit codes: 2 invalid input, 3 unknown reference, 5 permission denied, 6 illegal transition.",
		Example: "  tix task bulk move default-1 default-2 --to done\n" +
			"  tix task bulk priority default-1 default-2 --priority high\n" +
			"  tix task ls --filter tag:old -o ndjson | jq -r .ref | tix task bulk delete -",
		Args: minArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			in.Action = core.BulkAction(strings.ReplaceAll(args[0], "-", "_"))
			if !in.Action.Valid() {
				return usagef(cmd, "action %q is not one of %s", args[0], strings.Join(bulkActionNames(), ", "))
			}
			refs, err := readRefs(cmd, args[1:])
			if err != nil {
				return err
			}
			in.Refs = make([]string, len(refs))
			for i, ref := range refs {
				in.Refs[i] = ref.String()
			}
			if in.Priority, err = parsePriority(priority); err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			result, err := conn.Service.BulkTasks(ctx, in)
			if err != nil {
				return err
			}
			return g.renderBulk(cmd, result)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return bulkActionNames(), cobra.ShellCompDirectiveNoFileComp
			}
			return g.completeTaskRefs(cmd, args, toComplete)
		},
	}
	f := cmd.Flags()
	f.StringVar(&in.To, "to", "", "status to move to, for move")
	f.StringVar(&priority, "priority", "", "priority to set, for priority")
	f.StringVar(&in.Tag, "tag", "", "tag to add or remove, for tag-add and tag-remove")
	f.StringVar(&in.AssigneeActorID, "assignee", "", "actor handle or id to assign, empty to clear, for assign")
	f.StringVar(&in.ProjectRef, "project", "", "project to copy into, for copy")
	f.BoolVar(&in.Cascade, "cascade", false, "also delete each task's subtasks, for delete")
	registerCompletions(g, cmd, "tag", "project")
	_ = cmd.RegisterFlagCompletionFunc("to", flagCompletion(g.completeStatuses))
	return cmd
}

// renderBulk prints a bulk answer as one outcome per task and fails with the
// first refusal's kind, so the exit status says what went wrong first.
func (g *globals) renderBulk(cmd *cobra.Command, result *core.BulkResult) error {
	rows := make([]outcome, 0, len(result.Outcomes))
	var failure error
	for _, o := range result.Outcomes {
		row := outcome{Ref: o.Ref, Status: statusOK, Copy: o.CopyRef}
		if !o.Applied {
			row.Status, row.Code, row.Error = statusFailed, o.Code, o.Error
			if failure == nil {
				failure = core.Errorf(core.Kind(o.Code), "%s: %s", o.Ref, o.Error)
			}
		}
		rows = append(rows, row)
	}
	if err := g.render(cmd, rows); err != nil {
		return err
	}
	return failure
}
