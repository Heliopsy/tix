// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/core"
	"github.com/spf13/cobra"
)

// channelHelp says what a channel is, for every command in the group.
const channelHelp = "A channel is your own delivery endpoint for your inbox items, separate from the " +
	"tenant's webhooks: tix posts each item of the kinds it takes to its URL, shaped by its format " +
	"(generic for signed JSON, slack, discord or ntfy), and retries a refused delivery on the " +
	"webhook schedule. Only your items reach it, and nothing you muted."

// newInboxChannelCmd builds the inbox channel group.
func newInboxChannelCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channel",
		Short: "Send your inbox to Slack, Discord, ntfy or your own endpoint",
		Long:  "Manage your own notification channels.\n\n" + channelHelp,
		Args:  noArgs,
		RunE:  helpRunner,
	}
	cmd.AddCommand(channelLsCmd(g), channelAddCmd(g), channelEditCmd(g), channelRmCmd(g),
		channelTestCmd(g), channelDeliveriesCmd(g), channelRedeliverCmd(g))
	return cmd
}

func channelLsCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Short:   "List your channels",
		Long:    "List your notification channels. The signing secret is never listed.\n\n" + channelHelp + "\n\nExit codes: 5 permission denied.",
		Example: "  tix inbox channel ls\n  tix inbox channel ls -o json",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			out, err := conn.Service.ListInboxChannels(ctx)
			if err != nil {
				return err
			}
			return g.render(cmd, out)
		},
	}
}

func channelAddCmd(g *globals) *cobra.Command {
	var format string
	var kinds []string
	var off, dryRun bool
	cmd := &cobra.Command{
		Use:   "add NAME URL",
		Short: "Add a channel and print its signing secret once",
		Long: "Add a notification channel. The answer carries the channel's signing secret, which " +
			"no later command shows again.\n\n" + channelHelp + "\n\n" +
			"Exit codes: 2 invalid name, url, format or kind, 4 a channel of that name exists, " +
			"5 permission denied, 6 the most channels one person may keep.",
		Example: "  tix inbox channel add phone https://ntfy.sh/my-topic --format ntfy --kinds mention,assigned\n" +
			"  tix inbox channel add team https://hooks.slack.com/services/T0/B0/XYZ --format slack",
		Args: exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			active := !off
			in := core.InboxChannelInput{Name: args[0], URL: args[1], Format: format, Kinds: kinds, Active: &active}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("inbox.channel.add", args[0],
					map[string]any{"url": args[1], "format": format, "kinds": kinds, "active": active}))
			}
			out, err := conn.Service.CreateInboxChannel(ctx, in)
			if err != nil {
				return err
			}
			return g.render(cmd, out)
		},
	}
	f := cmd.Flags()
	f.StringVar(&format, "format", "generic", "generic, slack, discord or ntfy")
	f.StringSliceVar(&kinds, "kinds", nil, "kinds it receives: mention, assigned, status, comment (default all)")
	f.BoolVar(&off, "off", false, "add it turned off")
	f.BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

func channelEditCmd(g *globals) *cobra.Command {
	var name, url, format string
	var kinds []string
	var active, dryRun bool
	cmd := &cobra.Command{
		Use:   "edit ID",
		Short: "Change a channel",
		Long: "Change a channel's name, url, format, kinds or whether it is on. Only the flags given " +
			"are changed.\n\n" + channelHelp + "\n\n" +
			"Exit codes: 2 nothing to change or an invalid value, 3 no such channel of yours, " +
			"4 a channel of that name exists, 5 permission denied.",
		Example: "  tix inbox channel edit 01J000000000000000000A --kinds mention\n" +
			"  tix inbox channel edit 01J000000000000000000A --active=false",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var in core.InboxChannelUpdate
			fl := cmd.Flags()
			if fl.Changed("name") {
				in.Name = &name
			}
			if fl.Changed("url") {
				in.URL = &url
			}
			if fl.Changed("format") {
				in.Format = &format
			}
			if fl.Changed("kinds") {
				in.Kinds = &kinds
			}
			if fl.Changed("active") {
				in.Active = &active
			}
			if in == (core.InboxChannelUpdate{}) {
				return usagef(cmd, "nothing to change: pass --name, --url, --format, --kinds or --active")
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("inbox.channel.edit", args[0], nil))
			}
			out, err := conn.Service.UpdateInboxChannel(ctx, args[0], in)
			if err != nil {
				return err
			}
			return g.render(cmd, out)
		},
	}
	f := cmd.Flags()
	f.StringVar(&name, "name", "", "new name")
	f.StringVar(&url, "url", "", "new url")
	f.StringVar(&format, "format", "", "generic, slack, discord or ntfy")
	f.StringSliceVar(&kinds, "kinds", nil, "kinds it receives: mention, assigned, status, comment")
	f.BoolVar(&active, "active", true, "turn it on; --active=false turns it off")
	f.BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

func channelRmCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "rm ID",
		Short:   "Remove a channel and its delivery history",
		Long:    "Remove one of your channels and its deliveries.\n\nExit codes: 3 no such channel of yours, 5 permission denied.",
		Example: "  tix inbox channel rm 01J000000000000000000A",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("inbox.channel.rm", args[0], nil))
			}
			if err := conn.Service.DeleteInboxChannel(ctx, args[0]); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: args[0], Status: statusOK})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

func channelTestCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "test ID",
		Short: "Send a test message to a channel",
		Long: "Send a short test message to one of your channels and print the delivery. Where this " +
			"process delivers inline, the status says how the channel answered.\n\n" +
			"Exit codes: 3 no such channel of yours, 5 permission denied, 6 the channel is turned off.",
		Example: "  tix inbox channel test 01J000000000000000000A",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			d, err := conn.Service.TestInboxChannel(ctx, args[0])
			if err != nil {
				return err
			}
			return g.render(cmd, d)
		},
	}
}

func channelDeliveriesCmd(g *globals) *cobra.Command {
	var limit int
	var cursor string
	cmd := &cobra.Command{
		Use:     "deliveries ID",
		Short:   "List a channel's recent deliveries",
		Long:    "List one of your channels' deliveries, newest first, with status and the last answer.\n\nExit codes: 3 no such channel of yours, 5 permission denied.",
		Example: "  tix inbox channel deliveries 01J000000000000000000A",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			size, err := g.pageSize(cmd, limit)
			if err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			out, next, err := conn.Service.ListChannelDeliveries(ctx, core.ChannelDeliveryFilter{
				ChannelID: args[0], Page: core.Page{Limit: size, Cursor: cursor}})
			if err != nil {
				return err
			}
			if next != "" {
				g.diag(cmd, "more results available; next cursor %s", next)
			}
			return g.render(cmd, out)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", config.DefaultPageSize, "maximum records to return")
	cmd.Flags().StringVar(&cursor, "cursor", "", "continue from a previous page")
	return cmd
}

func channelRedeliverCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "redeliver DELIVERY",
		Short:   "Queue another attempt of a channel delivery",
		Long:    "Queue another attempt of one of your channels' deliveries; the ID is from tix inbox channel deliveries.\n\nExit codes: 3 no such delivery of yours, 5 permission denied.",
		Example: "  tix inbox channel redeliver 01J000000000000000000B",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("inbox.channel.redeliver", args[0], nil))
			}
			if err := conn.Service.RedeliverChannelDelivery(ctx, args[0]); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: args[0], Status: statusOK})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}
