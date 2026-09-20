package service

import (
	"context"
	"strings"
	"time"

	"github.com/thereisnotime/tix/internal/auth"
	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/store"
)

// Event types for the session lifecycle.
const (
	eventSessionCreated core.EventType = "session.created"
	eventSessionEnded   core.EventType = "session.ended"
)

// Audit actions recorded for sessions.
const (
	auditUserLogin  = "user.login"
	auditUserLogout = "user.logout"
)

// SessionTTL is how long a password login stays valid.
const SessionTTL = 12 * time.Hour

// errInvalidCredentials is the single answer every failed login gives, so that
// a wrong password, an unknown email and a disabled account are one outcome.
func errInvalidCredentials() error { return core.Unauthenticated("invalid credentials") }

// dummyHash is verified against when no account matches, so that a login for an
// address nobody holds costs the same as a login with the wrong password. It is
// per service because the hasher's cost is configurable.
func (l *Local) dummyHash() string {
	l.dummyOnce.Do(func() {
		h, err := l.hasher.Hash(strings.Repeat("tix-absent-", 4))
		if err == nil {
			l.dummy = h
		}
	})
	return l.dummy
}

// sessionCtxKey carries the session token a caller presented.
type sessionCtxKey struct{}

// WithSessionToken returns a context carrying the session token of the caller.
func WithSessionToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, sessionCtxKey{}, token)
}

// SessionTokenFrom returns the session token carried by ctx, if any.
func SessionTokenFrom(ctx context.Context) (string, bool) {
	t, ok := ctx.Value(sessionCtxKey{}).(string)
	return t, ok && t != ""
}

// candidate is what a login attempt resolved, all of it optional so that a
// failure carries no information about which part was missing.
type candidate struct {
	user   *core.User
	hash   string
	actor  *core.Actor
	role   core.Role
	tenant string
}

// findCandidate resolves the account an email names within one tenant. A user
// without an actor in this tenant is no candidate, which is what keeps login
// tenant-scoped even though user rows are global.
func (l *Local) findCandidate(ctx context.Context, scope core.TenantScope, email string) (candidate, error) {
	out := candidate{tenant: scope.TenantID}
	if email == "" {
		return out, nil
	}
	var (
		user *core.User
		hash string
	)
	if err := l.store.Unscoped(ctx, func(u store.UnscopedTx) error {
		found, h, err := u.GetUserByEmail(ctx, email)
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		user, hash = found, h
		return nil
	}); err != nil {
		return out, err
	}
	if user == nil {
		return out, nil
	}

	system := core.SystemActor(scope.TenantID)
	if err := l.read(ctx, system, func(tx store.Tx) error {
		a, err := tx.GetActor(ctx, user.ID)
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		role, err := userRole(ctx, tx, a.ID)
		if err != nil {
			return err
		}
		out.actor, out.role = a, role
		return nil
	}); err != nil {
		return out, err
	}
	if out.actor == nil {
		return out, nil
	}
	out.user, out.hash = user, hash
	return out, nil
}

// usable reports whether the candidate may be logged in with password.
func (c candidate) usable() bool {
	return c.user != nil && c.actor != nil && c.user.DisabledAt == nil && c.hash != ""
}

// Login exchanges an email and password for a session token, returned once.
func (l *Local) Login(ctx context.Context, email, password string) (*core.Session, error) {
	scope, err := core.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	email = strings.ToLower(strings.TrimSpace(email))

	cand, err := l.findCandidate(ctx, scope, email)
	if err != nil {
		return nil, err
	}
	stored := l.dummyHash()
	if cand.hash != "" {
		stored = cand.hash
	}
	verified := auth.Verify(stored, password) == nil
	if !verified || !cand.usable() {
		return nil, errInvalidCredentials()
	}

	hasher := l.hasher
	rehashed := ""
	if auth.NeedsRehash(cand.hash, hasher.Params()) {
		if rehashed, err = hasher.Hash(password); err != nil {
			return nil, err
		}
	}

	sessionActor := &core.Actor{
		ID:          cand.actor.ID,
		TenantID:    scope.TenantID,
		Kind:        core.ActorUser,
		Handle:      cand.actor.Handle,
		DisplayName: cand.actor.DisplayName,
		Role:        cand.role,
	}
	minted, err := auth.MintSession(l.clock, auth.SessionInput{
		ActorID:  sessionActor.ID,
		TenantID: scope.TenantID,
		Handle:   sessionActor.Handle,
		Role:     cand.role,
		TTL:      SessionTTL,
	})
	if err != nil {
		return nil, err
	}

	if err := l.write(ctx, sessionActor, func(m *mutation) error {
		if err := m.tx.CreateSession(ctx, sessionActor.ID, minted.Stored.TokenHash, minted.Session.ExpiresAt); err != nil {
			return err
		}
		if rehashed != "" {
			if err := m.tx.UpdateUser(ctx, cand.user, rehashed); err != nil {
				return err
			}
		}
		return m.Record(auditUserLogin, eventSessionCreated, "session", sessionActor.ID, "", nil,
			map[string]any{"actor_id": sessionActor.ID, "expires_at": minted.Session.ExpiresAt},
			map[string]any{"actor_id": sessionActor.ID, "handle": sessionActor.Handle})
	}); err != nil {
		return nil, err
	}
	return &minted.Session, nil
}

// Logout ends the session the caller presented. It needs no authorization
// beyond being authenticated, since an actor may always end its own session.
func (l *Local) Logout(ctx context.Context) error {
	actor, err := core.RequireActor(ctx)
	if err != nil {
		return err
	}
	token, ok := SessionTokenFrom(ctx)
	if !ok {
		return core.Unauthenticated("no session to end")
	}
	hash := auth.HashToken(token)

	return l.write(ctx, actor, func(m *mutation) error {
		if err := m.tx.DeleteSession(ctx, hash); err != nil {
			if core.IsKind(err, core.KindNotFound) {
				return core.Unauthenticated("no session to end")
			}
			return err
		}
		return m.Record(auditUserLogout, eventSessionEnded, "session", actor.ID, "",
			map[string]any{"actor_id": actor.ID}, nil, map[string]any{"actor_id": actor.ID})
	})
}
