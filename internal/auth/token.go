// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/id"
)

// TokenPrefix marks a tix personal access token.
const TokenPrefix = "tix_pat_"

// tokenBytes is the entropy behind every minted secret.
const tokenBytes = 32

// tokenEncoding renders secrets as unpadded lowercase base32.
var tokenEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// TokenLookup resolves the stored record for a token hash.
type TokenLookup interface {
	TokenByHash(ctx context.Context, hash string) (*core.APIToken, error)
}

// TokenToucher records the time a token last authenticated a request.
type TokenToucher interface {
	TouchToken(ctx context.Context, id string, at time.Time) error
}

// MintedToken carries a freshly issued token and the hash to persist.
type MintedToken struct {
	Issued *core.IssuedToken
	Hash   string
}

// MintAPIToken issues a token whose secret value is returned exactly once.
func MintAPIToken(clk clock.Clock, tenantID string, in core.CreateTokenInput) (*MintedToken, error) {
	if strings.TrimSpace(tenantID) == "" {
		return nil, core.Invalid("a token requires a tenant")
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	for _, s := range in.Scopes {
		if s != core.ScopeAll && !slices.Contains(core.AllScopes, s) {
			return nil, core.Invalid("unknown scope %q", s)
		}
	}
	secret, err := newSecret()
	if err != nil {
		return nil, err
	}
	value := TokenPrefix + secret
	now := clk.Now()
	return &MintedToken{
		Issued: &core.IssuedToken{
			APIToken: core.APIToken{
				ID:           id.NewAt(now),
				TenantID:     tenantID,
				ActorID:      in.ActorID,
				Name:         in.Name,
				Scopes:       slices.Clone(in.Scopes),
				ProjectID:    in.ProjectID,
				RequireClaim: in.RequireClaim,
				CreatedAt:    now,
				ExpiresAt:    in.ExpiresAt,
			},
			Token: value,
		},
		Hash: HashToken(value),
	}, nil
}

// ParseAPIToken checks the presented value's shape and returns its secret part.
func ParseAPIToken(value string) (string, error) {
	secret, ok := strings.CutPrefix(value, TokenPrefix)
	if !ok {
		return "", core.Unauthenticated("invalid credentials")
	}
	if !wellFormedSecret(secret) {
		return "", core.Unauthenticated("invalid credentials")
	}
	return secret, nil
}

// wellFormedSecret reports whether a value has the shape of a minted secret,
// the part of a token after its prefix and the whole of a session cookie.
func wellFormedSecret(s string) bool {
	if len(s) != tokenEncoding.EncodedLen(tokenBytes) || strings.ToLower(s) != s {
		return false
	}
	_, err := tokenEncoding.DecodeString(strings.ToUpper(s))
	return err == nil
}

// HashToken returns the hex SHA-256 digest that is stored in place of a secret.
func HashToken(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// EqualHash compares two hashes in constant time.
func EqualHash(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// newSecret returns a fresh random secret in the token alphabet.
func newSecret() (string, error) {
	buf := make([]byte, tokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", core.Internal("generating token entropy").Wrap(err)
	}
	return strings.ToLower(tokenEncoding.EncodeToString(buf)), nil
}

// TokenVerifier resolves a presented API token into an actor.
type TokenVerifier struct {
	lookup TokenLookup
	clk    clock.Clock
}

// NewTokenVerifier returns a verifier backed by lookup.
func NewTokenVerifier(lookup TokenLookup, clk clock.Clock) *TokenVerifier {
	return &TokenVerifier{lookup: lookup, clk: clk}
}

// Verify returns the actor a valid token grants, or an unauthenticated error.
func (v *TokenVerifier) Verify(ctx context.Context, value string) (*core.Actor, error) {
	return v.verify(ctx, value, nil)
}

// verify is Verify with a check of its own on the stored token, run before the
// token is recorded as used, so a token the caller refuses leaves no trace of
// having signed anything in.
func (v *TokenVerifier) verify(ctx context.Context, value string, accept func(*core.APIToken) error) (*core.Actor, error) {
	if _, err := ParseAPIToken(value); err != nil {
		NoteRefusal(ctx, Refusal{Reason: RefusedMalformedToken})
		return nil, err
	}
	hash := HashToken(value)
	tok, err := v.lookup.TokenByHash(ctx, hash)
	if withdrawn := (*withdrawnError)(nil); errors.As(err, &withdrawn) {
		NoteRefusal(ctx, Refusal{Reason: RefusedActorDisabled, TokenID: withdrawn.tokenID, TokenName: withdrawn.tokenName})
		return nil, core.Unauthenticated("invalid credentials")
	}
	if err != nil {
		return nil, core.Internal("looking up api token").Wrap(err)
	}
	if tok == nil {
		NoteRefusal(ctx, Refusal{Reason: RefusedUnknownToken, TokenRef: CredentialRef(value)})
		return nil, core.Unauthenticated("invalid credentials")
	}
	now := v.clk.Now()
	if !tok.Active(now) {
		NoteRefusal(ctx, Refusal{Reason: inactiveTokenReason(*tok, now), TokenID: tok.ID, TokenName: tok.Name})
		return nil, core.Unauthenticated("invalid credentials")
	}
	if accept != nil {
		if err := accept(tok); err != nil {
			return nil, err
		}
	}
	if slices.Contains(tok.Scopes, core.ScopeCaldav) && !core.DAVRequestFrom(ctx) {
		NoteRefusal(ctx, Refusal{Reason: RefusedCaldavOnly, TokenID: tok.ID, TokenName: tok.Name})
		return nil, core.Forbidden("a %s token is an app password and works only under %s, over Basic",
			core.ScopeCaldav, core.DAVRoot)
	}
	if slices.Contains(tok.Scopes, core.ScopeSCIMProvision) && !core.SCIMRequestFrom(ctx) {
		NoteRefusal(ctx, Refusal{Reason: RefusedSCIMOnly, TokenID: tok.ID, TokenName: tok.Name})
		return nil, core.Forbidden("a %s token works only on the SCIM endpoints, %s",
			core.ScopeSCIMProvision, core.SCIMPath)
	}
	if toucher, ok := v.lookup.(TokenToucher); ok {
		if err := toucher.TouchToken(ctx, tok.ID, now); err != nil {
			return nil, core.Internal("recording token use").Wrap(err)
		}
	}
	return &core.Actor{
		ID:        tok.ActorID,
		TenantID:  tok.TenantID,
		Kind:      core.ActorAgent,
		Handle:    tok.Name,
		Scopes:    slices.Clone(tok.Scopes),
		TokenID:   tok.ID,
		TokenName: tok.Name,
		ProjectID: tok.ProjectID,
	}, nil
}

// inactiveTokenReason tells a revoked token from an expired one. A token both
// revoked and expired is named revoked, since revoking it was the later act.
func inactiveTokenReason(tok core.APIToken, now time.Time) string {
	if tok.Revoked(now) {
		return RefusedTokenRevoked
	}
	return RefusedTokenExpired
}
