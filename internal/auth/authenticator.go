package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/thereisnotime/tix/internal/core"
)

// ErrNoCredential reports that a request carried nothing this authenticator handles.
var ErrNoCredential = core.Unauthenticated("no credential presented")

// Authenticator resolves the actor a request speaks for.
type Authenticator interface {
	Authenticate(ctx context.Context, r *http.Request) (*core.Actor, error)
}

// Chain tries each authenticator in order until one resolves or one fails hard.
type Chain struct {
	members []Authenticator
}

// NewChain returns a chain over the given authenticators, in order.
func NewChain(members ...Authenticator) *Chain {
	return &Chain{members: members}
}

// Authenticate returns the first resolved actor, or ErrNoCredential if none apply.
func (c *Chain) Authenticate(ctx context.Context, r *http.Request) (*core.Actor, error) {
	for _, m := range c.members {
		actor, err := m.Authenticate(ctx, r)
		if errors.Is(err, ErrNoCredential) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if actor != nil {
			return actor, nil
		}
	}
	return nil, ErrNoCredential
}

// BearerAuthenticator authenticates an API token from the Authorization header.
type BearerAuthenticator struct {
	verifier *TokenVerifier
}

// NewBearerAuthenticator returns an authenticator over the token verifier.
func NewBearerAuthenticator(v *TokenVerifier) *BearerAuthenticator {
	return &BearerAuthenticator{verifier: v}
}

// Authenticate resolves a bearer token, abstaining when the header is absent.
func (a *BearerAuthenticator) Authenticate(ctx context.Context, r *http.Request) (*core.Actor, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return nil, ErrNoCredential
	}
	scheme, value, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "bearer") {
		return nil, ErrNoCredential
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, ErrNoCredential
	}
	return a.verifier.Verify(ctx, value)
}

// CookieAuthenticator authenticates a browser session from its cookie.
type CookieAuthenticator struct {
	verifier *SessionVerifier
	name     string
}

// NewCookieAuthenticator returns an authenticator over the session verifier.
func NewCookieAuthenticator(v *SessionVerifier) *CookieAuthenticator {
	return &CookieAuthenticator{verifier: v, name: SessionCookieName}
}

// Authenticate resolves a session cookie, abstaining when it is absent.
func (a *CookieAuthenticator) Authenticate(ctx context.Context, r *http.Request) (*core.Actor, error) {
	cookie, err := r.Cookie(a.name)
	if err != nil || cookie.Value == "" {
		return nil, ErrNoCredential
	}
	return a.verifier.Verify(ctx, cookie.Value)
}

// StaticAuthenticator returns a fixed actor, as no-auth mode does.
type StaticAuthenticator struct {
	actor *core.Actor
}

// NewStaticAuthenticator returns an authenticator that always resolves to actor.
func NewStaticAuthenticator(actor *core.Actor) *StaticAuthenticator {
	return &StaticAuthenticator{actor: actor}
}

// Authenticate returns the configured actor, abstaining when there is none.
func (a *StaticAuthenticator) Authenticate(context.Context, *http.Request) (*core.Actor, error) {
	if a.actor == nil {
		return nil, ErrNoCredential
	}
	return a.actor, nil
}

// Compile-time assertions that every authenticator satisfies the interface.
var (
	_ Authenticator = (*Chain)(nil)
	_ Authenticator = (*BearerAuthenticator)(nil)
	_ Authenticator = (*CookieAuthenticator)(nil)
	_ Authenticator = (*StaticAuthenticator)(nil)
)
