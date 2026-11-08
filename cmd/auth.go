// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"context"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/client"
	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/sshlogin"
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
	cmd.AddCommand(actorLsCmd(g), actorShowCmd(g))
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
	cmd.AddCommand(userCreateCmd(g), userLsCmd(g), userShowCmd(g), userEditCmd(g), userRmCmd(g),
		newUserInviteCmd(g), newUserKeyCmd(g), newUserTOTPCmd(g), newUserTimeCmd(g))
	return cmd
}

func userCreateCmd(g *globals) *cobra.Command {
	var password, display, handle, role string
	var dryRun, mustChange bool
	cmd := &cobra.Command{
		Use:   "create EMAIL",
		Short: "Create a user",
		Long: "Create a credentialed human.\n\n" +
			"--require-password-change makes the first password sign-in choose a new password before " +
			"it is let in, which is what you want when you typed the password for somebody else. It " +
			"needs --password. To let the person choose their own password instead, use tix user invite.\n\n" +
			"Exit codes: 2 invalid input, 4 email already used, 5 permission denied.",
		Example: "  tix user create ada@example.com --role admin\n" +
			"  tix user create ada@example.com --password - --require-password-change",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			in := core.CreateUserInput{
				Email: args[0], Password: password, DisplayName: display,
				Handle: handle, Role: core.Role(role), RequirePasswordChange: mustChange,
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
	f.BoolVar(&mustChange, "require-password-change", false, "make the first sign-in choose a new password")
	f.BoolVar(&dryRun, "dry-run", false, "report what would be created without writing")
	return cmd
}

func userLsCmd(g *globals) *cobra.Command {
	var limit int
	var cursor string
	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List users",
		Long:    "List users.\n\nExit codes: 5 permission denied.",
		Example: "  tix user ls -o json",
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
			users, next, err := conn.Service.ListUsers(ctx, core.Page{Limit: size, Cursor: cursor})
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
	var disabled, dryRun, mustChange bool
	cmd := &cobra.Command{
		Use:   "edit ID",
		Short: "Change a user",
		Long: "Change a user's display name, password, role or disabled state.\n\n" +
			"--require-password-change asks the user to choose a new password at their next password " +
			"sign-in; =false lifts that. Sessions already open and API tokens are not affected.\n\n" +
			"Exit codes: 3 unknown user.",
		Example: "  tix user edit 01J000000000000000000A --role viewer\n" +
			"  tix user edit 01J000000000000000000A --require-password-change",
		Args: exactArgs(1),
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
			if cmd.Flags().Changed("require-password-change") {
				in.RequirePasswordChange = &mustChange
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
	f.BoolVar(&mustChange, "require-password-change", false,
		"ask for a new password at the next sign-in; =false lifts it")
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
		Use:   "create NAME",
		Short: "Mint an API token",
		Long: "Mint a token whose secret is shown exactly once.\n\n" +
			"The token expires after " + DefaultTokenExpiry + " unless --expires says otherwise, " +
			"matching the browser form. Pass --expires never for a token that does not expire; " +
			"nothing else produces one.\n\n" +
			"Exit codes: 2 unknown scope or unreadable expiry, 3 unknown project or actor, 5 permission denied.",
		Example: "  tix token create agent --scope task:read --scope task:claim",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			expiry, err := parseExpiry(expires, time.Now())
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
	f.StringVar(&expires, "expires", DefaultTokenExpiry,
		"when the token stops working: a duration such as 90d, a date, an RFC3339 timestamp, "+
			"or never for a token that does not expire")
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
	var email, password, sshKey, contextName string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Exchange a password or an SSH key for a session",
		Long: "Log in and print the session token.\n\n" +
			"--ssh-key signs a challenge from the server with an SSH key enrolled for you (tix user key add) " +
			"instead of sending a password. Alone it tries every key in ssh-agent, then ~/.ssh/id_ed25519, " +
			"id_ecdsa and id_rsa; --ssh-key=PATH uses only that private key and asks for its passphrase " +
			"without echo. The signature is made under the namespace tix-login@HOST, so it can only be " +
			"used to sign in to this server.\n\n" +
			"Where a second factor is required, the code is asked for on the terminal after the password. " +
			"No flag and no environment variable carries it: a live code passed as an argument lands in " +
			"shell history and in the process table. An unattended caller uses an API token instead, which " +
			"is exempt from the second factor; see tix token create.\n\n" +
			"An account an administrator marked to change its password is asked for a new one, twice and " +
			"without echo, before the session is printed. Without a terminal the sign-in stops and names " +
			"the web sign-in page, where the change can be made instead.\n\n" +
			"With --tenant KEY against a server the sign-in is for that tenant: the key goes in the " +
			"X-Tix-Tenant header, so a host serving several tenants knows which one is meant, and a host " +
			"bound to another tenant refuses it. Instead of printing the session, the command mints an API " +
			"token for your actor in that tenant (scopes * and account:read), ends the session, and stores " +
			"the token on the context --context names, or on the current context, or on a new context named " +
			"after the tenant. Run it once per tenant to hold one context for each; tix tenant ls marks which " +
			"of your tenants have one.\n\n" +
			"Exit codes: 2 missing credentials or a refused new password, 5 invalid credentials, " +
			"6 no terminal to prompt for a code or a new password.",
		Example: "  tix login --email ada@example.com --password -\n" +
			"  tix --server https://tix.example login --tenant test --context test --email ada@example.com --password -\n" +
			"  tix --server https://tix.example login --ssh-key\n" +
			"  tix --server https://tix.example login --ssh-key=~/.ssh/work_ed25519",
		GroupID: "setup",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if cmd.Flags().Changed("ssh-key") {
				if email != "" || password != "" {
					return usagef(cmd, "--ssh-key replaces --email and --password; pass one or the other")
				}
				return g.loginWithSSHKey(cmd, sshKey, contextName)
			}
			secret, err := body(cmd, password)
			if err != nil {
				return err
			}
			secret = strings.TrimRight(secret, "\r\n")
			if email == "" || secret == "" {
				return usagef(cmd, "both --email and --password are required")
			}
			conn, ctx, err := g.dialAnonymous(cmd)
			if err != nil {
				return err
			}
			svc, tl, err := g.loginService(conn, contextName)
			if err != nil {
				return err
			}
			in := core.LoginInput{Email: email, Password: secret, Surface: core.SurfaceCLI}
			result, err := svc.Login(ctx, in)
			if err != nil {
				return err
			}
			if c := result.Challenge; c != nil && c.Kind == core.ChallengePasswordChange {
				if in.NewPassword, err = g.askNewPassword(cmd, conn.Info.Target.URL); err != nil {
					return err
				}
				if result, err = svc.Login(ctx, in); err != nil {
					return err
				}
			}
			return g.finishLogin(cmd, svc, ctx, result, tl)
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "account email address")
	cmd.Flags().StringVar(&password, "password", "", "password, or - to read standard input")
	cmd.Flags().StringVar(&sshKey, "ssh-key", "",
		"sign in with an enrolled SSH key: alone, ssh-agent then the default keys; =PATH, that private key")
	cmd.Flags().Lookup("ssh-key").NoOptDefVal = sshKeyDiscover
	cmd.Flags().StringVar(&contextName, "context", "",
		"with --tenant, the context to store the token on; defaults to the current one, else the tenant key")
	return cmd
}

// sshKeyDiscover is what --ssh-key holds when given without a path.
const sshKeyDiscover = "auto"

// askNewPassword asks, twice and without echo, for the password an account
// marked to change its password signs in with from now on. With no terminal it
// refuses and names the web sign-in page instead, because a script has no one
// to choose a password and a flag would put it in shell history.
func (g *globals) askNewPassword(cmd *cobra.Command, server string) (string, error) {
	if !onATerminal(cmd) {
		where := "on the web interface"
		if server != "" {
			where = "on the web at " + strings.TrimRight(server, "/") + "/login"
		}
		return "", core.Precondition("a password change is required for this account: sign in %s to choose "+
			"a new password, or run tix login on a terminal", where)
	}
	g.diag(cmd, "this account must choose a new password before it can sign in")
	first, err := g.askSecret(cmd, "new password: ")
	if err != nil {
		return "", err
	}
	second, err := g.askSecret(cmd, "new password again: ")
	if err != nil {
		return "", err
	}
	if first != second {
		return "", core.Invalid("the two new passwords differ; nothing was changed")
	}
	return first, nil
}

// finishLogin takes a first step's answer through the second factor, when
// there is one, and prints the session.
func (g *globals) finishLogin(cmd *cobra.Command, svc core.Service, ctx context.Context,
	result *core.LoginResult, tl *tenantLogin,
) error {
	var err error
	if result.Challenge != nil {
		if result, err = g.answerChallenge(cmd, svc, ctx, result.Challenge); err != nil {
			return err
		}
	}
	if result.Session == nil {
		return core.Internal("the sign-in finished without a session")
	}
	showRecoveryCodes(cmd, result.RecoveryCodes)
	if tl != nil {
		return g.storeTenantLogin(cmd, ctx, tl, result.Session)
	}
	return g.render(cmd, result.Session)
}

// loginWithSSHKey signs the server's challenge with the key the flag names,
// or with each key the person holds until one is accepted.
func (g *globals) loginWithSSHKey(cmd *cobra.Command, path, contextName string) error {
	ask := g.passphrasePrompt(cmd)
	home := lookupEnv(g.environ, "HOME")
	var candidates []sshlogin.Candidate
	if path != sshKeyDiscover {
		candidates = []sshlogin.Candidate{sshlogin.FromFile(expandTilde(path, home), ask)}
	} else {
		found, closeAgent, err := sshlogin.Discover(lookupEnv(g.environ, "SSH_AUTH_SOCK"), home, ask)
		defer func() { _ = closeAgent() }()
		if err != nil {
			return err
		}
		candidates = found
	}
	conn, ctx, err := g.dialAnonymous(cmd)
	if err != nil {
		return err
	}
	svc, tl, err := g.loginService(conn, contextName)
	if err != nil {
		return err
	}
	target := conn.Info.Target
	result, err := sshlogin.Login(ctx, svc, audienceOf(target.URL), target.Tenant, candidates)
	if err != nil {
		return err
	}
	return g.finishLogin(cmd, svc, ctx, result, tl)
}

// audienceOf is the host a signature is bound to: the server this invocation
// was told to reach, or nothing for a database opened directly.
func audienceOf(serverURL string) string {
	if serverURL == "" {
		return ""
	}
	u, err := url.Parse(serverURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// expandTilde resolves a leading ~/ the shell left alone, which it does for
// --ssh-key=~/.ssh/key.
func expandTilde(path, home string) string {
	if rest, ok := strings.CutPrefix(path, "~/"); ok && home != "" {
		return filepath.Join(home, rest)
	}
	return path
}

// answerChallenge completes a sign-in that the password alone did not.
//
// It is a prompt and nothing else by design. The code may not arrive as a
// flag or as an environment variable, so a caller with no terminal is refused
// here rather than hanging on a question nobody can answer.
func (g *globals) answerChallenge(cmd *cobra.Command, svc core.Service, ctx context.Context,
	challenge *core.TOTPChallenge,
) (*core.LoginResult, error) {
	if !onATerminal(cmd) {
		return nil, errNoTerminal("this tenant requires a second factor on the command line, which")
	}
	if challenge.Kind == core.ChallengeEnrol {
		// An unenrolled person on a requiring surface enrols here instead of
		// being refused, which is what keeps turning the requirement on from
		// locking out the administrator who turned it on.
		g.diag(cmd, "this tenant requires a second factor, and you have none yet")
		g.showEnrolment(cmd, challenge.Enrolment)
	}
	code, err := g.ask(cmd, "second factor code: ")
	if err != nil {
		return nil, err
	}
	return svc.Login(ctx, core.LoginInput{
		Surface: core.SurfaceCLI, Ticket: challenge.Ticket, Code: code,
	})
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
