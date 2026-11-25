// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// taskMoveCmd builds `task move`, which moves tasks into another project.
func taskMoveCmd(g *globals) *cobra.Command {
	var (
		in     core.MoveTaskInput
		dryRun bool
	)
	cmd := &cobra.Command{
		Use:   "move REF... --to PROJECT",
		Short: "Move tasks to another project",
		Long: "Move a task into another project. It keeps its identifier, comments, artifacts, history, " +
			"dependencies, tags and assignee, and takes the next number in the target project. Its old " +
			"reference keeps resolving to it, so links and mentions still work.\n\n" +
			"The status is kept when the target workflow has a state of the same name, else the task starts " +
			"in the target's initial state; the result says which. Custom fields the target does not define " +
			"are dropped and named in the result. Subtasks stay where they are and keep their parent.\n\n" +
			"A task claimed by somebody else is refused until the claim is released; your own claim moves " +
			"with the task. To move a task to another status, use tix task mv.\n\n" +
			"Given several references, separate arguments or a comma separated list, each is moved and one " +
			"outcome per task is printed. Pass - to read references from standard input.\n\n" +
			"Exit codes: 2 invalid input or a required field the target lacks, 3 unknown reference or project, " +
			"4 claimed by somebody else, 5 permission denied, 6 archived target project.",
		Example: "  tix task move infra-4 --to web\n  tix task move infra-4,infra-7 --to web\n" +
			"  echo infra-4 infra-7 | tix task move - --to web --dry-run",
		Args: minArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			refs, err := readRefs(cmd, splitCommas(args))
			if err != nil {
				return err
			}
			if err := in.Validate(); err != nil {
				return core.Invalid("--to names the project to move into")
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			detail := map[string]any{"to": in.ProjectRef}
			if len(refs) > 1 {
				if dryRun {
					return g.dryBulk(cmd, conn, ctx, refs, "task.move", detail)
				}
				return g.bulkMoves(cmd, refs, func(ref core.TaskRef) (string, error) {
					moved, err := conn.Service.MoveTask(ctx, ref, in)
					if err != nil {
						return "", err
					}
					return moved.Task.Ref, nil
				})
			}
			if dryRun {
				if _, err := conn.Service.GetTask(ctx, refs[0]); err != nil {
					return err
				}
				if _, err := conn.Service.GetProject(ctx, in.ProjectRef); err != nil {
					return err
				}
				return g.render(cmd, newPlan("task.move", refs[0].String(), detail))
			}
			moved, err := conn.Service.MoveTask(ctx, refs[0], in)
			if err != nil {
				return err
			}
			if g.formatName() != output.FormatTable {
				return g.render(cmd, moved)
			}
			return g.render(cmd, moveRow{
				Ref: moved.Task.Ref, From: moved.FromRef, Status: moved.Task.Status,
				Kept: moved.StatusKept, Dropped: strings.Join(moved.DroppedFields, ","),
			})
		},
		ValidArgsFunction: g.completeTaskRefs,
	}
	f := cmd.Flags()
	f.StringVar(&in.ProjectRef, "to", "", "project to move into (required)")
	f.BoolVar(&dryRun, "dry-run", false, "report what would be moved without writing")
	_ = cmd.RegisterFlagCompletionFunc("to", flagCompletion(g.completeProjectKeys))
	return cmd
}

// moveRow is a move as the table shows it: the task's reference before and
// after, the status it landed in and what stayed behind.
type moveRow struct {
	Ref     string `json:"ref" yaml:"ref" show:"ref"`
	From    string `json:"from_ref" yaml:"from_ref" show:"ref"`
	Status  string `json:"status" yaml:"status"`
	Kept    bool   `json:"status_kept" yaml:"status_kept"`
	Dropped string `json:"dropped_fields,omitempty" yaml:"dropped_fields,omitempty"`
}

// splitCommas expands each argument holding a comma separated list of
// references into one argument per reference.
func splitCommas(args []string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		for _, part := range strings.Split(arg, ",") {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}
