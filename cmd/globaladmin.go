// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"github.com/heliopsy/tix/internal/core"
	"github.com/spf13/cobra"
)

// newGlobalAdminCmd builds the global-admin command group.
func newGlobalAdminCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "global-admin",
		Short: "Manage global admins, who administer every tenant",
		Long: "A global admin creates tenants and administers every tenant of the installation.\n\n" +
			"The first one is granted from a shell holding the database, with --db or a local store and no token; " +
			"after that a global admin can grant others over the API.",
		GroupID: "admin",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(globalAdminGrantCmd(g), globalAdminRevokeCmd(g), globalAdminLsCmd(g))
	return cmd
}

func globalAdminGrantCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "grant EMAIL",
		Short: "Make a user a global admin",
		Long: "Make the user with this email a global admin. Granting it again changes nothing.\n\n" +
			"Exit codes: 2 invalid email, 3 unknown user, 5 not a global admin.",
		Example: "  tix --db /var/lib/tix/tix.db global-admin grant ada@example.com",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := core.ValidateEmail(args[0]); err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("global_admin.grant", args[0], nil))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			user, err := conn.Service.GrantGlobalAdmin(ctx, core.GlobalAdminInput{Email: args[0]})
			if err != nil {
				return err
			}
			return g.render(cmd, user)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

func globalAdminRevokeCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "revoke EMAIL",
		Short: "Take global admin away from a user",
		Long: "Take global admin away from the user with this email. The last global admin cannot be revoked.\n\n" +
			"Exit codes: 2 invalid email, 3 not a global admin, 5 permission denied, 6 the last global admin.",
		Example: "  tix global-admin revoke ada@example.com",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := core.ValidateEmail(args[0]); err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("global_admin.revoke", args[0], nil))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if err := conn.Service.RevokeGlobalAdmin(ctx, args[0]); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: args[0], Status: statusOK})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

func globalAdminLsCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List global admins",
		Long:    "List every global admin of the installation.\n\nExit codes: 5 not a global admin.",
		Example: "  tix global-admin ls",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			users, err := conn.Service.ListGlobalAdmins(ctx)
			if err != nil {
				return err
			}
			return g.render(cmd, users)
		},
	}
}
