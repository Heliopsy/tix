// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"slices"
	"strings"

	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/core"
	"github.com/spf13/cobra"
)

// inboxHelp says what reaches an inbox, for every command in the group.
const inboxHelp = "Your inbox holds an item when somebody mentions you as @handle in a comment or " +
	"a description (mention), assigns you a task (assigned), comments on a task you are assigned " +
	"to or starred (comment), or moves a task assigned to you (status). Your own changes never " +
	"reach it, and a kind you muted with tix inbox prefs creates nothing."

// newInboxCmd builds the inbox command group.
func newInboxCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "inbox",
		Short:   "Read the mentions, assignments and changes that concern you",
		Long:    "Read the mentions, assignments, comments and status changes that concern you.\n\n" + inboxHelp,
		GroupID: "work",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(inboxLsCmd(g), inboxCountCmd(g), inboxReadCmd(g, true), inboxReadCmd(g, false),
		inboxReadAllCmd(g), inboxPrefsCmd(g))
	return cmd
}

func inboxLsCmd(g *globals) *cobra.Command {
	var unread bool
	var limit int
	var cursor string
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List your inbox, newest first",
		Long: "List your inbox items, newest first, each with its task, who caused it, what " +
			"happened and when.\n\n" + inboxHelp + "\n\nExit codes: 2 bad cursor, 5 permission denied.",
		Example: "  tix inbox ls\n  tix inbox ls --unread -o json",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			size, err := g.pageSize(cmd, limit)
			if err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			items, next, err := conn.Service.ListInbox(ctx, core.InboxFilter{Unread: unread,
				Page: core.Page{Limit: size, Cursor: cursor}})
			if err != nil {
				return err
			}
			if next != "" {
				g.diag(cmd, "more results available; next cursor %s", next)
			}
			return g.render(cmd, items)
		},
	}
	cmd.Flags().BoolVar(&unread, "unread", false, "list only unread items")
	cmd.Flags().IntVar(&limit, "limit", config.DefaultPageSize, "maximum records to return")
	cmd.Flags().StringVar(&cursor, "cursor", "", "continue from a previous page")
	return cmd
}

func inboxCountCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "count",
		Short:   "Count your unread inbox items",
		Long:    "Count your unread inbox items.\n\n" + inboxHelp + "\n\nExit codes: 5 permission denied.",
		Example: "  tix inbox count\n  tix inbox count -o json",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			n, err := conn.Service.CountInbox(ctx)
			if err != nil {
				return err
			}
			return g.render(cmd, n)
		},
	}
}

func inboxReadCmd(g *globals, read bool) *cobra.Command {
	use, short, action := "read ID", "Mark one inbox item read", "inbox.read"
	if !read {
		use, short, action = "unread ID", "Mark one inbox item unread", "inbox.unread"
	}
	var dryRun bool
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Long: short + ". The identifier is the ID column of tix inbox ls. An item already in that " +
			"state is left as it is.\n\nExit codes: 3 no such item of yours, 5 permission denied.",
		Example: "  tix inbox " + use[:len(use)-3] + " 01J000000000000000000A",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan(action, args[0], nil))
			}
			mark := conn.Service.ReadInboxItem
			if !read {
				mark = conn.Service.UnreadInboxItem
			}
			item, err := mark(ctx, args[0])
			if err != nil {
				return err
			}
			return g.render(cmd, item)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

func inboxReadAllCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "read-all",
		Short:   "Mark every inbox item read",
		Long:    "Mark every one of your inbox items read.\n\nExit codes: 5 permission denied.",
		Example: "  tix inbox read-all",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("inbox.read_all", "inbox", nil))
			}
			out, err := conn.Service.ReadAllInbox(ctx)
			if err != nil {
				return err
			}
			return g.render(cmd, out)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

func inboxPrefsCmd(g *globals) *cobra.Command {
	var mute, unmute []string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "prefs",
		Short: "Show or change the kinds of inbox item you muted",
		Long: "Show the kinds of inbox item you muted, or change them with --mute and --unmute. " +
			"The kinds are mention, assigned, status and comment.\n\n" + inboxHelp + "\n\n" +
			"Exit codes: 2 unknown kind, 5 permission denied.",
		Example: "  tix inbox prefs\n  tix inbox prefs --mute comment,status\n  tix inbox prefs --unmute status",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			prefs, err := conn.Service.GetInboxPrefs(ctx)
			if err != nil || len(mute)+len(unmute) == 0 {
				if err != nil {
					return err
				}
				return g.render(cmd, prefs)
			}
			in := core.InboxPrefsInput{Muted: mutedAfter(prefs.Muted, mute, unmute)}
			if dryRun {
				return g.render(cmd, newPlan("inbox.prefs", "inbox", map[string]any{"muted": in.Muted}))
			}
			out, err := conn.Service.PutInboxPrefs(ctx, in)
			if err != nil {
				return err
			}
			return g.render(cmd, out)
		},
	}
	cmd.Flags().StringSliceVar(&mute, "mute", nil, "kinds to mute: mention, assigned, status, comment")
	cmd.Flags().StringSliceVar(&unmute, "unmute", nil, "kinds to stop muting")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

// mutedAfter is the muted set once mute is added and unmute taken away.
func mutedAfter(have []core.InboxKind, mute, unmute []string) []string {
	out := []string{}
	for _, k := range have {
		out = append(out, string(k))
	}
	for _, k := range mute {
		k = strings.ToLower(strings.TrimSpace(k))
		if !slices.Contains(out, k) {
			out = append(out, k)
		}
	}
	return slices.DeleteFunc(out, func(k string) bool {
		return slices.ContainsFunc(unmute, func(u string) bool { return strings.EqualFold(strings.TrimSpace(u), k) })
	})
}
