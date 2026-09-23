// SPDX-License-Identifier: AGPL-3.0-or-later

// Package service implements the tix domain rules.
package service

import (
	"context"
	"sync"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/connections"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/id"
	"github.com/heliopsy/tix/internal/store"
)

// Local is the authoritative implementation of core.Service.
type Local struct {
	store  store.Store
	policy authz.Policy
	clock  clock.Clock
	ids    id.Generator
	hooks  HookMode
	hasher *auth.Hasher

	// conns is the live connections this process is holding. It is memory
	// only, and a connection lives in exactly one process, so a server lists
	// and ends its own and no other's.
	conns *connections.Registry

	// retentionDefaults are the configured windows a tenant that never moved
	// off the shipped default is pruned by.
	retentionDefaults core.RetentionPolicy

	// allowInsecureWebhooks permits a plaintext delivery target outside
	// loopback. Deliveries carry task content, so this is opt-in.
	allowInsecureWebhooks bool

	// allowPrivateWebhookTargets permits a delivery target on loopback, a
	// link-local address or a private range. Separate from the plaintext
	// allowance above because they are different decisions: accepting http is
	// about confidentiality, accepting an internal address is about what the
	// server can be pointed at on the operator's own network.
	allowPrivateWebhookTargets bool

	// noStarterProjects suppresses the starter lists a brand new installation
	// is given, for an operator who wants an empty one.
	noStarterProjects bool

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

// WithConnections sets the live connection registry the service reads and
// ends connections through. Without it the process-wide default is used, which
// is the same registry the surfaces accepting connections feed.
func WithConnections(r *connections.Registry) Option {
	return func(l *Local) {
		if r != nil {
			l.conns = r
		}
	}
}

// WithHooks sets the webhook delivery mode.
func WithHooks(m HookMode) Option { return func(l *Local) { l.hooks = m } }

// WithRetentionDefaults sets the configured retention windows layered under a
// tenant's stored policy.
func WithRetentionDefaults(p core.RetentionPolicy) Option {
	return func(l *Local) { l.retentionDefaults = p }
}

// WithHasher sets the password hasher. Tests use cheaper parameters; production
// must not.
func WithHasher(h *auth.Hasher) Option { return func(l *Local) { l.hasher = h } }

// WithInsecureWebhooks allows plaintext delivery targets outside loopback, for
// a deployment terminating TLS at a proxy.
func WithInsecureWebhooks(allow bool) Option {
	return func(l *Local) { l.allowInsecureWebhooks = allow }
}

// WithPrivateWebhookTargets allows delivery to loopback and private network
// addresses. Off by default, and an operator decision only: a tenant supplying
// a URL that reaches internal infrastructure is the request-forgery case this
// guards.
func WithPrivateWebhookTargets(allow bool) Option {
	return func(l *Local) { l.allowPrivateWebhookTargets = allow }
}

// WithoutStarterProjects leaves a brand new installation with only the default
// list, for an operator installing tix for one purpose.
func WithoutStarterProjects() Option {
	return func(l *Local) { l.noStarterProjects = true }
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
		conns:  connections.Default,
	}
	for _, opt := range opts {
		opt(l)
	}
	// An injected clock has to reach identifier generation too. A ULID carries
	// its own timestamp, so leaving the default generator on the wall clock
	// while the rest of a build runs on a fake one makes an event's identifier
	// disagree with its own OccurredAt. Only the default is replaced, so an
	// explicitly supplied generator still wins.
	if l.ids == id.Default {
		l.ids = id.NewGenerator(l.clock)
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
