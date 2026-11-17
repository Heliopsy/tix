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

// Audit actions and events for signing in through a provider.
const (
	auditSSOLogin       = "sso.login"
	auditSSOLink        = "sso.link"
	auditSSOUserCreated = "sso.user_created"
	auditSSODenied      = "sso.denied"

	eventSSOLogin       core.EventType = "sso.login"
	eventSSOLinked      core.EventType = "sso.linked"
	eventSSOUserCreated core.EventType = "sso.user_created"
	eventSSODenied      core.EventType = "sso.denied"

	subjectSSOIdentity = "sso_identity"
)

// ssoHTTPTimeout bounds one request to an identity provider.
const ssoHTTPTimeout = 15 * time.Second

// ssoClients returns the adapter builder, made on first use over the egress
// policy, so an identity provider is reached on the same terms as a webhook.
func (l *Local) ssoClients() *sso.Client {
	l.ssoOnce.Do(func() { l.ssoClient = sso.NewClient(l.egress.Client(ssoHTTPTimeout, nil), l.clock.Now) })
	return l.ssoClient
}

func (l *Local) forgetSSOIssuer(issuer string) {
	if issuer != "" {
		l.ssoClients().Forget(issuer)
	}
}

// SignInOptions reports the tenant's password policy and enabled providers.
func (l *Local) SignInOptions(ctx context.Context) (*core.SignInOptions, error) {
	scope, err := core.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	out := &core.SignInOptions{PasswordLogin: core.PasswordLoginAllowed, Providers: []core.SSOProviderOption{}}
	err = l.read(ctx, core.SystemActor(scope.TenantID), func(tx store.Tx) error {
		tenant, err := tx.GetTenant(ctx)
		if err != nil {
			return err
		}
		if tenant.PasswordLogin.Disabled() {
			out.PasswordLogin = core.PasswordLoginDisabled
		}
		all, err := tx.ListSSOProviders(ctx)
		if err != nil {
			return err
		}
		for _, p := range all {
			if p.Enabled && p.Kind.Implemented() {
				out.Providers = append(out.Providers, core.SSOProviderOption{Key: p.Key, DisplayName: p.DisplayName,
					Kind: p.Kind})
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// checkRedirectURI accepts the browser callback, on any host since the IdP
// holds the list of registered ones, or a loopback address for the command
// line, and answers the surface each is for.
func checkRedirectURI(raw string, surface core.Surface) (core.Surface, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || u.Fragment != "" || u.User != nil {
		return "", core.OnField(core.Invalid("the redirect uri must be an absolute url"), "redirect_uri")
	}
	switch {
	case u.Scheme == "http" && core.IsLoopbackHost(u.Hostname()) && u.Path != core.SSOCallbackPath:
		if surface == "" {
			surface = core.SurfaceCLI
		}
		if surface != core.SurfaceCLI {
			return "", core.OnField(core.Invalid("a loopback redirect is for a command-line sign-in"), "surface")
		}
		return surface, nil
	case (u.Scheme == "https" || u.Scheme == "http") && u.Path == core.SSOCallbackPath && u.RawQuery == "":
		if surface == "" {
			surface = core.SurfaceWeb
		}
		return surface, nil
	}
	return "", core.OnField(core.Invalid("the redirect uri must be this server's %s or a loopback address "+
		"such as http://127.0.0.1:PORT/callback", core.SSOCallbackPath), "redirect_uri")
}

// safeReturn keeps only a path on this origin as the screen to return to.
func safeReturn(next string) string {
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.ContainsAny(next, "\\\r\n") {
		return ""
	}
	return next
}

// BeginSSOLogin starts a sign-in through one of the tenant's providers.
func (l *Local) BeginSSOLogin(ctx context.Context, in core.BeginSSOInput) (*core.SSOBegin, error) {
	scope, err := core.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	provider, err := l.liveProvider(ctx, scope, strings.ToLower(strings.TrimSpace(in.Provider)), "key")
	if err != nil {
		return nil, err
	}
	if provider.Kind == core.SSOKindSAML {
		return l.beginSAML(ctx, scope, *provider, in)
	}
	surface, err := checkRedirectURI(in.RedirectURI, in.Surface)
	if err != nil {
		return nil, err
	}
	if !surface.Valid() {
		return nil, core.Invalid("%q is not a sign-in surface", surface)
	}
	adapter, err := l.adapterFor(ctx, scope, *provider)
	if err != nil {
		return nil, err
	}
	flow := auth.SSOFlow{TenantID: scope.TenantID, ProviderID: provider.ID, RedirectURI: strings.TrimSpace(in.RedirectURI),
		Surface: surface, Remember: in.Remember, Next: safeReturn(in.Next),
		ExpiresAt: l.clock.Now().Add(auth.SSOFlowTTL).UTC().Truncate(time.Second)}
	for _, v := range []*string{&flow.State, &flow.Nonce, &flow.Verifier} {
		if *v, err = auth.NewSSOSecret(); err != nil {
			return nil, err
		}
	}
	sealed, err := auth.SealSSOFlow(l.totpKey, flow)
	if err != nil {
		return nil, err
	}
	return &core.SSOBegin{AuthURL: adapter.AuthURL(flow.State, flow.Nonce, flow.Verifier, flow.RedirectURI),
		Flow: sealed, State: flow.State, ExpiresAt: flow.ExpiresAt, Kind: provider.Kind}, nil
}

// liveProvider reads an enabled provider by key or identifier, refusing one
// that is off or of a kind this build cannot sign in through.
func (l *Local) liveProvider(ctx context.Context, scope core.TenantScope, ref, field string) (*core.SSOProvider, error) {
	if ref == "" {
		return nil, core.OnField(core.Invalid("name the provider to sign in with"), "provider")
	}
	var out *core.SSOProvider
	err := l.read(ctx, core.SystemActor(scope.TenantID), func(tx store.Tx) error {
		p, err := tx.GetSSOProvider(ctx, ref)
		out = p
		return err
	})
	if core.IsKind(err, core.KindNotFound) || (err == nil && (!out.Enabled || !out.Kind.Implemented())) {
		return nil, core.OnField(core.NotFound("this tenant has no enabled sign-in provider %q", ref), field)
	}
	return out, err
}

func (l *Local) adapterFor(ctx context.Context, scope core.TenantScope, p core.SSOProvider) (sso.Adapter, error) {
	if err := l.requireSSOKey(); err != nil {
		return nil, err
	}
	secret, err := l.openSSOSecret(scope.TenantID, p)
	if err != nil {
		return nil, err
	}
	return l.ssoClients().Adapter(ctx, p, secret)
}

// CompleteSSOLogin finishes a sign-in with what the provider returned.
func (l *Local) CompleteSSOLogin(ctx context.Context, in core.CompleteSSOInput) (*core.LoginResult, error) {
	scope, err := core.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	if err := l.requireSSOKey(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.SAMLResponse) != "" {
		return l.completeSAML(ctx, scope, in)
	}
	flow, err := auth.OpenSSOFlow(l.totpKey, in.Flow, l.clock.Now())
	if err != nil {
		return nil, err
	}
	if flow.TenantID != scope.TenantID && l.servedHere(ctx, flow.TenantID) {
		scope = core.TenantScope{TenantID: flow.TenantID}
	}
	if flow.TenantID != scope.TenantID {
		return nil, core.Unauthenticated("this single sign-on was started for another tenant; start again")
	}
	if subtle.ConstantTimeCompare([]byte(flow.State), []byte(in.State)) != 1 {
		return nil, core.Unauthenticated("this single sign-on answer does not belong to the sign-in this " +
			"browser started (state mismatch); start again")
	}
	if e := strings.TrimSpace(in.Error); e != "" {
		return nil, core.Unauthenticated("the identity provider ended the sign-in: %s", truncateText(e, 80))
	}
	if strings.TrimSpace(in.Code) == "" {
		return nil, core.Invalid("the identity provider returned no code")
	}
	provider, err := l.liveProvider(ctx, scope, flow.ProviderID, "flow")
	if err != nil {
		return nil, err
	}
	adapter, err := l.adapterFor(ctx, scope, *provider)
	if err != nil {
		return nil, err
	}
	ident, err := adapter.Exchange(ctx, in.Code, flow.Verifier, flow.RedirectURI, flow.Nonce)
	if err != nil {
		if core.IsKind(err, core.KindUnauthenticated) {
			l.recordDenied(ctx, scope, *provider, sso.Identity{}, ssoDeniedToken, messageOf(err))
		}
		return nil, err
	}
	out, err := l.signInIdentity(withRemember(ctx, flow.Remember), scope, *provider, ident, flow.Surface)
	if err != nil {
		return nil, err
	}
	out.Next, out.Remember = flow.Next, flow.Remember
	return out, nil
}

func truncateText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
