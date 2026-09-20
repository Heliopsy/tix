package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/thereisnotime/tix/internal/core"
)

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
		projectEditCmd(g), projectArchiveCmd(g), projectRmCmd(g))
	return cmd
}

func projectCreateCmd(g *globals) *cobra.Command {
	var description, workflow string
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "create KEY NAME",
		Short:   "Create a project",
		Long:    "Create a project with a key used in task references.\n\nExit codes: 2 invalid key, 4 key already exists.",
		Example: "  tix project create infra Infrastructure\n  tix project create infra Infrastructure --workflow default",
		Args:    exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			in := core.CreateProjectInput{
				Key: args[0], Name: args[1], Description: description, WorkflowKey: workflow,
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
	cmd.Flags().StringVar(&workflow, "workflow", "", "workflow key to assign")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be created without writing")
	return cmd
}

func projectLsCmd(g *globals) *cobra.Command {
	var archived bool
	var limit int
	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List projects",
		Long:    "List projects in the tenant.\n\nExit codes: 5 permission denied.",
		Example: "  tix project ls\n  tix project ls --include-archived -o json",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			projects, next, err := conn.Service.ListProjects(ctx, core.ProjectFilter{
				IncludeArchived: archived,
				Page:            core.Page{Limit: limit},
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
	cmd.Flags().IntVar(&limit, "limit", core.DefaultPageLimit, "maximum records to return")
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
	var name, description, workflow string
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "edit KEY",
		Short:   "Change a project",
		Long:    "Change a project's name, description or workflow.\n\nExit codes: 3 unknown project.",
		Example: "  tix project edit infra --name \"Platform\"",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var in core.UpdateProjectInput
			if cmd.Flags().Changed("name") {
				in.Name = &name
			}
			if cmd.Flags().Changed("description") {
				in.Description = &description
			}
			if cmd.Flags().Changed("workflow") {
				in.WorkflowKey = &workflow
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
	cmd.Flags().StringVar(&name, "name", "", "new name")
	cmd.Flags().StringVar(&description, "description", "", "new description")
	cmd.Flags().StringVar(&workflow, "workflow", "", "new workflow key")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
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
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "rm KEY",
		Aliases: []string{"delete"},
		Short:   "Delete a project",
		Long:    "Delete a project and everything in it.\n\nExit codes: 3 unknown project, 6 project still holds tasks.",
		Example: "  tix project rm infra",
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
				return g.render(cmd, newPlan("project.delete", args[0], nil))
			}
			if err := conn.Service.DeleteProject(ctx, args[0]); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: args[0], Status: statusOK})
		},
		ValidArgsFunction: g.completeProjectKeys,
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be deleted without writing")
	return cmd
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
		Long:    "Show a workflow definition.\n\nExit codes: 3 unknown workflow.",
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
		Long:    "Delete a workflow no project uses.\n\nExit codes: 3 unknown workflow, 6 still in use.",
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
		options          []string
		required, idx    bool
		position         int
		dryRun           bool
	)
	cmd := &cobra.Command{
		Use:     "put PROJECT KEY",
		Short:   "Define or replace a custom field",
		Long:    "Define a custom field on a project's tasks.\n\nExit codes: 2 invalid definition, 3 unknown project.",
		Example: "  tix field put infra severity --type enum --option low --option high",
		Args:    exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			in := core.FieldDefInput{
				Key: args[1], Label: label, Type: core.FieldType(fieldType),
				Required: required, EnumOptions: options, Indexed: idx, Position: position,
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
