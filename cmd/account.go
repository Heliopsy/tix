// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/core"
	"github.com/spf13/cobra"
)

// newAccountCmd builds the group a person reads their own account with.
func newAccountCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account",
		Short: "Read your own account, across every tenant you belong to",
		Long: "Your account is one email, one password and one second factor, shared by every tenant you " +
			"belong to.",
		GroupID: "setup",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(accountLogCmd(g))
	return cmd
}

func accountLogCmd(g *globals) *cobra.Command {
	var limit int
	var cursor string
	cmd := &cobra.Command{
		Use:   "log",
		Short: "List the changes to your account, newest first",
		Long: "List every change to your password, second factor, disabled state, global admin standing and " +
			"memberships, in every tenant you belong to, with who made it, from which tenant and through which " +
			"channel. Only you and global admins can read it. A token needs the account:read scope, which * does " +
			"not carry: tix token create NAME --scope account:read.\n\n" +
			"Exit codes: 2 the caller is not a person, 5 a token without account:read.",
		Example: "  tix account log\n  tix account log -o json",
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
			entries, next, err := conn.Service.GetAccountLog(ctx, core.Page{Limit: size, Cursor: cursor, Direction: core.Descending})
			if err != nil {
				return err
			}
			if next != "" {
				g.diag(cmd, "more results available; next cursor %s", next)
			}
			return g.render(cmd, entries)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", config.DefaultPageSize, "maximum records to return")
	cmd.Flags().StringVar(&cursor, "cursor", "", "continue from a previous page")
	return cmd
}

func globalAdminAccountLogCmd(g *globals) *cobra.Command {
	var limit int
	var cursor string
	cmd := &cobra.Command{
		Use:   "account-log EMAIL",
		Short: "List the changes to any account, newest first",
		Long: "List every change to the account that signs in as EMAIL, in every tenant it belongs to. Only a " +
			"global admin may.\n\nExit codes: 3 no such account, 5 not a global admin.",
		Example: "  tix global-admin account-log villy@example.com",
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
			entries, next, err := conn.Service.GetAccountLogFor(ctx, args[0], core.Page{Limit: size, Cursor: cursor, Direction: core.Descending})
			if err != nil {
				return err
			}
			if next != "" {
				g.diag(cmd, "more results available; next cursor %s", next)
			}
			return g.render(cmd, entries)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", config.DefaultPageSize, "maximum records to return")
	cmd.Flags().StringVar(&cursor, "cursor", "", "continue from a previous page")
	return cmd
}
