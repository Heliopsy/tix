// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"fmt"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/spf13/cobra"
)

// importModes are the modes the import command accepts.
var importModes = flagValues(core.ImportModes)

// newImportCmd builds the snapshot import command.
func newImportCmd(g *globals) *cobra.Command {
	var mode string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Apply a tenant snapshot read from standard input",
		Long: "Read a snapshot from standard input, one record per line, and apply it " +
			"to the tenant in a single transaction.\n" +
			"A mode is required: merge creates and updates, replace also removes " +
			"records the snapshot does not carry.\n" +
			"A task is matched by its own identifier, never by its project-and-sequence " +
			"reference; a record older than what it would replace is skipped, not applied, " +
			"and every skip is named in the result.\n\n" +
			fmt.Sprintf("Exit codes: %d invalid snapshot or missing mode, %d unknown reference, %d permission denied.",
				core.KindInvalid.ExitCode(), core.KindNotFound.ExitCode(), core.KindForbidden.ExitCode()),
		Example: "  tix import --mode merge < snapshot.ndjson\n" +
			"  tix export | tix import --mode replace\n" +
			"  tix import --mode replace --dry-run < snapshot.ndjson",
		GroupID: "admin",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			in := core.ImportInput{Mode: core.ImportMode(mode), DryRun: dryRun}
			if err := in.Validate(); err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			result, err := conn.Service.ImportFrom(ctx, cmd.InOrStdin(), in)
			if err != nil {
				return err
			}
			return g.render(cmd, result)
		},
	}
	f := cmd.Flags()
	f.StringVar(&mode, "mode", "", "how to apply the snapshot: "+strings.Join(importModes, "|"))
	f.BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	_ = cmd.RegisterFlagCompletionFunc("mode", fixedCompletion(importModes))
	return cmd
}
