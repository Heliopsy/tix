// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"time"

	"github.com/heliopsy/tix/internal/connect"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/spf13/cobra"
)

// inviteHelp says what an invite is, once, for every command in the group.
const inviteHelp = "An invite link lets somebody choose their own password instead of being handed one. " +
	"Inviting a new address creates the account at once, with no password, so it can be assigned work " +
	"straight away; opening the link asks for a password and a display name and signs the person in. " +
	"An address of this tenant's user who holds no membership is given one, and keeps its password.\n\n" +
	"tix sends no email: send the link yourself. It works once, expires after 7 days unless --expires " +
	"says otherwise (1h to 30d), and is shown only when it is issued, because only its hash is kept."

// newUserInviteCmd builds the invite command: inviting is the command itself,
// and managing pending invites is its subcommands.
func newUserInviteCmd(g *globals) *cobra.Command {
	var display, handle, role, expires string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "invite EMAIL",
		Short: "Invite somebody by link instead of setting their password",
		Long: "Invite an email address to this tenant and print a single-use link.\n\n" + inviteHelp + "\n\n" +
			"Exit codes: 2 invalid input or expiry, 4 the address belongs to a member who can already " +
			"sign in, to another tenant's user, or to an account with a pending invite, 5 permission denied.",
		Example: "  tix user invite ada@example.com --role member\n" +
			"  tix user invite ada@example.com --expires 48h -o json",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ttl, err := inviteExpiry(expires)
			if err != nil {
				return err
			}
			in := core.CreateInviteInput{
				Email: args[0], DisplayName: display, Handle: handle, Role: core.Role(role), ExpiresIn: ttl,
			}
			if err := in.Validate(); err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("user_invite.create", in.Email, nil))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			issued, err := conn.Service.CreateInvite(ctx, in)
			if err != nil {
				return err
			}
			return g.renderIssuedInvite(cmd, conn, issued)
		},
	}
	f := cmd.Flags()
	f.StringVar(&display, "display-name", "", "display name until the person chooses one")
	f.StringVar(&handle, "handle", "", "actor handle; empty uses the part of the email before the @")
	f.StringVar(&role, "role", "", "role in the current tenant; empty means viewer")
	f.StringVar(&expires, "expires", "", "how long the link works, such as 48h or 7d (1h to 30d); empty means 7d")
	f.BoolVar(&dryRun, "dry-run", false, "report what would be created without writing")
	cmd.AddCommand(userInviteLsCmd(g), userInviteRevokeCmd(g), userInviteReissueCmd(g))
	return cmd
}

// inviteExpiry parses --expires, which is empty for the default.
func inviteExpiry(value string) (core.Duration, error) {
	if value == "" {
		return 0, nil
	}
	d, err := core.ParseDuration(value)
	if err != nil {
		return 0, core.Invalid("--expires %q: %v", value, err).WithDetail(core.DetailField, "expires_in")
	}
	return d, nil
}

// renderIssuedInvite prints an invite link, the one time it can be printed.
func (g *globals) renderIssuedInvite(cmd *cobra.Command, conn *connect.Conn, issued *core.IssuedInvite) error {
	if issued.Path == "" {
		issued.Path = core.InvitePath(issued.Token)
	}
	link := issued.URL
	if link == "" {
		link = issued.Path
	}
	g.diag(cmd, "send this link to them: tix does not send email")
	g.diag(cmd, "it works once, expires %s, and is not shown again", inviteTime(issued.ExpiresAt))
	if issued.URL == "" && conn.Info.Target.Mode != connect.ModeRemote {
		g.diag(cmd, "this is a local database: put the address people open tix at in front of the path")
	}
	if f := g.formatName(); f != output.FormatTable && f != "" {
		return g.render(cmd, issued)
	}
	return g.render(cmd, []issuedRow{{ID: issued.ID, Email: issued.Email, Role: issued.Role,
		Expires: inviteTime(issued.ExpiresAt), Link: link}})
}

// inviteRow is one pending invite as the table shows it.
type inviteRow struct {
	ID      string
	Email   string
	Role    core.Role
	By      string
	Expires string
	State   string
}

// issuedRow is a freshly issued invite as the table shows it: the link in a
// column of its own rather than buried in the record.
type issuedRow struct {
	ID      string
	Email   string
	Role    core.Role
	Expires string
	Link    string
}

// inviteTime renders an instant to the minute, which is all an expiry needs.
func inviteTime(t time.Time) string { return t.UTC().Format("2006-01-02 15:04 UTC") }

func userInviteLsCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List pending invites",
		Long: "List the invites neither accepted nor revoked, oldest first, expired ones included so they can " +
			"be re-issued. No link is shown: it was printed once, when it was issued.\n\nExit codes: 5 permission denied.",
		Example: "  tix user invite ls\n  tix user invite ls -o json",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			invites, err := conn.Service.ListInvites(ctx)
			if err != nil {
				return err
			}
			if f := g.formatName(); f != output.FormatTable && f != "" {
				return g.render(cmd, invites)
			}
			now := time.Now()
			rows := make([]inviteRow, 0, len(invites))
			for _, inv := range invites {
				by := inv.InvitedByHandle
				if by == "" {
					by = inv.InvitedBy
				}
				state := "pending"
				if !inv.ExpiresAt.After(now) {
					state = "expired"
				}
				rows = append(rows, inviteRow{ID: inv.ID, Email: inv.Email, Role: inv.Role, By: by,
					Expires: inviteTime(inv.ExpiresAt), State: state})
			}
			return g.render(cmd, rows)
		},
	}
}

func userInviteRevokeCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "revoke ID",
		Aliases: []string{"rm"},
		Short:   "Stop a pending invite's link",
		Long: "Stop a pending invite's link at once. The account it created stays, without a password, until " +
			"you re-issue the invite or delete the user.\n\nExit codes: 3 unknown or already revoked invite, " +
			"4 already accepted, 5 permission denied.",
		Example: "  tix user invite revoke 01J000000000000000000A",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if err := conn.Service.RevokeInvite(ctx, args[0]); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: args[0], Status: statusOK})
		},
	}
}

func userInviteReissueCmd(g *globals) *cobra.Command {
	var expires string
	cmd := &cobra.Command{
		Use:   "reissue ID",
		Short: "Replace an invite's link with a new one",
		Long: "Issue a new link for a pending or expired invite, with a fresh expiry, and stop the old one. " +
			"The new link is printed once.\n\nExit codes: 2 invalid expiry, 3 unknown or revoked invite, " +
			"4 already accepted, 5 permission denied.",
		Example: "  tix user invite reissue 01J000000000000000000A --expires 3d",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ttl, err := inviteExpiry(expires)
			if err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			issued, err := conn.Service.ReissueInvite(ctx, args[0], core.ReissueInviteInput{ExpiresIn: ttl})
			if err != nil {
				return err
			}
			return g.renderIssuedInvite(cmd, conn, issued)
		},
	}
	cmd.Flags().StringVar(&expires, "expires", "", "how long the new link works, such as 48h or 7d (1h to 30d); empty means 7d")
	return cmd
}
