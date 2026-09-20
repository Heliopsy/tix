// Package service implements the tix domain rules.
package service

import (
	"context"
	"sync"

	"github.com/thereisnotime/tix/internal/auth"
	"github.com/thereisnotime/tix/internal/authz"
	"github.com/thereisnotime/tix/internal/clock"
	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/id"
	"github.com/thereisnotime/tix/internal/store"
)

// Local is the authoritative implementation of core.Service.
type Local struct {
	store  store.Store
	policy authz.Policy
	clock  clock.Clock
	ids    id.Generator
	hooks  HookMode
	hasher *auth.Hasher

	// allowInsecureWebhooks permits a plaintext delivery target outside
	// loopback. Deliveries carry task content, so this is opt-in.
	allowInsecureWebhooks bool

	dummyOnce sync.Once
	dummy     string
}

// HookMode selects who delivers webhooks after a mutation commits.
type HookMode string

// Hook modes.
const (
	HookInline HookMode = "inline"
	HookServer HookMode = "server"
	HookOff    HookMode = "off"
)

// Option configures a Local.
type Option func(*Local)

// WithClock sets the time source.
func WithClock(c clock.Clock) Option { return func(l *Local) { l.clock = c } }

// WithIDs sets the identifier generator.
func WithIDs(g id.Generator) Option { return func(l *Local) { l.ids = g } }

// WithHooks sets the webhook delivery mode.
func WithHooks(m HookMode) Option { return func(l *Local) { l.hooks = m } }

// WithHasher sets the password hasher. Tests use cheaper parameters; production
// must not.
func WithHasher(h *auth.Hasher) Option { return func(l *Local) { l.hasher = h } }

// WithInsecureWebhooks allows plaintext delivery targets outside loopback, for
// a deployment terminating TLS at a proxy.
func WithInsecureWebhooks(allow bool) Option {
	return func(l *Local) { l.allowInsecureWebhooks = allow }
}

// New builds a Local over a store.
func New(st store.Store, opts ...Option) *Local {
	l := &Local{
		store:  st,
		policy: authz.New(),
		clock:  clock.New(),
		ids:    id.Default,
		hooks:  HookInline,
		hasher: auth.NewHasher(),
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// Close releases the underlying store.
func (l *Local) Close() error { return l.store.Close() }

// WhoAmI returns the authenticated actor.
func (l *Local) WhoAmI(ctx context.Context) (*core.Actor, error) {
	return core.RequireActor(ctx)
}

// authorize resolves the actor and checks the policy in one step. Every
// mutating and listing method starts here.
func (l *Local) authorize(ctx context.Context, action authz.Action, res authz.Resource) (*core.Actor, error) {
	actor, err := core.RequireActor(ctx)
	if err != nil {
		return nil, err
	}
	if res.TenantID == "" {
		res.TenantID = actor.TenantID
	}
	if err := l.policy.Can(actor, action, res); err != nil {
		return nil, err
	}
	return actor, nil
}

// Local implements the whole product surface. This assertion is what stopped
// the connect layer needing a shim for methods that did not exist yet.
var _ core.Service = (*Local)(nil)
