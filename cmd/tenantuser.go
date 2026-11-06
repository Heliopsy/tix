// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/core"
	"github.com/spf13/cobra"
)

// newTenantUserCmd builds the group a global admin manages another tenant's
// users with. Each command names the tenant first.
func newTenantUserCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Manage the users of any tenant, as a global admin",
		Long: "Manage the users of the tenant named by KEY, which need not be the one you are signed in to. " +
			"Only a global admin may; tix user manages the users of your own tenant.",
		Args: noArgs,
		RunE: helpRunner,
	}
	cmd.AddCommand(tenantUserLsCmd(g), tenantUserCreateCmd(g), tenantUserEditCmd(g), tenantUserRmCmd(g))
	return cmd
}

func tenantUserLsCmd(g *globals) *cobra.Command {
	var limit int
	var cursor string
	cmd := &cobra.Command{
		Use:     "ls KEY",
		Aliases: []string{"list"},
		Short:   "List a tenant's users with their roles",
		Long:    "List the users of a tenant and each one's role there.\n\nExit codes: 3 unknown tenant, 5 not a global admin.",
		Example: "  tix tenant user ls acme",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			size, err := g.pageSize(cmd, limit)
			if err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			users, next, err := conn.Service.ListTenantUsers(ctx, args[0], core.Page{Limit: size, Cursor: cursor})
			if err != nil {
				return err
			}
			if next != "" {
				g.diag(cmd, "more results available; next cursor %s", next)
			}
			return g.render(cmd, users)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", config.DefaultPageSize, "maximum records to return")
	cmd.Flags().StringVar(&cursor, "cursor", "", "continue from a previous page")
	return cmd
}

func tenantUserCreateCmd(g *globals) *cobra.Command {
	var password, display, handle, role string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "create KEY EMAIL",
		Short: "Create a user in a tenant",
		Long: "Create a credentialed human in the tenant named by KEY.\n\n" +
			"Exit codes: 2 invalid input, 3 unknown tenant, 4 email already used, 5 not a global admin.",
		Example: "  tix tenant user create acme ada@example.com --role admin --password '...'",
		Args:    exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			in := core.CreateUserInput{
				Email: args[1], Password: password, DisplayName: display,
				Handle: handle, Role: core.Role(role),
			}
			if err := in.Validate(); err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("tenant.user.create", args[0]+"/"+in.Email, nil))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			user, err := conn.Service.CreateTenantUser(ctx, args[0], in)
			if err != nil {
				return err
			}
			return g.render(cmd, user)
		},
	}
	f := cmd.Flags()
	f.StringVar(&password, "password", "", "initial password")
	f.StringVar(&display, "display-name", "", "display name")
	f.StringVar(&handle, "handle", "", "actor handle")
	f.StringVar(&role, "role", "", "role in that tenant")
	f.BoolVar(&dryRun, "dry-run", false, "report what would be created without writing")
	return cmd
}

func tenantUserEditCmd(g *globals) *cobra.Command {
	var display, password, role string
	var disabled, dryRun bool
	cmd := &cobra.Command{
		Use:   "edit KEY ID",
		Short: "Change a user of a tenant",
		Long: "Change the display name, password, role or disabled state of a user of the tenant named by KEY.\n\n" +
			"Exit codes: 3 unknown tenant or user, 5 not a global admin.",
		Example: "  tix tenant user edit acme 01J000000000000000000A --role admin",
		Args:    exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var in core.UpdateUserInput
			if cmd.Flags().Changed("display-name") {
				in.DisplayName = &display
			}
			if cmd.Flags().Changed("password") {
				in.Password = &password
			}
			if cmd.Flags().Changed("role") {
				r := core.Role(role)
				in.Role = &r
			}
			if cmd.Flags().Changed("disabled") {
				in.Disabled = &disabled
			}
			if dryRun {
				return g.render(cmd, newPlan("tenant.user.update", args[0]+"/"+args[1], nil))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			user, err := conn.Service.UpdateTenantUser(ctx, args[0], args[1], in)
			if err != nil {
				return err
			}
			return g.render(cmd, user)
		},
	}
	f := cmd.Flags()
	f.StringVar(&display, "display-name", "", "new display name")
	f.StringVar(&password, "password", "", "new password")
	f.StringVar(&role, "role", "", "new role")
	f.BoolVar(&disabled, "disabled", false, "disable or re-enable the user")
	f.BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

func tenantUserRmCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "rm KEY ID",
		Aliases: []string{"delete"},
		Short:   "Delete a user of a tenant",
		Long:    "Delete a user of the tenant named by KEY.\n\nExit codes: 3 unknown tenant or user, 5 not a global admin.",
		Example: "  tix tenant user rm acme 01J000000000000000000A",
		Args:    exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRun {
				return g.render(cmd, newPlan("tenant.user.delete", args[0]+"/"+args[1], nil))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if err := conn.Service.DeleteTenantUser(ctx, args[0], args[1]); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: args[1], Status: statusOK})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be deleted without writing")
	return cmd
}
