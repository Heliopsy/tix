// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/connect"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/demo"
	"github.com/heliopsy/tix/internal/output"
	"github.com/spf13/cobra"
)

func init() { builders = append(builders, newDemoCmd) }

// newDemoCmd builds the demo command group.
func newDemoCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "demo",
		Short:   "Fill a database with demonstration data",
		GroupID: "setup",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(demoSeedCmd(g))
	return cmd
}

func demoSeedCmd(g *globals) *cobra.Command {
	var days int
	var reset bool
	cmd := &cobra.Command{
		Use:   "seed",
		Short: "Seed backdated projects, people and tasks",
		Long: "Seed a database with a backlog spanning the last few months: projects, people, agents, " +
			"tasks with bodies, tags, due dates and comments, and completions spread across the window " +
			"by different actors, so the statistics and the boards have something to show.\n\n" +
			"The history is replayed through the ordinary write path against a clock that is advanced, " +
			"so it carries the audit trail the statistics are derived from.\n\n" +
			fmt.Sprintf("Exit codes: %d invalid window, %d the database already holds data.",
				core.KindInvalid.ExitCode(), core.KindConflict.ExitCode()),
		Example: "  tix demo seed --db /tmp/demo.db\n  tix demo seed --db /tmp/demo.db --days 30 --reset",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			now := time.Now().UTC()
			opts := demo.Options{Days: days, Now: now}
			resolved, err := g.resolve()
			if err != nil {
				return err
			}
			clk := clock.NewFake(now)
			overrides := g.overrides()
			overrides.Clock = clk
			conn, err := connect.Dial(cmd.Context(), resolved, overrides)
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()
			ctx := conn.Context(cmd.Context())

			populated, err := demo.HasData(ctx, conn.Service)
			if err != nil {
				return err
			}
			if populated && !reset {
				return core.Conflict("%s already holds tasks or users; "+
					"re-run with --reset to replace them, or point --db at a scratch database",
					conn.Info.Target.Describe())
			}
			if populated {
				if err := demo.Reset(ctx, conn.Service); err != nil {
					return err
				}
			}
			summary, err := demo.Seed(ctx, conn.Service, clk, conn.Actor, opts)
			if err != nil {
				return err
			}
			if g.formatName() != output.FormatTable {
				return g.render(cmd, summary)
			}
			return writeDemoSummary(cmd.OutOrStdout(), summary)
		},
	}
	f := cmd.Flags()
	f.IntVar(&days, "days", demo.DefaultDays, "how many days of history to write")
	f.BoolVar(&reset, "reset", false, "delete the tasks, projects and users already there first")
	return cmd
}

// writeDemoSummary renders the counts for a person. Through the generic
// renderer the two instants printed as Go's own rendering of a time, so a
// command whose whole purpose is to make the product presentable finished by
// showing "2026-06-27 11:32:15.832021175 +0000 UTC". The machine formats still
// carry the full instants, which is where that precision belongs.
func writeDemoSummary(w io.Writer, s *demo.Summary) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(tw, "WINDOW\t%s .. %s\t\n",
		s.Since.Format(core.StatsDayLayout), s.Until.Format(core.StatsDayLayout))
	for _, row := range []struct {
		label string
		n     int
	}{
		{"PROJECTS", s.Projects},
		{"ACTORS", s.Actors},
		{"CUSTOM FIELDS", s.FieldDefs},
		{"TASKS", s.Tasks},
		{"COMPLETED", s.Completed},
		{"COMMENTS", s.Comments},
		{"DEPENDENCIES", s.Dependencies},
	} {
		_, _ = fmt.Fprintf(tw, "%s\t%d\t\n", row.label, row.n)
	}
	return tw.Flush()
}
