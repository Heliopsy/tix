// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"context"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/spf13/cobra"
)

// repeatHelp is the one paragraph both task add and task edit give the rule.
const repeatHelp = "A rule reads " + core.RecurrenceSyntax + "; daily, weekly, monthly and yearly are " +
	"shorthands. The next due date counts from the previous one, or from completion with " +
	"\"after done\" or when the task has none. Moving to a cancelled state ends the series."

// noteSuccessor names the occurrence a finished recurring task spawned, under
// a table. Structured output already carries recurrence_next_id, and a
// successor that has since been purged is simply not named.
func noteSuccessor(ctx context.Context, g *globals, cmd *cobra.Command, svc core.Service, task *core.Task) {
	if task.RecurrenceNextID == "" || g.formatName() != output.FormatTable {
		return
	}
	next, err := svc.GetTask(ctx, core.TaskRef{ID: task.RecurrenceNextID})
	if err != nil {
		return
	}
	g.diag(cmd, "next occurrence: %s", output.ShowRef(next.Ref))
}

// noteCopiedFrom names the task a copy was copied from, in table output only,
// when the reader can still see it.
func noteCopiedFrom(ctx context.Context, g *globals, cmd *cobra.Command, svc core.Service, task *core.Task) {
	if task.CopiedFromID == "" || g.formatName() != output.FormatTable {
		return
	}
	source, err := svc.GetTask(ctx, core.TaskRef{ID: task.CopiedFromID})
	if err != nil {
		return
	}
	g.diag(cmd, "copied from: %s", output.ShowRef(source.Ref))
}
