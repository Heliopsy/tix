package server

import (
	"context"
	"time"

	"github.com/thereisnotime/tix/internal/auth"
	"github.com/thereisnotime/tix/internal/clock"
	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/store"
)

// StoreCredentials resolves API tokens and sessions inside the tenant the
// request has already been pinned to.
type StoreCredentials struct {
	store store.Store
}

// NewStoreCredentials returns a credential lookup over the store.
func NewStoreCredentials(st store.Store) *StoreCredentials {
	return &StoreCredentials{store: st}
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
		out = token
		return err
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
		session := auth.StoredSession{
			TokenHash: hash,
			ActorID:   actorID,
			TenantID:  scope.TenantID,
			ExpiresAt: expires,
		}
		if actor, err := tx.GetActor(ctx, actorID); err == nil {
			session.Handle = actor.Handle
			session.Role = actor.Role
		}
		out = &session
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// NewAuthenticator returns the bearer and cookie chain the API authenticates
// with, in that order.
func NewAuthenticator(st store.Store, clk clock.Clock) auth.Authenticator {
	creds := NewStoreCredentials(st)
	return auth.NewChain(
		auth.NewBearerAuthenticator(auth.NewTokenVerifier(creds, clk)),
		auth.NewCookieAuthenticator(auth.NewSessionVerifier(creds, clk)),
	)
}
