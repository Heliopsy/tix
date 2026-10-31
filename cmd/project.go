// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"errors"
	"os"
	"strings"

	"github.com/charmbracelet/x/term"
	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/core"
	"github.com/spf13/cobra"
)

// colorFlagUsage and iconFlagUsage describe the appearance flags once, so the
// create and edit forms of them cannot drift apart.
var (
	colorFlagUsage = "colour marking the project's rows: a palette colour (" + projectColorList() + ") or a custom #rrggbb"
	iconFlagUsage  = "an emoji or a short monogram shown beside the project's rows"
	iconNameUsage  = "a built-in icon shown beside the project's rows (" + builtinIconList() + ")"
	iconFileUsage  = "path to a PNG, JPEG, WebP or SVG image of at most 64 KiB to use as the icon"
)

// builtinIconList renders the built-in icon names for a flag's usage line.
func builtinIconList() string {
	icons := core.BuiltinIcons()
	parts := make([]string, 0, len(icons))
	for _, b := range icons {
		parts = append(parts, b.Name)
	}
	return strings.Join(parts, ", ")
}

// readIconFile loads the image --icon-file names, refusing one too large to
// upload before it is read whole.
func readIconFile(path string) (*core.ProjectIconFile, error) {
	if path == "" {
		return &core.ProjectIconFile{}, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, core.Invalid("reading icon file %q: %v", path, err)
	}
	if info.Size() > core.MaxProjectIconBytes {
		return nil, core.Invalid("icon file %q is %d bytes; the limit is %d", path, info.Size(), core.MaxProjectIconBytes)
	}
	data, err := os.ReadFile(path) // #nosec G304 -- the operator names the file to upload
	if err != nil {
		return nil, core.Invalid("reading icon file %q: %v", path, err)
	}
	return &core.ProjectIconFile{Data: data}, nil
}

// projectColorList renders the palette for a flag's usage line.
func projectColorList() string {
	colors := core.ProjectColors()
	parts := make([]string, 0, len(colors))
	for _, c := range colors {
		parts = append(parts, c.String())
	}
	return strings.Join(parts, ", ")
}

// newProjectCmd builds the project command group.
func newProjectCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "project",
		Short:   "Manage projects",
		GroupID: "work",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(projectCreateCmd(g), projectLsCmd(g), projectShowCmd(g),
		projectEditCmd(g), projectIconCmd(g), projectArchiveCmd(g), projectRmCmd(g))
	return cmd
}

func projectCreateCmd(g *globals) *cobra.Command {
	var description, workflow, color, icon, iconName, iconFile string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "create KEY NAME",
		Short: "Create a project",
		Long:  "Create a project with a key used in task references.\n\nExit codes: 2 invalid key, 4 key already exists.",
		Example: "  tix project create infra Infrastructure\n  tix project create infra Infrastructure --color blue --icon \U0001F680\n" +
			"  tix project create infra Infrastructure --icon-name server\n  tix project create web Web --icon-file logo.svg\n" +
			"  tix project create web Website --color '#1a7f5a'",
		Args: exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			file, err := readIconFile(iconFile)
			if err != nil {
				return err
			}
			in := core.CreateProjectInput{
				Key: args[0], Name: args[1], Description: description, WorkflowKey: workflow,
				Color: color, Icon: icon, IconName: iconName,
			}
			if !file.Empty() {
				in.IconFile = file
			}
			if err := in.Validate(); err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("project.create", in.Key, map[string]any{"name": in.Name}))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			project, err := conn.Service.CreateProject(ctx, in)
			if err != nil {
				return err
			}
			return g.render(cmd, project)
		},
	}
	cmd.Flags().StringVar(&description, "description", "", "project description")
	cmd.Flags().StringVar(&workflow, "workflow", "", "workflow key or identifier to assign")
	cmd.Flags().StringVar(&color, "color", "", colorFlagUsage)
	cmd.Flags().StringVar(&icon, "icon", "", iconFlagUsage)
	cmd.Flags().StringVar(&iconName, "icon-name", "", iconNameUsage)
	cmd.Flags().StringVar(&iconFile, "icon-file", "", iconFileUsage)
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be created without writing")
	return cmd
}

func projectLsCmd(g *globals) *cobra.Command {
	var archived bool
	var limit int
	var cursor string
	var keys []string
	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List projects",
		Long:    "List projects in the tenant.\n\nExit codes: 5 permission denied.",
		Example: "  tix project ls\n  tix project ls --include-archived -o json\n  tix project ls --key infra --key web",
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
			projects, next, err := conn.Service.ListProjects(ctx, core.ProjectFilter{
				Keys:            keys,
				IncludeArchived: archived,
				Page:            core.Page{Limit: size, Cursor: cursor},
			})
			if err != nil {
				return err
			}
			out := newList[core.Project](g, cmd)
			for _, p := range projects {
				if err := out.Write(p); err != nil {
					return out.fail(err)
				}
			}
			if next != "" {
				g.diag(cmd, "more results available; next cursor %s", next)
			}
			return out.Close()
		},
	}
	cmd.Flags().BoolVar(&archived, "include-archived", false, "include archived projects")
	cmd.Flags().StringSliceVar(&keys, "key", nil, "restrict to projects with these keys, repeatable")
	cmd.Flags().IntVar(&limit, "limit", config.DefaultPageSize, "maximum records to return")
	cmd.Flags().StringVar(&cursor, "cursor", "", "continue from a previous page")
	return cmd
}

func projectShowCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "show KEY",
		Short:   "Show one project",
		Long:    "Show a project by key or identifier.\n\nExit codes: 3 unknown project.",
		Example: "  tix project show infra",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			project, err := conn.Service.GetProject(ctx, args[0])
			if err != nil {
				return err
			}
			return g.render(cmd, project)
		},
		ValidArgsFunction: g.completeProjectKeys,
	}
}

func projectEditCmd(g *globals) *cobra.Command {
	var key, name, description, workflow, color, icon, iconName, iconFile string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "edit KEY",
		Short: "Change a project",
		Long: "Change a project's key, name, description, workflow, colour or icon.\n\n" +
			"A new key renames every task reference (infra-8 becomes ops-8). The old key stays an alias of the\n" +
			"project for as long as it exists, so old references, links and filters keep resolving, and no\n" +
			"other project can take it.\n\n" +
			"The icon is one of --icon, --icon-name or --icon-file; setting one replaces whichever the project had, " +
			"and setting any of them empty clears the icon.\n\n" +
			"Exit codes: 2 invalid key, colour or icon, 3 unknown project, 4 key held by another project.",
		Example: "  tix project edit infra --name \"Platform\"\n  tix project edit infra --key ops\n" +
			"  tix project edit infra --color \"\" --icon \"\"\n" +
			"  tix project edit infra --icon-name rocket\n  tix project edit infra --icon-file logo.png",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var in core.UpdateProjectInput
			if cmd.Flags().Changed("key") {
				if err := core.ValidateProjectKey(strings.TrimSpace(key)); err != nil {
					return err
				}
				in.Key = &key
			}
			if cmd.Flags().Changed("name") {
				in.Name = &name
			}
			if cmd.Flags().Changed("description") {
				in.Description = &description
			}
			if cmd.Flags().Changed("workflow") {
				in.WorkflowKey = &workflow
			}
			if cmd.Flags().Changed("color") {
				in.Color = &color
			}
			if cmd.Flags().Changed("icon") {
				in.Icon = &icon
			}
			if cmd.Flags().Changed("icon-name") {
				in.IconName = &iconName
			}
			if cmd.Flags().Changed("icon-file") {
				file, err := readIconFile(iconFile)
				if err != nil {
					return err
				}
				in.IconFile = file
			}
			if in.IconChanged() {
				if err := in.ValidateIcon(); err != nil {
					return err
				}
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				if _, err := conn.Service.GetProject(ctx, args[0]); err != nil {
					return err
				}
				return g.render(cmd, newPlan("project.update", args[0], nil))
			}
			project, err := conn.Service.UpdateProject(ctx, args[0], in)
			if err != nil {
				return err
			}
			return g.render(cmd, project)
		},
		ValidArgsFunction: g.completeProjectKeys,
	}
	cmd.Flags().StringVar(&key, "key", "", "new key; old task references keep resolving through the old key")
	cmd.Flags().StringVar(&name, "name", "", "new name")
	cmd.Flags().StringVar(&description, "description", "", "new description")
	cmd.Flags().StringVar(&workflow, "workflow", "", "new workflow key or identifier")
	cmd.Flags().StringVar(&color, "color", "", colorFlagUsage+", or empty to clear it")
	cmd.Flags().StringVar(&icon, "icon", "", iconFlagUsage+", or empty to clear it")
	cmd.Flags().StringVar(&iconName, "icon-name", "", iconNameUsage+", or empty to clear it")
	cmd.Flags().StringVar(&iconFile, "icon-file", "", iconFileUsage+", or empty to clear it")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

func projectIconCmd(g *globals) *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:   "icon KEY",
		Short: "Save a project's uploaded icon image",
		Long: "Write a project's uploaded icon image to the file --out names, or to standard output when that is not a terminal.\n\n" +
			"Exit codes: 2 standard output is a terminal and no --out was given, 3 unknown project or no uploaded icon.",
		Example: "  tix project icon infra --out infra.png\n  tix project icon infra > infra.png",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if out == "" && writesToTerminal(cmd) {
				return core.Invalid("standard output is a terminal; name a file with --out or redirect the output")
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			img, err := conn.Service.GetProjectIcon(ctx, args[0])
			if err != nil {
				return err
			}
			if out == "" {
				_, err = cmd.OutOrStdout().Write(img.Data)
				return err
			}
			if err := os.WriteFile(out, img.Data, 0o600); err != nil {
				return core.Internal("writing icon to %q: %v", out, err).Wrap(err)
			}
			return g.render(cmd, outcome{Ref: args[0], Status: statusOK})
		},
		ValidArgsFunction: g.completeProjectKeys,
	}
	cmd.Flags().StringVar(&out, "out", "", "file to write the image to")
	return cmd
}

// writesToTerminal reports whether the command's output is a terminal, where
// image bytes would be noise.
func writesToTerminal(cmd *cobra.Command) bool {
	f, ok := cmd.OutOrStdout().(*os.File)
	return ok && term.IsTerminal(f.Fd())
}

func projectArchiveCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "archive KEY",
		Short:   "Archive a project",
		Long:    "Archive a project so it stops appearing in default listings.\n\nExit codes: 3 unknown project.",
		Example: "  tix project archive infra",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				if _, err := conn.Service.GetProject(ctx, args[0]); err != nil {
					return err
				}
				return g.render(cmd, newPlan("project.archive", args[0], nil))
			}
			if err := conn.Service.ArchiveProject(ctx, args[0]); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: args[0], Status: statusOK})
		},
		ValidArgsFunction: g.completeProjectKeys,
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

func projectRmCmd(g *globals) *cobra.Command {
	var dryRun, deleteTasks bool
	var moveTo string
	cmd := &cobra.Command{
		Use:     "rm KEY",
		Aliases: []string{"delete"},
		Short:   "Delete a project",
		Long: "Delete a project permanently. A project holding tasks needs to be told what happens to them:\n" +
			"--move-tasks-to KEY moves them to another project, renumbered there with their status mapped onto\n" +
			"its workflow, and --delete-tasks deletes them permanently with the project. --dry-run prints the\n" +
			"task count and the status mapping without writing.\n\n" +
			"Exit codes: 2 both flags passed, 3 unknown project, 6 project still holds tasks and no flag says what happens to them.",
		Example: "  tix project rm infra\n  tix project rm infra --move-tasks-to ops\n  tix project rm infra --delete-tasks",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			in := core.DeleteProjectInput{MoveTasksTo: moveTo, DeleteTasks: deleteTasks}
			if dryRun {
				plan, err := conn.Service.PlanProjectDeletion(ctx, args[0], in)
				if err != nil {
					return err
				}
				return g.render(cmd, plan)
			}
			done, err := conn.Service.DeleteProject(ctx, args[0], in)
			if err != nil {
				return withDeletionFlags(err)
			}
			return g.render(cmd, done)
		},
		ValidArgsFunction: g.completeProjectKeys,
	}
	cmd.Flags().StringVar(&moveTo, "move-tasks-to", "", "move the project's tasks to this project before deleting it")
	cmd.Flags().BoolVar(&deleteTasks, "delete-tasks", false, "delete the project's tasks permanently with it")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be deleted or moved without writing")
	_ = cmd.RegisterFlagCompletionFunc("move-tasks-to", g.completeProjectKeys)
	return cmd
}

// withDeletionFlags names the flags that answer a refusal for want of a choice
// about the tasks, which the service states in terms of no one surface.
func withDeletionFlags(err error) error {
	var domain *core.Error
	if !errors.As(err, &domain) || domain.Kind != core.KindPrecondition || domain.Details["tasks"] == nil {
		return err
	}
	return core.Precondition("%s; pass --move-tasks-to KEY or --delete-tasks", domain.Message).
		WithDetail("tasks", domain.Details["tasks"])
}

// newWorkflowCmd builds the workflow command group.
func newWorkflowCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "workflow",
		Short:   "Manage workflow state machines",
		GroupID: "admin",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(workflowPutCmd(g), workflowGetCmd(g), workflowLsCmd(g), workflowRmCmd(g))
	return cmd
}

func workflowPutCmd(g *globals) *cobra.Command {
	var file string
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "put",
		Short:   "Define or replace a workflow",
		Long:    "Define a workflow from a YAML or JSON document, or - for standard input.\n\nExit codes: 2 invalid definition, 6 tasks sit in a removed state.",
		Example: "  tix workflow put -f workflow.yaml\n  cat workflow.yaml | tix workflow put -f -",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := readDocument(cmd, file)
			if err != nil {
				return err
			}
			var in core.WorkflowInput
			if err := decodeFile(data, &in); err != nil {
				return err
			}
			if err := in.Validate(); err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("workflow.put", in.Key, nil))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			wf, err := conn.Service.PutWorkflow(ctx, in)
			if err != nil {
				return err
			}
			return g.render(cmd, wf)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", StdinMarker, "workflow document, or - for standard input")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate without writing")
	return cmd
}

func workflowGetCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "get KEY",
		Short:   "Show one workflow",
		Long:    "Show a workflow definition by key or identifier.\n\nExit codes: 3 unknown workflow.",
		Example: "  tix workflow get default -o yaml",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			wf, err := conn.Service.GetWorkflow(ctx, args[0])
			if err != nil {
				return err
			}
			return g.render(cmd, wf)
		},
	}
}

func workflowLsCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List workflows",
		Long:    "List every workflow in the tenant.\n\nExit codes: 5 permission denied.",
		Example: "  tix workflow ls",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			workflows, err := conn.Service.ListWorkflows(ctx)
			if err != nil {
				return err
			}
			return g.render(cmd, workflows)
		},
	}
}

func workflowRmCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "rm KEY",
		Aliases: []string{"delete"},
		Short:   "Delete a workflow",
		Long:    "Delete a workflow no project uses, by key or identifier.\n\nExit codes: 3 unknown workflow, 6 still in use.",
		Example: "  tix workflow rm review",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				if _, err := conn.Service.GetWorkflow(ctx, args[0]); err != nil {
					return err
				}
				return g.render(cmd, newPlan("workflow.delete", args[0], nil))
			}
			if err := conn.Service.DeleteWorkflow(ctx, args[0]); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: args[0], Status: statusOK})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be deleted without writing")
	return cmd
}

// newFieldCmd builds the custom field command group.
func newFieldCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "field",
		Short:   "Manage project custom fields",
		GroupID: "admin",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(fieldPutCmd(g), fieldLsCmd(g), fieldRmCmd(g))
	return cmd
}

func fieldPutCmd(g *globals) *cobra.Command {
	var (
		label, fieldType string
		defaultValue     string
		options          []string
		required, idx    bool
		position         int
		dryRun           bool
	)
	cmd := &cobra.Command{
		Use:   "put PROJECT KEY",
		Short: "Define or replace a custom field",
		Long:  "Define a custom field on a project's tasks.\n\nExit codes: 2 invalid definition, 3 unknown project.",
		Example: "  tix field put infra severity --type enum --option low --option high --default low\n" +
			"  tix field put infra points --type int --default 1",
		Args: exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			in := core.FieldDefInput{
				Key: args[1], Label: label, Type: core.FieldType(fieldType),
				Required: required, EnumOptions: options, Indexed: idx, Position: position,
			}
			if cmd.Flags().Changed("default") {
				in.Default = fieldDefault(in.Type, defaultValue)
			}
			if in.Label == "" {
				in.Label = in.Key
			}
			if err := in.Validate(); err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("field.put", args[0]+"/"+in.Key, nil))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			def, err := conn.Service.PutFieldDef(ctx, args[0], in)
			if err != nil {
				return err
			}
			return g.render(cmd, def)
		},
		ValidArgsFunction: g.completeProjectKeys,
	}
	f := cmd.Flags()
	f.StringVar(&label, "label", "", "human label, defaulting to the key")
	f.StringVar(&fieldType, "type", string(core.FieldString), "field type")
	f.StringSliceVar(&options, "option", nil, "enum option, repeatable")
	f.StringVar(&defaultValue, "default", "", "value a task created without one is given")
	f.BoolVar(&required, "required", false, "require a value on every task")
	f.BoolVar(&idx, "indexed", false, "index the field for filtering")
	f.IntVar(&position, "position", 0, "ordering position")
	f.BoolVar(&dryRun, "dry-run", false, "validate without writing")
	return cmd
}

func fieldLsCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "ls PROJECT",
		Short:   "List a project's custom fields",
		Long:    "List the custom fields defined on a project.\n\nExit codes: 3 unknown project.",
		Example: "  tix field ls infra",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			defs, err := conn.Service.ListFieldDefs(ctx, args[0])
			if err != nil {
				return err
			}
			return g.render(cmd, defs)
		},
		ValidArgsFunction: g.completeProjectKeys,
	}
}

func fieldRmCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "rm PROJECT KEY",
		Aliases: []string{"delete"},
		Short:   "Delete a custom field",
		Long:    "Delete a custom field definition from a project.\n\nExit codes: 3 unknown project or field.",
		Example: "  tix field rm infra severity",
		Args:    exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("field.delete", args[0]+"/"+args[1], nil))
			}
			if err := conn.Service.DeleteFieldDef(ctx, args[0], args[1]); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: args[1], Status: statusOK})
		},
		ValidArgsFunction: g.completeProjectKeys,
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be deleted without writing")
	return cmd
}

// readDocument reads a document from a path, or from standard input for "-".
func readDocument(cmd *cobra.Command, path string) ([]byte, error) {
	if path == "" || path == StdinMarker {
		text, err := readAll(cmd.InOrStdin())
		return []byte(text), err
	}
	data, err := os.ReadFile(path) // #nosec G304 -- the path is chosen by the user
	if err != nil {
		return nil, core.Invalid("reading %q: %v", path, err)
	}
	return data, nil
}
