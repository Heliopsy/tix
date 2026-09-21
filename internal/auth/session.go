package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/id"
)

// SessionCookieName is the cookie a browser session token travels in.
const SessionCookieName = "tix_session"

// StoredSession is the persisted half of a session: a hash and its metadata.
type StoredSession struct {
	ID        string
	TokenHash string
	ActorID   string
	TenantID  string
	Handle    string
	Role      core.Role
	CreatedAt time.Time
	ExpiresAt time.Time
	RevokedAt *time.Time
}

// Active reports whether the session may still authenticate at time now.
func (s StoredSession) Active(now time.Time) bool {
	return s.RevokedAt == nil && s.ExpiresAt.After(now)
}

// String renders the session without its hash or token value.
func (s StoredSession) String() string {
	return fmt.Sprintf("session %s actor=%s tenant=%s expires=%s",
		s.ID, s.ActorID, s.TenantID, s.ExpiresAt.Format(time.RFC3339))
}

// SessionInput describes the session to mint after a successful login.
type SessionInput struct {
	ActorID  string
	TenantID string
	Handle   string
	Role     core.Role
	TTL      time.Duration
}

// MintedSession carries the session returned to the caller and the row to store.
type MintedSession struct {
	Session core.Session
	Stored  StoredSession
}

// SessionLookup resolves the stored session for a token hash.
type SessionLookup interface {
	SessionByHash(ctx context.Context, hash string) (*StoredSession, error)
}

// MintSession issues an opaque session token and the hashed record to persist.
func MintSession(clk clock.Clock, in SessionInput) (*MintedSession, error) {
	switch {
	case strings.TrimSpace(in.ActorID) == "":
		return nil, core.Invalid("a session requires an actor")
	case strings.TrimSpace(in.TenantID) == "":
		return nil, core.Invalid("a session requires a tenant")
	case in.TTL <= 0:
		return nil, core.Invalid("a session requires a positive lifetime")
	}
	secret, err := newSecret()
	if err != nil {
		return nil, err
	}
	now := clk.Now()
	expires := now.Add(in.TTL)
	return &MintedSession{
		Session: core.Session{
			Token:     secret,
			ActorID:   in.ActorID,
			TenantID:  in.TenantID,
			ExpiresAt: expires,
		},
		Stored: StoredSession{
			ID:        id.NewAt(now),
			TokenHash: HashToken(secret),
			ActorID:   in.ActorID,
			TenantID:  in.TenantID,
			Handle:    in.Handle,
			Role:      in.Role,
			CreatedAt: now,
			ExpiresAt: expires,
		},
	}, nil
}

// SessionVerifier resolves a presented session token into an actor.
type SessionVerifier struct {
	lookup SessionLookup
	clk    clock.Clock
}

// NewSessionVerifier returns a verifier backed by lookup.
func NewSessionVerifier(lookup SessionLookup, clk clock.Clock) *SessionVerifier {
	return &SessionVerifier{lookup: lookup, clk: clk}
}

// Verify returns the actor a live session grants, or an unauthenticated error.
func (v *SessionVerifier) Verify(ctx context.Context, token string) (*core.Actor, error) {
	if token == "" {
		return nil, core.Unauthenticated("invalid credentials")
	}
	stored, err := v.lookup.SessionByHash(ctx, HashToken(token))
	if err != nil {
		return nil, core.Internal("looking up session").Wrap(err)
	}
	if stored == nil {
		return nil, core.Unauthenticated("invalid credentials")
	}
	if !EqualHash(stored.TokenHash, HashToken(token)) {
		return nil, core.Unauthenticated("invalid credentials")
	}
	if !stored.Active(v.clk.Now()) {
		return nil, core.Unauthenticated("invalid credentials")
	}
	return &core.Actor{
		ID:       stored.ActorID,
		TenantID: stored.TenantID,
		Kind:     core.ActorUser,
		Handle:   stored.Handle,
		Role:     stored.Role,
	}, nil
}

// NewSessionCookie builds the session cookie, marking it Secure under TLS.
//
// HttpOnly and SameSite are always set. Secure follows the caller, because the
// server binds loopback over plain HTTP by default and a Secure cookie would
// never be sent there; a non-loopback bind requires TLS or an explicit opt-out.
func NewSessionCookie(token string, expires time.Time, secure bool) *http.Cookie {
	return &http.Cookie{ // #nosec G124 -- HttpOnly and SameSite are always set; Secure tracks TLS, see above
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// ClearSessionCookie builds the cookie that removes a session from the browser.
func ClearSessionCookie(secure bool) *http.Cookie {
	// #nosec G124 -- delegates to NewSessionCookie, which sets HttpOnly and SameSite.
	c := NewSessionCookie("", time.Unix(0, 0).UTC(), secure)
	c.MaxAge = -1
	return c
}
