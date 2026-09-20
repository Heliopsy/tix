package cmd

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/thereisnotime/tix/internal/core"
)

// newDepCmd builds the dependency command group.
func newDepCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "dep",
		Short:   "Manage task dependencies",
		GroupID: "work",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(depAddCmd(g), depRmCmd(g), depLsCmd(g))
	return cmd
}

func depAddCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "add REF DEPENDS_ON",
		Short:   "Make one task depend on another",
		Long:    "Record that REF waits for DEPENDS_ON to reach a terminal state.\n\nExit codes: 3 unknown reference, 6 dependency cycle.",
		Example: "  tix dep add default-2 default-1",
		Args:    exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, dep, err := twoRefs(args)
			if err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				return g.dryBulk(cmd, conn, ctx, []core.TaskRef{ref, dep}, "dependency.add", nil)
			}
			if err := conn.Service.AddDependency(ctx, ref, dep); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: ref.String(), Status: statusOK})
		},
		ValidArgsFunction: g.completeTaskRefs,
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

func depRmCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "rm REF DEPENDS_ON",
		Short:   "Remove a dependency",
		Long:    "Remove the edge recording that REF waits for DEPENDS_ON.\n\nExit codes: 3 unknown reference.",
		Example: "  tix dep rm default-2 default-1",
		Args:    exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, dep, err := twoRefs(args)
			if err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				return g.dryBulk(cmd, conn, ctx, []core.TaskRef{ref, dep}, "dependency.remove", nil)
			}
			if err := conn.Service.RemoveDependency(ctx, ref, dep); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: ref.String(), Status: statusOK})
		},
		ValidArgsFunction: g.completeTaskRefs,
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

func depLsCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "ls REF",
		Short:   "List what a task depends on",
		Long:    "List the dependencies recorded for a task.\n\nExit codes: 3 unknown reference.",
		Example: "  tix dep ls default-2",
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
			deps, err := conn.Service.ListDependencies(ctx, ref)
			if err != nil {
				return err
			}
			return g.render(cmd, deps)
		},
		ValidArgsFunction: g.completeTaskRefs,
	}
}

// twoRefs parses a pair of task references.
func twoRefs(args []string) (core.TaskRef, core.TaskRef, error) {
	ref, err := parseRef(args[0])
	if err != nil {
		return core.TaskRef{}, core.TaskRef{}, err
	}
	dep, err := parseRef(args[1])
	if err != nil {
		return core.TaskRef{}, core.TaskRef{}, err
	}
	return ref, dep, nil
}

// newTagCmd builds the tag command group.
func newTagCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tag",
		Short:   "Manage task tags",
		GroupID: "work",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(labelAddCmd(g), labelRmCmd(g), labelLsCmd(g))
	return cmd
}

func labelAddCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "add REF LABEL...",
		Short:   "Attach tags to a task",
		Long:    "Attach one or more tags to a task, creating them if needed.\n\nExit codes: 3 unknown reference.",
		Example: "  tix tag add default-1 urgent ops",
		Args:    minArgs(2),
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
				return g.dryBulk(cmd, conn, ctx, []core.TaskRef{ref}, "tag.add",
					map[string]any{"tags": args[1:]})
			}
			for _, name := range args[1:] {
				if err := conn.Service.AddTag(ctx, ref, name); err != nil {
					return err
				}
			}
			return g.render(cmd, outcome{Ref: ref.String(), Status: statusOK})
		},
		ValidArgsFunction: g.completeTaskRefs,
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

func labelRmCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "rm REF LABEL",
		Short:   "Detach a tag from a task",
		Long:    "Detach a tag from a task.\n\nExit codes: 3 unknown reference or tag.",
		Example: "  tix tag rm default-1 urgent",
		Args:    exactArgs(2),
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
				return g.dryBulk(cmd, conn, ctx, []core.TaskRef{ref}, "tag.remove",
					map[string]any{"tag": args[1]})
			}
			if err := conn.Service.RemoveTag(ctx, ref, args[1]); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: ref.String(), Status: statusOK})
		},
		ValidArgsFunction: g.completeTaskRefs,
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

func labelLsCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Short:   "List tags",
		Long:    "List every tag defined in the tenant.\n\nExit codes: 5 permission denied.",
		Example: "  tix tag ls -o json",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			tags, err := conn.Service.ListTags(ctx)
			if err != nil {
				return err
			}
			return g.render(cmd, tags)
		},
	}
}

// newCommentCmd builds the comment command group.
func newCommentCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "comment",
		Short:   "Manage task comments",
		GroupID: "work",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(commentAddCmd(g), commentLsCmd(g), commentEditCmd(g), commentRmCmd(g))
	return cmd
}

func commentAddCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "add REF [BODY]",
		Short:   "Comment on a task",
		Long:    "Add a comment to a task. With no body, or with -, the body is read from standard input.\n\nExit codes: 3 unknown reference.",
		Example: "  tix comment add default-1 \"looks done\"\n  echo \"from a pipe\" | tix comment add default-1 -",
		Args:    rangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := parseRef(args[0])
			if err != nil {
				return err
			}
			text := StdinMarker
			if len(args) == 2 {
				text = args[1]
			}
			text, err = body(cmd, text)
			if err != nil {
				return err
			}
			if strings.TrimSpace(text) == "" {
				return core.Invalid("comment body is required")
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				return g.dryBulk(cmd, conn, ctx, []core.TaskRef{ref}, "comment.add", nil)
			}
			comment, err := conn.Service.AddComment(ctx, ref, text)
			if err != nil {
				return err
			}
			return g.render(cmd, comment)
		},
		ValidArgsFunction: g.completeTaskRefs,
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

func commentLsCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "ls REF",
		Short:   "List a task's comments",
		Long:    "List every comment on a task.\n\nExit codes: 3 unknown reference.",
		Example: "  tix comment ls default-1",
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
			comments, err := conn.Service.ListComments(ctx, ref)
			if err != nil {
				return err
			}
			out := newList[core.Comment](g, cmd)
			for _, c := range comments {
				if err := out.Write(c); err != nil {
					return out.fail(err)
				}
			}
			return out.Close()
		},
		ValidArgsFunction: g.completeTaskRefs,
	}
}

func commentEditCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "edit ID [BODY]",
		Short:   "Change a comment",
		Long:    "Replace a comment's body. With no body, or with -, it is read from standard input.\n\nExit codes: 3 unknown comment, 5 not the author.",
		Example: "  tix comment edit 01J0000000000000000000 \"corrected\"",
		Args:    rangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			text := StdinMarker
			if len(args) == 2 {
				text = args[1]
			}
			text, err := body(cmd, text)
			if err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("comment.edit", args[0], nil))
			}
			comment, err := conn.Service.EditComment(ctx, args[0], text)
			if err != nil {
				return err
			}
			return g.render(cmd, comment)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

func commentRmCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "rm ID",
		Short:   "Delete a comment",
		Long:    "Delete a comment.\n\nExit codes: 3 unknown comment, 5 not the author.",
		Example: "  tix comment rm 01J0000000000000000000",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("comment.delete", args[0], nil))
			}
			if err := conn.Service.DeleteComment(ctx, args[0]); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: args[0], Status: statusOK})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}
