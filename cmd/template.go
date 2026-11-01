// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/core"
	"github.com/spf13/cobra"
)

// templateHelp says what a template is and what its titles may carry. The
// placeholders are written in capitals: they match in any case, and a bare
// lowercase word in braces reads as an unfilled token wherever this text is
// rendered as a page.
const templateHelp = "A template is a named shape of a task: its title, description, priority, tags, " +
	"custom fields, repeat rule and, when saved with them, its subtasks. It is not a task, so it never " +
	"appears in a listing, a claim or the statistics. A template belongs to one project or, with " +
	"--tenant-wide, to every project.\n\n" +
	"Titles may carry placeholders, filled in each time the template is used and matched in any case: " +
	"{DATE} is today's date as YYYY-MM-DD, {N} is the use number (1 for the first use) and {PROJECT} " +
	"is the key of the project the task lands in. Any other word in braces stays as written."

// newTemplateCmd builds the template command group.
func newTemplateCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "template",
		Short:   "Save tasks as templates and create tasks from them",
		Long:    "Save tasks as reusable templates and create tasks from them.\n\n" + templateHelp,
		GroupID: "work",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(templateLsCmd(g), templateSaveCmd(g), templateShowCmd(g), templateEditCmd(g),
		templateRmCmd(g), templateUseCmd(g))
	return cmd
}

func templateLsCmd(g *globals) *cobra.Command {
	var (
		project, cursor string
		limit           int
	)
	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List templates",
		Long: "List the templates you can read, by name. --project keeps that project's templates " +
			"and the tenant-wide ones.\n\nExit codes: 3 unknown project, 5 permission denied.",
		Example: "  tix template ls\n  tix template ls --project infra -o json",
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
			items, next, err := conn.Service.ListTemplates(ctx, core.TemplateFilter{
				ProjectRef: project, Page: core.Page{Limit: size, Cursor: cursor},
			})
			if err != nil {
				return err
			}
			out := newList[core.TaskTemplate](g, cmd)
			for _, t := range items {
				if err := out.Write(t); err != nil {
					return out.fail(err)
				}
			}
			if next != "" {
				g.diag(cmd, "more results available; next cursor %s", next)
			}
			return out.Close()
		},
	}
	f := cmd.Flags()
	f.StringVarP(&project, "project", "p", "", "keep this project's templates and the tenant-wide ones")
	f.IntVar(&limit, "limit", config.DefaultPageSize, "maximum records to return")
	f.StringVar(&cursor, "cursor", "", "continue from a previous page")
	_ = cmd.RegisterFlagCompletionFunc("project", flagCompletion(g.completeProjectKeys))
	return cmd
}

func templateSaveCmd(g *globals) *cobra.Command {
	var (
		in     core.SaveTemplateInput
		dueIn  string
		dryRun bool
	)
	cmd := &cobra.Command{
		Use:   "save REF",
		Short: "Save a task as a template",
		Long: "Save a task as a template under --name. The template keeps the task's title, " +
			"description, priority, tags, custom fields and repeat rule; --with-subtasks keeps its live " +
			"subtasks too, at any depth. Status, assignee, due date, comments and history are not kept. " +
			"--due-in gives the template a due offset such as 3d, so each task made from it falls due " +
			"that long after it is made.\n\n" + templateHelp + "\n\n" +
			"Exit codes: 2 invalid input, 3 unknown reference, 4 a template of that name exists, " +
			"5 permission denied.",
		Example: "  tix template save infra-4 --name release --with-subtasks\n" +
			"  tix template save infra-9 --name standup --tenant-wide --due-in 1d",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			in.TaskRef = args[0]
			d, err := parseDuration(dueIn)
			if err != nil {
				return err
			}
			in.DueIn = d
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				ref, err := parseRef(args[0])
				if err != nil {
					return err
				}
				if _, err := conn.Service.GetTask(ctx, ref); err != nil {
					return err
				}
				return g.render(cmd, newPlan("template.save", in.Name, map[string]any{
					"task": args[0], "tenant_wide": in.TenantWide, "with_subtasks": in.WithSubtasks,
				}))
			}
			tpl, err := conn.Service.SaveTemplate(ctx, in)
			if err != nil {
				return err
			}
			return g.render(cmd, tpl)
		},
		ValidArgsFunction: g.completeTaskRefs,
	}
	f := cmd.Flags()
	f.StringVar(&in.Name, "name", "", "name of the template (required)")
	f.BoolVar(&in.TenantWide, "tenant-wide", false, "share the template with every project")
	f.BoolVar(&in.WithSubtasks, "with-subtasks", false, "keep the subtasks, recursively")
	f.StringVar(&dueIn, "due-in", "", "due offset such as 3d or 12h")
	f.BoolVar(&dryRun, "dry-run", false, "report what would be saved without writing")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func templateShowCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "show NAME",
		Short: "Show a template",
		Long: "Show a template by name or identifier. -o json or -o yaml prints the whole task shape, " +
			"subtasks included.\n\nExit codes: 3 unknown template, 5 permission denied.",
		Example: "  tix template show release\n  tix template show release -o yaml",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			tpl, err := conn.Service.GetTemplate(ctx, args[0])
			if err != nil {
				return err
			}
			return g.render(cmd, tpl)
		},
	}
}

func templateEditCmd(g *globals) *cobra.Command {
	var (
		name, project, dueIn, title, bodyFlag, priority, repeat string
		tags, fields                                            []string
		tenantWide, noRepeat, dryRun                            bool
	)
	cmd := &cobra.Command{
		Use:   "edit NAME",
		Short: "Rename or change a template",
		Long: "Rename a template, move it to another project or to the whole tenant, or change the " +
			"task it creates. Only the flags you pass are applied. --field key=null removes a custom " +
			"field, --due-in 0 removes the due offset and --no-repeat the repeat rule. Subtasks are " +
			"changed by saving the task again.\n\n" + templateHelp + "\n\n" +
			"Exit codes: 2 invalid input, 3 unknown template or project, 4 a template of the new name " +
			"exists, 5 permission denied.",
		Example: "  tix template edit release --name ship\n  tix template edit ship --tenant-wide --due-in 2d\n" +
			"  tix template edit ship --title \"Release {N}\" --tag ops --tag release",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var in core.UpdateTemplateInput
			flags := cmd.Flags()
			if flags.Changed("name") {
				in.Name = &name
			}
			switch {
			case flags.Changed("project") && tenantWide:
				return usagef(cmd, "--project and --tenant-wide are mutually exclusive")
			case flags.Changed("project"):
				in.ProjectRef = &project
			case tenantWide:
				none := ""
				in.ProjectRef = &none
			}
			if flags.Changed("due-in") {
				d, err := parseDuration(dueIn)
				if err != nil {
					return err
				}
				in.DueIn = &d
			}
			if flags.Changed("title") {
				in.Title = &title
			}
			if flags.Changed("body") {
				text, err := body(cmd, bodyFlag)
				if err != nil {
					return err
				}
				in.Body = &text
			}
			if flags.Changed("priority") {
				prio, err := parsePriority(priority)
				if err != nil {
					return err
				}
				in.Priority = &prio
			}
			if flags.Changed("tag") {
				in.Tags = &tags
			}
			switch {
			case flags.Changed("repeat") && noRepeat:
				return usagef(cmd, "--repeat and --no-repeat are mutually exclusive")
			case flags.Changed("repeat"):
				in.Recurrence = &repeat
			case noRepeat:
				cleared := ""
				in.Recurrence = &cleared
			}
			custom, err := parseFields(fields)
			if err != nil {
				return err
			}
			in.CustomFields = custom
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				if _, err := conn.Service.GetTemplate(ctx, args[0]); err != nil {
					return err
				}
				return g.render(cmd, newPlan("template.update", args[0], nil))
			}
			tpl, err := conn.Service.UpdateTemplate(ctx, args[0], in)
			if err != nil {
				return err
			}
			return g.render(cmd, tpl)
		},
	}
	f := cmd.Flags()
	f.StringVar(&name, "name", "", "new name")
	f.StringVarP(&project, "project", "p", "", "move the template to this project")
	f.BoolVar(&tenantWide, "tenant-wide", false, "share the template with every project")
	f.StringVar(&dueIn, "due-in", "", "new due offset such as 3d; 0 removes it")
	f.StringVar(&title, "title", "", "new title of the task it creates")
	f.StringVar(&bodyFlag, "body", "", "new description, or - to read standard input")
	f.StringVar(&priority, "priority", "", "new priority")
	f.StringSliceVarP(&tags, "tag", "l", nil, "replace the tag set")
	f.StringSliceVar(&fields, "field", nil, "custom field as key=value, key=null removes it, repeatable")
	f.StringVar(&repeat, "repeat", "", "new repeat rule, such as 'every month on day 1'")
	f.BoolVar(&noRepeat, "no-repeat", false, "remove the repeat rule")
	f.BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	_ = cmd.RegisterFlagCompletionFunc("project", flagCompletion(g.completeProjectKeys))
	registerCompletions(g, cmd, "tag")
	return cmd
}

func templateRmCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "rm NAME",
		Aliases: []string{"delete"},
		Short:   "Delete a template",
		Long: "Delete a template. Tasks already made from it are not touched.\n\n" +
			"Exit codes: 3 unknown template, 5 permission denied.",
		Example: "  tix template rm release\n  tix template rm release --dry-run",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				if _, err := conn.Service.GetTemplate(ctx, args[0]); err != nil {
					return err
				}
				return g.render(cmd, newPlan("template.delete", args[0], nil))
			}
			if err := conn.Service.DeleteTemplate(ctx, args[0]); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: args[0], Status: statusOK})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be deleted without writing")
	return cmd
}

func templateUseCmd(g *globals) *cobra.Command {
	var (
		in            core.UseTemplateInput
		dueIn         string
		noDue, dryRun bool
	)
	cmd := &cobra.Command{
		Use:   "use NAME",
		Short: "Create a task from a template",
		Long: "Create a task, and a subtask for each one the template holds, from a template. The task " +
			"goes to the template's project, or for a tenant-wide template to --project. It starts in " +
			"the project's initial state and keeps the custom fields that project defines. --title " +
			"replaces the template's title and may carry placeholders too. --due-in sets the due date " +
			"that long from now, overriding the template's own offset, and --no-due sets none.\n\n" +
			templateHelp + "\n\n" +
			"Exit codes: 2 invalid input, a missing project or a required field, 3 unknown template or " +
			"project, 5 permission denied.",
		Example: "  tix template use release\n  tix template use standup --project web --due-in 3d\n" +
			"  tix template use release --title \"Release {N} ({DATE})\"",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch {
			case cmd.Flags().Changed("due-in") && noDue:
				return usagef(cmd, "--due-in and --no-due are mutually exclusive")
			case cmd.Flags().Changed("due-in"):
				d, err := parseDuration(dueIn)
				if err != nil {
					return err
				}
				in.DueIn = &d
			case noDue:
				var zero core.Duration
				in.DueIn = &zero
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				tpl, err := conn.Service.GetTemplate(ctx, args[0])
				if err != nil {
					return err
				}
				return g.render(cmd, newPlan("template.use", tpl.Name, map[string]any{
					"project": in.ProjectRef, "tasks": tpl.Task.Count(),
				}))
			}
			task, err := conn.Service.UseTemplate(ctx, args[0], in)
			if err != nil {
				return err
			}
			return g.render(cmd, task)
		},
	}
	f := cmd.Flags()
	f.StringVarP(&in.ProjectRef, "project", "p", "", "project to create the task in (default: the template's)")
	f.StringVar(&in.Title, "title", "", "title of the task (default: the template's)")
	f.StringVar(&dueIn, "due-in", "", "due date this long from now, such as 3d")
	f.BoolVar(&noDue, "no-due", false, "set no due date, whatever the template says")
	f.BoolVar(&dryRun, "dry-run", false, "report what would be created without writing")
	_ = cmd.RegisterFlagCompletionFunc("project", flagCompletion(g.completeProjectKeys))
	return cmd
}
