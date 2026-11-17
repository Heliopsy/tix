// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/spf13/cobra"
)

// newTenantSCIMCmd builds the SCIM provisioning commands.
func newTenantSCIMCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scim",
		Short: "Provision this tenant's users and groups from an identity provider over SCIM",
		Long: "SCIM 2.0 lets an identity provider such as Entra ID, Okta, Authentik or JumpCloud create, " +
			"update, deactivate and remove this tenant's people and groups. tix serve answers it under " +
			core.SCIMPath + ", for a bearer token carrying " + string(core.ScopeSCIMProvision) + ". " +
			"Enable it, give the provider the endpoint and the token, and map the provider's groups to roles.",
		Args: noArgs,
		RunE: helpRunner,
	}
	cmd.AddCommand(scimShowCmd(g), scimEnableCmd(g), scimDisableCmd(g), scimTokenCmd(g),
		scimMapCmd(g), scimUnmapCmd(g))
	return cmd
}

// scimSummary is the settings as one table row.
type scimSummary struct {
	Enabled      bool
	Endpoint     string
	Token        string
	Tokens       int
	Users        int
	Groups       int
	LastActivity string
}

// scimMapping is one group mapping as a table row.
type scimMapping struct {
	Group   string
	Role    core.Role
	Created string
}

// renderSCIM prints the settings, and the group mappings after them.
func (g *globals) renderSCIM(cmd *cobra.Command, s *core.SCIMSettings) error {
	if g.formatName() != output.FormatTable {
		return g.render(cmd, s)
	}
	endpoint := s.URL
	if endpoint == "" {
		endpoint = s.Path + " on your tix serve address"
	}
	last := "never"
	if s.LastActivityAt != nil {
		last = s.LastActivityAt.UTC().Format(time.RFC3339)
	}
	if err := g.render(cmd, scimSummary{Enabled: s.Enabled, Endpoint: endpoint, Token: s.Token,
		Tokens: len(s.Tokens), Users: s.Users, Groups: s.Groups, LastActivity: last}); err != nil {
		return err
	}
	if s.Token != "" {
		g.diag(cmd, "the token is shown once; give it to the identity provider now")
	}
	if len(s.GroupRoles) == 0 {
		return nil
	}
	rows := make([]scimMapping, 0, len(s.GroupRoles))
	for _, m := range s.GroupRoles {
		rows = append(rows, scimMapping{Group: m.Group, Role: m.Role, Created: m.CreatedAt.UTC().Format(time.RFC3339)})
	}
	return g.render(cmd, rows)
}

// scimSettingsCmd builds a command that changes or reads the settings and
// prints them.
func scimSettingsCmd(g *globals, use, short, long, example string,
	call func(svc core.Service, ctx context.Context) (*core.SCIMSettings, error),
) *cobra.Command {
	return &cobra.Command{
		Use: use, Short: short, Long: long, Example: example, Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			s, err := call(conn.Service, ctx)
			if err != nil {
				return err
			}
			return g.renderSCIM(cmd, s)
		},
	}
}

func scimShowCmd(g *globals) *cobra.Command {
	return scimSettingsCmd(g, "show",
		"Show whether SCIM is on, its endpoint, token, counts, last activity and group mappings",
		"Show the tenant's SCIM provisioning: whether it is on, the endpoint to give the identity provider, "+
			"how many provisioning tokens are live, how many users and groups the provider pushed, when it "+
			"last changed anything, and the group to role mappings. tix audit ls --via scim lists the changes.\n\n"+
			fmt.Sprintf("Exit codes: %d permission denied.", core.KindForbidden.ExitCode()),
		"  tix tenant scim show\n  tix tenant scim show -o json",
		func(svc core.Service, ctx context.Context) (*core.SCIMSettings, error) {
			return svc.GetSCIM(ctx)
		})
}

func scimEnableCmd(g *globals) *cobra.Command {
	return scimSettingsCmd(g, "enable", "Turn SCIM provisioning on and mint its token",
		"Turn SCIM provisioning on. The first time it creates the tenant's scim agent; whenever that agent "+
			"holds no live token it mints one named scim, carrying "+string(core.ScopeSCIMProvision)+" and "+
			"nothing else, and prints it once.\n\n"+
			fmt.Sprintf("Exit codes: %d permission denied, %d a token named scim already exists or the handle "+
				"scim belongs to a person.", core.KindForbidden.ExitCode(), core.KindConflict.ExitCode()),
		"  tix tenant scim enable",
		func(svc core.Service, ctx context.Context) (*core.SCIMSettings, error) {
			return svc.EnableSCIM(ctx)
		})
}

func scimDisableCmd(g *globals) *cobra.Command {
	return scimSettingsCmd(g, "disable", "Turn SCIM provisioning off, keeping the token",
		"Turn SCIM provisioning off. Every SCIM endpoint answers 404 until it is enabled again. The token "+
			"is kept, so turning it back on resumes the same provider; tix tenant scim token replaces it.\n\n"+
			fmt.Sprintf("Exit codes: %d permission denied.", core.KindForbidden.ExitCode()),
		"  tix tenant scim disable",
		func(svc core.Service, ctx context.Context) (*core.SCIMSettings, error) {
			return svc.DisableSCIM(ctx)
		})
}

func scimTokenCmd(g *globals) *cobra.Command {
	return scimSettingsCmd(g, "token", "Mint a new provisioning token, revoking the old one",
		"Revoke the scim agent's live tokens and mint a new one, printed once. The old token stops at once, "+
			"so give the new one to the identity provider straight away.\n\n"+
			fmt.Sprintf("Exit codes: %d permission denied, %d a token named scim already exists.",
				core.KindForbidden.ExitCode(), core.KindConflict.ExitCode()),
		"  tix tenant scim token",
		func(svc core.Service, ctx context.Context) (*core.SCIMSettings, error) {
			return svc.RotateSCIMToken(ctx)
		})
}

func scimMapCmd(g *globals) *cobra.Command {
	var role string
	cmd := &cobra.Command{
		Use:   "map-group NAME",
		Short: "Give the members of a SCIM group a tenant role",
		Long: "Map a SCIM group, by its display name and ignoring case, to a role: viewer, member or admin. A " +
			"person the provider pushed holds the highest role mapped to their groups, and drops to viewer " +
			"when they leave their last mapped group. Mapping a name again changes its role. A group can be " +
			"mapped before the provider pushes it.\n\n" +
			fmt.Sprintf("Exit codes: %d invalid name or role, %d permission denied.",
				core.KindInvalid.ExitCode(), core.KindForbidden.ExitCode()),
		Example: "  tix tenant scim map-group \"Engineering\" --role member\n" +
			"  tix tenant scim map-group tix-admins --role admin",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			m, err := conn.Service.MapSCIMGroup(ctx, core.SCIMGroupRoleInput{Group: args[0], Role: core.Role(role)})
			if err != nil {
				return err
			}
			if g.formatName() != output.FormatTable {
				return g.render(cmd, m)
			}
			return g.render(cmd, []scimMapping{{Group: m.Group, Role: m.Role,
				Created: m.CreatedAt.UTC().Format(time.RFC3339)}})
		},
	}
	cmd.Flags().StringVar(&role, "role", "", "the role the group's members get: viewer, member or admin")
	_ = cmd.MarkFlagRequired("role")
	return cmd
}

func scimUnmapCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "unmap-group NAME",
		Short: "Remove a SCIM group's role mapping",
		Long: "Remove the role mapping of a SCIM group. Members the provider pushed who are left in no mapped " +
			"group drop to viewer.\n\n" +
			fmt.Sprintf("Exit codes: %d no such mapping, %d permission denied.",
				core.KindNotFound.ExitCode(), core.KindForbidden.ExitCode()),
		Example: "  tix tenant scim unmap-group Engineering",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if err := conn.Service.UnmapSCIMGroup(ctx, args[0]); err != nil {
				return err
			}
			g.diag(cmd, "mapping for %s removed", args[0])
			return nil
		},
	}
}
