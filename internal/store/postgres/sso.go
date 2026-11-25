// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/id"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

var ssoProviderColumns = []string{
	"id", "key", "kind", "display_name", "enabled", "auto_create", "default_role", "trust_mfa",
	"allowed_domains", "role_mapping", "config", "secret_sealed", "secret_nonce", "secret_hint",
	"created_at", "updated_at",
}

// ssoConfig is the kind's own settings, the JSON in sso_providers.config.
type ssoConfig struct {
	Issuer        string   `json:"issuer,omitempty"`
	ClientID      string   `json:"client_id,omitempty"`
	Scopes        []string `json:"scopes,omitempty"`
	EmailClaim    string   `json:"email_claim,omitempty"`
	NameClaim     string   `json:"name_claim,omitempty"`
	UsernameClaim string   `json:"username_claim,omitempty"`
	GroupsClaim   string   `json:"groups_claim,omitempty"`
	MFAValues     []string `json:"mfa_values,omitempty"`
	AllowedOrgs   []string `json:"allowed_orgs,omitempty"`
	WebURL        string   `json:"web_url,omitempty"`
	APIURL        string   `json:"api_url,omitempty"`

	IdPMetadataURL    string   `json:"idp_metadata_url,omitempty"`
	IdPEntityID       string   `json:"idp_entity_id,omitempty"`
	IdPSSOURL         string   `json:"idp_sso_url,omitempty"`
	IdPCertificates   []string `json:"idp_certificates,omitempty"`
	SPEntityID        string   `json:"sp_entity_id,omitempty"`
	NameIDFormat      string   `json:"name_id_format,omitempty"`
	AllowIdPInitiated bool     `json:"allow_idp_initiated,omitempty"`

	IdPPinned       []string                   `json:"idp_pinned,omitempty"`
	IdPRetiring     []core.RetiringCertificate `json:"idp_retiring,omitempty"`
	MetadataRefresh *core.SAMLMetadataRefresh  `json:"metadata_refresh,omitempty"`
}

func ssoConfigOf(p *core.SSOProvider) ssoConfig {
	return ssoConfig{Issuer: p.Issuer, ClientID: p.ClientID, Scopes: p.Scopes, EmailClaim: p.EmailClaim,
		NameClaim: p.NameClaim, UsernameClaim: p.UsernameClaim, GroupsClaim: p.GroupsClaim, MFAValues: p.MFAValues,
		AllowedOrgs: p.AllowedOrgs, WebURL: p.WebURL, APIURL: p.APIURL,
		IdPMetadataURL: p.IdPMetadataURL, IdPEntityID: p.IdPEntityID, IdPSSOURL: p.IdPSSOURL,
		IdPCertificates: p.IdPCertificates, SPEntityID: p.SPEntityID, NameIDFormat: p.NameIDFormat,
		AllowIdPInitiated: p.AllowIdPInitiated, IdPPinned: p.IdPPinned, IdPRetiring: p.IdPRetiring,
		MetadataRefresh: p.MetadataRefresh}
}

func (c ssoConfig) applyTo(p *core.SSOProvider) {
	p.Issuer, p.ClientID, p.Scopes, p.EmailClaim = c.Issuer, c.ClientID, c.Scopes, c.EmailClaim
	p.NameClaim, p.UsernameClaim, p.GroupsClaim, p.MFAValues = c.NameClaim, c.UsernameClaim, c.GroupsClaim, c.MFAValues
	p.AllowedOrgs, p.WebURL, p.APIURL = c.AllowedOrgs, c.WebURL, c.APIURL
	p.IdPMetadataURL, p.IdPEntityID, p.IdPSSOURL = c.IdPMetadataURL, c.IdPEntityID, c.IdPSSOURL
	p.IdPCertificates, p.SPEntityID, p.NameIDFormat = c.IdPCertificates, c.SPEntityID, c.NameIDFormat
	p.AllowIdPInitiated, p.IdPPinned, p.IdPRetiring = c.AllowIdPInitiated, c.IdPPinned, c.IdPRetiring
	p.MetadataRefresh = c.MetadataRefresh
}

func scanSSOProvider(s scanner) (core.SSOProvider, error) {
	var (
		p                        core.SSOProvider
		enabled, autoCreate, mfa bool
		domains, mapping, config string
		created, updated         sql.NullTime
		cfg                      ssoConfig
	)
	if err := s.Scan(&p.ID, &p.Key, &p.Kind, &p.DisplayName, &enabled, &autoCreate, &p.DefaultRole, &mfa,
		&domains, &mapping, &config, &p.SealedSecret, &p.SecretNonce, &p.SecretHint, &created, &updated); err != nil {
		return core.SSOProvider{}, mapErr(err, "scanning sso provider")
	}
	p.Enabled, p.AutoCreate, p.TrustMFA = enabled, autoCreate, mfa
	if err := decodeSSOJSON(p.Key, domains, mapping, config, &p, &cfg); err != nil {
		return core.SSOProvider{}, err
	}
	p.CreatedAt, p.UpdatedAt = scanTime(created), scanTime(updated)
	return p, nil
}

func decodeSSOJSON(key, domains, mapping, config string, p *core.SSOProvider, cfg *ssoConfig) error {
	if err := sqlb.ParseJSON(domains, &p.AllowedDomains); err != nil {
		return core.Internal("decoding allowed domains of sso provider %q", key).Wrap(err)
	}
	if err := sqlb.ParseJSON(mapping, &p.RoleMapping); err != nil {
		return core.Internal("decoding role mapping of sso provider %q", key).Wrap(err)
	}
	if err := sqlb.ParseJSON(config, cfg); err != nil {
		return core.Internal("decoding settings of sso provider %q", key).Wrap(err)
	}
	cfg.applyTo(p)
	if len(p.AllowedDomains) == 0 {
		p.AllowedDomains = nil
	}
	if len(p.RoleMapping) == 0 {
		p.RoleMapping = nil
	}
	p.SecretSet = p.SealedSecret != ""
	return nil
}

func encodeSSOJSON(p *core.SSOProvider) (domains, mapping, config string, err error) {
	if domains, err = sqlb.JSONText(p.AllowedDomains, "[]"); err != nil {
		return "", "", "", core.Internal("encoding allowed domains of sso provider %q", p.Key).Wrap(err)
	}
	if mapping, err = sqlb.JSONText(p.RoleMapping, "{}"); err != nil {
		return "", "", "", core.Internal("encoding role mapping of sso provider %q", p.Key).Wrap(err)
	}
	if config, err = sqlb.JSONText(ssoConfigOf(p), "{}"); err != nil {
		return "", "", "", core.Internal("encoding settings of sso provider %q", p.Key).Wrap(err)
	}
	return domains, mapping, config, nil
}

// ListSSOProviders returns the tenant's providers, by key.
func (t *tx) ListSSOProviders(ctx context.Context) ([]core.SSOProvider, error) {
	b := t.builder("sso_providers").Select(ssoProviderColumns...).OrderBy("key", core.Ascending)
	rows, err := t.query(ctx, b, "listing sso providers")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []core.SSOProvider{}
	for rows.Next() {
		p, err := scanSSOProvider(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, mapRowsErr(rows, "listing sso providers")
}

// GetSSOProvider returns a provider by identifier or key.
func (t *tx) GetSSOProvider(ctx context.Context, ref string) (*core.SSOProvider, error) {
	for _, col := range []string{"id", "key"} {
		b := t.builder("sso_providers").Select(ssoProviderColumns...).Where(col+" = ?", ref).Limit(1)
		q, args := b.SelectQuery()
		p, err := scanSSOProvider(t.ex.QueryRowContext(ctx, q, args...))
		if err == nil {
			return &p, nil
		}
		if !core.IsKind(err, core.KindNotFound) {
			return nil, err
		}
	}
	return nil, core.NotFound("sso provider %q", ref)
}

// CreateSSOProvider inserts a provider.
func (t *tx) CreateSSOProvider(ctx context.Context, p *core.SSOProvider) error {
	if p.ID == "" {
		p.ID = id.New()
	}
	now := t.store.clock.Now()
	p.CreatedAt, p.UpdatedAt = now, now
	domains, mapping, config, err := encodeSSOJSON(p)
	if err != nil {
		return err
	}
	ins := t.insert("sso_providers").
		Set("id", p.ID).
		Set("key", p.Key).
		Set("kind", string(p.Kind)).
		Set("display_name", p.DisplayName).
		Set("enabled", p.Enabled).
		Set("auto_create", p.AutoCreate).
		Set("default_role", string(p.DefaultRole)).
		Set("trust_mfa", p.TrustMFA).
		Set("allowed_domains", domains).
		Set("role_mapping", mapping).
		Set("config", config).
		Set("secret_sealed", p.SealedSecret).
		Set("secret_nonce", p.SecretNonce).
		Set("secret_hint", p.SecretHint).
		Set("created_at", timeArg(p.CreatedAt)).
		Set("updated_at", timeArg(p.UpdatedAt))
	_, err = t.execInsert(ctx, ins, "creating sso provider %q", p.Key)
	return err
}

// UpdateSSOProvider rewrites a provider's settings; its key and kind stay.
func (t *tx) UpdateSSOProvider(ctx context.Context, p *core.SSOProvider) error {
	p.UpdatedAt = t.store.clock.Now()
	domains, mapping, config, err := encodeSSOJSON(p)
	if err != nil {
		return err
	}
	upd := t.builder("sso_providers").Where("id = ?", p.ID).
		Set("display_name", p.DisplayName).
		Set("enabled", p.Enabled).
		Set("auto_create", p.AutoCreate).
		Set("default_role", string(p.DefaultRole)).
		Set("trust_mfa", p.TrustMFA).
		Set("allowed_domains", domains).
		Set("role_mapping", mapping).
		Set("config", config).
		Set("secret_sealed", p.SealedSecret).
		Set("secret_nonce", p.SecretNonce).
		Set("secret_hint", p.SecretHint).
		Set("updated_at", timeArg(p.UpdatedAt))
	n, err := t.execUpdate(ctx, upd, "updating sso provider %q", p.Key)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("sso provider %q", p.Key)
	}
	return nil
}

// RecordSSOProviderRefresh rewrites a provider's own settings after a read
// of its IdP metadata, leaving updated_at to the changes an admin makes.
func (t *tx) RecordSSOProviderRefresh(ctx context.Context, p *core.SSOProvider) error {
	_, _, config, err := encodeSSOJSON(p)
	if err != nil {
		return err
	}
	upd := t.builder("sso_providers").Where("id = ?", p.ID).Set("config", config)
	n, err := t.execUpdate(ctx, upd, "recording the metadata refresh of sso provider %q", p.Key)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("sso provider %q", p.Key)
	}
	return nil
}

// DeleteSSOProvider removes a provider and, by cascade, its identities.
func (t *tx) DeleteSSOProvider(ctx context.Context, providerID string) error {
	if _, err := t.execDelete(ctx, t.builder("sso_assertions").Where("provider_id = ?", providerID),
		"deleting used assertions of sso provider %q", providerID); err != nil {
		return err
	}
	if _, err := t.execDelete(ctx, t.builder("sso_handoffs").Where("provider_id = ?", providerID),
		"deleting pending handoffs of sso provider %q", providerID); err != nil {
		return err
	}
	if _, err := t.execDelete(ctx, t.builder("user_identities").Where("provider_id = ?", providerID),
		"deleting identities of sso provider %q", providerID); err != nil {
		return err
	}
	n, err := t.execDelete(ctx, t.builder("sso_providers").Where("id = ?", providerID),
		"deleting sso provider %q", providerID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("sso provider %q", providerID)
	}
	return nil
}

var userIdentityColumns = []string{"id", "provider_id", "subject", "user_id", "email", "created_at", "last_login_at"}

func scanUserIdentity(s scanner) (core.UserIdentity, error) {
	var (
		i              core.UserIdentity
		created, login sql.NullTime
	)
	if err := s.Scan(&i.ID, &i.ProviderID, &i.Subject, &i.UserID, &i.Email, &created, &login); err != nil {
		return core.UserIdentity{}, mapErr(err, "scanning user identity")
	}
	i.CreatedAt, i.LastLoginAt = scanTime(created), scanNullTime(login)
	return i, nil
}

func (t *tx) userIdentityWhere(ctx context.Context, providerID, col, value string) (*core.UserIdentity, error) {
	b := t.builder("user_identities").Select(userIdentityColumns...).
		Where("provider_id = ?", providerID).Where(col+" = ?", value).Limit(1)
	q, args := b.SelectQuery()
	i, err := scanUserIdentity(t.ex.QueryRowContext(ctx, q, args...))
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound("user identity")
		}
		return nil, err
	}
	return &i, nil
}

// GetUserIdentity returns the identity a provider knows by subject.
func (t *tx) GetUserIdentity(ctx context.Context, providerID, subject string) (*core.UserIdentity, error) {
	return t.userIdentityWhere(ctx, providerID, "subject", subject)
}

// GetUserIdentityByUser returns the identity a provider links to a user.
func (t *tx) GetUserIdentityByUser(ctx context.Context, providerID, userID string) (*core.UserIdentity, error) {
	return t.userIdentityWhere(ctx, providerID, "user_id", userID)
}

// CreateUserIdentity links an identity to a user.
func (t *tx) CreateUserIdentity(ctx context.Context, i *core.UserIdentity) error {
	if i.ID == "" {
		i.ID = id.New()
	}
	i.CreatedAt = t.store.clock.Now()
	ins := t.insert("user_identities").
		Set("id", i.ID).
		Set("provider_id", i.ProviderID).
		Set("subject", i.Subject).
		Set("user_id", i.UserID).
		Set("email", i.Email).
		Set("created_at", timeArg(i.CreatedAt)).
		Set("last_login_at", nullTimeArg(i.LastLoginAt))
	_, err := t.execInsert(ctx, ins, "linking identity %q", i.Subject)
	return err
}

// TouchUserIdentity records a sign-in through an identity.
func (t *tx) TouchUserIdentity(ctx context.Context, identityID, email string, at time.Time) error {
	upd := t.builder("user_identities").Where("id = ?", identityID).
		Set("email", email).
		Set("last_login_at", timeArg(at))
	n, err := t.execUpdate(ctx, upd, "recording sign-in of identity %q", identityID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("user identity %q", identityID)
	}
	return nil
}

// ConsumeSSOAssertion records a SAML assertion as used, after pruning the
// tenant's expired ones; an id already recorded is a conflict.
func (t *tx) ConsumeSSOAssertion(ctx context.Context, providerID, assertionID string, expiresAt, now time.Time) error {
	if _, err := t.execDelete(ctx, t.builder("sso_assertions").Where("expires_at < ?", timeArg(now)),
		"pruning used sso assertions"); err != nil {
		return err
	}
	ins := t.insert("sso_assertions").
		Set("id", id.New()).
		Set("provider_id", providerID).
		Set("assertion_id", assertionID).
		Set("expires_at", timeArg(expiresAt))
	_, err := t.execInsert(ctx, ins, "recording sso assertion %q", assertionID)
	return err
}

var ssoHandoffColumns = []string{
	"id", "provider_id", "code_hash", "code_challenge", "user_id", "method", "second_factor", "expires_at",
}

// CreateSSOHandoff records an app sign-in's code, after pruning the tenant's
// expired ones.
func (t *tx) CreateSSOHandoff(ctx context.Context, h *store.SSOHandoff, now time.Time) error {
	if err := t.pruneSSOHandoffs(ctx, now); err != nil {
		return err
	}
	if h.ID == "" {
		h.ID = id.New()
	}
	ins := t.insert("sso_handoffs").
		Set("id", h.ID).
		Set("provider_id", h.ProviderID).
		Set("code_hash", h.CodeHash).
		Set("code_challenge", h.CodeChallenge).
		Set("user_id", h.UserID).
		Set("method", h.Method).
		Set("second_factor", h.SecondFactor).
		Set("expires_at", timeArg(h.ExpiresAt))
	_, err := t.execInsert(ctx, ins, "recording an sso handoff")
	return err
}

// TakeSSOHandoff deletes and returns an unexpired code by its hash, after
// pruning the tenant's expired ones. The delete decides: a take that deletes
// no row lost to another.
func (t *tx) TakeSSOHandoff(ctx context.Context, codeHash string, now time.Time) (*store.SSOHandoff, error) {
	if err := t.pruneSSOHandoffs(ctx, now); err != nil {
		return nil, err
	}
	q, args := t.builder("sso_handoffs").Select(ssoHandoffColumns...).Where("code_hash = ?", codeHash).Limit(1).
		SelectQuery()
	h, err := scanSSOHandoff(t.ex.QueryRowContext(ctx, q, args...))
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound("sso handoff")
		}
		return nil, err
	}
	n, err := t.execDelete(ctx, t.builder("sso_handoffs").Where("id = ?", h.ID), "taking an sso handoff")
	if err != nil {
		return nil, err
	}
	if n == 0 || !h.ExpiresAt.After(now) {
		return nil, core.NotFound("sso handoff")
	}
	return h, nil
}

// pruneSSOHandoffs deletes the tenant's codes that expired before now.
func (t *tx) pruneSSOHandoffs(ctx context.Context, now time.Time) error {
	_, err := t.execDelete(ctx, t.builder("sso_handoffs").Where("expires_at < ?", timeArg(now)),
		"pruning expired sso handoffs")
	return err
}

func scanSSOHandoff(row scanner) (*store.SSOHandoff, error) {
	var (
		h       store.SSOHandoff
		expires sql.NullTime
	)
	if err := row.Scan(&h.ID, &h.ProviderID, &h.CodeHash, &h.CodeChallenge, &h.UserID, &h.Method, &h.SecondFactor,
		&expires); err != nil {
		return nil, mapErr(err, "reading sso handoff")
	}
	h.ExpiresAt = scanTime(expires)
	return &h, nil
}
