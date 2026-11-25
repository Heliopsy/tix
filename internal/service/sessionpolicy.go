// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// WithSessionPolicy sets the server's session settings. A zero lifetime keeps
// the built-in 12h; a zero maximum lifetime turns renewal off.
func WithSessionPolicy(p core.SessionPolicy) Option {
	return func(l *Local) {
		if p.TTL <= 0 {
			p.TTL = core.DefaultSessionTTL
		}
		l.sessionDefaults = core.SessionPolicy{TTL: p.TTL, IdleTimeout: max(p.IdleTimeout, 0),
			RememberTTL: max(p.RememberTTL, 0), MaxLifetime: max(p.MaxLifetime, 0),
			RememberMaxLifetime: max(p.RememberMaxLifetime, 0)}
	}
}

// defaultSessionPolicy is what a service built without WithSessionPolicy uses.
func defaultSessionPolicy() core.SessionPolicy {
	return core.SessionPolicy{TTL: core.DefaultSessionTTL, RememberTTL: core.DefaultSessionRememberTTL,
		MaxLifetime: core.DefaultSessionMaxLifetime, RememberMaxLifetime: core.DefaultSessionRememberMaxLifetime}
}

// SessionPolicy reports the session settings a new sign-in on the current
// tenant is held to. It is public: the sign-in screen asks before anybody has
// signed in.
func (l *Local) SessionPolicy(ctx context.Context) (*core.SessionPolicy, error) {
	scope, err := core.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	out, err := l.sessionPolicyFor(ctx, scope.TenantID)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// sessionPolicyFor is the server's session settings narrowed by a tenant's.
func (l *Local) sessionPolicyFor(ctx context.Context, tenantID string) (core.SessionPolicy, error) {
	var out core.SessionPolicy
	err := l.store.View(ctx, core.TenantScope{TenantID: tenantID}, func(tx store.Tx) error {
		t, err := tx.GetTenant(ctx)
		if err != nil {
			return err
		}
		out = core.EffectiveSessionPolicy(l.sessionDefaults, *t)
		return nil
	})
	return out, err
}

// tightenSessionSettings applies a tenant's session settings, refusing any
// that would loosen the server's.
func (l *Local) tightenSessionSettings(t *core.Tenant, in core.UpdateTenantInput) error {
	server := l.sessionDefaults
	if in.SessionTTL != nil {
		if d := *in.SessionTTL; d > server.TTL {
			return core.Invalid("session lifetime %s is longer than this server's %s; a tenant can only shorten it",
				d.Human(), server.TTL.Human()).WithDetail(core.DetailField, "session_ttl")
		}
		t.SessionTTL = *in.SessionTTL
	}
	if in.SessionIdleTimeout != nil {
		if d := *in.SessionIdleTimeout; server.IdleTimeout > 0 && d > server.IdleTimeout {
			return core.Invalid("session idle timeout %s is longer than this server's %s; a tenant can only shorten it",
				d.Human(), server.IdleTimeout.Human()).WithDetail(core.DetailField, "session_idle_timeout")
		}
		t.SessionIdleTimeout = *in.SessionIdleTimeout
	}
	if in.SessionRememberTTL != nil {
		d, off, err := core.ParseSessionRemember(*in.SessionRememberTTL)
		if err != nil {
			return err
		}
		if d > 0 && server.RememberTTL == 0 {
			return core.Invalid("keep me signed in is off on this server; a tenant cannot turn it on").
				WithDetail(core.DetailField, "session_remember_ttl")
		}
		if d > server.RememberTTL {
			return core.Invalid("keep me signed in for %s is longer than this server's %s; a tenant can only shorten it",
				d.Human(), server.RememberTTL.Human()).WithDetail(core.DetailField, "session_remember_ttl")
		}
		t.SessionRememberTTL, t.SessionRememberOff = d, off
	}
	return nil
}

// rememberCtxKey carries a sign-in's "keep me signed in" choice to the point
// the session is minted.
type rememberCtxKey struct{}

// withRemember records whether the sign-in asked to be kept signed in.
func withRemember(ctx context.Context, remember bool) context.Context {
	return context.WithValue(ctx, rememberCtxKey{}, remember)
}

// rememberFrom reports whether the sign-in asked to be kept signed in.
func rememberFrom(ctx context.Context) bool {
	v, _ := ctx.Value(rememberCtxKey{}).(bool)
	return v
}

// signedInCtxKey carries the sign-in time a session continuing an earlier one
// keeps, so its maximum lifetime runs from the original sign-in.
type signedInCtxKey struct{}

// withSignedIn records when the sign-in a new session continues happened.
func withSignedIn(ctx context.Context, at time.Time) context.Context {
	return context.WithValue(ctx, signedInCtxKey{}, at)
}

// signedInFrom returns the sign-in time a new session continues, or zero.
func signedInFrom(ctx context.Context) time.Time {
	v, _ := ctx.Value(signedInCtxKey{}).(time.Time)
	return v
}
