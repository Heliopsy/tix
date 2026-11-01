// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"github.com/heliopsy/tix/internal/connect"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
	"github.com/spf13/cobra"
)

// calendarHelp says what a feed is, once, for every command in the group.
const calendarHelp = "A calendar feed is a private iCalendar address that Google Calendar, Apple Calendar or " +
	"Outlook subscribes to. Each task with a due date is an all-day event linking back to the task. " +
	"Without a filter the feed lists the tasks assigned to you that are not finished; with one it " +
	"lists what the filter expression selects, the same language as tix task ls --filter.\n\n" +
	"The address carries a secret that reads the feed and nothing else. It is shown when the feed is " +
	"created or rotated and never again, because only its hash is kept. Anybody holding it reads what " +
	"it lists, so rotate it if it leaks."

// newCalendarCmd builds the calendar command group.
func newCalendarCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "calendar",
		Aliases: []string{"cal"},
		Short:   "Subscribe to your due dates from a calendar app",
		Long:    "Manage your calendar feed of due dates.\n\n" + calendarHelp,
		GroupID: "work",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(calendarShowCmd(g), calendarCreateCmd(g), calendarRotateCmd(g),
		calendarFilterCmd(g), calendarRevokeCmd(g))
	return cmd
}

func calendarShowCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "show",
		Aliases: []string{"status"},
		Short:   "Show your calendar feed",
		Long: "Show whether you have a calendar feed, since when and what it lists. The address is not " +
			"shown: it was printed once, when the feed was created or rotated.\n\n" + calendarHelp + "\n\n" +
			"Exit codes: 3 you have no feed, 5 permission denied.",
		Example: "  tix calendar show\n  tix calendar show -o json",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			feed, err := conn.Service.GetCalendarFeed(ctx)
			if err != nil {
				return err
			}
			return g.render(cmd, feed)
		},
	}
}

func calendarCreateCmd(g *globals) *cobra.Command {
	var filter string
	cmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"url", "new"},
		Short:   "Create your calendar feed and print its address",
		Long: "Create your calendar feed and print its address, once. Paste it into your calendar app's " +
			"subscribe-by-URL. A feed you already have is not replaced: rotate it for a new address.\n\n" +
			calendarHelp + "\n\n" +
			"Exit codes: 2 the filter does not parse, or you hold no membership in this tenant, " +
			"3 the filter names something that does not exist, 4 you already have a feed, 5 permission denied.",
		Example: "  tix calendar create\n  tix calendar create --filter 'project:infra -status:done'",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			feed, err := conn.Service.CreateCalendarFeed(ctx, core.CalendarFeedInput{Filter: filter})
			if err != nil {
				return err
			}
			return g.renderIssuedFeed(cmd, conn, feed)
		},
	}
	cmd.Flags().StringVar(&filter, "filter", "", "filter expression the feed lists; empty for your unfinished tasks")
	return cmd
}

func calendarRotateCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "rotate",
		Short: "Replace your calendar feed's address",
		Long: "Issue a new address for your calendar feed, keeping its filter, and stop the old one at " +
			"once. Calendars subscribed to the old address stop updating.\n\n" + calendarHelp + "\n\n" +
			"Exit codes: 3 you have no feed, 5 permission denied.",
		Example: "  tix calendar rotate",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			feed, err := conn.Service.RotateCalendarFeed(ctx)
			if err != nil {
				return err
			}
			return g.renderIssuedFeed(cmd, conn, feed)
		},
	}
}

func calendarFilterCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "filter [EXPR]",
		Short: "Change what your calendar feed lists",
		Long: "Change the filter expression your calendar feed lists with. With no expression the feed " +
			"goes back to the tasks assigned to you that are not finished. The address stays the same.\n\n" +
			calendarHelp + "\n\n" +
			"Exit codes: 2 the filter does not parse, 3 you have no feed or the filter names something " +
			"that does not exist, 5 permission denied.",
		Example: "  tix calendar filter 'tag:ops'\n  tix calendar filter",
		Args:    rangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var in core.CalendarFeedInput
			if len(args) == 1 {
				in.Filter = args[0]
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			feed, err := conn.Service.SetCalendarFeedFilter(ctx, in)
			if err != nil {
				return err
			}
			return g.render(cmd, feed)
		},
	}
}

func calendarRevokeCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "revoke",
		Aliases: []string{"rm"},
		Short:   "Stop your calendar feed",
		Long: "Stop your calendar feed. Its address answers not found from now on, and subscribed " +
			"calendars keep only the events they last fetched.\n\n" + calendarHelp + "\n\n" +
			"Exit codes: 3 you have no feed, 5 permission denied.",
		Example: "  tix calendar revoke",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if err := conn.Service.RevokeCalendarFeed(ctx); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: "calendar", Status: statusOK})
		},
	}
}

// renderIssuedFeed prints a freshly issued feed. A server answers with the
// absolute address it was reached at; a local database has no address, so
// the path is printed with a note on what to put in front of it.
func (g *globals) renderIssuedFeed(cmd *cobra.Command, conn *connect.Conn, feed *core.IssuedCalendarFeed) error {
	if feed.Path == "" {
		feed.Path = wire.CalendarFeedPath(feed.Token)
	}
	g.diag(cmd, "the feed address is shown once and cannot be retrieved again")
	if feed.URL == "" && conn.Info.Target.Mode != connect.ModeRemote {
		g.diag(cmd, "this is a local database: put the address tix serve answers on in front of the path")
	}
	return g.render(cmd, feed)
}
