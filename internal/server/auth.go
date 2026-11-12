// SPDX-License-Identifier: AGPL-3.0-or-later

package server

import (
	"context"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/store"
)

// StoreCredentials resolves API tokens and sessions inside the tenant the
// request has already been pinned to.
type StoreCredentials struct {
	store store.Store
	// sessions are the server's session settings, which the tenant narrows
	// into the idle timeout and renewal a session is held to.
	sessions core.SessionPolicy
}

// NewStoreCredentials returns a credential lookup over the store.
func NewStoreCredentials(st store.Store, sessions core.SessionPolicy) *StoreCredentials {
	return &StoreCredentials{store: st, sessions: sessions}
}

// TokenByHash returns the token behind a hash, or nil when there is none.
func (c *StoreCredentials) TokenByHash(ctx context.Context, hash string) (*core.APIToken, error) {
	scope, ok := core.TenantFrom(ctx)
	if !ok {
		return nil, nil
	}
	var out *core.APIToken
	err := c.store.View(ctx, scope, func(tx store.Tx) error {
		token, err := tx.GetTokenByHash(ctx, hash)
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		withdrawn, err := service.StandingWithdrawn(ctx, tx, token.ActorID)
		if err != nil || withdrawn {
			return err
		}
		out = token
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// TouchToken records that a token authenticated a request.
func (c *StoreCredentials) TouchToken(ctx context.Context, id string, at time.Time) error {
	scope, ok := core.TenantFrom(ctx)
	if !ok {
		return nil
	}
	return c.store.Update(ctx, scope, func(tx store.Tx) error {
		return tx.TouchToken(ctx, id, at)
	})
}

// SessionByHash returns the session behind a hash, or nil when there is none.
func (c *StoreCredentials) SessionByHash(ctx context.Context, hash string) (*auth.StoredSession, error) {
	scope, ok := core.TenantFrom(ctx)
	if !ok {
		return nil, nil
	}
	var out *auth.StoredSession
	err := c.store.View(ctx, scope, func(tx store.Tx) error {
		actorID, expires, err := tx.GetSessionByHash(ctx, hash)
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if withdrawn, err := service.StandingWithdrawn(ctx, tx, actorID); err != nil || withdrawn {
			return err
		}
		session := auth.StoredSession{
			TokenHash: hash,
			ActorID:   actorID,
			TenantID:  scope.TenantID,
			ExpiresAt: expires,
		}
		if session.LastSeenAt, err = tx.GetSessionLastSeen(ctx, hash); err != nil {
			return err
		}
		origin, err := tx.GetSessionOrigin(ctx, hash)
		if err != nil {
			return err
		}
		session.CreatedAt, session.Remembered = origin.SignedInAt, origin.Remembered
		tenant, err := tx.GetTenant(ctx)
		if err != nil {
			return err
		}
		policy := core.EffectiveSessionPolicy(c.sessions, *tenant)
		session.IdleTimeout = policy.IdleTimeout.D()
		window, limit := policy.Renewal(origin.Remembered)
		session.Window, session.Limit = window.D(), limit.D()
		if actor, err := tx.GetActor(ctx, actorID); err == nil {
			session.Handle = actor.Handle
		}
		// The role is per tenant and lives on the membership, not on the actor,
		// so a session that read it off the actor would carry no role at all
		// and be denied every action.
		if member, err := tx.GetMember(ctx, actorID); err == nil && member != nil {
			session.Role = member.Role
		}
		out = &session
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// TouchSession records that a session authenticated a request and, when
// expires is not zero, the expiry that use renewed it to.
func (c *StoreCredentials) TouchSession(ctx context.Context, hash string, at, expires time.Time) error {
	scope, ok := core.TenantFrom(ctx)
	if !ok {
		return nil
	}
	return c.store.Update(ctx, scope, func(tx store.Tx) error {
		return tx.TouchSession(ctx, hash, at, expires)
	})
}

// NewAuthenticator returns the bearer and cookie chain the API authenticates
// with, in that order. sessions are the server's session settings.
func NewAuthenticator(st store.Store, clk clock.Clock, sessions core.SessionPolicy) auth.Authenticator {
	creds := NewStoreCredentials(st, sessions)
	return auth.NewChain(
		auth.NewBearerAuthenticator(auth.NewTokenVerifier(creds, clk)),
		auth.NewCookieAuthenticator(auth.NewSessionVerifier(creds, clk)),
	)
}
