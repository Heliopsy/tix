// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
)

// SSOKind is the protocol a sign-in provider speaks.
type SSOKind string

// Provider kinds. SAML is reserved: it is a known kind that this build
// refuses, so that the next one adds an implementation rather than a value.
const (
	SSOKindOIDC   SSOKind = "oidc"
	SSOKindGitHub SSOKind = "oauth_github"
	SSOKindSAML   SSOKind = "saml"
)

// SSOKinds lists every kind this build implements.
var SSOKinds = []SSOKind{SSOKindOIDC, SSOKindGitHub}

// Implemented reports whether this build signs people in through kind k.
func (k SSOKind) Implemented() bool { return slices.Contains(SSOKinds, k) }

// SSOCallbackPath is the browser address every provider returns to. The
// redirect URL registered with a provider is the public address plus this.
const SSOCallbackPath = "/login/sso/callback"

// LoginMethodSSOPrefix opens the sign-in method of a session minted through a
// provider; the provider's key follows it.
const LoginMethodSSOPrefix = "sso:"

// LoginMethodSSO is the sign-in method recorded for a provider's key.
func LoginMethodSSO(key string) string { return LoginMethodSSOPrefix + key }

// IsSSOMethod reports whether a session's sign-in method was a provider.
func IsSSOMethod(method string) bool { return strings.HasPrefix(method, LoginMethodSSOPrefix) }

// PasswordLogin is a tenant's password sign-in policy.
type PasswordLogin string

// Password sign-in policies.
const (
	PasswordLoginAllowed  PasswordLogin = "allowed"
	PasswordLoginDisabled PasswordLogin = "disabled"
)

// Valid reports whether p is a known policy; empty reads as allowed.
func (p PasswordLogin) Valid() bool {
	return p == "" || p == PasswordLoginAllowed || p == PasswordLoginDisabled
}

// Disabled reports whether password sign-in is off.
func (p PasswordLogin) Disabled() bool { return p == PasswordLoginDisabled }

// DefaultOIDCScopes are requested when a provider names none.
var DefaultOIDCScopes = []string{"openid", "profile", "email"}

// DefaultMFAValues are the amr or acr values that count as a second factor
// under trust_mfa when a provider names none (RFC 8176).
var DefaultMFAValues = []string{"mfa"}

// SSOProvider is a tenant's sign-in provider. The client secret is never part
// of it once stored: SecretSet and SecretHint say what a screen may show.
type SSOProvider struct {
	// ID is the provider's identifier.
	ID string `json:"id" yaml:"id"`
	// Key is the provider's short name in the tenant, used in addresses, on
	// the command line and in a session's sign-in method.
	Key string `json:"key" yaml:"key"`
	// Kind is the protocol: oidc or oauth_github.
	Kind SSOKind `json:"kind" yaml:"kind"`
	// DisplayName is the name on the "Sign in with" button.
	DisplayName string `json:"display_name" yaml:"display_name"`
	// Enabled reports whether the provider is offered at sign-in.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// AutoCreate creates an account for a verified email nobody holds.
	AutoCreate bool `json:"auto_create" yaml:"auto_create"`
	// DefaultRole is the role a created person gets when no group maps.
	DefaultRole Role `json:"default_role" yaml:"default_role"`
	// TrustMFA skips the tenant's second factor when the provider asserts one.
	TrustMFA bool `json:"trust_mfa" yaml:"trust_mfa"`
	// AllowedDomains restricts sign-in to emails at these domains; empty is any.
	AllowedDomains []string `json:"allowed_domains,omitempty" yaml:"allowed_domains,omitempty"`
	// RoleMapping maps a group, or a GitHub organisation, to a tenant role.
	RoleMapping map[string]Role `json:"role_mapping,omitempty" yaml:"role_mapping,omitempty"`

	// Issuer is the OIDC issuer, whose discovery document is read.
	Issuer string `json:"issuer,omitempty" yaml:"issuer,omitempty"`
	// ClientID is the client registered with the provider.
	ClientID string `json:"client_id" yaml:"client_id"`
	// Scopes are the OAuth scopes requested.
	Scopes []string `json:"scopes,omitempty" yaml:"scopes,omitempty"`
	// EmailClaim, NameClaim, UsernameClaim and GroupsClaim name the ID token
	// claims read; empty is email, name, preferred_username and groups.
	EmailClaim    string `json:"email_claim,omitempty" yaml:"email_claim,omitempty"`
	NameClaim     string `json:"name_claim,omitempty" yaml:"name_claim,omitempty"`
	UsernameClaim string `json:"username_claim,omitempty" yaml:"username_claim,omitempty"`
	GroupsClaim   string `json:"groups_claim,omitempty" yaml:"groups_claim,omitempty"`
	// MFAValues are the amr or acr values trust_mfa accepts as a second factor.
	MFAValues []string `json:"mfa_values,omitempty" yaml:"mfa_values,omitempty"`
	// AllowedOrgs restricts a GitHub provider to members of these organisations.
	AllowedOrgs []string `json:"allowed_orgs,omitempty" yaml:"allowed_orgs,omitempty"`
	// WebURL and APIURL point a GitHub provider at GitHub Enterprise Server;
	// empty is github.com.
	WebURL string `json:"web_url,omitempty" yaml:"web_url,omitempty"`
	APIURL string `json:"api_url,omitempty" yaml:"api_url,omitempty"`

	// SecretSet reports that a client secret is stored.
	SecretSet bool `json:"secret_set" yaml:"secret_set"`
	// SecretHint is the stored secret's last four characters, for a long one.
	SecretHint string `json:"secret_hint,omitempty" yaml:"secret_hint,omitempty"`
	// SealedSecret and SecretNonce are the sealed secret. Never serialised.
	SealedSecret string `json:"-" yaml:"-"`
	SecretNonce  string `json:"-" yaml:"-"`

	// RedirectURL is the address to register with the provider, when the
	// server knows its public address.
	RedirectURL string    `json:"redirect_url,omitempty" yaml:"redirect_url,omitempty"`
	CreatedAt   time.Time `json:"created_at" yaml:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" yaml:"updated_at"`
}

// Redacted is the provider with nothing of its secret but whether one is set,
// as an audit entry or event records it.
func (p SSOProvider) Redacted() SSOProvider {
	p.SealedSecret, p.SecretNonce, p.SecretHint, p.RedirectURL = "", "", "", ""
	return p
}

// CreateSSOProviderInput configures a new provider.
type CreateSSOProviderInput struct {
	// Key is the provider's short name: lowercase letters, digits and dashes.
	Key string `json:"key"`
	// Kind is oidc or oauth_github.
	Kind SSOKind `json:"kind"`
	// DisplayName is the name on the sign-in button; empty is the key.
	DisplayName string `json:"display_name,omitempty"`
	// Disabled creates the provider switched off.
	Disabled bool `json:"disabled,omitempty"`
	// AutoCreate creates an account for a verified email nobody holds.
	AutoCreate bool `json:"auto_create,omitempty"`
	// DefaultRole is the role a created person gets; empty is member.
	DefaultRole Role `json:"default_role,omitempty"`
	// TrustMFA skips the tenant's second factor when the provider asserts one.
	TrustMFA bool `json:"trust_mfa,omitempty"`
	// AllowedDomains restricts sign-in to emails at these domains.
	AllowedDomains []string `json:"allowed_domains,omitempty"`
	// RoleMapping maps a group to a tenant role.
	RoleMapping map[string]Role `json:"role_mapping,omitempty"`
	// Issuer is the OIDC issuer.
	Issuer string `json:"issuer,omitempty"`
	// ClientID is the client registered with the provider.
	ClientID string `json:"client_id"`
	// ClientSecret is the client's secret. It is sealed and never returned.
	ClientSecret string `json:"client_secret,omitempty"`
	// Scopes are the OAuth scopes requested.
	Scopes []string `json:"scopes,omitempty"`
	// EmailClaim names the claim read as the email.
	EmailClaim string `json:"email_claim,omitempty"`
	// NameClaim names the claim read as the display name.
	NameClaim string `json:"name_claim,omitempty"`
	// UsernameClaim names the claim read as the handle.
	UsernameClaim string `json:"username_claim,omitempty"`
	// GroupsClaim names the claim read as the groups.
	GroupsClaim string `json:"groups_claim,omitempty"`
	// MFAValues are the amr or acr values trust_mfa accepts.
	MFAValues []string `json:"mfa_values,omitempty"`
	// AllowedOrgs restricts a GitHub provider to these organisations.
	AllowedOrgs []string `json:"allowed_orgs,omitempty"`
	// WebURL is a GitHub Enterprise Server address.
	WebURL string `json:"web_url,omitempty"`
	// APIURL is a GitHub Enterprise Server API address.
	APIURL string `json:"api_url,omitempty"`
}

// UpdateSSOProviderInput changes a provider; an omitted field is unchanged.
type UpdateSSOProviderInput struct {
	// DisplayName is the name on the sign-in button.
	DisplayName *string `json:"display_name,omitempty"`
	// Enabled switches the provider on or off.
	Enabled *bool `json:"enabled,omitempty"`
	// AutoCreate creates an account for a verified email nobody holds.
	AutoCreate *bool `json:"auto_create,omitempty"`
	// DefaultRole is the role a created person gets.
	DefaultRole *Role `json:"default_role,omitempty"`
	// TrustMFA skips the tenant's second factor when the provider asserts one.
	TrustMFA *bool `json:"trust_mfa,omitempty"`
	// AllowedDomains replaces the allowed email domains; empty clears them.
	AllowedDomains *[]string `json:"allowed_domains,omitempty"`
	// RoleMapping replaces the group mapping; empty clears it.
	RoleMapping *map[string]Role `json:"role_mapping,omitempty"`
	// Issuer is the OIDC issuer.
	Issuer *string `json:"issuer,omitempty"`
	// ClientID is the client registered with the provider.
	ClientID *string `json:"client_id,omitempty"`
	// ClientSecret replaces the stored secret; empty clears it.
	ClientSecret *string `json:"client_secret,omitempty"`
	// Scopes replaces the requested scopes; empty is the default.
	Scopes *[]string `json:"scopes,omitempty"`
	// EmailClaim names the claim read as the email.
	EmailClaim *string `json:"email_claim,omitempty"`
	// NameClaim names the claim read as the display name.
	NameClaim *string `json:"name_claim,omitempty"`
	// UsernameClaim names the claim read as the handle.
	UsernameClaim *string `json:"username_claim,omitempty"`
	// GroupsClaim names the claim read as the groups.
	GroupsClaim *string `json:"groups_claim,omitempty"`
	// MFAValues replaces the amr or acr values trust_mfa accepts.
	MFAValues *[]string `json:"mfa_values,omitempty"`
	// AllowedOrgs replaces the allowed GitHub organisations.
	AllowedOrgs *[]string `json:"allowed_orgs,omitempty"`
	// WebURL is a GitHub Enterprise Server address.
	WebURL *string `json:"web_url,omitempty"`
	// APIURL is a GitHub Enterprise Server API address.
	APIURL *string `json:"api_url,omitempty"`
}

// Apply returns p with the input's fields set, the secret excepted.
func (in UpdateSSOProviderInput) Apply(p SSOProvider) SSOProvider {
	setS := func(dst *string, v *string) {
		if v != nil {
			*dst = strings.TrimSpace(*v)
		}
	}
	setB := func(dst *bool, v *bool) {
		if v != nil {
			*dst = *v
		}
	}
	setL := func(dst *[]string, v *[]string) {
		if v != nil {
			*dst = slices.Clone(*v)
		}
	}
	setS(&p.DisplayName, in.DisplayName)
	setB(&p.Enabled, in.Enabled)
	setB(&p.AutoCreate, in.AutoCreate)
	setB(&p.TrustMFA, in.TrustMFA)
	if in.DefaultRole != nil {
		p.DefaultRole = *in.DefaultRole
	}
	setL(&p.AllowedDomains, in.AllowedDomains)
	if in.RoleMapping != nil {
		p.RoleMapping = map[string]Role{}
		for k, v := range *in.RoleMapping {
			p.RoleMapping[k] = v
		}
	}
	setS(&p.Issuer, in.Issuer)
	setS(&p.ClientID, in.ClientID)
	setL(&p.Scopes, in.Scopes)
	setS(&p.EmailClaim, in.EmailClaim)
	setS(&p.NameClaim, in.NameClaim)
	setS(&p.UsernameClaim, in.UsernameClaim)
	setS(&p.GroupsClaim, in.GroupsClaim)
	setL(&p.MFAValues, in.MFAValues)
	setL(&p.AllowedOrgs, in.AllowedOrgs)
	setS(&p.WebURL, in.WebURL)
	setS(&p.APIURL, in.APIURL)
	return p
}

// Provider returns the provider the input describes, the secret excepted.
func (in CreateSSOProviderInput) Provider() SSOProvider {
	p := SSOProvider{
		Key: strings.ToLower(strings.TrimSpace(in.Key)), Kind: in.Kind,
		DisplayName: strings.TrimSpace(in.DisplayName), Enabled: !in.Disabled,
		AutoCreate: in.AutoCreate, DefaultRole: in.DefaultRole, TrustMFA: in.TrustMFA,
		AllowedDomains: slices.Clone(in.AllowedDomains),
		Issuer:         strings.TrimSpace(in.Issuer), ClientID: strings.TrimSpace(in.ClientID),
		Scopes:     slices.Clone(in.Scopes),
		EmailClaim: strings.TrimSpace(in.EmailClaim), NameClaim: strings.TrimSpace(in.NameClaim),
		UsernameClaim: strings.TrimSpace(in.UsernameClaim), GroupsClaim: strings.TrimSpace(in.GroupsClaim),
		MFAValues: slices.Clone(in.MFAValues), AllowedOrgs: slices.Clone(in.AllowedOrgs),
		WebURL: strings.TrimSpace(in.WebURL), APIURL: strings.TrimSpace(in.APIURL),
	}
	if len(in.RoleMapping) > 0 {
		p.RoleMapping = map[string]Role{}
		for k, v := range in.RoleMapping {
			p.RoleMapping[k] = v
		}
	}
	if p.DisplayName == "" {
		p.DisplayName = p.Key
	}
	return p
}

var ssoKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

// Normalize trims and lowercases the provider's lists and fills defaults.
func (p *SSOProvider) Normalize() {
	clean := func(in []string, lower bool) []string {
		var out []string
		for _, v := range in {
			v = strings.TrimSpace(v)
			if lower {
				v = strings.ToLower(v)
			}
			if v != "" && !slices.Contains(out, v) {
				out = append(out, v)
			}
		}
		return out
	}
	domains := make([]string, 0, len(p.AllowedDomains))
	for _, d := range p.AllowedDomains {
		domains = append(domains, strings.TrimPrefix(strings.TrimSpace(d), "@"))
	}
	p.AllowedDomains = clean(domains, true)
	p.Scopes = clean(p.Scopes, false)
	p.MFAValues = clean(p.MFAValues, false)
	p.AllowedOrgs = clean(p.AllowedOrgs, true)
	if p.DefaultRole == "" {
		p.DefaultRole = RoleMember
	}
	if len(p.RoleMapping) == 0 {
		p.RoleMapping = nil
	}
	p.WebURL = strings.TrimRight(p.WebURL, "/")
	p.APIURL = strings.TrimRight(p.APIURL, "/")
}

// Validate refuses a provider that cannot sign anybody in, naming the field.
func (p SSOProvider) Validate() error {
	switch {
	case !ssoKeyPattern.MatchString(p.Key):
		return OnField(Invalid("a provider key is 1 to 32 lowercase letters, digits and dashes, not %q", p.Key), "key")
	case p.Kind == SSOKindSAML:
		return OnField(Invalid("saml providers are not implemented by this build; use oidc or oauth_github"), "kind")
	case !p.Kind.Implemented():
		return OnField(Invalid("provider kind %q is not one of oidc, oauth_github", p.Kind), "kind")
	case strings.TrimSpace(p.DisplayName) == "" || len(p.DisplayName) > 64:
		return OnField(Invalid("a display name is 1 to 64 characters"), "display_name")
	case p.ClientID == "":
		return OnField(Invalid("a client id is required"), "client_id")
	case !p.DefaultRole.Valid():
		return OnField(Invalid("default role %q is not one of viewer, member, admin", p.DefaultRole), "default_role")
	}
	for group, role := range p.RoleMapping {
		if strings.TrimSpace(group) == "" || !role.Valid() {
			return OnField(Invalid("role mapping %q=%q needs a group and one of viewer, member, admin", group, role),
				"role_mapping")
		}
	}
	for _, d := range p.AllowedDomains {
		if strings.ContainsAny(d, "@/ ") || !strings.Contains(d, ".") {
			return OnField(Invalid("allowed domain %q is not a domain name", d), "allowed_domains")
		}
	}
	if p.Kind == SSOKindOIDC {
		return p.validateOIDC()
	}
	return p.validateGitHub()
}

func (p SSOProvider) validateOIDC() error {
	if err := checkProviderURL(p.Issuer, "issuer", true); err != nil {
		return err
	}
	if len(p.AllowedOrgs) > 0 || p.WebURL != "" || p.APIURL != "" {
		return OnField(Invalid("allowed organisations and GitHub addresses apply to oauth_github providers only"),
			"allowed_orgs")
	}
	if len(p.Scopes) > 0 && !slices.Contains(p.Scopes, "openid") {
		return OnField(Invalid("an oidc provider's scopes must include openid"), "scopes")
	}
	return nil
}

func (p SSOProvider) validateGitHub() error {
	if p.Issuer != "" || p.EmailClaim != "" || p.NameClaim != "" || p.UsernameClaim != "" || p.GroupsClaim != "" ||
		len(p.MFAValues) > 0 {
		return OnField(Invalid("issuer, claims and mfa values apply to oidc providers only"), "issuer")
	}
	if p.TrustMFA {
		return OnField(Invalid("GitHub does not report a second factor, so trust_mfa cannot be set on it"), "trust_mfa")
	}
	if (p.WebURL == "") != (p.APIURL == "") {
		return OnField(Invalid("a GitHub Enterprise provider needs both web_url and api_url"), "web_url")
	}
	if p.WebURL != "" {
		if err := checkProviderURL(p.WebURL, "web_url", true); err != nil {
			return err
		}
		return checkProviderURL(p.APIURL, "api_url", true)
	}
	return nil
}

// checkProviderURL refuses an address that is not https, a loopback http
// address excepted for a provider run on the same machine.
func checkProviderURL(raw, field string, required bool) error {
	if raw == "" {
		if required {
			return OnField(Invalid("%s is required", field), field)
		}
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.RawQuery != "" || u.Fragment != "" {
		return OnField(Invalid("%s %q is not an absolute url", field, raw), field)
	}
	if u.Scheme == "https" || (u.Scheme == "http" && IsLoopbackHost(u.Hostname())) {
		return nil
	}
	return OnField(Invalid("%s must be an https url", field), field)
}

// IsLoopbackHost reports whether host names this machine.
func IsLoopbackHost(host string) bool {
	return host == "127.0.0.1" || host == "::1" || host == "localhost"
}

// SSOProviderOption is a provider as the sign-in screen offers it.
type SSOProviderOption struct {
	// Key is the provider's short name, which starts a sign-in through it.
	Key string `json:"key" yaml:"key"`
	// DisplayName is the name on the button.
	DisplayName string `json:"display_name" yaml:"display_name"`
	// Kind is the provider's protocol.
	Kind SSOKind `json:"kind" yaml:"kind"`
}

// SignInOptions is what a sign-in screen offers on this tenant.
type SignInOptions struct {
	// PasswordLogin is the tenant's password sign-in policy.
	PasswordLogin PasswordLogin `json:"password_login" yaml:"password_login"`
	// Providers are the tenant's enabled providers.
	Providers []SSOProviderOption `json:"providers" yaml:"providers"`
}

// BeginSSOInput starts a sign-in through a provider.
type BeginSSOInput struct {
	// Provider is the provider's key.
	Provider string `json:"provider"`
	// RedirectURI is where the provider returns the code: the browser
	// callback, or a loopback address the command line listens on.
	RedirectURI string `json:"redirect_uri"`
	// Surface is the sign-in path the session is for.
	Surface Surface `json:"surface,omitempty"`
	// Remember asks for a keep me signed in session.
	Remember bool `json:"remember,omitempty"`
	// Next is the screen to return to, for a browser sign-in.
	Next string `json:"next,omitempty"`
}

// SSOBegin is a started sign-in: the address to open and the sealed flow.
type SSOBegin struct {
	// AuthURL is the provider's authorization address to open.
	AuthURL string `json:"auth_url" yaml:"auth_url"`
	// Flow is the sealed sign-in state, handed back to complete it.
	Flow string `json:"flow" yaml:"flow"`
	// State is the value the provider returns with the code.
	State string `json:"state" yaml:"state"`
	// ExpiresAt is when the flow can no longer be completed.
	ExpiresAt time.Time `json:"expires_at" yaml:"expires_at"`
}

// CompleteSSOInput finishes a sign-in with what the provider returned.
type CompleteSSOInput struct {
	// Flow is the sealed state the sign-in began with.
	Flow string `json:"flow"`
	// Code is the authorization code the provider returned.
	Code string `json:"code,omitempty"`
	// State is the state the provider returned.
	State string `json:"state"`
	// Error is the error the provider returned instead of a code.
	Error string `json:"error,omitempty"`
}

// UserIdentity links a person to their identity at one of the tenant's
// providers.
type UserIdentity struct {
	// ID is the link's identifier.
	ID string `json:"id" yaml:"id"`
	// ProviderID is the provider that vouches for the identity.
	ProviderID string `json:"provider_id" yaml:"provider_id"`
	// Subject is the provider's stable identifier for the person.
	Subject string `json:"subject" yaml:"subject"`
	// UserID is the account the identity signs in.
	UserID string `json:"user_id" yaml:"user_id"`
	// Email is the email the provider last reported.
	Email string `json:"email" yaml:"email"`
	// CreatedAt is when the identity was linked.
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	// LastLoginAt is when the identity last signed in.
	LastLoginAt *time.Time `json:"last_login_at,omitempty" yaml:"last_login_at,omitempty"`
}
