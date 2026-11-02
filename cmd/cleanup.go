// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"fmt"
	"strconv"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/spf13/cobra"
)

// newCleanupCmd builds the housekeeping command group.
func newCleanupCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Inspect and change automatic cleanup of finished tasks and the trash",
		Long: "Housekeeping moves tasks finished in a done or cancelled state to the trash after a number of days, " +
			"and empties the trash of tasks deleted longer ago. Each rule is set for the tenant and can be " +
			"overridden per project. Every rule is off until set, and all of them pause while the tenant's " +
			"trash is off. tix serve applies them on the retention pruner's tick.",
		GroupID: "admin",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(cleanupShowCmd(g), cleanupSetCmd(g), cleanupPreviewCmd(g), cleanupRunCmd(g))
	return cmd
}

func cleanupShowCmd(g *globals) *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:     "show",
		Aliases: []string{"get"},
		Short:   "Show the cleanup rules and what they would act on now",
		Long: "Show the tenant's cleanup rules, or with --project the project's overrides and the rules in effect, " +
			"and how many tasks each would act on now.\n\n" +
			fmt.Sprintf("Exit codes: %d unknown project, %d permission denied.",
				core.KindNotFound.ExitCode(), core.KindForbidden.ExitCode()),
		Example: "  tix cleanup show\n  tix cleanup show --project OPS -o json",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			view, err := conn.Service.GetHousekeeping(ctx, project)
			if err != nil {
				return err
			}
			return g.renderHousekeeping(cmd, view)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "show one project's overrides and effective rules")
	return cmd
}

func cleanupSetCmd(g *globals) *cobra.Command {
	var in core.HousekeepingInput
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Change the cleanup rules for the tenant or one project",
		Long: "Change one or more cleanup rules. Each takes off or a number of days (30 or 30d); with --project " +
			"it also takes inherit, which hands the rule back to the tenant. A rule left unset keeps its value. " +
			"Turning on --done or --cancelled is refused while the tenant's trash is off, because those tasks " +
			"would be deleted for good.\n\n" +
			fmt.Sprintf("Exit codes: %d invalid value, %d unknown project, %d permission denied, %d the trash is off.",
				core.KindInvalid.ExitCode(), core.KindNotFound.ExitCode(), core.KindForbidden.ExitCode(),
				core.KindPrecondition.ExitCode()),
		Example: "  tix cleanup set --done 30 --cancelled 7 --trash 90\n" +
			"  tix cleanup set --project OPS --done off --trash inherit",
		Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if in.Done == "" && in.Cancelled == "" && in.Trash == "" {
				return usagef(cmd, "at least one of --done, --cancelled and --trash is required")
			}
			if dryRun {
				return g.render(cmd, newPlan("housekeeping.put", in.Project, map[string]any{
					"done": in.Done, "cancelled": in.Cancelled, "trash": in.Trash,
				}))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			view, err := conn.Service.PutHousekeeping(ctx, in)
			if err != nil {
				return err
			}
			return g.renderHousekeeping(cmd, view)
		},
	}
	f := cmd.Flags()
	f.StringVar(&in.Project, "project", "", "set this project's overrides instead of the tenant default")
	f.StringVar(&in.Done, "done", "", "days after which tasks in a done state go to the trash, or off, or inherit")
	f.StringVar(&in.Cancelled, "cancelled", "", "days after which tasks in a cancelled state go to the trash, or off, or inherit")
	f.StringVar(&in.Trash, "trash", "", "days after which tasks in the trash are deleted for good, or off, or inherit")
	f.BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

func cleanupPreviewCmd(g *globals) *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "preview",
		Short: "Count what cleanup would act on now, changing nothing",
		Long: "Count the tasks each cleanup rule would act on now, for the tenant or one project. Nothing is " +
			"written.\n\n" + fmt.Sprintf("Exit codes: %d unknown project, %d permission denied.",
			core.KindNotFound.ExitCode(), core.KindForbidden.ExitCode()),
		Example: "  tix cleanup preview\n  tix cleanup preview --project OPS",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return g.runHousekeeping(cmd, core.HousekeepingRunInput{Project: project, DryRun: true})
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "preview one project only")
	return cmd
}

func cleanupRunCmd(g *globals) *cobra.Command {
	in := core.HousekeepingRunInput{}
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Apply the cleanup rules once, now",
		Long: "Apply the cleanup rules once, at most --limit tasks per rule, the way tix serve does on every " +
			"retention tick.\n\n" + fmt.Sprintf("Exit codes: %d invalid limit, %d unknown project, %d permission denied.",
			core.KindInvalid.ExitCode(), core.KindNotFound.ExitCode(), core.KindForbidden.ExitCode()),
		Example: "  tix cleanup run\n  tix cleanup run --project OPS --limit 50",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return g.runHousekeeping(cmd, in)
		},
	}
	cmd.Flags().StringVar(&in.Project, "project", "", "apply to one project only")
	cmd.Flags().IntVar(&in.Limit, "limit", 0, "maximum tasks per rule, zero for the default of 200")
	cmd.Flags().BoolVar(&in.DryRun, "dry-run", false, "count without acting, like tix cleanup preview")
	return cmd
}

// runHousekeeping runs or previews one pass and reports it.
func (g *globals) runHousekeeping(cmd *cobra.Command, in core.HousekeepingRunInput) error {
	conn, ctx, err := g.dial(cmd)
	if err != nil {
		return err
	}
	res, err := conn.Service.RunHousekeeping(ctx, in)
	if err != nil {
		return err
	}
	if res.Paused {
		g.diag(cmd, "the trash is off, so cleanup is paused and acts on nothing")
	}
	if g.formatName() != output.FormatTable {
		return g.render(cmd, res)
	}
	return g.render(cmd, housekeepingRun{DryRun: res.DryRun, Paused: res.Paused,
		Done: res.Done, Cancelled: res.Cancelled, Trash: res.Trash})
}

// housekeepingRun is a run's table view, one column per rule.
type housekeepingRun struct {
	DryRun    bool `json:"dry_run"`
	Paused    bool `json:"paused"`
	Done      int  `json:"done"`
	Cancelled int  `json:"cancelled"`
	Trash     int  `json:"trash"`
}

// housekeepingRow is one rule of the table view.
type housekeepingRow struct {
	Rule      string `json:"rule"`
	Tenant    string `json:"tenant"`
	Project   string `json:"project,omitempty"`
	Effective string `json:"effective"`
	Pending   int    `json:"pending"`
}

// renderHousekeeping prints a policy as one row per rule in table form.
func (g *globals) renderHousekeeping(cmd *cobra.Command, v *core.HousekeepingView) error {
	if v.Paused {
		g.diag(cmd, "the trash is off, so cleanup is paused and acts on nothing")
	}
	if g.formatName() != output.FormatTable {
		return g.render(cmd, v)
	}
	rows := make([]housekeepingRow, 0, len(core.HousekeepingRules()))
	for _, r := range core.HousekeepingRules() {
		row := housekeepingRow{
			Rule: string(r), Tenant: ruleWord(v.Tenant.Rule(r), "off"),
			Effective: ruleWord(v.Effective.Rule(r), "off"), Pending: v.Pending.Of(r),
		}
		if v.Override != nil {
			row.Project = ruleWord(v.Override.Rule(r), "inherit")
		}
		rows = append(rows, row)
	}
	return g.render(cmd, rows)
}

// ruleWord renders a rule's days, or what an unset rule means at its level.
func ruleWord(days *int, unset string) string {
	switch {
	case days == nil:
		return unset
	case *days == 0:
		return "off"
	default:
		return strconv.Itoa(*days) + "d"
	}
}
