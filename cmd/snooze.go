// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/heliopsy/tix/internal/query"
)

// wakeResult reports what one wake pass cleared.
type wakeResult struct {
	Woken int `json:"woken" yaml:"woken"`
}

func taskSnoozeCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "snooze REF UNTIL",
		Short: "Hide a task from listings until later",
		Long: "Hide a task from every default listing, for everybody, until UNTIL. It comes back by " +
			"itself when the time passes; nothing has to run for that. A snoozed task is not handed " +
			"out by claim next and cannot be claimed by reference until it wakes.\n\n" +
			"UNTIL is " + strings.Join(query.UntilPresets, ", ") + " (three hours on, 09:00 tomorrow, " +
			"09:00 next Monday), a duration such as 90m, 2h, 3d or 1w, a date (09:00 that day) or a " +
			"timestamp, read in the local time zone. Snoozing a snoozed task moves its wake time.\n\n" +
			"is:snoozed lists snoozed tasks, snoozed:any lists them beside the rest.\n\n" +
			"Exit codes: 2 unreadable or past time, 3 unknown reference, 5 permission denied.",
		Example: "  tix task snooze ops-3 tomorrow\n  tix task snooze ops-3 3d\n" +
			"  tix task snooze ops-3 '2026-12-01 14:00'",
		Args: exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := parseRef(args[0])
			if err != nil {
				return err
			}
			until, err := query.ParseUntil(args[1], time.Now(), time.Local)
			if err != nil {
				return &usageError{cmd: cmd, err: err}
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("task.snooze", ref.String(),
					map[string]any{"snoozed_until": until.UTC()}))
			}
			task, err := conn.Service.SnoozeTask(ctx, ref, core.SnoozeInput{Until: until})
			if err != nil {
				return err
			}
			return g.render(cmd, task)
		},
		ValidArgsFunction: g.completeTaskRefs,
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report when the task would wake without writing")
	return cmd
}

func taskUnsnoozeCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "unsnooze REF",
		Short: "Bring a snoozed task back now",
		Long: "Clear a task's snooze so it lists and can be claimed again at once. Unsnoozing a task " +
			"that is not snoozed succeeds and changes nothing.\n\n" +
			"Exit codes: 3 unknown reference, 5 permission denied.",
		Example: "  tix task unsnooze ops-3",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := parseRef(args[0])
			if err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("task.unsnooze", ref.String(), nil))
			}
			task, err := conn.Service.UnsnoozeTask(ctx, ref)
			if err != nil {
				return err
			}
			return g.render(cmd, task)
		},
		ValidArgsFunction: g.completeTaskRefs,
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

func taskWakeCmd(g *globals) *cobra.Command {
	var limit int
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "wake",
		Short: "Clear snoozes that have run out and announce them",
		Long: "Clear every snooze whose time has passed, recording task.wake and emitting task.woke for " +
			"each. A woken task is already back in every listing; this writes it down and tells " +
			"subscribers. A running server does it every sweep interval, so this is for a store no " +
			"server is attached to.\n\nExit codes: 5 permission denied.",
		Example: "  tix task wake --limit 100",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("task.wake", "", map[string]any{"limit": limit}))
			}
			woken, err := conn.Service.WakeSnoozed(ctx, limit)
			if err != nil {
				return err
			}
			return g.render(cmd, wakeResult{Woken: woken})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum tasks to wake, zero for the default")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be woken without writing")
	return cmd
}

// noteSnoozed says on stderr until when a shown task sleeps, beside the table
// that has no column for it. Structured formats carry snoozed_until already.
func noteSnoozed(g *globals, cmd *cobra.Command, task *core.Task) {
	if !task.SnoozedAt(time.Now()) || g.formatName() != output.FormatTable {
		return
	}
	g.diag(cmd, "snoozed until: %s", task.SnoozedUntil.Local().Format("2006-01-02 15:04"))
}
