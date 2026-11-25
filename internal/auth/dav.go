// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// DAVChallenge is the WWW-Authenticate value every 401 under /dav/ carries.
// Without it Apple's clients and Thunderbird never send credentials.
const DAVChallenge = `Basic realm="tix", charset="UTF-8"`

// DAVOwner is what a Basic username is compared with: the sign-in email of
// an app password's owner and their handle in the tenant.
type DAVOwner struct {
	Email  string
	Handle string
}

// DAVOwnerLookup finds the owner of an app password in the request's tenant.
type DAVOwnerLookup interface {
	DAVOwner(ctx context.Context, actorID string) (*DAVOwner, error)
}

// BasicDAVAuthenticator authenticates an app password presented over HTTP
// Basic, and nothing else. It never hashes a password: a value that is not a
// token's shape is refused before any lookup, so an account password typed
// into a client costs nothing to refuse and is never checked.
type BasicDAVAuthenticator struct {
	verifier *TokenVerifier
	owners   DAVOwnerLookup
}

// NewBasicDAVAuthenticator returns an authenticator over the token verifier
// and the owner lookup.
func NewBasicDAVAuthenticator(v *TokenVerifier, owners DAVOwnerLookup) *BasicDAVAuthenticator {
	return &BasicDAVAuthenticator{verifier: v, owners: owners}
}

// Authenticate resolves an app password. Anything else a request presents,
// a cookie, a bearer header, an account password or another token, is
// refused as dav_password_required; nothing at all is ErrNoCredential.
func (a *BasicDAVAuthenticator) Authenticate(ctx context.Context, r *http.Request) (*core.Actor, error) {
	refused := func() (*core.Actor, error) {
		NoteRefusal(ctx, Refusal{Reason: RefusedDAVPasswordRequired})
		return nil, core.Unauthenticated("an app password is required; create one with tix dav password create")
	}
	if r.Header.Get("Authorization") == "" {
		if c, err := r.Cookie(SessionCookieName); err == nil && c.Value != "" {
			return refused()
		}
		return nil, ErrNoCredential
	}
	// The Basic password slot carries an app password: a minted 256-bit token,
	// hashed like every other token, never a password a person chose.
	username, appToken, ok := r.BasicAuth()
	if !ok {
		return refused()
	}
	if _, err := ParseAPIToken(appToken); err != nil {
		return refused()
	}
	return a.verifier.verify(ctx, appToken, func(tok *core.APIToken) error {
		if !slices.Equal(tok.Scopes, []core.Scope{core.ScopeCaldav}) {
			NoteRefusal(ctx, Refusal{Reason: RefusedDAVPasswordRequired, TokenID: tok.ID, TokenName: tok.Name})
			return core.Unauthenticated("an app password is required; this token is not one")
		}
		owner, err := a.owners.DAVOwner(ctx, tok.ActorID)
		if err != nil {
			return core.Internal("looking up the owner of an app password").Wrap(err)
		}
		if !owner.Names(username) {
			NoteRefusal(ctx, Refusal{Reason: RefusedDAVUserMismatch, TokenID: tok.ID, TokenName: tok.Name})
			return core.Unauthenticated("invalid credentials")
		}
		return nil
	})
}

// Names reports whether a Basic username names the owner, by email or by
// handle, without regard to case.
func (o *DAVOwner) Names(username string) bool {
	username = strings.TrimSpace(username)
	if o == nil || username == "" {
		return false
	}
	return (o.Email != "" && strings.EqualFold(username, o.Email)) ||
		(o.Handle != "" && strings.EqualFold(username, o.Handle))
}

var _ Authenticator = (*BasicDAVAuthenticator)(nil)
