// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"github.com/heliopsy/tix/internal/core"
	extsync "github.com/heliopsy/tix/internal/sync"
	"github.com/spf13/cobra"
)

// newSyncCmd builds the external import command group.
func newSyncCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Import from Jira, OpenProject and shaped files",
		Long: "Import from an external system into tix.\n\n" +
			"Imports are one way: tix never writes back. Every imported entity records an\n" +
			"external reference, so a second run refreshes what changed instead of\n" +
			"duplicating it.\n\n" +
			"Endpoints, mapping files and credentials are read from the environment as\n" +
			extsync.EnvPrefix + "<SOURCE>_URL, _TOKEN, _USER, _PASSWORD, _FILE, _MAPPING,\n" +
			"_QUERY, _PROJECT and _PAGE_SIZE. Nothing a source authenticates with is ever\n" +
			"stored, audited or logged.",
		GroupID: "admin",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(newSyncSourceCmd(g), syncRunCmd(g))
	return cmd
}

// newSyncSourceCmd builds the import source command group.
func newSyncSourceCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "source",
		Aliases: []string{"sources"},
		Short:   "Manage external import sources",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(syncSourceAddCmd(g), syncSourceLsCmd(g), syncSourceRmCmd(g))
	return cmd
}

func syncSourceAddCmd(g *globals) *cobra.Command {
	var id string
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "add SYSTEM NAME",
		Aliases: []string{"put"},
		Short:   "Register or update an import source",
		Long: "Register an import source, or update one by passing --id.\n\n" +
			"SYSTEM is one of generic, jira or openproject. NAME selects the environment\n" +
			"variables the source is configured from.\n\n" +
			"Exit codes: 2 unknown system or missing name, 5 permission denied.",
		Example: "  tix sync source add generic ops\n  tix sync source add jira platform",
		Args:    exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			in := core.SyncSourceInput{ID: id, System: args[0], Name: args[1]}
			if err := in.Validate(); err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("sync.source.put", in.Name,
					map[string]any{"system": in.System}))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			source, err := conn.Service.PutSyncSource(ctx, in)
			if err != nil {
				return err
			}
			g.diag(cmd, "configure this source through %s*", extsync.EnvName(in.Name, ""))
			return g.render(cmd, source)
		},
	}
	f := cmd.Flags()
	f.StringVar(&id, "id", "", "identifier of the source to update")
	f.BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

func syncSourceLsCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List import sources",
		Long:    "List import sources with their cursors.\n\nExit codes: 5 permission denied.",
		Example: "  tix sync source ls",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			sources, err := conn.Service.ListSyncSources(ctx)
			if err != nil {
				return err
			}
			return g.render(cmd, sources)
		},
	}
}

func syncSourceRmCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "rm ID",
		Aliases: []string{"delete"},
		Short:   "Remove an import source",
		Long: "Remove an import source and its cursor. Imported tasks are left alone.\n\n" +
			"Exit codes: 3 unknown source, 5 permission denied.",
		Example: "  tix sync source rm 01J0000000000000000000",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRun {
				return g.render(cmd, newPlan("sync.source.delete", args[0], nil))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if err := conn.Service.DeleteSyncSource(ctx, args[0]); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: args[0], Status: statusOK})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

func syncRunCmd(g *globals) *cobra.Command {
	var dryRun, full bool
	cmd := &cobra.Command{
		Use:   "run ID",
		Short: "Import from a configured source",
		Long: "Import from a configured source.\n\n" +
			"Only records changed since the last successful run are fetched, unless --full\n" +
			"is given. --dry-run reports every creation, update and skip, with the reason\n" +
			"for each skip, and writes nothing.\n\n" +
			"Exit codes: 2 unusable mapping or configuration, 3 unknown source, 5 permission denied.",
		Example: "  tix sync run 01J0000000000000000000 --dry-run\n" +
			"  tix sync run 01J0000000000000000000 --full",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			result, err := conn.Service.RunSync(ctx, core.RunSyncInput{
				SourceID: args[0], DryRun: dryRun, Full: full,
			})
			if err != nil {
				return err
			}
			if dryRun {
				g.diag(cmd, "dry run: nothing was written")
			}
			return g.render(cmd, result)
		},
	}
	f := cmd.Flags()
	f.BoolVar(&dryRun, "dry-run", false, "report what would be created, updated and skipped without writing")
	f.BoolVar(&full, "full", false, "ignore the stored cursor and reconsider every source record")
	return cmd
}
