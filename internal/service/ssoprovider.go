// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// Audit actions and events for sign-in providers.
const (
	auditSSOProviderCreate = "sso.provider.create"
	auditSSOProviderUpdate = "sso.provider.update"
	auditSSOProviderDelete = "sso.provider.delete"

	eventSSOProviderCreated core.EventType = "sso.provider.created"
	eventSSOProviderUpdated core.EventType = "sso.provider.updated"
	eventSSOProviderDeleted core.EventType = "sso.provider.deleted"

	subjectSSOProvider = "sso_provider"
)

// ssoSecretHintMin is the shortest secret whose last four characters show.
const ssoSecretHintMin = 16

// ssoSealContext binds a sealed client secret to its one provider.
func ssoSealContext(tenantID, providerID string) string {
	return "sso-provider\x00" + tenantID + "\x00" + providerID
}

// ListSSOProviders lists the tenant's sign-in providers.
func (l *Local) ListSSOProviders(ctx context.Context) ([]core.SSOProvider, error) {
	actor, err := l.authorize(ctx, authz.ActionSSOAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out []core.SSOProvider
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		out, err = tx.ListSSOProviders(ctx)
		return err
	}); err != nil {
		return nil, err
	}
	for i := range out {
		l.withAddresses(ctx, &out[i])
	}
	return out, nil
}

// GetSSOProvider returns one of the tenant's providers by key or identifier.
func (l *Local) GetSSOProvider(ctx context.Context, ref string) (*core.SSOProvider, error) {
	actor, err := l.authorize(ctx, authz.ActionSSOAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out *core.SSOProvider
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		out, err = lookupSSOProvider(ctx, tx, ref)
		return err
	}); err != nil {
		return nil, err
	}
	l.withAddresses(ctx, out)
	return out, nil
}

func lookupSSOProvider(ctx context.Context, tx store.Tx, ref string) (*core.SSOProvider, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, core.Invalid("a provider key or identifier is required")
	}
	p, err := tx.GetSSOProvider(ctx, ref)
	if core.IsKind(err, core.KindNotFound) && strings.ToLower(ref) != ref {
		return tx.GetSSOProvider(ctx, strings.ToLower(ref))
	}
	return p, err
}

// CreateSSOProvider adds a sign-in provider.
func (l *Local) CreateSSOProvider(ctx context.Context, in core.CreateSSOProviderInput) (*core.SSOProvider, error) {
	actor, err := l.authorize(ctx, authz.ActionSSOAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	p := in.Provider()
	if p.Kind == core.SSOKindSAML {
		md, err := l.readIdPMetadata(ctx, samlMetadataSource{addr: p.IdPMetadataURL,
			xml: strings.TrimSpace(in.IdPMetadataXML)}, p.IdPEntityID)
		if err != nil {
			return nil, err
		}
		applyIdPMetadata(&p, md, p.IdPEntityID != "", p.IdPSSOURL != "", len(p.IdPCertificates) > 0)
		if strings.TrimSpace(in.ClientSecret) != "" {
			return nil, core.OnField(core.Invalid("a saml provider takes no client secret"), "client_secret")
		}
	} else if in.IdPMetadataXML != "" {
		return nil, core.OnField(core.Invalid("idp metadata applies to saml providers only"), "idp_metadata_xml")
	}
	p.Normalize()
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if err := l.requireSSOKey(); err != nil {
		return nil, err
	}
	p.ID = l.ids.New()
	secret := strings.TrimSpace(in.ClientSecret)
	if secret == "" && p.Kind == core.SSOKindGitHub {
		return nil, core.OnField(core.Invalid("a GitHub provider needs its client secret"), "client_secret")
	}
	if err := l.sealSSOSecret(actor.TenantID, &p, secret); err != nil {
		return nil, err
	}
	err = l.write(ctx, actor, func(m *mutation) error {
		switch _, err := m.tx.GetSSOProvider(ctx, p.Key); {
		case err == nil:
			return core.OnField(core.Conflict("this tenant already has a provider %q", p.Key), "key")
		case !core.IsKind(err, core.KindNotFound):
			return err
		}
		if err := m.tx.CreateSSOProvider(ctx, &p); err != nil {
			return err
		}
		return m.Record(auditSSOProviderCreate, eventSSOProviderCreated, subjectSSOProvider, p.ID, "",
			nil, p.Redacted(), map[string]any{"key": p.Key, "kind": string(p.Kind)})
	})
	if err != nil {
		return nil, err
	}
	l.withAddresses(ctx, &p)
	return &p, nil
}

// UpdateSSOProvider changes a provider.
func (l *Local) UpdateSSOProvider(ctx context.Context, ref string, in core.UpdateSSOProviderInput,
) (*core.SSOProvider, error) {
	actor, err := l.authorize(ctx, authz.ActionSSOAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	src := samlMetadataSource{}
	if in.IdPMetadataURL != nil {
		src.addr = strings.TrimSpace(*in.IdPMetadataURL)
	}
	if in.IdPMetadataXML != nil {
		src.xml = strings.TrimSpace(*in.IdPMetadataXML)
	}
	entity := ""
	if in.IdPEntityID != nil {
		entity = strings.TrimSpace(*in.IdPEntityID)
	}
	md, err := l.readIdPMetadata(ctx, src, entity)
	if err != nil {
		return nil, err
	}
	var out core.SSOProvider
	err = l.write(ctx, actor, func(m *mutation) error {
		existing, err := lookupSSOProvider(ctx, m.tx, ref)
		if err != nil {
			return err
		}
		next := in.Apply(*existing)
		if md != nil && next.Kind != core.SSOKindSAML {
			return core.OnField(core.Invalid("idp metadata applies to saml providers only"), "idp_metadata_url")
		}
		applyIdPMetadata(&next, md, in.IdPEntityID != nil, in.IdPSSOURL != nil, in.IdPCertificates != nil)
		next.Normalize()
		if err := next.Validate(); err != nil {
			return err
		}
		if in.ClientSecret != nil {
			secret := strings.TrimSpace(*in.ClientSecret)
			if secret != "" && next.Kind == core.SSOKindSAML {
				return core.OnField(core.Invalid("a saml provider takes no client secret"), "client_secret")
			}
			if secret == "" && next.Kind == core.SSOKindGitHub {
				return core.OnField(core.Invalid("a GitHub provider needs its client secret"), "client_secret")
			}
			next.SealedSecret, next.SecretNonce, next.SecretHint = "", "", ""
			if err := l.sealSSOSecret(actor.TenantID, &next, secret); err != nil {
				return err
			}
		}
		if existing.Enabled && !next.Enabled {
			if err := keepAProvider(ctx, m.tx, existing.ID); err != nil {
				return err
			}
		}
		if err := m.tx.UpdateSSOProvider(ctx, &next); err != nil {
			return err
		}
		out = next
		return m.Record(auditSSOProviderUpdate, eventSSOProviderUpdated, subjectSSOProvider, next.ID, "",
			existing.Redacted(), next.Redacted(), map[string]any{"key": next.Key, "kind": string(next.Kind),
				"secret_changed": in.ClientSecret != nil})
	})
	if err != nil {
		return nil, err
	}
	l.forgetSSOIssuer(out.Issuer)
	l.withAddresses(ctx, &out)
	return &out, nil
}

// DeleteSSOProvider removes a provider and its linked identities.
func (l *Local) DeleteSSOProvider(ctx context.Context, ref string) error {
	actor, err := l.authorize(ctx, authz.ActionSSOAdmin, authz.Resource{})
	if err != nil {
		return err
	}
	return l.write(ctx, actor, func(m *mutation) error {
		existing, err := lookupSSOProvider(ctx, m.tx, ref)
		if err != nil {
			return err
		}
		if existing.Enabled {
			if err := keepAProvider(ctx, m.tx, existing.ID); err != nil {
				return err
			}
		}
		if err := m.tx.DeleteSSOProvider(ctx, existing.ID); err != nil {
			return err
		}
		return m.Record(auditSSOProviderDelete, eventSSOProviderDeleted, subjectSSOProvider, existing.ID, "",
			existing.Redacted(), nil, map[string]any{"key": existing.Key, "kind": string(existing.Kind)})
	})
}

// keepAProvider refuses taking the provider leaving out of service while
// password sign-in is off and it is the tenant's last enabled one: nobody
// but a global admin could sign in afterwards.
func keepAProvider(ctx context.Context, tx store.Tx, leaving string) error {
	tenant, err := tx.GetTenant(ctx)
	if err != nil {
		return err
	}
	if !tenant.PasswordLogin.Disabled() {
		return nil
	}
	n, err := enabledProviders(ctx, tx, leaving)
	if err != nil || n > 0 {
		return err
	}
	return core.Precondition("password sign-in is disabled on this tenant and this is its last enabled provider; " +
		"allow password sign-in first (tix tenant edit --password-login allowed)")
}

// enabledProviders counts the tenant's enabled providers other than except.
func enabledProviders(ctx context.Context, tx store.Tx, except string) (int, error) {
	all, err := tx.ListSSOProviders(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, p := range all {
		if p.Enabled && p.ID != except && p.Kind.Implemented() {
			n++
		}
	}
	return n, nil
}

// requireSSOKey refuses provider configuration on a server with no sealing
// key, which can neither keep a secret nor seal a sign-in flow.
func (l *Local) requireSSOKey() error {
	if l.totpSealer == nil {
		return core.Precondition("single sign-on needs the server's sealing key, auth.totp_key " +
			"(TIX_AUTH_TOTP_KEY), a 32 byte base64 value such as openssl rand -base64 32 prints")
	}
	return nil
}

// sealSSOSecret seals a client secret onto p, or leaves it without one.
func (l *Local) sealSSOSecret(tenantID string, p *core.SSOProvider, secret string) error {
	p.SecretSet = false
	if secret == "" {
		return nil
	}
	if err := l.requireSSOKey(); err != nil {
		return err
	}
	sealed, nonce, err := l.totpSealer.Seal(ssoSealContext(tenantID, p.ID), secret)
	if err != nil {
		return core.Internal("sealing the client secret")
	}
	p.SealedSecret, p.SecretNonce, p.SecretSet = sealed, nonce, true
	if len(secret) >= ssoSecretHintMin {
		p.SecretHint = secret[len(secret)-4:]
	}
	return nil
}

// openSSOSecret returns a provider's client secret, empty for a public client.
func (l *Local) openSSOSecret(tenantID string, p core.SSOProvider) (string, error) {
	if p.SealedSecret == "" {
		return "", nil
	}
	if err := l.requireSSOKey(); err != nil {
		return "", err
	}
	secret, err := l.totpSealer.Open(ssoSealContext(tenantID, p.ID), p.SealedSecret, p.SecretNonce)
	if err != nil {
		return "", core.Precondition("the client secret of provider %q does not open with the server's sealing key; "+
			"store it again with tix tenant sso edit %s --client-secret -", p.Key, p.Key)
	}
	return secret, nil
}

// passwordLoginOf reads a tenant's password policy, empty as allowed.
func passwordLoginOf(t core.Tenant) core.PasswordLogin {
	if t.PasswordLogin == "" {
		return core.PasswordLoginAllowed
	}
	return t.PasswordLogin
}

// passwordPolicyAllowed checks a change to password sign-in: it needs
// sso:admin, since it decides who can sign in at all, and turning passwords
// off needs an enabled provider to turn to. It reads the target tenant in its
// own transaction because a global admin may be changing another tenant.
func (l *Local) passwordPolicyAllowed(ctx context.Context, actor *core.Actor, ref string, global bool,
	in core.UpdateTenantInput,
) error {
	if in.PasswordLogin == nil {
		return nil
	}
	if !actor.HasScope(core.ScopeSSOAdmin) {
		return core.Forbidden("changing password sign-in needs the %s scope", core.ScopeSSOAdmin).
			WithDetail(core.DetailField, "password_login")
	}
	if !in.PasswordLogin.Disabled() {
		return nil
	}
	target := actor.TenantID
	if strings.TrimSpace(ref) != "" {
		if err := l.store.ViewUnscoped(ctx, func(u store.UnscopedTx) error {
			t, err := lookupTenant(ctx, u, ref)
			if err == nil && (t.ID == actor.TenantID || global) {
				target = t.ID
			}
			return err
		}); err != nil {
			return err
		}
	}
	n := 0
	if err := l.read(ctx, core.SystemActor(target), func(tx store.Tx) error {
		var err error
		n, err = enabledProviders(ctx, tx, "")
		return err
	}); err != nil {
		return err
	}
	if n == 0 {
		return core.OnField(core.Precondition("password sign-in can be disabled only once the tenant has an "+
			"enabled single sign-on provider; add one with tix tenant sso add"), "password_login")
	}
	return nil
}

// Refusal reasons of the password policy and an SSO session's confinement.
// #nosec G101 -- refusal reasons, not credentials.
const (
	ReasonPasswordLoginDisabled = "password_login_disabled"
	ReasonSSOSessionConfined    = "sso_session_confined"
)

// passwordSignInAllowed refuses a sign-in by password, or a switch carrying
// one, into a tenant that turned password sign-in off. A global admin is the
// exception, the break-glass path, and an SSH key sign-in is not a password.
func (l *Local) passwordSignInAllowed(ctx context.Context, scope core.TenantScope, user *core.User, method string) error {
	if user.GlobalAdmin || method == core.LoginMethodSSHKey {
		return nil
	}
	var disabled bool
	if err := l.read(ctx, core.SystemActor(scope.TenantID), func(tx store.Tx) error {
		t, err := tx.GetTenant(ctx)
		if err != nil {
			return err
		}
		disabled = t.PasswordLogin.Disabled()
		return nil
	}); err != nil {
		return err
	}
	if !disabled {
		return nil
	}
	return core.Forbidden("this tenant signs in through single sign-on only; use a Sign in with button on the "+
		"sign-in screen, or tix login --sso").WithDetail("reason", ReasonPasswordLoginDisabled)
}
