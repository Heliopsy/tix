// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// newTenantSSOCmd builds the single sign-on provider commands.
func newTenantSSOCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sso",
		Short: "Manage this tenant's single sign-on providers",
		Long: "Manage the providers people of this tenant can sign in through: an OpenID Connect\n" +
			"identity provider (kind oidc: Keycloak, Authentik, Entra ID, Google, Okta), a GitHub\n" +
			"OAuth app (kind oauth_github), or a SAML 2.0 identity provider (kind saml). Each\n" +
			"enabled provider is a Sign in with button on the web sign-in screen; oidc and\n" +
			"oauth_github are also a choice for tix login --sso. Register\n" +
			"PUBLIC_URL/login/sso/callback as the redirect URL with an OIDC or GitHub provider. A\n" +
			"SAML IdP takes PUBLIC_URL/login/sso/saml/KEY/acs as its ACS URL and\n" +
			"PUBLIC_URL/login/sso/saml/KEY/metadata as the SP entity id, or imports the metadata\n" +
			"from that address (tix tenant sso metadata KEY prints it). tix tenant sso ls prints\n" +
			"the addresses when the server knows its public address. See docs/sso.md.\n\n" +
			"Every command needs the sso:admin scope, which a tenant admin holds.",
		Args: noArgs,
		RunE: helpRunner,
	}
	cmd.AddCommand(ssoAddCmd(g), ssoLsCmd(g), ssoShowCmd(g), ssoEditCmd(g), ssoRmCmd(g), ssoMetadataCmd(g), ssoRefreshCmd(g))
	return cmd
}

// ssoFlags are the settings add and edit share.
type ssoFlags struct {
	name, issuer, clientID, secret, defaultRole       string
	emailClaim, nameClaim, usernameClaim, groupsClaim string
	webURL, apiURL                                    string
	scopes, domains, roleMap, mfaValues, orgs         []string
	autoCreate, trustMFA                              bool

	idpMetadataURL, idpMetadataFile, idpEntityID, idpSSOURL string
	spEntityID, nameIDFormat                                string
	idpCerts                                                []string
	idpInitiated                                            bool
}

func (s *ssoFlags) bind(cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringVar(&s.name, "name", "", "the name on the Sign in with button; defaults to the key")
	f.StringVar(&s.issuer, "issuer", "", "OIDC issuer URL; its /.well-known/openid-configuration is read")
	f.StringVar(&s.clientID, "client-id", "", "client id registered with the provider")
	f.StringVar(&s.secret, "client-secret", "", "client secret, or - to read standard input; sealed and never shown again")
	f.StringSliceVar(&s.scopes, "scope", nil, "OIDC scope to request, repeatable; default openid, profile, email")
	f.StringVar(&s.emailClaim, "email-claim", "", "ID token claim, or SAML attribute, read as the email (default email)")
	f.StringVar(&s.nameClaim, "name-claim", "", "claim or SAML attribute read as the display name (default name)")
	f.StringVar(&s.usernameClaim, "username-claim", "", "claim or SAML attribute read as the handle (default preferred_username)")
	f.StringVar(&s.groupsClaim, "groups-claim", "", "claim or SAML attribute read as the groups (default groups)")
	f.StringSliceVar(&s.domains, "allowed-domain", nil, "admit only emails at this domain, repeatable")
	f.StringSliceVar(&s.roleMap, "role-map", nil, "GROUP=ROLE: people in GROUP get ROLE at sign-in, repeatable")
	f.StringVar(&s.defaultRole, "default-role", "", "role of an account auto-create makes when no group maps (default member)")
	f.BoolVar(&s.autoCreate, "auto-create", false, "create an account for a verified email nobody holds")
	f.BoolVar(&s.trustMFA, "trust-mfa", false, "skip the tenant's second factor when the provider reports one (OIDC, SAML)")
	f.StringSliceVar(&s.mfaValues, "mfa-value", nil, "amr, acr or SAML AuthnContextClassRef value --trust-mfa accepts, repeatable (default mfa; SAML has no default)")
	f.StringSliceVar(&s.orgs, "allowed-org", nil, "GitHub organisation whose members may sign in, repeatable")
	f.StringVar(&s.webURL, "web-url", "", "GitHub Enterprise Server address, such as https://github.example.com")
	f.StringVar(&s.apiURL, "api-url", "", "GitHub Enterprise Server API address, such as https://github.example.com/api/v3")
	f.StringVar(&s.idpMetadataURL, "idp-metadata-url", "", "SAML IdP metadata address, read now for the entity id, sign-in URL and certificates")
	f.StringVar(&s.idpMetadataFile, "idp-metadata-file", "", "SAML IdP metadata XML file, or - for standard input; read and not kept")
	f.StringVar(&s.idpEntityID, "idp-entity-id", "", "SAML IdP entity id, the issuer of its responses")
	f.StringVar(&s.idpSSOURL, "idp-sso-url", "", "SAML IdP HTTP-Redirect sign-in URL")
	f.StringSliceVar(&s.idpCerts, "idp-cert", nil, "file holding SAML IdP signing certificates as PEM, repeatable")
	f.StringVar(&s.spEntityID, "sp-entity-id", "", "tix's SAML entity id (default PUBLIC_URL/login/sso/saml/KEY/metadata)")
	f.StringVar(&s.nameIDFormat, "name-id-format", "", "SAML NameID format to request, such as urn:oasis:names:tc:SAML:2.0:nameid-format:persistent")
	f.BoolVar(&s.idpInitiated, "allow-idp-initiated", false, "accept SAML sign-ins started at the IdP, which no browser flow binds")
}

// samlFiles reads --idp-metadata-file and --idp-cert.
func samlFiles(cmd *cobra.Command, s *ssoFlags) (metadata string, certs []string, err error) {
	if s.idpMetadataFile == StdinMarker {
		if metadata, err = readAll(cmd.InOrStdin()); err != nil {
			return "", nil, err
		}
	} else if s.idpMetadataFile != "" {
		if err := localFile(cmd, "--idp-metadata-file", s.idpMetadataFile); err != nil {
			return "", nil, err
		}
		raw, err := os.ReadFile(s.idpMetadataFile) // #nosec G304 -- the operator names the metadata file
		if err != nil {
			return "", nil, usagef(cmd, "reading --idp-metadata-file %q: %v", s.idpMetadataFile, err)
		}
		metadata = string(raw)
	}
	for _, path := range s.idpCerts {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if err := localFile(cmd, "--idp-cert", path); err != nil {
			return "", nil, err
		}
		raw, err := os.ReadFile(path) // #nosec G304 -- the operator names the certificate file
		if err != nil {
			return "", nil, usagef(cmd, "reading --idp-cert %q: %v", path, err)
		}
		certs = append(certs, core.SplitCertificates(string(raw))...)
	}
	return metadata, certs, nil
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
			"A saml provider needs the IdP's entity id, sign-in URL and signing certificate: give\n" +
			"--idp-metadata-url or --idp-metadata-file, or --idp-entity-id, --idp-sso-url and\n" +
			"--idp-cert. An explicit flag wins over the metadata. It takes no client id or secret.\n\n" +
			"Exit codes: 2 an invalid setting, 4 KEY is taken, 5 permission denied, 6 no sealing key\n" +
			"(auth.totp_key) on the server, 7 the IdP metadata could not be read.",
		Example: "  printf %s \"$SECRET\" | tix tenant sso add kc --kind oidc --name Keycloak \\\n" +
			"      --issuer https://id.example.com/realms/acme --client-id tix --client-secret - \\\n" +
			"      --allowed-domain example.com --role-map tix-admins=admin --auto-create\n" +
			"  printf %s \"$SECRET\" | tix tenant sso add github --kind oauth_github --name GitHub \\\n" +
			"      --client-id Iv1.abc --client-secret - --allowed-org acme\n" +
			"  tix tenant sso add okta --kind saml --name Okta \\\n" +
			"      --idp-metadata-url https://acme.okta.com/app/exk1/sso/saml/metadata --auto-create",
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
			metadata, certs, err := samlFiles(cmd, &s)
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
				IdPMetadataURL: s.idpMetadataURL, IdPMetadataXML: metadata, IdPEntityID: s.idpEntityID,
				IdPSSOURL: s.idpSSOURL, IdPCertificates: certs, SPEntityID: s.spEntityID,
				NameIDFormat: s.nameIDFormat, AllowIdPInitiated: s.idpInitiated,
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
	cmd.Flags().StringVar(&kind, "kind", string(core.SSOKindOIDC), "provider kind: oidc, oauth_github or saml")
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
			"provider while password sign-in is disabled. --idp-metadata-url, given even\n" +
			"unchanged, reads the SAML IdP's metadata again, as after a certificate rollover.\n\n" +
			"Exit codes: 2 an invalid setting, 3 unknown provider, 5 permission denied, 6 the last\n" +
			"enabled provider while password sign-in is disabled.",
		Example: "  tix tenant sso edit kc --role-map tix-admins=admin --role-map tix-viewers=viewer\n" +
			"  printf %s \"$NEW_SECRET\" | tix tenant sso edit kc --client-secret -\n" +
			"  tix tenant sso edit kc --enabled=false\n" +
			"  tix tenant sso edit okta --idp-metadata-url https://acme.okta.com/app/exk1/sso/saml/metadata",
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
	str("idp-metadata-url", s.idpMetadataURL, &in.IdPMetadataURL)
	str("idp-entity-id", s.idpEntityID, &in.IdPEntityID)
	str("idp-sso-url", s.idpSSOURL, &in.IdPSSOURL)
	str("sp-entity-id", s.spEntityID, &in.SPEntityID)
	str("name-id-format", s.nameIDFormat, &in.NameIDFormat)
	flag("allow-idp-initiated", s.idpInitiated, &in.AllowIdPInitiated)
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
	metadata, certs, err := samlFiles(cmd, s)
	if err != nil {
		return in, err
	}
	if changed("idp-metadata-file") {
		in.IdPMetadataXML = &metadata
	}
	if changed("idp-cert") {
		in.IdPCertificates = &certs
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

func ssoMetadataCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "metadata KEY",
		Short: "Print a SAML provider's service provider metadata",
		Long: "Print the SP metadata to give the SAML identity provider: tix's entity id and its\n" +
			"assertion consumer service, built on the address this command reached the server at.\n" +
			"The same document is served without a credential at\n" +
			"PUBLIC_URL/login/sso/saml/KEY/metadata, which most IdPs can import by URL.\n\n" +
			"Exit codes: 3 no saml provider KEY.",
		Example: "  tix tenant sso metadata okta > tix-sp-metadata.xml\n" +
			"  tix tenant sso metadata okta -o json",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			md, err := conn.Service.SAMLMetadata(ctx, core.SAMLMetadataInput{Provider: args[0]})
			if err != nil {
				return err
			}
			if g.formatName() != output.FormatTable {
				return g.render(cmd, md)
			}
			_, err = cmd.OutOrStdout().Write([]byte(md.XML))
			return err
		},
	}
}

func ssoRefreshCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "refresh KEY",
		Short: "Read a SAML provider's IdP metadata again now",
		Long: "Read the IdP metadata again from the provider's stored --idp-metadata-url, as tix serve\n" +
			"does on its own every auth.saml_metadata_refresh (12h by default). New signing\n" +
			"certificates are added; one the metadata no longer names keeps verifying for seven\n" +
			"days, so a rollover in flight still signs in, and is dropped after. A setting given\n" +
			"with --idp-entity-id, --idp-sso-url or --idp-cert stays as given. A failed read is\n" +
			"recorded on the provider and the certificates it has keep working.\n\n" +
			"Exit codes: 2 the metadata is not valid, 3 unknown provider, 5 permission denied,\n" +
			"6 the provider has no IdP metadata URL, 7 the metadata could not be read.",
		Example: "  tix tenant sso refresh okta\n" +
			"  tix tenant sso refresh okta -o json",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			p, err := conn.Service.RefreshSAMLMetadata(ctx, args[0])
			if err != nil {
				return err
			}
			return g.render(cmd, p)
		},
	}
}
