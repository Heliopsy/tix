// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"github.com/heliopsy/tix/internal/core"
	"github.com/spf13/cobra"
)

// viewHelp names the views every command in the group takes.
const viewHelp = "A view is board (the task screen grouped by status, whose groups are states), " +
	"lanes (the task screen grouped by project, whose groups are project keys) or " +
	"project:KEY (one project's board, whose groups are the states of its workflow). " +
	"The arrangement is yours alone: it changes nothing anybody else sees, and it follows you " +
	"to the browser and the terminal interface."

// newViewCmd builds the view command group.
func newViewCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "view",
		Short:   "Arrange the columns and lanes of your views",
		Long:    "Arrange the columns and lanes of your views: hide, collapse, star and order them.\n\n" + viewHelp,
		GroupID: "work",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(viewLsCmd(g), viewSetCmd(g), viewOrderCmd(g), viewResetCmd(g))
	return cmd
}

func viewLsCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "ls VIEW",
		Short: "List your arrangement of a view",
		Long: "List the groups of a view you hid, collapsed, starred or moved. A group you never " +
			"touched is not listed.\n\n" + viewHelp + "\n\n" +
			"Exit codes: 2 unknown view, 3 unknown project, 5 permission denied.",
		Example: "  tix view ls board\n  tix view ls lanes -o json\n  tix view ls project:infra",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			prefs, err := conn.Service.ListGroupPrefs(ctx, args[0])
			if err != nil {
				return err
			}
			return g.render(cmd, prefs)
		},
	}
}

func viewSetCmd(g *globals) *cobra.Command {
	var hidden, collapsed, starred, dryRun bool
	cmd := &cobra.Command{
		Use:   "set VIEW GROUP",
		Short: "Hide, collapse or star one group of a view",
		Long: "Hide, collapse or star one column or lane of a view. Only the flags given are " +
			"changed; =false undoes one. A hidden group is not drawn, a collapsed one is drawn as " +
			"a strip with its name and count, and a starred one is drawn before the rest.\n\n" +
			viewHelp + "\n\n" +
			"Exit codes: 2 no flag given, unknown view or a group the view cannot hold, " +
			"3 unknown project, 5 permission denied.",
		Example: "  tix view set board done --hidden\n  tix view set lanes homelab --starred\n" +
			"  tix view set project:infra doing --collapsed=false",
		Args: exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var in core.GroupPrefInput
			flags := cmd.Flags()
			if flags.Changed("hidden") {
				in.Hidden = &hidden
			}
			if flags.Changed("collapsed") {
				in.Collapsed = &collapsed
			}
			if flags.Changed("starred") {
				in.Starred = &starred
			}
			if in.Hidden == nil && in.Collapsed == nil && in.Starred == nil {
				return usagef(cmd, "nothing to change: pass --hidden, --collapsed or --starred")
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("view.group.set", args[0]+" "+args[1], groupDetail(in)))
			}
			pref, err := conn.Service.SetGroupPref(ctx, args[0], args[1], in)
			if err != nil {
				return err
			}
			return g.render(cmd, pref)
		},
	}
	f := cmd.Flags()
	f.BoolVar(&hidden, "hidden", false, "hide the group; --hidden=false shows it again")
	f.BoolVar(&collapsed, "collapsed", false, "draw the group as a strip; --collapsed=false expands it")
	f.BoolVar(&starred, "starred", false, "draw the group first; --starred=false unstars it")
	f.BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

// groupDetail is what a dry run of set reports it would change.
func groupDetail(in core.GroupPrefInput) map[string]any {
	out := map[string]any{}
	if in.Hidden != nil {
		out["hidden"] = *in.Hidden
	}
	if in.Collapsed != nil {
		out["collapsed"] = *in.Collapsed
	}
	if in.Starred != nil {
		out["starred"] = *in.Starred
	}
	return out
}

func viewOrderCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "order VIEW GROUP...",
		Short: "Put groups of a view first, in the order given",
		Long: "Put the named groups of a view first, in the order given. Groups not named keep " +
			"the place they had. Starred groups are still drawn before the rest.\n\n" + viewHelp + "\n\n" +
			"Exit codes: 2 unknown view, a group named twice or one the view cannot hold, " +
			"3 unknown project, 5 permission denied.",
		Example: "  tix view order lanes work homelab\n  tix view order board doing todo",
		Args:    minArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			in := core.GroupOrderInput{Groups: args[1:]}
			if dryRun {
				return g.render(cmd, newPlan("view.order", args[0], map[string]any{"groups": in.Groups}))
			}
			prefs, err := conn.Service.OrderGroups(ctx, args[0], in)
			if err != nil {
				return err
			}
			return g.render(cmd, prefs)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

func viewResetCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "reset VIEW [GROUP]",
		Short: "Forget your arrangement of a view or one of its groups",
		Long: "Forget your arrangement of one group of a view, or of the whole view when no group " +
			"is named, so it is drawn as it was before you touched it.\n\n" + viewHelp + "\n\n" +
			"Exit codes: 2 unknown view or a group the view cannot hold, 3 unknown project, " +
			"5 permission denied.",
		Example: "  tix view reset board\n  tix view reset lanes homelab",
		Args:    rangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			group := ""
			if len(args) == 2 {
				group = args[1]
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			target := args[0]
			if group != "" {
				target += " " + group
			}
			if dryRun {
				return g.render(cmd, newPlan("view.reset", target, nil))
			}
			if err := conn.Service.ResetGroupPrefs(ctx, args[0], group); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: target, Status: statusOK})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}
