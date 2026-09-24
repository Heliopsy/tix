// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"fmt"
	"io"
	"strconv"
	"text/tabwriter"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/spf13/cobra"
)

func init() { builders = append(builders, newStatsCmd) }

// newStatsCmd builds the statistics command.
func newStatsCmd(g *globals) *cobra.Command {
	var project, since, window string
	var top, oldest int
	cmd := &cobra.Command{
		Use:     "stats",
		Short:   "Report throughput, ageing and who closed what",
		GroupID: "work",
		Long: "Report what was completed and created over a window ending now, how long work took, " +
			"where it is sitting, and who moved the most tasks to a terminal state.\n\n" +
			fmt.Sprintf("Exit codes: %d invalid window, %d unknown project, %d permission denied.",
				core.KindInvalid.ExitCode(), core.KindNotFound.ExitCode(), core.KindForbidden.ExitCode()),
		Example: "  tix stats\n  tix stats --window 720h -p infra\n  tix stats --since 2026-01-01 -o json",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			in := core.StatsInput{ProjectRef: project, TopActors: top, Oldest: oldest}
			if cmd.Flags().Changed("since") && cmd.Flags().Changed("window") {
				return usagef(cmd, "--since and --window contradict each other")
			}
			at, err := parseTime(since)
			if err != nil {
				return err
			}
			if at != nil {
				in.Since = *at
			}
			if in.Window, err = parseDuration(window); err != nil {
				return err
			}
			if err := in.Validate(); err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			stats, err := conn.Service.Stats(ctx, in)
			if err != nil {
				return err
			}
			if g.formatName() != output.FormatTable {
				return g.render(cmd, stats)
			}
			return writeStatsTable(cmd.OutOrStdout(), stats)
		},
	}
	f := cmd.Flags()
	f.StringVarP(&project, "project", "p", "", "project key to report on, instead of the whole tenant")
	f.StringVar(&since, "since", "", "start of the window, as RFC3339 or YYYY-MM-DD")
	f.StringVar(&window, "window", "", "how far back the window reaches, as a duration such as 720h")
	f.IntVar(&top, "top", 0, "how many actors the leaderboard names")
	f.IntVar(&oldest, "oldest", 0, "how many ageing tasks to report")
	registerCompletions(g, cmd, "project")
	return cmd
}

// writeStatsTable renders the figures for a person. It is written here rather
// than in internal/output because the leaderboard cannot be shown without the
// line saying what it counts, and a generic renderer has nowhere to put that.
func writeStatsTable(w io.Writer, s *core.Stats) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	scope := "whole tenant"
	if s.ProjectKey != "" {
		scope = "project " + s.ProjectKey
	}
	_, _ = fmt.Fprintf(tw, "WINDOW\t%s .. %s\t(%s)\n",
		s.Since.Format(core.StatsDayLayout), s.Until.Format(core.StatsDayLayout), scope)
	_, _ = fmt.Fprintf(tw, "COMPLETED\t%d\t\n", s.Completed)
	_, _ = fmt.Fprintf(tw, "CREATED\t%d\t\n", s.Created)
	_, _ = fmt.Fprintf(tw, "MEDIAN LEAD TIME\t%s\t\n", s.MedianLeadTime)
	_, _ = fmt.Fprintf(tw, "SLOWEST LEAD TIME\t%s\t\n", s.SlowestLeadTime)

	statsSection(tw, "COMPLETED PER DAY", len(s.PerDay))
	for _, day := range s.PerDay {
		_, _ = fmt.Fprintf(tw, "  %s\t%d\t\n", day.Date, day.Completed)
	}

	statsSection(tw, "TASKS BY STATE CATEGORY", len(s.ByCategory))
	for _, c := range s.ByCategory {
		_, _ = fmt.Fprintf(tw, "  %s\t%d\t\n", c.Category, c.Count)
	}

	_, _ = fmt.Fprintf(tw, "\nMOST ACTIVE ACTORS\t%s\t\n", statsMeasure(s))
	for _, a := range s.TopActors {
		name := a.Handle
		if name == "" {
			name = a.ActorID
		}
		_, _ = fmt.Fprintf(tw, "  %s\t%d\t\n", name, a.Moved)
	}
	if len(s.TopActors) == 0 {
		_, _ = fmt.Fprint(tw, "  none\t\t\n")
	}

	statsSection(tw, "OLDEST TASKS NOT YET IN A TERMINAL STATE", len(s.Oldest))
	for _, o := range s.Oldest {
		_, _ = fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n", o.Ref, o.Status, o.Age, o.Title)
	}
	return tw.Flush()
}

// statsMeasure states what the leaderboard counts, so the number is never shown
// bare. A ticket-closing count read as a productivity measure is worse than no
// leaderboard at all.
func statsMeasure(s *core.Stats) string {
	if s.LeaderboardMeasure == "" {
		return "(" + core.StatsLeaderboardMeasure + ")"
	}
	return "(" + s.LeaderboardMeasure + ")"
}

// statsSection writes a section heading, saying so when it has no rows.
func statsSection(w io.Writer, title string, rows int) {
	if rows == 0 {
		_, _ = fmt.Fprintf(w, "\n%s\t%s\t\n", title, "none")
		return
	}
	_, _ = fmt.Fprintf(w, "\n%s\t%s\t\n", title, strconv.Itoa(rows))
}
