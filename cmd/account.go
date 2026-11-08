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
		Short: "Read your own account and manage your sessions",
		Long: "Your account is one email, one password and one second factor, shared by every tenant you " +
			"belong to.",
		GroupID: "setup",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(accountLogCmd(g), accountSessionsCmd(g))
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

// sessionsHelp says who may manage sessions, shared by both commands.
const sessionsHelp = "A session is a browser or terminal sign-in in this tenant. Only your own are listed or " +
	"revoked. A token needs the account:read scope, which * does not carry; the token tix login stores has it."

func accountSessionsCmd(g *globals) *cobra.Command {
	var limit int
	var cursor string
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "List your own sessions, newest first",
		Long: "List your unexpired sessions in this tenant: when each signed in and expires, how, whether a " +
			"second factor passed, the user agent it signed in with, and which one this request presented. " +
			"Tokens are never shown.\n\n" + sessionsHelp + "\n\n" +
			"Exit codes: 5 a token without account:read.",
		Example: "  tix account sessions\n  tix account sessions -o json",
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
			sessions, next, err := conn.Service.ListSessions(ctx, core.Page{Limit: size, Cursor: cursor})
			if err != nil {
				return err
			}
			if next != "" {
				g.diag(cmd, "more results available; next cursor %s", next)
			}
			return g.render(cmd, sessions)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", config.DefaultPageSize, "maximum records to return")
	cmd.Flags().StringVar(&cursor, "cursor", "", "continue from a previous page")
	cmd.AddCommand(accountSessionRevokeCmd(g))
	return cmd
}

func accountSessionRevokeCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "revoke ID",
		Aliases: []string{"rm"},
		Short:   "End one of your own sessions",
		Long: "End the session with this identifier, from tix account sessions. Whatever holds it is signed " +
			"out at once.\n\n" + sessionsHelp + "\n\n" +
			"Exit codes: 3 no session of yours has that identifier, 5 a token without account:read.",
		Example: "  tix account sessions revoke 01J8Z3K4M5N6P7Q8R9S0T1V2W3",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if err := conn.Service.RevokeSession(ctx, args[0]); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: args[0], Status: statusOK})
		},
	}
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
