// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// taskCheckCmd builds `task check` and `task uncheck`, which tick or clear one
// checklist item of a task's description.
func taskCheckCmd(g *globals, done bool) *cobra.Command {
	var (
		version int
		dryRun  bool
	)
	use, short := "uncheck", "Clear one checklist item of a task"
	if done {
		use, short = "check", "Tick one checklist item of a task"
	}
	cmd := &cobra.Command{
		Use:   use + " REF N",
		Short: short,
		Long: short + ". A checklist item is a line of the description written - [ ] or - [x]; " +
			"N counts them from 1 in reading order, nested items included, as tix task show " +
			"reports them. Only that item's marker changes. An item already in that state " +
			"succeeds and changes nothing.\n\n" +
			"Exit codes: 2 invalid input or no such item, 3 unknown reference, 4 version conflict, 5 permission denied.",
		Example: "  tix task " + use + " default-1 2\n  tix task " + use + " default-1 2 --version 7",
		Args:    exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := parseRef(args[0])
			if err != nil {
				return err
			}
			item, err := strconv.Atoi(strings.TrimSpace(args[1]))
			if err != nil || item < 1 {
				return usagef(cmd, "checklist item %q must be a number from 1", args[1])
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				if _, err := conn.Service.GetTask(ctx, ref); err != nil {
					return err
				}
				return g.render(cmd, newPlan("task.update", ref.String(), nil))
			}
			task, err := conn.Service.CheckTaskItem(ctx, ref, core.CheckItemInput{Item: item, Done: done, Version: version})
			if err != nil {
				return err
			}
			if err := g.render(cmd, task); err != nil {
				return err
			}
			noteChecklist(g, cmd, task)
			return nil
		},
		ValidArgsFunction: g.completeTaskRefs,
	}
	cmd.Flags().IntVar(&version, "version", 0, "expected version for optimistic locking")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

// noteChecklist states how much of a task's checklist is ticked, in table
// output only, when the description has one.
func noteChecklist(g *globals, cmd *cobra.Command, task *core.Task) {
	if task.Checklist == nil || g.formatName() != output.FormatTable {
		return
	}
	g.diag(cmd, "checklist: %d/%d done", task.Checklist.Done, task.Checklist.Total)
}
