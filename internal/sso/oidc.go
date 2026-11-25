// SPDX-License-Identifier: AGPL-3.0-or-later

package sso

import (
	"context"
	"errors"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/heliopsy/tix/internal/core"
)

// oidcAdapter signs in through an OpenID Connect provider.
type oidcAdapter struct {
	client   *Client
	provider *oidc.Provider
	cfg      core.SSOProvider
	secret   string
}

func (c *Client) oidcAdapter(ctx context.Context, p core.SSOProvider, secret string) (*oidcAdapter, error) {
	provider, err := c.discover(ctx, p.Issuer)
	if err != nil {
		return nil, err
	}
	return &oidcAdapter{client: c, provider: provider, cfg: p, secret: secret}, nil
}

func (a *oidcAdapter) oauth(redirectURI string) *oauth2.Config {
	scopes := a.cfg.Scopes
	if len(scopes) == 0 {
		scopes = core.DefaultOIDCScopes
	}
	return &oauth2.Config{ClientID: a.cfg.ClientID, ClientSecret: a.secret, Endpoint: a.provider.Endpoint(),
		RedirectURL: redirectURI, Scopes: scopes}
}

// AuthURL starts an authorization code flow with PKCE (S256) and a nonce.
func (a *oidcAdapter) AuthURL(state, nonce, verifier, redirectURI string) string {
	return a.oauth(redirectURI).AuthCodeURL(state, oauth2.S256ChallengeOption(verifier), oidc.Nonce(nonce))
}

// Exchange trades the code for tokens and verifies the ID token: signature
// against the issuer's key set, issuer, audience, expiry, then the nonce.
func (a *oidcAdapter) Exchange(ctx context.Context, code, verifier, redirectURI, nonce string) (Identity, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, a.client.http)
	token, err := a.oauth(redirectURI).Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return Identity{}, exchangeError(err)
	}
	raw, _ := token.Extra("id_token").(string)
	if raw == "" {
		return Identity{}, Refused("the identity provider returned no id token; check that the scopes include openid")
	}
	idToken, err := a.provider.Verifier(&oidc.Config{ClientID: a.cfg.ClientID, Now: a.client.now}).Verify(ctx, raw)
	if err != nil {
		return Identity{}, verifyError(err)
	}
	if idToken.Nonce != nonce {
		return Identity{}, Refused("the identity provider's token does not answer this sign-in (nonce mismatch); start again")
	}
	claims := map[string]any{}
	if err := idToken.Claims(&claims); err != nil {
		return Identity{}, Refused("the identity provider's token carries unreadable claims")
	}
	out := a.identity(idToken.Subject, claims)
	if out.Email == "" && a.provider.UserInfoEndpoint() != "" {
		a.fillFromUserInfo(ctx, token, &out)
	}
	return out, nil
}

// fillFromUserInfo reads the email from the userinfo endpoint, for a provider
// that leaves it out of the ID token. The subject must match, or the answer
// is about somebody else and is ignored.
func (a *oidcAdapter) fillFromUserInfo(ctx context.Context, token *oauth2.Token, out *Identity) {
	info, err := a.provider.UserInfo(oidc.ClientContext(ctx, a.client.http), oauth2.StaticTokenSource(token))
	if err != nil || info.Subject != out.Subject {
		return
	}
	claims := map[string]any{}
	if info.Claims(&claims) != nil {
		return
	}
	more := a.identity(out.Subject, claims)
	out.Email, out.EmailVerified = more.Email, more.EmailVerified
	if out.Name == "" {
		out.Name = more.Name
	}
	if out.Username == "" {
		out.Username = more.Username
	}
}

// identity reads the configured claims.
func (a *oidcAdapter) identity(subject string, claims map[string]any) Identity {
	claim := func(name, fallback string) string {
		if name == "" {
			name = fallback
		}
		return name
	}
	out := Identity{
		Subject:       subject,
		Email:         strings.ToLower(strings.TrimSpace(text(claims[claim(a.cfg.EmailClaim, "email")]))),
		EmailVerified: truthy(claims["email_verified"]),
		Name:          strings.TrimSpace(text(claims[claim(a.cfg.NameClaim, "name")])),
		Username:      strings.TrimSpace(text(claims[claim(a.cfg.UsernameClaim, "preferred_username")])),
		Groups:        texts(claims[claim(a.cfg.GroupsClaim, "groups")]),
	}
	values := a.cfg.MFAValues
	if len(values) == 0 {
		values = core.DefaultMFAValues
	}
	acr := text(claims["acr"])
	out.MFA = anyOf(texts(claims["amr"]), values) || (acr != "" && anyOf([]string{acr}, values))
	return out
}

func text(v any) string {
	s, _ := v.(string)
	return s
}

func truthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(t, "true")
	}
	return false
}

func texts(v any) []string {
	switch t := v.(type) {
	case string:
		if t == "" {
			return nil
		}
		return []string{t}
	case []any:
		out := make([]string, 0, len(t))
		for _, e := range t {
			if s, ok := e.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// exchangeError reports a failed code exchange without the provider's body,
// which can echo the code.
func exchangeError(err error) error {
	var re *oauth2.RetrieveError
	if errors.As(err, &re) {
		code := re.ErrorCode
		if code == "" {
			code = "an error"
		}
		return Refused("the identity provider refused the sign-in code (%s); start again", code)
	}
	return core.Upstream("exchanging the sign-in code with the identity provider failed").Wrap(err)
}

// verifyError names which check an ID token failed, which is the first thing
// an administrator setting a provider up needs to know.
func verifyError(err error) error {
	var expired *oidc.TokenExpiredError
	msg := err.Error()
	switch {
	case errors.As(err, &expired):
		return Refused("the identity provider's token has expired; start again")
	case strings.Contains(msg, "id token issued by a different provider"):
		return Refused("the identity provider's token names another issuer; check the provider's issuer")
	case strings.Contains(msg, "expected audience"):
		return Refused("the identity provider's token was issued for another client; check the client id")
	case strings.Contains(msg, "failed to verify signature"), strings.Contains(msg, "malformed jwt"):
		return Refused("the identity provider's token signature does not verify")
	default:
		return Refused("the identity provider's token was refused: %s", strings.TrimPrefix(msg, "oidc: "))
	}
}
