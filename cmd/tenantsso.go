// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/heliopsy/tix/internal/core"
)

// newTenantSSOCmd builds the single sign-on provider commands.
func newTenantSSOCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sso",
		Short: "Manage this tenant's single sign-on providers",
		Long: "Manage the providers people of this tenant can sign in through: an OpenID Connect\n" +
			"identity provider (kind oidc: Keycloak, Authentik, Entra ID, Google, Okta) or a GitHub\n" +
			"OAuth app (kind oauth_github). Each enabled provider is a Sign in with button on the\n" +
			"web sign-in screen and a choice for tix login --sso. Register\n" +
			"PUBLIC_URL/login/sso/callback as the redirect URL with the provider; tix tenant sso\n" +
			"ls prints it when the server knows its public address. See docs/sso.md.\n\n" +
			"Every command needs the sso:admin scope, which a tenant admin holds.",
		Args: noArgs,
		RunE: helpRunner,
	}
	cmd.AddCommand(ssoAddCmd(g), ssoLsCmd(g), ssoShowCmd(g), ssoEditCmd(g), ssoRmCmd(g))
	return cmd
}

// ssoFlags are the settings add and edit share.
type ssoFlags struct {
	name, issuer, clientID, secret, defaultRole       string
	emailClaim, nameClaim, usernameClaim, groupsClaim string
	webURL, apiURL                                    string
	scopes, domains, roleMap, mfaValues, orgs         []string
	autoCreate, trustMFA                              bool
}

func (s *ssoFlags) bind(cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringVar(&s.name, "name", "", "the name on the Sign in with button; defaults to the key")
	f.StringVar(&s.issuer, "issuer", "", "OIDC issuer URL; its /.well-known/openid-configuration is read")
	f.StringVar(&s.clientID, "client-id", "", "client id registered with the provider")
	f.StringVar(&s.secret, "client-secret", "", "client secret, or - to read standard input; sealed and never shown again")
	f.StringSliceVar(&s.scopes, "scope", nil, "OIDC scope to request, repeatable; default openid, profile, email")
	f.StringVar(&s.emailClaim, "email-claim", "", "ID token claim read as the email (default email)")
	f.StringVar(&s.nameClaim, "name-claim", "", "claim read as the display name (default name)")
	f.StringVar(&s.usernameClaim, "username-claim", "", "claim read as the handle (default preferred_username)")
	f.StringVar(&s.groupsClaim, "groups-claim", "", "claim read as the groups (default groups)")
	f.StringSliceVar(&s.domains, "allowed-domain", nil, "admit only emails at this domain, repeatable")
	f.StringSliceVar(&s.roleMap, "role-map", nil, "GROUP=ROLE: people in GROUP get ROLE at sign-in, repeatable")
	f.StringVar(&s.defaultRole, "default-role", "", "role of an account auto-create makes when no group maps (default member)")
	f.BoolVar(&s.autoCreate, "auto-create", false, "create an account for a verified email nobody holds")
	f.BoolVar(&s.trustMFA, "trust-mfa", false, "skip the tenant's second factor when the ID token reports one (OIDC)")
	f.StringSliceVar(&s.mfaValues, "mfa-value", nil, "amr or acr value --trust-mfa accepts, repeatable (default mfa)")
	f.StringSliceVar(&s.orgs, "allowed-org", nil, "GitHub organisation whose members may sign in, repeatable")
	f.StringVar(&s.webURL, "web-url", "", "GitHub Enterprise Server address, such as https://github.example.com")
	f.StringVar(&s.apiURL, "api-url", "", "GitHub Enterprise Server API address, such as https://github.example.com/api/v3")
}

// roleMapping parses the --role-map values.
func roleMapping(cmd *cobra.Command, values []string) (map[string]core.Role, error) {
	out := map[string]core.Role{}
	for _, v := range values {
		group, role, ok := strings.Cut(v, "=")
		if !ok || strings.TrimSpace(group) == "" {
			return nil, usagef(cmd, "--role-map takes GROUP=ROLE, not %q", v)
		}
		out[strings.TrimSpace(group)] = core.Role(strings.TrimSpace(role))
	}
	return out, nil
}

// clientSecret reads --client-secret, from standard input for -.
func clientSecret(cmd *cobra.Command, value string) (string, error) {
	secret, err := body(cmd, value)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(secret, "\r\n"), nil
}

func ssoAddCmd(g *globals) *cobra.Command {
	var s ssoFlags
	var kind string
	var disabled bool
	cmd := &cobra.Command{
		Use:   "add KEY",
		Short: "Add a single sign-on provider",
		Long: "Add a provider under KEY, a short name of lowercase letters, digits and dashes used in\n" +
			"addresses and in tix login --sso KEY.\n\n" +
			"Linking: a sign-in uses the account already linked to the provider's subject; else\n" +
			"one whose email the provider verified, if it is a member of this tenant; else, with\n" +
			"--auto-create, a new account for an email nobody holds; else it is refused.\n" +
			"A global admin cannot sign in through a provider.\n\n" +
			"Exit codes: 2 an invalid setting, 4 KEY is taken, 5 permission denied, 6 no sealing key\n" +
			"(auth.totp_key) on the server.",
		Example: "  printf %s \"$SECRET\" | tix tenant sso add kc --kind oidc --name Keycloak \\\n" +
			"      --issuer https://id.example.com/realms/acme --client-id tix --client-secret - \\\n" +
			"      --allowed-domain example.com --role-map tix-admins=admin --auto-create\n" +
			"  printf %s \"$SECRET\" | tix tenant sso add github --kind oauth_github --name GitHub \\\n" +
			"      --client-id Iv1.abc --client-secret - --allowed-org acme",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mapping, err := roleMapping(cmd, s.roleMap)
			if err != nil {
				return err
			}
			secret, err := clientSecret(cmd, s.secret)
			if err != nil {
				return err
			}
			in := core.CreateSSOProviderInput{
				Key: args[0], Kind: core.SSOKind(kind), DisplayName: s.name, Disabled: disabled,
				AutoCreate: s.autoCreate, DefaultRole: core.Role(s.defaultRole), TrustMFA: s.trustMFA,
				AllowedDomains: s.domains, RoleMapping: mapping, Issuer: s.issuer, ClientID: s.clientID,
				ClientSecret: secret, Scopes: s.scopes, EmailClaim: s.emailClaim, NameClaim: s.nameClaim,
				UsernameClaim: s.usernameClaim, GroupsClaim: s.groupsClaim, MFAValues: s.mfaValues,
				AllowedOrgs: s.orgs, WebURL: s.webURL, APIURL: s.apiURL,
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			p, err := conn.Service.CreateSSOProvider(ctx, in)
			if err != nil {
				return err
			}
			return g.render(cmd, p)
		},
	}
	s.bind(cmd)
	cmd.Flags().StringVar(&kind, "kind", string(core.SSOKindOIDC), "provider kind: oidc or oauth_github")
	cmd.Flags().BoolVar(&disabled, "disabled", false, "add the provider switched off")
	return cmd
}

func ssoLsCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List this tenant's single sign-on providers",
		Long:    "List the tenant's providers. A client secret shows only as set.\n\nExit codes: 5 permission denied.",
		Example: "  tix tenant sso ls",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			providers, err := conn.Service.ListSSOProviders(ctx)
			if err != nil {
				return err
			}
			return g.render(cmd, providers)
		},
	}
}

func ssoShowCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "show KEY",
		Short:   "Show one single sign-on provider",
		Long:    "Show a provider by key or identifier.\n\nExit codes: 3 unknown provider, 5 permission denied.",
		Example: "  tix tenant sso show kc",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			p, err := conn.Service.GetSSOProvider(ctx, args[0])
			if err != nil {
				return err
			}
			return g.render(cmd, p)
		},
	}
}

func ssoEditCmd(g *globals) *cobra.Command {
	var s ssoFlags
	var enabled bool
	cmd := &cobra.Command{
		Use:   "edit KEY",
		Short: "Change a single sign-on provider",
		Long: "Change the settings named by the flags given; the rest stay. A list flag replaces the\n" +
			"whole list, and given as \"\" clears it. --client-secret \"\" removes the stored secret.\n" +
			"--enabled=false switches the provider off, which is refused for the last enabled\n" +
			"provider while password sign-in is disabled.\n\n" +
			"Exit codes: 2 an invalid setting, 3 unknown provider, 5 permission denied, 6 the last\n" +
			"enabled provider while password sign-in is disabled.",
		Example: "  tix tenant sso edit kc --role-map tix-admins=admin --role-map tix-viewers=viewer\n" +
			"  printf %s \"$NEW_SECRET\" | tix tenant sso edit kc --client-secret -\n" +
			"  tix tenant sso edit kc --enabled=false",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			in, err := ssoEditInput(cmd, &s, enabled)
			if err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			p, err := conn.Service.UpdateSSOProvider(ctx, args[0], in)
			if err != nil {
				return err
			}
			return g.render(cmd, p)
		},
	}
	s.bind(cmd)
	cmd.Flags().BoolVar(&enabled, "enabled", true, "offer the provider at sign-in")
	return cmd
}

// ssoEditInput sets each field whose flag was given.
func ssoEditInput(cmd *cobra.Command, s *ssoFlags, enabled bool) (core.UpdateSSOProviderInput, error) {
	var in core.UpdateSSOProviderInput
	changed := cmd.Flags().Changed
	str := func(flag string, v string, into **string) {
		if changed(flag) {
			*into = &v
		}
	}
	list := func(flag string, v []string, into **[]string) {
		if changed(flag) {
			clean := []string{}
			for _, e := range v {
				if strings.TrimSpace(e) != "" {
					clean = append(clean, e)
				}
			}
			*into = &clean
		}
	}
	flag := func(name string, v bool, into **bool) {
		if changed(name) {
			*into = &v
		}
	}
	str("name", s.name, &in.DisplayName)
	str("issuer", s.issuer, &in.Issuer)
	str("client-id", s.clientID, &in.ClientID)
	str("email-claim", s.emailClaim, &in.EmailClaim)
	str("name-claim", s.nameClaim, &in.NameClaim)
	str("username-claim", s.usernameClaim, &in.UsernameClaim)
	str("groups-claim", s.groupsClaim, &in.GroupsClaim)
	str("web-url", s.webURL, &in.WebURL)
	str("api-url", s.apiURL, &in.APIURL)
	list("scope", s.scopes, &in.Scopes)
	list("allowed-domain", s.domains, &in.AllowedDomains)
	list("mfa-value", s.mfaValues, &in.MFAValues)
	list("allowed-org", s.orgs, &in.AllowedOrgs)
	flag("auto-create", s.autoCreate, &in.AutoCreate)
	flag("trust-mfa", s.trustMFA, &in.TrustMFA)
	flag("enabled", enabled, &in.Enabled)
	if changed("default-role") {
		role := core.Role(s.defaultRole)
		in.DefaultRole = &role
	}
	if changed("role-map") {
		var values []string
		for _, v := range s.roleMap {
			if strings.TrimSpace(v) != "" {
				values = append(values, v)
			}
		}
		mapping, err := roleMapping(cmd, values)
		if err != nil {
			return in, err
		}
		in.RoleMapping = &mapping
	}
	if changed("client-secret") {
		secret, err := clientSecret(cmd, s.secret)
		if err != nil {
			return in, err
		}
		in.ClientSecret = &secret
	}
	return in, nil
}

func ssoRmCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "rm KEY",
		Aliases: []string{"delete"},
		Short:   "Remove a single sign-on provider",
		Long: "Remove a provider and every identity linked through it. The accounts stay.\n\n" +
			"Exit codes: 3 unknown provider, 5 permission denied, 6 the last enabled provider while\n" +
			"password sign-in is disabled.",
		Example: "  tix tenant sso rm kc",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRun {
				return g.render(cmd, newPlan("sso.provider.delete", args[0], nil))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if err := conn.Service.DeleteSSOProvider(ctx, args[0]); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: args[0], Status: statusOK})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}
