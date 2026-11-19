// SPDX-License-Identifier: AGPL-3.0-or-later

// Package samlrefresh reads the IdP metadata of every tenant's SAML
// providers again on a schedule, so a signing certificate an identity
// provider rotates reaches tix before the old one stops working.
package samlrefresh

import (
	"context"
	"errors"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
)

// CheckEvery bounds how long a due provider waits to be noticed. Due-ness
// is read from each provider's last refresh, so a restart or a second
// replica does not read a provider early.
const CheckEvery = 15 * time.Minute

// Service is what the refresher needs of the service: the providers of the
// tenant the context names, and a refresh of one of them.
type Service interface {
	ListSSOProviders(ctx context.Context) ([]core.SSOProvider, error)
	RefreshSAMLMetadata(ctx context.Context, ref string) (*core.SSOProvider, error)
}

// Result counts what one pass did.
type Result struct {
	// Refreshed is the providers read, Changed the ones whose certificates
	// or sign-in address the read changed, Failed the reads that failed.
	Refreshed, Changed, Failed int
}

// Refresher refreshes due SAML providers on a ticker until its context is
// cancelled.
type Refresher struct {
	svc      Service
	tenants  func(ctx context.Context) ([]string, error)
	as       func(ctx context.Context, tenantID string) context.Context
	clock    clock.Clock
	interval time.Duration
	onError  func(tenantID, key string, err error)
	onChange func(tenantID string, p *core.SSOProvider)
}

// Option configures a Refresher.
type Option func(*Refresher)

// WithErrorHandler reports a failed read, or a tenant that could not be
// listed (key empty). A failure never stops the loop.
func WithErrorHandler(fn func(tenantID, key string, err error)) Option {
	return func(r *Refresher) { r.onError = fn }
}

// WithChangeHandler reports a read that changed a provider.
func WithChangeHandler(fn func(tenantID string, p *core.SSOProvider)) Option {
	return func(r *Refresher) { r.onChange = fn }
}

// New builds a refresher reading every interval. tenants lists the tenants
// to visit and as gives a context acting in one of them. A non-positive
// interval disables it.
func New(svc Service, tenants func(context.Context) ([]string, error),
	as func(context.Context, string) context.Context, clk clock.Clock, interval time.Duration, opts ...Option,
) *Refresher {
	r := &Refresher{svc: svc, tenants: tenants, as: as, clock: clk, interval: interval}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Enabled reports whether the refresher reads anything when run.
func (r *Refresher) Enabled() bool { return r.interval > 0 }

// Run looks for due providers once at start and then on every tick, until
// ctx is cancelled.
func (r *Refresher) Run(ctx context.Context) error {
	if !r.Enabled() {
		<-ctx.Done()
		return ctx.Err()
	}
	ticker := r.clock.NewTicker(min(r.interval, CheckEvery))
	defer ticker.Stop()
	r.RunOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C():
			r.RunOnce(ctx)
		}
	}
}

// RunOnce refreshes every provider that is due now, tenant by tenant.
func (r *Refresher) RunOnce(ctx context.Context) Result {
	var res Result
	tenants, err := r.tenants(ctx)
	if err != nil {
		r.report("", "", err)
		return res
	}
	for _, tenantID := range tenants {
		if ctx.Err() != nil {
			return res
		}
		r.tenant(ctx, tenantID, &res)
	}
	return res
}

func (r *Refresher) tenant(ctx context.Context, tenantID string, res *Result) {
	tctx := r.as(ctx, tenantID)
	providers, err := r.svc.ListSSOProviders(tctx)
	if err != nil {
		r.report(tenantID, "", err)
		return
	}
	now := r.clock.Now()
	for _, p := range providers {
		if !p.MetadataRefreshDue(now, r.interval) {
			continue
		}
		res.Refreshed++
		after, err := r.svc.RefreshSAMLMetadata(tctx, p.ID)
		if err != nil {
			res.Failed++
			r.report(tenantID, p.Key, err)
			continue
		}
		if after.MetadataRefresh != nil && after.MetadataRefresh.Result == core.SAMLRefreshUpdated {
			res.Changed++
			if r.onChange != nil {
				r.onChange(tenantID, after)
			}
		}
	}
}

func (r *Refresher) report(tenantID, key string, err error) {
	if r.onError != nil && !errors.Is(err, context.Canceled) {
		r.onError(tenantID, key, err)
	}
}
