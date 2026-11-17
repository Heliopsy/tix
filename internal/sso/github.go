// SPDX-License-Identifier: AGPL-3.0-or-later

package sso

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/oauth2"

	"github.com/heliopsy/tix/internal/core"
)

// GitHub's own addresses, used when a provider names no Enterprise Server.
const (
	githubWebURL = "https://github.com"
	githubAPIURL = "https://api.github.com"
)

// githubAdapter signs in through a GitHub OAuth app. GitHub is not an OIDC
// provider: there is no ID token, so the identity is read from the API with
// the access token the code is exchanged for.
type githubAdapter struct {
	client *Client
	cfg    core.SSOProvider
	secret string
	web    string
	api    string
}

func (c *Client) githubAdapter(p core.SSOProvider, secret string) *githubAdapter {
	out := &githubAdapter{client: c, cfg: p, secret: secret, web: githubWebURL, api: githubAPIURL}
	if p.WebURL != "" {
		out.web, out.api = p.WebURL, p.APIURL
	}
	return out
}

// orgs are the organisations the sign-in asks about: the allowed ones and
// those the role mapping names.
func (a *githubAdapter) orgs() []string {
	out := slices.Clone(a.cfg.AllowedOrgs)
	for group := range a.cfg.RoleMapping {
		if g := strings.ToLower(group); !slices.Contains(out, g) {
			out = append(out, g)
		}
	}
	slices.Sort(out)
	return out
}

func (a *githubAdapter) oauth(redirectURI string) *oauth2.Config {
	scopes := []string{"read:user", "user:email"}
	if len(a.orgs()) > 0 {
		scopes = append(scopes, "read:org")
	}
	return &oauth2.Config{
		ClientID: a.cfg.ClientID, ClientSecret: a.secret, RedirectURL: redirectURI, Scopes: scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL: a.web + "/login/oauth/authorize", TokenURL: a.web + "/login/oauth/access_token",
			AuthStyle: oauth2.AuthStyleInParams,
		},
	}
}

// AuthURL starts the OAuth flow with PKCE; GitHub has no nonce.
func (a *githubAdapter) AuthURL(state, _, verifier, redirectURI string) string {
	return a.oauth(redirectURI).AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))
}

// Exchange trades the code for an access token and reads the account, its
// primary email and its organisations.
func (a *githubAdapter) Exchange(ctx context.Context, code, verifier, redirectURI, _ string) (Identity, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, a.client.http)
	token, err := a.oauth(redirectURI).Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return Identity{}, exchangeError(err)
	}
	var user struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
	}
	if err := a.get(ctx, token, "/user", &user); err != nil {
		return Identity{}, err
	}
	if user.ID == 0 {
		return Identity{}, Refused("GitHub returned no account for this sign-in")
	}
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := a.get(ctx, token, "/user/emails", &emails); err != nil {
		return Identity{}, err
	}
	out := Identity{Subject: strconv.FormatInt(user.ID, 10), Name: user.Name, Username: strings.ToLower(user.Login)}
	for _, e := range emails {
		if e.Primary {
			out.Email, out.EmailVerified = strings.ToLower(strings.TrimSpace(e.Email)), e.Verified
		}
	}
	for _, org := range a.orgs() {
		member, err := a.activeMember(ctx, token, org)
		if err != nil {
			return Identity{}, err
		}
		if member {
			out.Groups = append(out.Groups, org)
		}
	}
	if len(a.cfg.AllowedOrgs) > 0 && !anyOf(out.Groups, a.cfg.AllowedOrgs) {
		return Identity{}, Refused("this GitHub account is not a member of %s; the provider allows only members",
			strings.Join(a.cfg.AllowedOrgs, " or "))
	}
	return out, nil
}

// activeMember reports whether the token's account is an active member of
// org. A 404 or 403 is no membership, or an organisation that has not granted
// this app access, which is the same answer for a sign-in.
func (a *githubAdapter) activeMember(ctx context.Context, token *oauth2.Token, org string) (bool, error) {
	var m struct {
		State string `json:"state"`
	}
	err := a.get(ctx, token, "/user/memberships/orgs/"+url.PathEscape(org), &m)
	if core.IsKind(err, core.KindNotFound) {
		return false, nil
	}
	return m.State == "active", err
}

func (a *githubAdapter) get(ctx context.Context, token *oauth2.Token, path string, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.api+path, nil)
	if err != nil {
		return core.Internal("building a GitHub request").Wrap(err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	token.SetAuthHeader(req)
	resp, err := a.client.http.Do(req)
	if err != nil {
		return core.Upstream("reaching the GitHub API failed").Wrap(err)
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden:
		return core.NotFound("GitHub %s", path)
	case resp.StatusCode != http.StatusOK:
		return core.Upstream("the GitHub API answered %s for %s", resp.Status, path)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(into); err != nil {
		return core.Upstream("reading the GitHub API's answer for %s failed", path).Wrap(err)
	}
	return nil
}
