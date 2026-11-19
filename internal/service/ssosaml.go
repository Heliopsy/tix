// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"crypto/subtle"
	"net/url"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/sso"
	"github.com/heliopsy/tix/internal/store"
)

// Reasons a SAML response is refused, recorded on sso.denied.
// #nosec G101 -- refusal reasons, not credentials.
const (
	ssoDeniedReplay      = "replay"
	ssoDeniedUnsolicited = "unsolicited"
)

// withAddresses fills the addresses to register with the provider, when the
// server knows its public address: the OIDC callback, or a SAML provider's
// assertion consumer service and metadata.
func (l *Local) withAddresses(ctx context.Context, p *core.SSOProvider) {
	l.withRefreshSchedule(p)
	base := l.webBase(ctx)
	if base == "" {
		return
	}
	if p.Kind == core.SSOKindSAML {
		p.RedirectURL, p.MetadataURL = base+core.SAMLACSPath(p.Key), base+core.SAMLMetadataPath(p.Key)
		return
	}
	p.RedirectURL = base + core.SSOCallbackPath
}

// samlMetadataSource is the IdP metadata a create or edit names: a document
// given inline, or an address to read.
type samlMetadataSource struct {
	addr, xml string
}

func (s samlMetadataSource) given() bool { return s.addr != "" || s.xml != "" }

// readIdPMetadata reads the IdP metadata a provider's settings name. It runs
// before the transaction, since it may make a request.
func (l *Local) readIdPMetadata(ctx context.Context, src samlMetadataSource, entityID string) (*sso.IdPMetadata, error) {
	if !src.given() {
		return nil, nil
	}
	if src.xml != "" {
		md, err := sso.ParseIdPMetadata([]byte(src.xml), entityID)
		if err != nil {
			return nil, core.OnField(err, "idp_metadata_xml")
		}
		return &md, nil
	}
	if err := (core.SSOProvider{IdPMetadataURL: src.addr}).CheckMetadataURL(); err != nil {
		return nil, err
	}
	md, err := l.ssoClients().FetchIdPMetadata(ctx, src.addr, entityID)
	if err != nil {
		return nil, err
	}
	return &md, nil
}

// applyIdPMetadata fills from metadata each IdP setting the caller did not
// give explicitly.
func applyIdPMetadata(p *core.SSOProvider, md *sso.IdPMetadata, entity, ssoURL, certs bool) {
	if md == nil {
		return
	}
	if !entity {
		p.IdPEntityID = md.EntityID
	}
	if !ssoURL {
		p.IdPSSOURL = md.SSOURL
	}
	if !certs {
		p.IdPCertificates = md.Certificates
	}
}

// beginSAML starts a SAML sign-in: an AuthnRequest over HTTP-Redirect whose
// ID the flow keeps, with the flow's state as the RelayState.
func (l *Local) beginSAML(ctx context.Context, scope core.TenantScope, p core.SSOProvider, in core.BeginSSOInput,
) (*core.SSOBegin, error) {
	surface := in.Surface
	if surface == "" {
		surface = core.SurfaceWeb
	}
	if s, err := checkRedirectURI(in.RedirectURI, in.Surface); err == nil && s == core.SurfaceCLI {
		surface = s
	}
	if surface != core.SurfaceWeb {
		return nil, core.OnField(core.Invalid("%s is a saml provider, which signs in through the browser only; "+
			"use its Sign in with button on the sign-in screen", p.DisplayName), "provider")
	}
	acs, err := samlACS(in.RedirectURI, p.Key, true)
	if err != nil {
		return nil, err
	}
	if err := l.requireSSOKey(); err != nil {
		return nil, err
	}
	sp, err := sso.NewSAML(p, acs, l.clock.Now())
	if err != nil {
		return nil, err
	}
	state, err := auth.NewSSOSecret()
	if err != nil {
		return nil, err
	}
	addr, requestID, err := sp.AuthnRequest(state)
	if err != nil {
		return nil, err
	}
	flow := auth.SSOFlow{TenantID: scope.TenantID, ProviderID: p.ID, State: state, Nonce: requestID,
		RedirectURI: acs, Surface: surface, Remember: in.Remember, Next: safeReturn(in.Next),
		ExpiresAt: l.clock.Now().Add(auth.SSOFlowTTL).UTC().Truncate(time.Second)}
	sealed, err := auth.SealSSOFlow(l.totpKey, flow)
	if err != nil {
		return nil, err
	}
	return &core.SSOBegin{AuthURL: addr, Flow: sealed, State: state, ExpiresAt: flow.ExpiresAt, Kind: p.Kind}, nil
}

// samlACS answers a SAML provider's assertion consumer service on the origin
// of raw, which is the browser callback (when begin allows it) or the
// consumer service itself.
func samlACS(raw, key string, callbackToo bool) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	ok := err == nil && u.Host != "" && u.User == nil && u.Fragment == "" && u.RawQuery == "" &&
		(u.Scheme == "https" || u.Scheme == "http") &&
		(u.Path == core.SAMLACSPath(key) || (callbackToo && u.Path == core.SSOCallbackPath))
	if !ok {
		return "", core.OnField(core.Invalid("the redirect uri of a saml provider is this server's %s",
			core.SAMLACSPath(key)), "redirect_uri")
	}
	return u.Scheme + "://" + u.Host + core.SAMLACSPath(key), nil
}

// samlFlow is what a SAML response is checked against: the provider, the
// consumer service it was posted to, the request it must answer and the
// session it may mint.
type samlFlow struct {
	scope     core.TenantScope
	provider  *core.SSOProvider
	acs       string
	requestID string
	surface   core.Surface
	remember  bool
	next      string
}

// completeSAML finishes a SAML sign-in from the posted response.
func (l *Local) completeSAML(ctx context.Context, scope core.TenantScope, in core.CompleteSSOInput,
) (*core.LoginResult, error) {
	var (
		f   samlFlow
		err error
	)
	if strings.TrimSpace(in.Flow) != "" {
		f, err = l.solicitedSAML(ctx, scope, in)
	} else {
		f, err = l.unsolicitedSAML(ctx, scope, in)
	}
	if err != nil {
		return nil, err
	}
	sp, err := sso.NewSAML(*f.provider, f.acs, l.clock.Now())
	if err != nil {
		return nil, err
	}
	assertion, err := sp.Verify(in.SAMLResponse, f.requestID)
	if err != nil {
		if core.IsKind(err, core.KindUnauthenticated) {
			l.recordDenied(ctx, f.scope, *f.provider, sso.Identity{}, ssoDeniedToken, messageOf(err))
		}
		return nil, err
	}
	err = l.write(ctx, core.SystemActor(f.scope.TenantID), func(m *mutation) error {
		return m.tx.ConsumeSSOAssertion(ctx, f.provider.ID, assertion.ID, assertion.ExpiresAt, m.now)
	})
	if core.IsKind(err, core.KindConflict) {
		msg := "this SAML response has already been used to sign in; start again"
		l.recordDenied(ctx, f.scope, *f.provider, assertion.Identity, ssoDeniedReplay, msg)
		return nil, core.Unauthenticated("%s", msg)
	}
	if err != nil {
		return nil, err
	}
	out, err := l.signInIdentity(withRemember(ctx, f.remember), f.scope, *f.provider, assertion.Identity, f.surface)
	if err != nil {
		return nil, err
	}
	out.Next, out.Remember = f.next, f.remember
	return out, nil
}

// solicitedSAML opens the flow a SAML response answers, checking it was
// started in this browser for the provider the response was posted to.
func (l *Local) solicitedSAML(ctx context.Context, scope core.TenantScope, in core.CompleteSSOInput) (samlFlow, error) {
	flow, err := auth.OpenSSOFlow(l.totpKey, in.Flow, l.clock.Now())
	if err != nil {
		return samlFlow{}, err
	}
	if flow.TenantID != scope.TenantID && l.servedHere(ctx, flow.TenantID) {
		scope = core.TenantScope{TenantID: flow.TenantID}
	}
	if flow.TenantID != scope.TenantID {
		return samlFlow{}, core.Unauthenticated("this single sign-on was started for another tenant; start again")
	}
	if subtle.ConstantTimeCompare([]byte(flow.State), []byte(in.State)) != 1 {
		return samlFlow{}, core.Unauthenticated("this SAML response does not belong to the sign-in this browser " +
			"started (RelayState mismatch); start again")
	}
	provider, err := l.liveProvider(ctx, scope, flow.ProviderID, "flow")
	if err != nil {
		return samlFlow{}, err
	}
	key := strings.ToLower(strings.TrimSpace(in.Provider))
	if provider.Kind != core.SSOKindSAML || flow.Nonce == "" || (key != "" && key != provider.Key) {
		return samlFlow{}, core.Unauthenticated("this SAML response was posted for a sign-in through another " +
			"provider; start again")
	}
	return samlFlow{scope: scope, provider: provider, acs: flow.RedirectURI, requestID: flow.Nonce,
		surface: flow.Surface, remember: flow.Remember, next: flow.Next}, nil
}

// unsolicitedSAML admits an IdP-initiated response, which answers no flow,
// only for a provider that allows it.
func (l *Local) unsolicitedSAML(ctx context.Context, scope core.TenantScope, in core.CompleteSSOInput) (samlFlow, error) {
	provider, err := l.liveProvider(ctx, scope, strings.ToLower(strings.TrimSpace(in.Provider)), "provider")
	if err != nil {
		return samlFlow{}, err
	}
	if provider.Kind != core.SSOKindSAML {
		return samlFlow{}, core.OnField(core.Invalid("%q is not a saml provider", provider.Key), "provider")
	}
	if !provider.AllowIdPInitiated {
		msg := "this sign-in was started at the identity provider, which " + provider.DisplayName +
			" does not accept; start from the sign-in screen, or have an admin allow IdP-initiated sign-in"
		l.recordDenied(ctx, scope, *provider, sso.Identity{}, ssoDeniedUnsolicited, msg)
		return samlFlow{}, core.Unauthenticated("%s", msg)
	}
	acs, err := samlACS(in.RedirectURI, provider.Key, false)
	if err != nil {
		return samlFlow{}, err
	}
	return samlFlow{scope: scope, provider: provider, acs: acs, surface: core.SurfaceWeb}, nil
}

// SAMLMetadata answers a SAML provider's service provider metadata, built on
// the origin the caller reached the server at.
func (l *Local) SAMLMetadata(ctx context.Context, in core.SAMLMetadataInput) (*core.SAMLMetadata, error) {
	scope, err := core.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	ref := strings.TrimSpace(in.Provider)
	var p *core.SSOProvider
	err = l.read(ctx, core.SystemActor(scope.TenantID), func(tx store.Tx) error {
		p, err = lookupSSOProvider(ctx, tx, ref)
		return err
	})
	if (err == nil && p.Kind != core.SSOKindSAML) || core.IsKind(err, core.KindNotFound) ||
		core.IsKind(err, core.KindInvalid) {
		return nil, core.OnField(core.NotFound("this tenant has no saml provider %q", ref), "provider")
	}
	if err != nil {
		return nil, err
	}
	base := strings.TrimSpace(in.BaseURL)
	if base == "" {
		base = l.webBase(ctx)
	}
	origin := sso.Origin(base)
	if origin == "" {
		return nil, core.OnField(core.Invalid("name the base url the metadata's addresses are built on, or set "+
			"server.public_url"), "base_url")
	}
	sp, err := sso.NewSAML(*p, origin+core.SAMLACSPath(p.Key), l.clock.Now())
	if err != nil {
		return nil, err
	}
	return &core.SAMLMetadata{EntityID: sp.EntityID(), ACSURL: origin + core.SAMLACSPath(p.Key),
		XML: sp.SPMetadata()}, nil
}
