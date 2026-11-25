// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/connect"
	"github.com/heliopsy/tix/internal/core"
	"github.com/spf13/cobra"
)

// davHelp says what CalDAV and an app password are, once, for every command
// in the group.
const davHelp = "tix serve answers CalDAV under /dav/, so Apple Reminders, Thunderbird, tasks.org or jtx Board " +
	"through DAVx5, and Evolution list your tasks as to-dos. They can tick them off, " +
	"edit, add and cancel them too, unless the app password is read-only.\n\n" +
	"A client signs in with your sign-in email as the username and an app password: a token that works " +
	"only for CalDAV, which any person can create for themselves. Each app password chooses its " +
	"collections: assigned (Assigned to me, the default), projects (one list per project) or all. It is " +
	"shown once, when it is created, because only its hash is kept. See docs/caldav.md."

// newDAVCmd builds the dav command group.
func newDAVCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "dav",
		Aliases: []string{"caldav"},
		Short:   "Show your tasks in reminders and calendar apps over CalDAV",
		Long:    "Manage CalDAV app passwords and see how to connect a client.\n\n" + davHelp,
		GroupID: "work",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	password := &cobra.Command{
		Use:     "password",
		Aliases: []string{"passwords"},
		Short:   "Manage your CalDAV app passwords",
		Long:    "Create, list and revoke the app passwords your CalDAV clients sign in with.\n\n" + davHelp,
		Args:    noArgs,
		RunE:    helpRunner,
	}
	password.AddCommand(davPasswordListCmd(g), davPasswordCreateCmd(g), davPasswordRevokeCmd(g))
	cmd.AddCommand(davShowCmd(g), password)
	return cmd
}

func davShowCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "show",
		Aliases: []string{"status", "info"},
		Short:   "Show whether CalDAV is on, its address and your username",
		Long: "Show whether this server answers CalDAV, the address to give a client, the username to sign " +
			"in with and whether the lists accept changes.\n\n" + davHelp + "\n\n" +
			"Exit codes: 5 permission denied, as for an agent.",
		Example: "  tix dav show\n  tix dav show -o json",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			info, err := conn.Service.GetDAVInfo(ctx)
			if err != nil {
				return err
			}
			g.noteLocalDAV(cmd, conn, info.URL)
			return g.render(cmd, info)
		},
	}
}

func davPasswordListCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List your app passwords",
		Long: "List your app passwords, revoked ones included, with what each exposes and when a client " +
			"last used it. The passwords themselves are never shown again.\n\n" + davHelp + "\n\n" +
			"Exit codes: 5 permission denied.",
		Example: "  tix dav password list\n  tix dav password list -o json",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			list, err := conn.Service.ListDAVPasswords(ctx)
			if err != nil {
				return err
			}
			return g.render(cmd, list)
		},
	}
}

func davPasswordCreateCmd(g *globals) *cobra.Command {
	var (
		collections string
		readOnly    bool
		expires     string
	)
	cmd := &cobra.Command{
		Use:     "create NAME",
		Aliases: []string{"new", "add"},
		Short:   "Create an app password and print it once",
		Long: "Create an app password for one client and print it, with the address and the username to " +
			"sign in with. NAME is yours to recognise the client by, such as iPhone; it is unique among your " +
			"live app passwords. Unlike tix token create, an app password does not expire unless --expires " +
			"says so, because a phone that silently stops syncing is worse than one you revoke.\n\n" +
			davHelp + "\n\n" +
			"Exit codes: 2 an unreadable name, collections or expiry, 4 you already have one by that name, " +
			"5 permission denied, as for an agent.",
		Example: "  tix dav password create iPhone\n" +
			"  tix dav password create 'Work Thunderbird' --collections projects\n" +
			"  tix dav password create tablet --read-only --expires 30d",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			in := core.CreateDAVPasswordInput{Name: args[0], Collections: core.DAVCollections(collections),
				ReadOnly: readOnly}
			if strings.TrimSpace(expires) != "" && !strings.EqualFold(strings.TrimSpace(expires), ExpiryNever) {
				at, err := parseExpiry(expires, time.Now())
				if err != nil {
					return err
				}
				in.ExpiresAt = at
			}
			if _, err := in.Normalize(); err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			issued, err := conn.Service.CreateDAVPassword(ctx, in)
			if err != nil {
				return err
			}
			g.diag(cmd, "the app password is shown once and cannot be retrieved again")
			g.noteLocalDAV(cmd, conn, issued.URL)
			return g.render(cmd, issued)
		},
	}
	f := cmd.Flags()
	f.StringVar(&collections, "collections", string(core.DAVCollectionsAssigned),
		"which lists the client shows: assigned, projects or all")
	f.BoolVar(&readOnly, "read-only", false, "let this client list tasks but never change them")
	f.StringVar(&expires, "expires", ExpiryNever,
		"when the password stops working: a duration such as 90d, a date, an RFC3339 timestamp, or never")
	names := make([]string, len(core.AllDAVCollections))
	for i, c := range core.AllDAVCollections {
		names[i] = string(c)
	}
	_ = cmd.RegisterFlagCompletionFunc("collections", fixedCompletion(names))
	return cmd
}

func davPasswordRevokeCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "revoke ID",
		Aliases: []string{"rm"},
		Short:   "Revoke one of your app passwords",
		Long: "Revoke one of your app passwords. The client using it is refused from its next request; " +
			"nothing else of yours is affected. An administrator revokes somebody else's with tix token " +
			"revoke.\n\n" + davHelp + "\n\n" +
			"Exit codes: 3 no app password of yours has that identifier, 5 permission denied.",
		Example: "  tix dav password revoke 01JD8Y3M6Q0000000000000000",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if err := conn.Service.RevokeDAVPassword(ctx, args[0]); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: args[0], Status: statusOK})
		},
	}
}

// noteLocalDAV explains a missing address: a local database is not a server,
// and CalDAV is answered by tix serve at whatever address that is reached on.
func (g *globals) noteLocalDAV(cmd *cobra.Command, conn *connect.Conn, url string) {
	if url == "" && conn.Info.Target.Mode != connect.ModeRemote {
		g.diag(cmd, "this is a local database: CalDAV is answered by tix serve, at /dav/ on its address")
	}
}
