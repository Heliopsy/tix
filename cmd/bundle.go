package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/spf13/cobra"
)

// init registers component sharing alongside the other command groups.
func init() { builders = append(builders, newBundleCmd) }

// collisionPolicies are the policies the import subcommand accepts.
var collisionPolicies = []string{
	string(core.CollisionSkip), string(core.CollisionRename), string(core.CollisionReplace),
}

// componentKinds are the kinds the --kind flag accepts.
var componentKinds = kindNames()

// kindNames lists every shareable component kind as a flag value.
func kindNames() []string {
	names := make([]string, 0, len(core.ComponentKinds))
	for _, k := range core.ComponentKinds {
		names = append(names, string(k))
	}
	return names
}

// newBundleCmd builds the component sharing command group.
func newBundleCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bundle",
		Short: "Share reusable components between projects, tenants and installations",
		Long: "Export reusable components as a bundle and import one somewhere else.\n\n" +
			"A bundle carries configuration and never work items, so sharing a way of " +
			"working cannot disclose what anyone is working on. Webhook endpoints travel " +
			"without their signing secrets and arrive inactive.\n\n" +
			"Both directions stream: a bundle written to standard output pipes straight " +
			"into an import on another installation, with no intermediate file.",
		Example: "  tix bundle export --kind workflow > review.bundle\n" +
			"  tix bundle export | tix bundle import --on-collision skip",
		GroupID: "admin",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(bundleExportCmd(g), bundleImportCmd(g))
	return cmd
}

// bundleExportCmd builds the command that streams a bundle to standard output.
func bundleExportCmd(g *globals) *cobra.Command {
	var name string
	var kinds, workflows, projects, webhooks []string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Stream a component bundle to standard output",
		Long: "Stream the selected components to standard output as one bundle document.\n" +
			"Components are written as they are walked, so the bundle never has to be " +
			"assembled in memory and pipes straight into tix bundle import.\n\n" +
			"Without a selector every kind the caller may read is exported. " +
			"Diagnostics go to standard error so the bundle is never polluted.\n\n" +
			fmt.Sprintf("Exit codes: %d unknown kind or selector, %d unknown component, %d permission denied.",
				core.KindInvalid.ExitCode(), core.KindNotFound.ExitCode(), core.KindForbidden.ExitCode()),
		Example: "  tix bundle export --kind workflow --workflow review > review.bundle\n" +
			"  tix bundle export --project infra --name platform-kit\n" +
			"  tix bundle export | tix bundle import --on-collision rename",
		Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			in := core.BundleExportInput{
				Name:         name,
				Kinds:        parseKinds(kinds),
				WorkflowKeys: workflows,
				ProjectRefs:  projects,
				WebhookIDs:   webhooks,
			}
			if err := in.Validate(); err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			return conn.Service.ExportBundle(ctx, in, cmd.OutOrStdout())
		},
	}
	f := cmd.Flags()
	f.StringVar(&name, "name", "", "name to label the bundle with")
	f.StringSliceVar(&kinds, "kind", nil,
		"component kind to export, repeatable: "+strings.Join(componentKinds, "|"))
	f.StringSliceVar(&workflows, "workflow", nil, "workflow key to export, repeatable")
	f.StringSliceVarP(&projects, "project", "p", nil, "project to export as a template, repeatable")
	f.StringSliceVar(&webhooks, "webhook", nil, "webhook endpoint id to export, repeatable")
	_ = cmd.RegisterFlagCompletionFunc("kind", fixedCompletion(componentKinds))
	registerCompletions(g, cmd, "project")
	return cmd
}

// bundleImportCmd builds the command that applies a bundle.
func bundleImportCmd(g *globals) *cobra.Command {
	var onCollision, project string
	var preview bool
	cmd := &cobra.Command{
		Use:   "import [FILE]",
		Short: "Apply a component bundle read from standard input or a file",
		Long: "Read a bundle from standard input, or from FILE, and apply it in one " +
			"transaction: either every component lands or none does.\n\n" +
			"A collision policy is required and has no default, because replace " +
			"overwrites a component others may be using. Skip leaves the existing " +
			"component alone, rename gives the incoming one a free key and reports it.\n\n" +
			fmt.Sprintf("Exit codes: %d invalid bundle or missing policy, %d unknown reference, %d permission denied.",
				core.KindInvalid.ExitCode(), core.KindNotFound.ExitCode(), core.KindForbidden.ExitCode()),
		Example: "  tix bundle import --on-collision skip < review.bundle\n" +
			"  tix bundle import --on-collision rename review.bundle\n" +
			"  tix bundle export | tix bundle import --on-collision replace\n" +
			"  tix bundle import --on-collision replace --preview review.bundle",
		Args: rangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			in := core.BundleImportInput{
				OnCollision: core.CollisionPolicy(onCollision),
				Preview:     preview,
				ProjectRef:  project,
			}
			if err := in.Validate(); err != nil {
				return err
			}
			src, closeSrc, err := bundleSource(cmd, args)
			if err != nil {
				return err
			}
			defer closeSrc()
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			result, err := conn.Service.ImportBundle(ctx, src, in)
			if err != nil {
				return err
			}
			return renderBundleResult(g, cmd, result)
		},
	}
	f := cmd.Flags()
	f.StringVar(&onCollision, "on-collision", "",
		"what to do when a key is taken, required: "+strings.Join(collisionPolicies, "|"))
	f.BoolVar(&preview, "preview", false, "report the plan and write nothing at all")
	f.StringVarP(&project, "project", "p", "", "project to import project-scoped components into")
	_ = cmd.RegisterFlagCompletionFunc("on-collision", fixedCompletion(collisionPolicies))
	registerCompletions(g, cmd, "project")
	return cmd
}

// parseKinds turns flag values into component kinds, leaving validation to core.
func parseKinds(values []string) []core.ComponentKind {
	if len(values) == 0 {
		return nil
	}
	kinds := make([]core.ComponentKind, 0, len(values))
	for _, v := range values {
		kinds = append(kinds, core.ComponentKind(strings.TrimSpace(v)))
	}
	return kinds
}

// bundleSource opens the bundle: the named file, or standard input.
func bundleSource(cmd *cobra.Command, args []string) (io.Reader, func(), error) {
	if len(args) == 0 || args[0] == StdinMarker {
		return cmd.InOrStdin(), func() {}, nil
	}
	f, err := os.Open(args[0]) // #nosec G304 -- the path is chosen by the user
	if err != nil {
		return nil, nil, core.Invalid("reading bundle %q: %v", args[0], err)
	}
	return f, func() { _ = f.Close() }, nil
}

// renderBundleResult writes an import result, tabulating the outcomes so the
// table format stays readable while every other format keeps the whole document.
func renderBundleResult(g *globals, cmd *cobra.Command, result *core.BundleResult) error {
	if result == nil {
		return core.Internal("the import reported no result")
	}
	if g.formatName() != output.FormatTable {
		return g.render(cmd, result)
	}
	for _, warning := range result.Warnings {
		g.diag(cmd, "warning: %s", warning)
	}
	if err := g.render(cmd, result.Outcomes); err != nil {
		return err
	}
	if !result.Preview {
		return nil
	}
	_, err := fmt.Fprintln(cmd.OutOrStdout(), "preview only: nothing was written")
	return err
}
