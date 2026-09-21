package cmd

import (
	"fmt"

	"github.com/heliopsy/tix/internal/core"
	"github.com/spf13/cobra"
)

// init registers snapshot transfer alongside the other command groups.
func init() { builders = append(builders, newExportCmd, newImportCmd) }

// newExportCmd builds the snapshot export command.
func newExportCmd(g *globals) *cobra.Command {
	var projects []string
	var artifacts, comments bool
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Stream a tenant snapshot to standard output",
		Long: "Stream a snapshot of the tenant to standard output, one record per line.\n" +
			"Records are written as the data is walked, so a snapshot of any size " +
			"streams in constant memory and pipes straight into tix import.\n\n" +
			fmt.Sprintf("Exit codes: %d unknown project, %d permission denied.",
				core.KindNotFound.ExitCode(), core.KindForbidden.ExitCode()),
		Example: "  tix export > snapshot.ndjson\n" +
			"  tix export -p infra --comments\n" +
			"  tix export | tix import --mode merge",
		GroupID: "admin",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			in := core.ExportInput{
				ProjectRefs:      projects,
				IncludeArtifacts: artifacts,
				IncludeComments:  comments,
			}
			return conn.Service.ExportTo(ctx, in, cmd.OutOrStdout())
		},
	}
	f := cmd.Flags()
	f.StringSliceVarP(&projects, "project", "p", nil, "project to export, repeatable; every project by default")
	f.BoolVar(&comments, "comments", false, "include task comments")
	f.BoolVar(&artifacts, "artifacts", false, "include task artifacts")
	registerCompletions(g, cmd, "project")
	return cmd
}
