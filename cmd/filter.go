// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"github.com/heliopsy/tix/internal/core"
	"github.com/spf13/cobra"
)

// filterHelp says what a saved filter is, for every command in the group.
const filterHelp = "A saved filter is a named filter expression with the view (list, board or lanes) " +
	"and the sort the task screen opens with. It is yours: it is listed in the browser's sidebar " +
	"and the terminal interface's filter picker, and tix task ls --saved NAME starts from it. " +
	"A shared filter is listed for everybody in the tenant; only its owner or an administrator " +
	"may change it. A filter is named by its name or its id."

// newFilterCmd builds the saved filter command group.
func newFilterCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "filter",
		Short:   "Keep named task filters",
		Long:    "Keep named task filters: save, list, rename, share, order and remove them.\n\n" + filterHelp,
		GroupID: "work",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(filterLsCmd(g), filterShowCmd(g), filterSaveCmd(g), filterSetCmd(g),
		filterRenameCmd(g), filterOrderCmd(g), filterRmCmd(g))
	return cmd
}

func filterLsCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List your saved filters and the shared ones",
		Long: "List your saved filters in your order, then the ones others shared with the tenant, " +
			"by name.\n\n" + filterHelp + "\n\nExit codes: 5 permission denied.",
		Example: "  tix filter ls\n  tix filter ls -o json",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			filters, err := conn.Service.ListSavedFilters(ctx)
			if err != nil {
				return err
			}
			return g.render(cmd, filters)
		},
	}
}

func filterShowCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "show NAME",
		Short:   "Show one saved filter",
		Long:    "Show one saved filter you own or that was shared with you.\n\n" + filterHelp + "\n\nExit codes: 2 a name two shared filters carry, 3 no such filter, 5 permission denied.",
		Example: "  tix filter show ops\n  tix filter show ops -o yaml",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			f, err := conn.Service.GetSavedFilter(ctx, args[0])
			if err != nil {
				return err
			}
			return g.render(cmd, f)
		},
	}
}

func filterSaveCmd(g *globals) *cobra.Command {
	var in core.SavedFilterInput
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "save NAME",
		Short: "Save a filter under a name",
		Long: "Save a filter expression under a name, with the view and sort the task screen opens " +
			"it in. Names are unique among your own filters, without regard to case.\n\n" + filterHelp + "\n\n" +
			"Exit codes: 2 a bad name, view, sort or expression, 4 a name you already use, 5 permission denied.",
		Example: "  tix filter save ops --filter 'tag:ops -status:done'\n" +
			"  tix filter save 'my week' --filter 'assignee:me due:week' --view list --sort due_at\n" +
			"  tix filter save team --filter 'status:review' --view board --shared",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			in.Name = args[0]
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("filter.save", in.Name, map[string]any{
					"query": in.Query, "view": in.View, "sort": in.Sort, "shared": in.Shared}))
			}
			f, err := conn.Service.CreateSavedFilter(ctx, in)
			if err != nil {
				return err
			}
			return g.render(cmd, f)
		},
	}
	f := cmd.Flags()
	f.StringVar(&in.Query, "filter", "", "the filter expression, such as 'status:todo -tag:ops'")
	f.StringVar(&in.View, "view", "", "the view it opens in: list, board or lanes; empty keeps the reader's")
	f.StringVar(&in.Sort, "sort", "", "the sort it opens with; empty keeps the default")
	f.BoolVar(&in.Shared, "shared", false, "list it for everybody in the tenant")
	f.BoolVar(&dryRun, "dry-run", false, "report what would be saved without writing")
	_ = cmd.RegisterFlagCompletionFunc("sort", fixedCompletion(core.TaskSortFields))
	_ = cmd.RegisterFlagCompletionFunc("view", fixedCompletion(core.FilterViews[1:]))
	return cmd
}

func filterSetCmd(g *globals) *cobra.Command {
	var name, expr, view, sort string
	var shared, dryRun bool
	cmd := &cobra.Command{
		Use:   "set NAME",
		Short: "Change a saved filter",
		Long: "Change a saved filter's name, expression, view, sort or sharing. Only the flags " +
			"given are changed; --shared=false stops sharing it.\n\n" + filterHelp + "\n\n" +
			"Exit codes: 2 no flag given or a bad value, 3 no such filter, 4 a name its owner already " +
			"uses, 5 not yours to change.",
		Example: "  tix filter set ops --filter 'tag:ops status:todo'\n  tix filter set ops --shared\n" +
			"  tix filter set ops --view board --sort priority",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var in core.SavedFilterUpdate
			flags := cmd.Flags()
			if flags.Changed("name") {
				in.Name = &name
			}
			if flags.Changed("filter") {
				in.Query = &expr
			}
			if flags.Changed("view") {
				in.View = &view
			}
			if flags.Changed("sort") {
				in.Sort = &sort
			}
			if flags.Changed("shared") {
				in.Shared = &shared
			}
			if in == (core.SavedFilterUpdate{}) {
				return usagef(cmd, "nothing to change: pass --name, --filter, --view, --sort or --shared")
			}
			return runFilterUpdate(g, cmd, args[0], in, dryRun)
		},
	}
	f := cmd.Flags()
	f.StringVar(&name, "name", "", "a new name")
	f.StringVar(&expr, "filter", "", "a new filter expression")
	f.StringVar(&view, "view", "", "the view it opens in: list, board, lanes, or empty")
	f.StringVar(&sort, "sort", "", "the sort it opens with, or empty")
	f.BoolVar(&shared, "shared", false, "list it for everybody in the tenant; --shared=false stops")
	f.BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	_ = cmd.RegisterFlagCompletionFunc("sort", fixedCompletion(core.TaskSortFields))
	_ = cmd.RegisterFlagCompletionFunc("view", fixedCompletion(core.FilterViews[1:]))
	return cmd
}

func filterRenameCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "rename NAME NEW-NAME",
		Short:   "Rename a saved filter",
		Long:    "Rename a saved filter. The same as tix filter set NAME --name NEW-NAME.\n\n" + filterHelp + "\n\nExit codes: 2 a bad name, 3 no such filter, 4 a name its owner already uses, 5 not yours to change.",
		Example: "  tix filter rename ops 'ops board'",
		Args:    exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFilterUpdate(g, cmd, args[0], core.SavedFilterUpdate{Name: &args[1]}, dryRun)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

func runFilterUpdate(g *globals, cmd *cobra.Command, ref string, in core.SavedFilterUpdate, dryRun bool) error {
	conn, ctx, err := g.dial(cmd)
	if err != nil {
		return err
	}
	if dryRun {
		return g.render(cmd, newPlan("filter.update", ref, filterDetail(in)))
	}
	f, err := conn.Service.UpdateSavedFilter(ctx, ref, in)
	if err != nil {
		return err
	}
	return g.render(cmd, f)
}

// filterDetail is what a dry run of set reports it would change.
func filterDetail(in core.SavedFilterUpdate) map[string]any {
	out := map[string]any{}
	if in.Name != nil {
		out["name"] = *in.Name
	}
	if in.Query != nil {
		out["query"] = *in.Query
	}
	if in.View != nil {
		out["view"] = *in.View
	}
	if in.Sort != nil {
		out["sort"] = *in.Sort
	}
	if in.Shared != nil {
		out["shared"] = *in.Shared
	}
	return out
}

func filterOrderCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "order NAME...",
		Short: "Put your saved filters in an order",
		Long: "Put the named filters of your own first, in the order given; the rest of yours " +
			"follow in the order they had. Shared filters others own are listed after yours.\n\n" +
			filterHelp + "\n\nExit codes: 2 a filter named twice, 3 a filter you do not own, 5 permission denied.",
		Example: "  tix filter order 'my week' ops",
		Args:    minArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			in := core.SavedFilterOrderInput{Filters: args}
			if dryRun {
				return g.render(cmd, newPlan("filter.order", "", map[string]any{"filters": args}))
			}
			filters, err := conn.Service.OrderSavedFilters(ctx, in)
			if err != nil {
				return err
			}
			return g.render(cmd, filters)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

func filterRmCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "rm NAME",
		Aliases: []string{"delete"},
		Short:   "Remove a saved filter",
		Long:    "Remove a saved filter. The tasks it selected are not touched.\n\n" + filterHelp + "\n\nExit codes: 3 no such filter, 5 not yours to remove.",
		Example: "  tix filter rm ops",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("filter.delete", args[0], nil))
			}
			if err := conn.Service.DeleteSavedFilter(ctx, args[0]); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: args[0], Status: statusOK})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}
