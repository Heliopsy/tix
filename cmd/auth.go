// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"strings"

	"github.com/heliopsy/tix/internal/client"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
	"github.com/spf13/cobra"
)

// newActorCmd builds the actor command group.
func newActorCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "actor",
		Short:   "Resolve actor identifiers",
		GroupID: "admin",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(actorShowCmd(g))
	return cmd
}

func actorShowCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "show ID",
		Short:   "Show the identity behind an actor handle or identifier",
		Long:    "Resolve an actor handle or identifier to its handle, kind and display name.\n\nExit codes: 3 unknown actor, 5 permission denied.",
		Example: "  tix actor show 01J000000000000000000A\n  tix actor show ada",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			actor, err := conn.Service.GetActor(ctx, args[0])
			if err != nil {
				return err
			}
			return g.render(cmd, actor)
		},
	}
}

// newUserCmd builds the user command group.
func newUserCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "user",
		Short:   "Manage users",
		GroupID: "admin",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(userCreateCmd(g), userLsCmd(g), userShowCmd(g), userEditCmd(g), userRmCmd(g), newUserKeyCmd(g))
	return cmd
}

func userCreateCmd(g *globals) *cobra.Command {
	var password, display, handle, role string
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "create EMAIL",
		Short:   "Create a user",
		Long:    "Create a credentialed human.\n\nExit codes: 2 invalid input, 4 email already used, 5 permission denied.",
		Example: "  tix user create ada@example.com --role admin",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			in := core.CreateUserInput{
				Email: args[0], Password: password, DisplayName: display,
				Handle: handle, Role: core.Role(role),
			}
			if err := in.Validate(); err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("user.create", in.Email, nil))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			user, err := conn.Service.CreateUser(ctx, in)
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
	f.StringVar(&role, "role", "", "role in the current tenant")
	f.BoolVar(&dryRun, "dry-run", false, "report what would be created without writing")
	return cmd
}

func userLsCmd(g *globals) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List users",
		Long:    "List users.\n\nExit codes: 5 permission denied.",
		Example: "  tix user ls -o json",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			users, next, err := conn.Service.ListUsers(ctx, core.Page{Limit: limit})
			if err != nil {
				return err
			}
			if next != "" {
				g.diag(cmd, "more results available; next cursor %s", next)
			}
			return g.render(cmd, users)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", core.DefaultPageLimit, "maximum records to return")
	return cmd
}

func userShowCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "show ID",
		Short:   "Show one user",
		Long:    "Show a user by identifier.\n\nExit codes: 3 unknown user.",
		Example: "  tix user show 01J000000000000000000A",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			user, err := conn.Service.GetUser(ctx, args[0])
			if err != nil {
				return err
			}
			return g.render(cmd, user)
		},
	}
}

func userEditCmd(g *globals) *cobra.Command {
	var display, password, role string
	var disabled, dryRun bool
	cmd := &cobra.Command{
		Use:     "edit ID",
		Short:   "Change a user",
		Long:    "Change a user's display name, password, role or disabled state.\n\nExit codes: 3 unknown user.",
		Example: "  tix user edit 01J000000000000000000A --role viewer",
		Args:    exactArgs(1),
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
				return g.render(cmd, newPlan("user.update", args[0], nil))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			user, err := conn.Service.UpdateUser(ctx, args[0], in)
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

func userRmCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "rm ID",
		Aliases: []string{"delete"},
		Short:   "Delete a user",
		Long:    "Delete a user.\n\nExit codes: 3 unknown user, 5 permission denied.",
		Example: "  tix user rm 01J000000000000000000A",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRun {
				return g.render(cmd, newPlan("user.delete", args[0], nil))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if err := conn.Service.DeleteUser(ctx, args[0]); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: args[0], Status: statusOK})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be deleted without writing")
	return cmd
}

// newTokenCmd builds the API token command group.
func newTokenCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "token",
		Short:   "Manage API tokens",
		GroupID: "admin",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(tokenCreateCmd(g), tokenLsCmd(g), tokenRmCmd(g))
	return cmd
}

func tokenCreateCmd(g *globals) *cobra.Command {
	var actor, project, expires string
	var scopes []string
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "create NAME",
		Short:   "Mint an API token",
		Long:    "Mint a token whose secret is shown exactly once.\n\nExit codes: 2 unknown scope, 3 unknown project or actor, 5 permission denied.",
		Example: "  tix token create agent --scope task:read --scope task:claim",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			expiry, err := parseTime(expires)
			if err != nil {
				return err
			}
			in := core.CreateTokenInput{
				Name: args[0], ActorID: actor, ProjectID: project,
				Scopes: toScopes(scopes), ExpiresAt: expiry,
			}
			if err := in.Validate(); err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("token.create", in.Name, map[string]any{"scopes": scopes}))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			issued, err := conn.Service.CreateToken(ctx, in)
			if err != nil {
				return err
			}
			g.diag(cmd, "the token value is shown once and cannot be retrieved again")
			return g.render(cmd, issued)
		},
	}
	f := cmd.Flags()
	f.StringSliceVar(&scopes, "scope", nil, "scope to grant, repeatable")
	f.StringVar(&actor, "actor", "", "actor the token acts as, by handle or identifier")
	f.StringVar(&project, "project", "", "restrict the token to one project, by key or identifier")
	f.StringVar(&expires, "expires", "", "expiry date")
	f.BoolVar(&dryRun, "dry-run", false, "report what would be created without writing")
	_ = cmd.RegisterFlagCompletionFunc("scope", fixedCompletion(scopeNames()))
	return cmd
}

func tokenLsCmd(g *globals) *cobra.Command {
	var actor string
	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List API tokens",
		Long:    "List tokens, optionally for one actor.\n\nExit codes: 5 permission denied.",
		Example: "  tix token ls",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			tokens, err := conn.Service.ListTokens(ctx, actor)
			if err != nil {
				return err
			}
			return g.render(cmd, tokens)
		},
	}
	cmd.Flags().StringVar(&actor, "actor", "", "restrict to one actor")
	return cmd
}

func tokenRmCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "rm ID",
		Aliases: []string{"revoke", "delete"},
		Short:   "Revoke an API token",
		Long:    "Revoke a token so it can no longer authenticate.\n\nExit codes: 3 unknown token.",
		Example: "  tix token rm 01J000000000000000000A",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRun {
				return g.render(cmd, newPlan("token.revoke", args[0], nil))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if err := conn.Service.RevokeToken(ctx, args[0]); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: args[0], Status: statusOK})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be revoked without writing")
	return cmd
}

// newLoginCmd builds the login command.
func newLoginCmd(g *globals) *cobra.Command {
	var email, password string
	cmd := &cobra.Command{
		Use:     "login",
		Short:   "Exchange a password for a session",
		Long:    "Log in and print the session token.\n\nExit codes: 2 missing credentials, 5 invalid credentials.",
		Example: "  tix login --email ada@example.com --password -",
		GroupID: "setup",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			secret, err := body(cmd, password)
			if err != nil {
				return err
			}
			secret = strings.TrimRight(secret, "\r\n")
			if email == "" || secret == "" {
				return usagef(cmd, "both --email and --password are required")
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			session, err := conn.Service.Login(ctx, email, secret)
			if err != nil {
				return err
			}
			return g.render(cmd, session)
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "account email address")
	cmd.Flags().StringVar(&password, "password", "", "password, or - to read standard input")
	return cmd
}

// toScopes converts flag strings to scopes.
func toScopes(values []string) []core.Scope {
	out := make([]core.Scope, 0, len(values))
	for _, v := range values {
		out = append(out, core.Scope(v))
	}
	return out
}

// scopeNames lists every scope for completion.
func scopeNames() []string {
	out := make([]string, 0, len(core.AllScopes)+1)
	out = append(out, string(core.ScopeAll))
	for _, s := range core.AllScopes {
		out = append(out, string(s))
	}
	return out
}

// EnvSession names the variable holding a session token.
const EnvSession = "TIX_SESSION"

// newLogoutCmd builds the logout command.
func newLogoutCmd(g *globals) *cobra.Command {
	var session string
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "End a session",
		Long: "End the session tix login issued. The token is read from --session, " +
			"and from " + EnvSession + " when the flag is absent.\n\n" +
			"Exit codes: 2 no session token given, 5 the session is not valid.",
		Example: "  tix logout --session -\n  TIX_SESSION=$token tix logout",
		GroupID: "setup",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			token, err := g.sessionToken(cmd, session)
			if err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			svc := conn.Service
			if remote, ok := svc.(*client.Client); ok {
				svc = remote.WithSession(token)
			}
			if err := svc.Logout(service.WithSessionToken(ctx, token)); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: "session", Status: statusOK})
		},
	}
	cmd.Flags().StringVar(&session, "session", "", "session token to end, or - to read standard input")
	return cmd
}

// sessionToken returns the session token the invocation presents.
func (g *globals) sessionToken(cmd *cobra.Command, value string) (string, error) {
	token, err := body(cmd, value)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(token) == "" {
		token = lookupEnv(g.environ, EnvSession)
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", usagef(cmd, "a session token is required; pass --session or set %s", EnvSession)
	}
	return token, nil
}
