// SPDX-License-Identifier: AGPL-3.0-or-later

// Package sso speaks the protocols of single sign-on providers: OpenID Connect
// and GitHub's OAuth. It turns a provider's answer into an Identity and
// decides nothing about who that identity may be in tix; linking, creation,
// roles and the second factor are the service's.
package sso

import (
	"context"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/heliopsy/tix/internal/core"
)

// Identity is what a provider vouched for, as every kind reports it. SAML
// fills the same fields from its assertion.
type Identity struct {
	// Subject is the provider's stable identifier for the person.
	Subject string
	// Email is the person's email, lowercased.
	Email string
	// EmailVerified reports that the provider verified Email.
	EmailVerified bool
	// Name is the display name.
	Name string
	// Username is a short name, the handle a created account gets.
	Username string
	// Groups are the person's groups, or GitHub organisations.
	Groups []string
	// MFA reports that the provider asserted a second factor the provider's
	// configuration accepts.
	MFA bool
}

// Adapter is one provider's redirect-and-code exchange.
type Adapter interface {
	// AuthURL is the address that starts the sign-in at the provider.
	AuthURL(state, nonce, verifier, redirectURI string) string
	// Exchange trades the returned code for the identity, checking the
	// token as the protocol requires.
	Exchange(ctx context.Context, code, verifier, redirectURI, nonce string) (Identity, error)
}

// discoveryTTL is how long a provider's discovery document, and the key set
// it points to, is reused before it is read again. The key set also refreshes
// itself when a token names a key it does not hold.
const discoveryTTL = time.Hour

// Client builds adapters over one HTTP client, caching each issuer's
// discovery document and key set.
type Client struct {
	http *http.Client
	now  func() time.Time

	mu        sync.Mutex
	providers map[string]cachedProvider
}

type cachedProvider struct {
	provider *oidc.Provider
	at       time.Time
}

// NewClient returns a client that makes every request through hc and reads
// time from now.
func NewClient(hc *http.Client, now func() time.Time) *Client {
	return &Client{http: hc, now: now, providers: map[string]cachedProvider{}}
}

// Adapter returns the adapter for a provider, given its opened client secret.
func (c *Client) Adapter(ctx context.Context, p core.SSOProvider, secret string) (Adapter, error) {
	switch p.Kind {
	case core.SSOKindOIDC:
		return c.oidcAdapter(ctx, p, secret)
	case core.SSOKindGitHub:
		return c.githubAdapter(p, secret), nil
	default:
		return nil, core.Invalid("provider kind %q is not implemented by this build", p.Kind)
	}
}

// discover returns the issuer's provider, from the cache while it is fresh.
func (c *Client) discover(ctx context.Context, issuer string) (*oidc.Provider, error) {
	c.mu.Lock()
	cached, ok := c.providers[issuer]
	c.mu.Unlock()
	if ok && c.now().Sub(cached.at) < discoveryTTL {
		return cached.provider, nil
	}
	provider, err := oidc.NewProvider(oidc.ClientContext(context.WithoutCancel(ctx), c.http), issuer)
	if err != nil {
		return nil, core.Upstream("reading the identity provider's discovery document at %s failed", issuer).Wrap(err)
	}
	c.mu.Lock()
	c.providers[issuer] = cachedProvider{provider: provider, at: c.now()}
	c.mu.Unlock()
	return provider, nil
}

// Forget drops an issuer's cached discovery, for a provider that changed.
func (c *Client) Forget(issuer string) {
	c.mu.Lock()
	delete(c.providers, issuer)
	c.mu.Unlock()
}

// Refused is a sign-in the provider's answer does not support. Its message
// is safe to show the person signing in.
func Refused(format string, args ...any) *core.Error {
	return core.Unauthenticated(format, args...)
}

// anyOf reports whether any value of have is in want.
func anyOf(have, want []string) bool {
	return slices.ContainsFunc(have, func(v string) bool { return slices.Contains(want, v) })
}
